package ui

import "fmt"

// Air conditioner-related EPC codes (ECHONET Lite 0x0130 home air conditioner
// class). OperatingStatusEPC (0x80) is the shared superclass property
// defined in common_decode.go.
const (
	AirconOperationModeEPC  = 0xB0 // state enum
	TargetTemperatureEPC    = 0xB3 // number 0-50 Celsius, or 0xFD=undefined
	RoomTemperatureEPC      = 0xBB // signed number -127-125 Celsius, or 0x7E=unmeasurable
	AirFlowLevelEPC         = 0xA0 // level 1-8, or 0x41=auto
	undefinedTargetTempEDT  = 0xFD
	unmeasurableRoomTempEDT = 0x7E
	autoAirFlowLevelEDT     = 0x41
	airFlowLevelBase        = 0x31
	airFlowLevelMax         = 8
)

var airconOperationModeNames = map[byte]string{
	0x40: "その他",
	0x41: "自動",
	0x42: "冷房",
	0x43: "暖房",
	0x44: "除湿",
	0x45: "送風",
}

// DecodeAirconOperationMode decodes EPC 0xB0 (1 byte state enum) into a Japanese label.
func DecodeAirconOperationMode(edt []byte) (string, bool) {
	return decodeStateEnum(edt, airconOperationModeNames)
}

// DecodeTargetTemperature decodes EPC 0xB3 (1 byte unsigned 0-50 Celsius, or
// 0xFD for "undefined") into a temperature in Celsius. Returns ok=false when
// the device reports the value as undefined.
func DecodeTargetTemperature(edt []byte) (int, bool) {
	if len(edt) != 1 || edt[0] == undefinedTargetTempEDT {
		return 0, false
	}
	return int(edt[0]), true
}

// DecodeRoomTemperature decodes EPC 0xBB (1 byte signed -127-125 Celsius, or
// 0x7E for "unmeasurable") into a temperature in Celsius. Returns ok=false
// when the device reports the value as unmeasurable.
func DecodeRoomTemperature(edt []byte) (int, bool) {
	if len(edt) != 1 || edt[0] == unmeasurableRoomTempEDT {
		return 0, false
	}
	return int(int8(edt[0])), true
}

// DecodeAirFlowLevel decodes EPC 0xA0 (1 byte level 0x31-0x38 for levels
// 1-8, or 0x41 for "auto") into a Japanese label.
func DecodeAirFlowLevel(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if edt[0] == autoAirFlowLevelEDT {
		return "自動", true
	}
	if edt[0] >= airFlowLevelBase && edt[0] < airFlowLevelBase+airFlowLevelMax {
		return fmt.Sprintf("風量%d", edt[0]-airFlowLevelBase+1), true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}
