package ui

import "fmt"

// EV charger/discharger (V2H) EPC codes (ECHONET Lite 0x027E electric
// vehicle charger and discharger class). OperatingStatusEPC (0x80),
// OperationModeEPC (0xDA), ChargeDischargePowerEPC (0xD3), and
// RemainingCapacityPercentEPC (0xE4) are shared with battery_decode.go —
// same EPC numbers and generic decode shape (state / signed power / percent).
const (
	VehicleConnectionStatusEPC     = 0xC7 // state enum
	CumulativeDischargingEnergyEPC = 0xD6 // number 0-999999.999kWh (raw x0.001), decode with DecodeCumulativeEnergyKWh
	CumulativeChargingEnergyEPC    = 0xD8 // number 0-999999.999kWh (raw x0.001), decode with DecodeCumulativeEnergyKWh
)

// V2H's 0xDA enum diverges from the battery class despite sharing the same
// EPC: 0x46 is "充放電" here (vs. battery's "自動"), 0x48 is "準備" here
// (vs. battery's "再起動"). Kept separate from operationModeNames for that
// reason.
var v2hOperationModeNames = map[byte]string{
	0x40: "その他",
	0x42: "充電",
	0x43: "放電",
	0x44: "待機",
	0x46: "充放電",
	0x47: "停止",
	0x48: "準備",
	0x49: "自動",
}

var vehicleConnectionStatusNames = map[byte]string{
	0x30: "車両未接続",
	0x40: "車両接続・充電不可・放電不可",
	0x41: "車両接続・充電可・放電不可",
	0x42: "車両接続・充電不可・放電可",
	0x43: "車両接続・充電可・放電可",
	0x44: "車両接続・充電可否不明",
	0xFF: "不定",
}

// DecodeV2HOperationMode decodes EPC 0xDA (1 byte state enum) for EV
// charger/dischargers into a Japanese label.
func DecodeV2HOperationMode(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := v2hOperationModeNames[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}

// DecodeVehicleConnectionStatus decodes EPC 0xC7 (1 byte state enum) into a Japanese label.
func DecodeVehicleConnectionStatus(edt []byte) (string, bool) {
	if len(edt) != 1 {
		return "", false
	}
	if name, ok := vehicleConnectionStatusNames[edt[0]]; ok {
		return name, true
	}
	return fmt.Sprintf("不明(0x%02X)", edt[0]), true
}
