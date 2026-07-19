package echonet

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"
)

var tidCounter atomic.Uint32

func nextTID() uint16 {
	return uint16(tidCounter.Add(1))
}

// Send sends a frame to the given IP address (unicast) and waits for a matching response.
func Send(ip string, frame *Frame, timeout time.Duration) (*Frame, error) {
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", UDPPort))
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial udp %s: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	if _, err := conn.Write(frame.Encode()); err != nil {
		return nil, fmt.Errorf("write udp: %w", err)
	}

	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read udp: %w", err)
	}

	resp, err := Decode(buf[:n])
	if err != nil {
		return nil, err
	}
	if resp.TID != frame.TID {
		return nil, fmt.Errorf("TID mismatch: got %d, want %d", resp.TID, frame.TID)
	}
	return resp, nil
}

// DiscoverResult holds the result of a device discovery.
type DiscoverResult struct {
	IP   string
	EOJs []uint32
}

// Discover sends an instance list Get to the ECHONET Lite multicast address
// and collects responses until the timeout expires.
func Discover(timeoutSec int) ([]DiscoverResult, error) {
	localAddr := &net.UDPAddr{Port: 0}
	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	// Enable multicast loopback so that emulators running on the same host respond.
	p := ipv4.NewPacketConn(conn)
	_ = p.SetMulticastLoopback(true)

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	tid := nextTID()
	frame := NewGetRequest(tid, NodeProfileEOJ, 0xD6)
	dst := &net.UDPAddr{
		IP:   net.ParseIP(MulticastAddr),
		Port: UDPPort,
	}
	if _, err := conn.WriteTo(frame.Encode(), dst); err != nil {
		return nil, fmt.Errorf("multicast write: %w", err)
	}

	seen := map[string]bool{}
	var results []DiscoverResult
	buf := make([]byte, 1500)

	for time.Now().Before(deadline) {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		ip := src.IP.String()
		if seen[ip] {
			continue
		}
		seen[ip] = true

		resp, err := Decode(buf[:n])
		if err != nil {
			continue
		}
		if resp.ESV != ESVGetRes && resp.ESV != ESVInf {
			continue
		}

		var eojs []uint32
		for _, p := range resp.Props {
			if p.EPC != 0xD5 && p.EPC != 0xD6 {
				continue
			}
			eojList := parseInstanceList(p.EDT)
			eojs = append(eojs, eojList...)
		}
		results = append(results, DiscoverResult{IP: ip, EOJs: eojs})
	}
	return results, nil
}

// GetProperty retrieves a single EPC value from a device via unicast UDP.
func GetProperty(ip string, eoj uint32, epc byte, timeout time.Duration) ([]byte, error) {
	tid := nextTID()
	req := NewGetRequest(tid, eoj, epc)
	resp, err := Send(ip, req, timeout)
	if err != nil {
		return nil, err
	}
	if resp.ESV == ESVGetSNA {
		return nil, fmt.Errorf("device returned Get_SNA (EPC %02X not available)", epc)
	}
	if resp.ESV != ESVGetRes {
		return nil, fmt.Errorf("unexpected ESV: %02X", resp.ESV)
	}
	for _, p := range resp.Props {
		if p.EPC == epc {
			return p.EDT, nil
		}
	}
	return nil, fmt.Errorf("EPC %02X not found in response", epc)
}

// parseInstanceList decodes the EDT of EPC 0xD5/0xD6 to a slice of EOJ values.
// Format: [count(1)] [EOJ(3)] × count
func parseInstanceList(edt []byte) []uint32 {
	if len(edt) < 1 {
		return nil
	}
	count := int(edt[0])
	eojs := make([]uint32, 0, count)
	for i := 0; i < count && 1+i*3+2 < len(edt); i++ {
		base := 1 + i*3
		eoj := uint32(edt[base])<<16 | uint32(edt[base+1])<<8 | uint32(edt[base+2])
		eojs = append(eojs, eoj)
	}
	return eojs
}
