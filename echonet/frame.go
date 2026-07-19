package echonet

import (
	"encoding/binary"
	"fmt"
)

const (
	EHD1 = 0x10
	EHD2 = 0x81

	ESVGet    = 0x62
	ESVGetRes = 0x72
	ESVGetSNA = 0x52
	ESVInf    = 0x73
	ESVInfReq = 0x63

	// ControllerEOJ is used as the source object in requests.
	ControllerEOJ = uint32(0x05FF01)
	// NodeProfileEOJ is the ECHONET Lite node profile object.
	NodeProfileEOJ = uint32(0x0EF001)

	UDPPort           = 3610
	MulticastAddr     = "224.0.23.0"
)

// Frame represents an ECHONET Lite frame.
type Frame struct {
	TID  uint16
	SEOJ uint32 // 3 bytes used: 0x00GGCCII
	DEOJ uint32 // 3 bytes used
	ESV  byte
	Props []Property
}

// Property is an EPC + EDT pair.
type Property struct {
	EPC byte
	EDT []byte
}

// Encode serializes the frame to a byte slice.
func (f *Frame) Encode() []byte {
	buf := []byte{EHD1, EHD2}
	buf = append(buf, byte(f.TID>>8), byte(f.TID))
	buf = append(buf, eojBytes(f.SEOJ)...)
	buf = append(buf, eojBytes(f.DEOJ)...)
	buf = append(buf, f.ESV)
	buf = append(buf, byte(len(f.Props)))
	for _, p := range f.Props {
		buf = append(buf, p.EPC)
		buf = append(buf, byte(len(p.EDT)))
		buf = append(buf, p.EDT...)
	}
	return buf
}

// Decode parses a byte slice into a Frame.
func Decode(data []byte) (*Frame, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}
	if data[0] != EHD1 || data[1] != EHD2 {
		return nil, fmt.Errorf("invalid EHD: %02X %02X", data[0], data[1])
	}

	f := &Frame{}
	f.TID = binary.BigEndian.Uint16(data[2:4])
	f.SEOJ = uint32(data[4])<<16 | uint32(data[5])<<8 | uint32(data[6])
	f.DEOJ = uint32(data[7])<<16 | uint32(data[8])<<8 | uint32(data[9])
	f.ESV = data[10]

	opc := int(data[11])
	pos := 12
	for propIdx := range opc {
		if pos+2 > len(data) {
			return nil, fmt.Errorf("frame truncated at property %d", propIdx)
		}
		epc := data[pos]
		pdc := int(data[pos+1])
		pos += 2
		if pos+pdc > len(data) {
			return nil, fmt.Errorf("frame truncated at EDT (EPC=%02X)", epc)
		}
		edt := make([]byte, pdc)
		copy(edt, data[pos:pos+pdc])
		pos += pdc
		f.Props = append(f.Props, Property{EPC: epc, EDT: edt})
	}
	return f, nil
}

// NewGetRequest builds a Get request frame for the given EPC codes.
func NewGetRequest(tid uint16, deoj uint32, epcs ...byte) *Frame {
	props := make([]Property, len(epcs))
	for i, epc := range epcs {
		props[i] = Property{EPC: epc}
	}
	return &Frame{
		TID:   tid,
		SEOJ:  ControllerEOJ,
		DEOJ:  deoj,
		ESV:   ESVGet,
		Props: props,
	}
}

func eojBytes(eoj uint32) []byte {
	return []byte{byte(eoj >> 16), byte(eoj >> 8), byte(eoj)}
}

// ParseEOJHex parses a 4- or 6-character hex string to a 3-byte EOJ.
// "0130" → 0x013001, "013001" → 0x013001
func ParseEOJHex(s string) (uint32, error) {
	switch len(s) {
	case 4:
		var g, c byte
		if _, err := fmt.Sscanf(s, "%02X%02X", &g, &c); err != nil {
			return 0, fmt.Errorf("invalid EOJ: %s", s)
		}
		return uint32(g)<<16 | uint32(c)<<8 | 0x01, nil
	case 6:
		var g, c, i byte
		if _, err := fmt.Sscanf(s, "%02X%02X%02X", &g, &c, &i); err != nil {
			return 0, fmt.Errorf("invalid EOJ: %s", s)
		}
		return uint32(g)<<16 | uint32(c)<<8 | uint32(i), nil
	default:
		return 0, fmt.Errorf("invalid EOJ length: %s", s)
	}
}
