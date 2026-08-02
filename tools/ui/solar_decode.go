package ui

import "fmt"

// Solar power generation EPC codes (ECHONET Lite 0x0279 household solar
// power generation class). OperatingStatusEPC (0x80) is a superclass
// property shared with battery_decode.go.
const (
	InstantaneousElectricPowerGenerationEPC = 0xE0 // number 0-65533W
	CumulativeElectricEnergyOfGenerationEPC = 0xE1 // number 0-999999.999kWh (raw x0.001)
	CumulativeElectricEnergySoldEPC         = 0xE3 // number 0-999999.999kWh (raw x0.001)
	SystemInterconnectionStatusEPC          = 0xD0 // state enum
	OutputPowerRestraintStatusEPC           = 0xD1 // state enum
)

var systemInterconnectionStatusNames = map[byte]string{
	0x00: "系統連系(逆潮流可)",
	0x01: "独立",
	0x02: "系統連系(逆潮流不可)",
	0x03: "不明",
}

var outputPowerRestraintStatusNames = map[byte]string{
	0x41: "抑制中(出力制御)",
	0x42: "抑制中(出力制御以外)",
	0x43: "抑制中(抑制要因不明)",
	0x44: "抑制未実施",
	0x45: "不明",
}

// DecodeSystemInterconnectionStatus decodes EPC 0xD0 (1 byte state enum) into a Japanese label.
func DecodeSystemInterconnectionStatus(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := systemInterconnectionStatusNames[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}

// DecodeOutputPowerRestraintStatus decodes EPC 0xD1 (1 byte state enum) into a Japanese label.
func DecodeOutputPowerRestraintStatus(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := outputPowerRestraintStatusNames[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}

// DecodeUnsignedPowerW decodes a 2-byte unsigned power value in watts (0-65533), used by EPC 0xE0.
func DecodeUnsignedPowerW(edt []byte) (uint16, bool) {
	if len(edt) != 2 {
		return 0, false
	}
	v := uint16(edt[0])<<8 | uint16(edt[1])
	if v > 65533 {
		return 0, false
	}
	return v, true
}

// DecodeCumulativeEnergyKWh decodes a 4-byte unsigned cumulative energy value
// in units of 0.001kWh into kWh, used by EPC 0xE1 and 0xE3.
func DecodeCumulativeEnergyKWh(edt []byte) (float64, bool) {
	if len(edt) != 4 {
		return 0, false
	}
	raw := uint32(edt[0])<<24 | uint32(edt[1])<<16 | uint32(edt[2])<<8 | uint32(edt[3])
	if raw > 999999999 {
		return 0, false
	}
	return float64(raw) / 1000, true
}

// SelfConsumptionPercent derives the self-consumption rate (0-100) from
// cumulative generated and sold energy (both kWh, as returned by
// DecodeCumulativeEnergyKWh). It drives the dashboard gauge the same way
// DecodePercent drives the battery dashboard's remaining-capacity gauge.
func SelfConsumptionPercent(generatedKWh, soldKWh float64) (int, bool) {
	if generatedKWh <= 0 || soldKWh < 0 || soldKWh > generatedKWh {
		return 0, false
	}
	return int((generatedKWh - soldKWh) / generatedKWh * 100), true
}
