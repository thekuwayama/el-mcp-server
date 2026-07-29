// Package ui provides EDT decoders for ECHONET Lite properties used by
// MCP Apps dashboards.
package ui

import "fmt"

// Battery-related EPC codes (ECHONET Lite 0x027D storage battery class).
const (
	OperatingStatusEPC          = 0x80 // super class: operation status
	OperationModeEPC            = 0xDA // state enum
	RemainingCapacityPercentEPC = 0xE4 // number 0-100%
	ChargeDischargePowerEPC     = 0xD3 // signed number, W
)

var operationModeNames = map[byte]string{
	0x40: "その他",
	0x41: "急速充電",
	0x42: "充電",
	0x43: "放電",
	0x44: "待機",
	0x45: "テスト",
	0x46: "自動",
	0x48: "再起動",
}

// DecodeOperationMode decodes EPC 0xDA (1 byte state enum) into a Japanese label.
func DecodeOperationMode(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := operationModeNames[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}

// DecodePercent decodes a 1-byte unsigned percentage value (0-100), used by EPC 0xE4.
func DecodePercent(edt []byte) (int, bool) {
	if len(edt) != 1 || edt[0] > 100 {
		return 0, false
	}
	return int(edt[0]), true
}

// DecodeSignedPowerW decodes a 4-byte signed power value in watts, used by EPC 0xD3.
func DecodeSignedPowerW(edt []byte) (int32, bool) {
	if len(edt) != 4 {
		return 0, false
	}
	return int32(edt[0])<<24 | int32(edt[1])<<16 | int32(edt[2])<<8 | int32(edt[3]), true
}

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
