package echonet

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"
)

var tidCounter atomic.Uint32

func nextTID() uint16 {
	return uint16(tidCounter.Add(1))
}

// udpMu serializes access to UDP port UDPPort. ECHONET Lite peers conventionally
// send responses back to the fixed port UDPPort regardless of the request's
// source port, so both unicast Send and multicast Discover must themselves bind
// to UDPPort to receive them. Only one such binding can exist on the host at a
// time, so callers share this lock instead of using an ephemeral source port.
var udpMu sync.Mutex

// Send sends a frame to the given IP address (unicast) and waits for a matching response.
func Send(ip string, frame *Frame, timeout time.Duration) (*Frame, error) {
	udpMu.Lock()
	defer udpMu.Unlock()

	raddr := &net.UDPAddr{IP: net.ParseIP(ip), Port: UDPPort}
	if raddr.IP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: UDPPort})
	if err != nil {
		return nil, fmt.Errorf("listen udp :%d: %w", UDPPort, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	if _, err := conn.WriteToUDP(frame.Encode(), raddr); err != nil {
		return nil, fmt.Errorf("write udp: %w", err)
	}

	buf := make([]byte, 1500)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			return nil, fmt.Errorf("read udp: %w", err)
		}
		if !src.IP.Equal(raddr.IP) {
			continue
		}
		resp, err := Decode(buf[:n])
		if err != nil {
			continue
		}
		if resp.TID != frame.TID {
			continue
		}
		return resp, nil
	}
}

// DiscoverResult holds the result of a device discovery.
type DiscoverResult struct {
	IP   string
	EOJs []uint32
}

// Discover sends an instance list Get to the ECHONET Lite multicast address
// and collects responses until the timeout expires.
func Discover(timeoutSec int) ([]DiscoverResult, error) {
	udpMu.Lock()
	defer udpMu.Unlock()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: UDPPort})
	if err != nil {
		return nil, fmt.Errorf("listen udp :%d: %w", UDPPort, err)
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

// SetProperty writes a single EPC value to a device via unicast UDP SetC,
// waiting for the device's Set_Res (success) or SetC_SNA (failure) response.
func SetProperty(ip string, eoj uint32, epc byte, edt []byte, timeout time.Duration) error {
	tid := nextTID()
	req := NewSetCRequest(tid, eoj, epc, edt)
	resp, err := Send(ip, req, timeout)
	if err != nil {
		return err
	}
	if resp.ESV == ESVSetCSNA {
		return fmt.Errorf("device returned SetC_SNA (EPC %02X write rejected)", epc)
	}
	if resp.ESV != ESVSetRes {
		return fmt.Errorf("unexpected ESV: %02X", resp.ESV)
	}
	return nil
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
