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

	data, err := frame.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode frame: %w", err)
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

	if _, err := conn.WriteToUDP(data, raddr); err != nil {
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

	// Explicitly select the outgoing interface for the multicast send. Without
	// this, the socket (bound to 0.0.0.0) relies on the OS's default route to
	// pick an interface, which can point at a non-multicast-capable interface
	// (e.g. a VPN tunnel) and fail with "sendto: no route to host".
	iface, err := defaultMulticastInterface()
	if err != nil {
		return nil, err
	}
	if err := p.SetMulticastInterface(iface); err != nil {
		return nil, fmt.Errorf("set multicast interface %s: %w", iface.Name, err)
	}

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
	data, err := frame.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode frame: %w", err)
	}
	if _, err := conn.WriteTo(data, dst); err != nil {
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
		// Only record the IP once a valid response to this request arrives, so
		// that stray packets (e.g. a late response to an earlier get_property,
		// a Get_SNA or an unsolicited INF) do not hide the real response.
		eojs, ok := parseDiscoverResponse(buf[:n], tid)
		if !ok {
			continue
		}
		seen[ip] = true
		results = append(results, DiscoverResult{IP: ip, EOJs: eojs})
	}
	return results, nil
}

// parseDiscoverResponse checks whether data is a Get_Res to the discovery
// request identified by tid and, if so, returns the EOJs listed in its
// self-node instance list S (EPC 0xD6). It reports false for frames that fail
// to decode, carry a different TID, are not Get_Res, or lack EPC 0xD6.
func parseDiscoverResponse(data []byte, tid uint16) ([]uint32, bool) {
	resp, err := Decode(data)
	if err != nil {
		return nil, false
	}
	if resp.TID != tid || resp.ESV != ESVGetRes {
		return nil, false
	}
	for _, p := range resp.Props {
		if p.EPC == 0xD6 {
			return parseInstanceList(p.EDT), true
		}
	}
	return nil, false
}

// defaultMulticastInterface returns the first up, non-loopback,
// non-point-to-point network interface with an IPv4 address and multicast
// support, for use as the outgoing interface of a multicast send.
func defaultMulticastInterface() (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		if !isMulticastCandidate(iface.Flags) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil {
				return &iface, nil
			}
		}
	}
	return nil, fmt.Errorf("no active IPv4 multicast-capable network interface found")
}

// isMulticastCandidate reports whether an interface with the given flags can
// be used to reach ECHONET Lite devices on the LAN by multicast. Point-to-point
// interfaces are excluded: VPN tunnels such as macOS utun also carry the
// MULTICAST flag but do not lead to the local network.
func isMulticastCandidate(flags net.Flags) bool {
	return flags&net.FlagUp != 0 &&
		flags&net.FlagLoopback == 0 &&
		flags&net.FlagPointToPoint == 0 &&
		flags&net.FlagMulticast != 0
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
