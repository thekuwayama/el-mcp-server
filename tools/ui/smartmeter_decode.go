package ui

import "math"

// Smart meter EPC codes (ECHONET Lite 0x0288 low-voltage smart electric
// energy meter class). OperatingStatusEPC (0x80) is the shared superclass
// property defined in common_decode.go.
const (
	InstantaneousElectricPowerEPC   = 0xE7 // signed number, W (noData=0x7FFFFFFE)
	InstantaneousCurrentsEPC        = 0xE8 // R相/T相, signed number x0.1A each (noData=0x7FFE)
	CumulativeElectricEnergyEPC     = 0xE0 // raw cumulative energy, normal direction (noData=0xFFFFFFFE); scale by CoefficientEPC x CumulativeElectricEnergyUnitEPC to get kWh
	CoefficientEPC                  = 0xD3 // number 0-999999, multiplier applied to CumulativeElectricEnergyEPC (assumed 1 if not reported)
	CumulativeElectricEnergyUnitEPC = 0xE1 // state enum, multiplying factor applied to CumulativeElectricEnergyEPC
	NormalDirectionEnergyLogEPC     = 0xE2 // collection day (2 bytes) + 48 half-hourly raw cumulative readings, normal direction (import)
	ReverseDirectionEnergyLogEPC    = 0xE4 // same layout as 0xE2, reverse direction (export); conditionally required
	EnergyLogDayEPC                 = 0xE5 // collection day for 0xE2/0xE4, 0 (today) to 99 days ago; Set selects the day
)

// SlotsPerDay is the number of half-hourly readings in one day's energy log
// (00:00 to 23:30).
const SlotsPerDay = 48

// MaxEnergyLogDay is the largest collection day (days ago) EPC 0xE5 accepts.
const MaxEnergyLogDay = 99

// rawEnergyModulus is where the 8-digit raw cumulative reading wraps around
// back to 0.
const rawEnergyModulus = 100000000

// EnergyLog is one day's half-hourly cumulative readings decoded from EPC
// 0xE2/0xE4. Readings[i] is the raw cumulative reading at slot i (00:00,
// 00:30, ... 23:30); it is only meaningful when Valid[i] is true.
type EnergyLog struct {
	Day      int
	Readings [SlotsPerDay]uint32
	Valid    [SlotsPerDay]bool
}

// DecodeEnergyLog decodes EPC 0xE2/0xE4: a 2-byte collection day followed by
// 48 4-byte raw cumulative readings. 0xFFFFFFFE (or any out-of-range value)
// marks a slot without data, e.g. the future slots of today or a slot lost to
// a power outage.
func DecodeEnergyLog(edt []byte) (EnergyLog, bool) {
	if len(edt) != 2+SlotsPerDay*4 {
		return EnergyLog{}, false
	}
	log := EnergyLog{Day: int(edt[0])<<8 | int(edt[1])}
	if log.Day > MaxEnergyLogDay {
		return EnergyLog{}, false
	}
	for i := range SlotsPerDay {
		v, ok := DecodeRawCumulativeEnergy(edt[2+i*4 : 6+i*4])
		log.Readings[i] = v
		log.Valid[i] = ok
	}
	return log, true
}

// EncodeEnergyLogDay encodes a collection day (days ago) as the EDT of EPC
// 0xE5.
func EncodeEnergyLogDay(daysAgo int) []byte {
	return []byte{byte(daysAgo)}
}

// SlotEnergiesKWh converts one day's cumulative readings into the energy
// consumed (or exported) in each half-hour slot, in kWh. Slot i covers
// Readings[i] to Readings[i+1]; the last slot (23:30-24:00) ends at the next
// day's 00:00 reading, so next must be the following day's log (nil if not
// available, e.g. for today). A slot is nil when either endpoint has no data.
// A decreasing reading is treated as the 8-digit counter wrapping around.
func SlotEnergiesKWh(log EnergyLog, next *EnergyLog, coefficient *uint32, unitFactor float64) [SlotsPerDay]*float64 {
	var out [SlotsPerDay]*float64
	for i := range SlotsPerDay {
		start, startOK := log.Readings[i], log.Valid[i]
		var end uint32
		var endOK bool
		if i+1 < SlotsPerDay {
			end, endOK = log.Readings[i+1], log.Valid[i+1]
		} else if next != nil {
			end, endOK = next.Readings[0], next.Valid[0]
		}
		if !startOK || !endOK {
			continue
		}
		diff := (uint64(end) + rawEnergyModulus - uint64(start)) % rawEnergyModulus
		kwh := roundKWh(CumulativeEnergyKWh(uint32(diff), coefficient, unitFactor))
		out[i] = &kwh
	}
	return out
}

// roundKWh rounds to 4 decimal places, the finest resolution EPC 0xE1 can
// express, to drop floating-point noise such as 0.30000000000000004.
func roundKWh(v float64) float64 {
	return math.Round(v*10000) / 10000
}

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
// decoded from EPC 0xE8. A phase is nil when the meter reports no data for
// it; a single-phase two-wire meter always reports no data for the T phase.
type Currents struct {
	RPhaseA *float64 `json:"r_phase_a,omitempty"`
	TPhaseA *float64 `json:"t_phase_a,omitempty"`
}

// DecodeInstantaneousCurrents decodes EPC 0xE8 (2-byte signed x0.1A per
// phase, R-phase then T-phase). Each phase is decoded independently: 0x7FFE
// means "no data" for that phase (single-phase two-wire meters set it for
// the T phase), and any other value outside the valid range
// (-3276.7A to 3276.5A, i.e. 0x7FFF and 0x8000) is also treated as no data.
// Decoding fails only when the EDT is malformed or neither phase has data.
func DecodeInstantaneousCurrents(edt []byte) (Currents, bool) {
	if len(edt) != 4 {
		return Currents{}, false
	}
	c := Currents{
		RPhaseA: decodePhaseCurrentA(edt[0:2]),
		TPhaseA: decodePhaseCurrentA(edt[2:4]),
	}
	if c.RPhaseA == nil && c.TPhaseA == nil {
		return Currents{}, false
	}
	return c, true
}

// decodePhaseCurrentA decodes one phase of EPC 0xE8 (2-byte signed x0.1A),
// returning nil for "no data" (0x7FFE) or a value outside -32767 to 32765.
func decodePhaseCurrentA(b []byte) *float64 {
	raw := int16(b[0])<<8 | int16(b[1])
	if raw < -32767 || raw > 32765 {
		return nil
	}
	a := float64(raw) / 10
	return &a
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
