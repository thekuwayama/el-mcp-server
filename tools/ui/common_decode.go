package ui

import "fmt"

// OperatingStatusEPC (0x80) is the superclass "operation status" property
// shared by every ECHONET Lite device class covered here.
const OperatingStatusEPC = 0x80

// DecodeOperatingStatus decodes EPC 0x80 (1 byte state: 0x30=ON, 0x31=OFF).
func DecodeOperatingStatus(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	switch edt[0] {
	case 0x30:
		return "稼働中", true
	case 0x31:
		return "停止", true
	default:
		return fmt.Sprintf("不明(0x%02X)", edt[0]), true
	}
}

// decodeStateEnum decodes a 1-byte ECHONET Lite state-enum property by
// looking it up in names, falling back to a "不明(0x..)" label for values
// outside the table. Shared by the per-class operation-mode/status decoders
// in battery_decode.go, solar_decode.go, and v2h_decode.go.
func decodeStateEnum(edt []byte, names map[byte]string) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := names[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}
