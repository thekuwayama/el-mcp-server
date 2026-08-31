package ui

// Smart meter EPC codes (ECHONET Lite 0x0288 low-voltage smart electric
// energy meter class). OperatingStatusEPC (0x80) is the shared superclass
// property defined in common_decode.go.
const (
	InstantaneousElectricPowerEPC   = 0xE7 // signed number, W (noData=0x7FFFFFFE)
	InstantaneousCurrentsEPC        = 0xE8 // R相/T相, signed number x0.1A each (noData=0x7FFE)
	CumulativeElectricEnergyEPC     = 0xE0 // raw cumulative energy, normal direction (noData=0xFFFFFFFE); scale by CoefficientEPC x CumulativeElectricEnergyUnitEPC to get kWh
	CoefficientEPC                  = 0xD3 // number 0-999999, multiplier applied to CumulativeElectricEnergyEPC (assumed 1 if not reported)
	CumulativeElectricEnergyUnitEPC = 0xE1 // state enum, multiplying factor applied to CumulativeElectricEnergyEPC
)

// DecodeInstantaneousPowerW decodes EPC 0xE7 (4-byte signed instantaneous
// power in W). 0x7FFFFFFE means "no data".
func DecodeInstantaneousPowerW(edt []byte) (int32, bool) {
	if len(edt) != 4 {
		return 0, false
	}
	raw := int32(edt[0])<<24 | int32(edt[1])<<16 | int32(edt[2])<<8 | int32(edt[3])
	if raw == 0x7FFFFFFE {
		return 0, false
	}
	return raw, true
}

// Currents holds the R-phase/T-phase instantaneous current measurements (A)
// decoded from EPC 0xE8.
type Currents struct {
	RPhaseA float64 `json:"r_phase_a"`
	TPhaseA float64 `json:"t_phase_a"`
}

// DecodeInstantaneousCurrents decodes EPC 0xE8 (2-byte signed x0.1A per
// phase, R-phase then T-phase). 0x7FFE means "no data" for that phase.
func DecodeInstantaneousCurrents(edt []byte) (Currents, bool) {
	if len(edt) != 4 {
		return Currents{}, false
	}
	r := int16(edt[0])<<8 | int16(edt[1])
	t := int16(edt[2])<<8 | int16(edt[3])
	if r == 0x7FFE || t == 0x7FFE {
		return Currents{}, false
	}
	return Currents{RPhaseA: float64(r) / 10, TPhaseA: float64(t) / 10}, true
}

// DecodeRawCumulativeEnergy decodes EPC 0xE0 (4-byte unsigned raw cumulative
// energy, normal direction, 0-99999999). The raw value must be multiplied by
// the coefficient (EPC 0xD3) and unit factor (EPC 0xE1) to obtain kWh; see
// CumulativeEnergyKWh. 0xFFFFFFFE means "no data".
func DecodeRawCumulativeEnergy(edt []byte) (uint32, bool) {
	if len(edt) != 4 {
		return 0, false
	}
	raw := uint32(edt[0])<<24 | uint32(edt[1])<<16 | uint32(edt[2])<<8 | uint32(edt[3])
	if raw == 0xFFFFFFFE || raw > 99999999 {
		return 0, false
	}
	return raw, true
}

// DecodeCoefficient decodes EPC 0xD3 (4-byte unsigned multiplier, 0-999999).
func DecodeCoefficient(edt []byte) (uint32, bool) {
	if len(edt) != 4 {
		return 0, false
	}
	raw := uint32(edt[0])<<24 | uint32(edt[1])<<16 | uint32(edt[2])<<8 | uint32(edt[3])
	if raw > 999999 {
		return 0, false
	}
	return raw, true
}

var energyUnitFactors = map[byte]float64{
	0x00: 1,
	0x01: 0.1,
	0x02: 0.01,
	0x03: 0.001,
	0x04: 0.0001,
	0x0A: 10,
	0x0B: 100,
	0x0C: 1000,
	0x0D: 10000,
}

// DecodeEnergyUnit decodes EPC 0xE1 (1-byte state enum) into the multiplying
// factor applied to the raw cumulative energy value (EPC 0xE0).
func DecodeEnergyUnit(edt []byte) (float64, bool) {
	if len(edt) != 1 {
		return 0, false
	}
	f, ok := energyUnitFactors[edt[0]]
	return f, ok
}

// CumulativeEnergyKWh combines the raw cumulative energy (EPC 0xE0), the
// coefficient (EPC 0xD3, treated as 1 when not reported by the device), and
// the unit factor (EPC 0xE1) into an energy value in kWh.
func CumulativeEnergyKWh(raw uint32, coefficient *uint32, unitFactor float64) float64 {
	coef := uint32(1)
	if coefficient != nil {
		coef = *coefficient
	}
	return float64(raw) * float64(coef) * unitFactor
}
