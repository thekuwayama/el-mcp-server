package ui

import "testing"

func TestDecodeInstantaneousPowerW(t *testing.T) {
	// -320 W as a 4-byte big-endian two's complement value.
	if v, ok := DecodeInstantaneousPowerW([]byte{0xFF, 0xFF, 0xFE, 0xC0}); !ok || v != -320 {
		t.Errorf("DecodeInstantaneousPowerW(-320) = (%d, %v), want (-320, true)", v, ok)
	}
	if _, ok := DecodeInstantaneousPowerW([]byte{0x7F, 0xFF, 0xFF, 0xFE}); ok {
		t.Error("DecodeInstantaneousPowerW with noData sentinel should fail")
	}
	if _, ok := DecodeInstantaneousPowerW([]byte{0x00, 0x00, 0x00}); ok {
		t.Error("DecodeInstantaneousPowerW with 3 bytes should fail")
	}
}

func TestDecodeInstantaneousCurrents(t *testing.T) {
	// R相 12.3A, T相 -4.5A
	edt := []byte{0x00, 0x7B, 0xFF, 0xD3}
	got, ok := DecodeInstantaneousCurrents(edt)
	if !ok || got.RPhaseA != 12.3 || got.TPhaseA != -4.5 {
		t.Errorf("DecodeInstantaneousCurrents(%v) = (%+v, %v), want ({12.3 -4.5}, true)", edt, got, ok)
	}
	if _, ok := DecodeInstantaneousCurrents([]byte{0x7F, 0xFE, 0x00, 0x00}); ok {
		t.Error("DecodeInstantaneousCurrents with noData R-phase should fail")
	}
	if _, ok := DecodeInstantaneousCurrents([]byte{0x00, 0x00}); ok {
		t.Error("DecodeInstantaneousCurrents with 2 bytes should fail")
	}
}

func TestDecodeRawCumulativeEnergy(t *testing.T) {
	if v, ok := DecodeRawCumulativeEnergy([]byte{0x00, 0x00, 0x04, 0xD2}); !ok || v != 1234 {
		t.Errorf("DecodeRawCumulativeEnergy(1234) = (%d, %v), want (1234, true)", v, ok)
	}
	if _, ok := DecodeRawCumulativeEnergy([]byte{0xFF, 0xFF, 0xFF, 0xFE}); ok {
		t.Error("DecodeRawCumulativeEnergy with noData sentinel should fail")
	}
}

func TestDecodeCoefficient(t *testing.T) {
	if v, ok := DecodeCoefficient([]byte{0x00, 0x00, 0x00, 0x0A}); !ok || v != 10 {
		t.Errorf("DecodeCoefficient(10) = (%d, %v), want (10, true)", v, ok)
	}
	if _, ok := DecodeCoefficient([]byte{0x00, 0x0F, 0x42, 0x40}); ok {
		t.Error("DecodeCoefficient above 999999 should fail")
	}
}

func TestDecodeEnergyUnit(t *testing.T) {
	if v, ok := DecodeEnergyUnit([]byte{0x03}); !ok || v != 0.001 {
		t.Errorf("DecodeEnergyUnit(0x03) = (%v, %v), want (0.001, true)", v, ok)
	}
	if _, ok := DecodeEnergyUnit([]byte{0x99}); ok {
		t.Error("DecodeEnergyUnit with unknown enum value should fail")
	}
}

// energyLogEDT builds an EPC 0xE2/0xE4 EDT for day with the given readings;
// slots beyond len(readings) are filled with the 0xFFFFFFFE no-data sentinel.
func energyLogEDT(day int, readings ...uint32) []byte {
	edt := []byte{byte(day >> 8), byte(day)}
	for i := range SlotsPerDay {
		v := uint32(0xFFFFFFFE)
		if i < len(readings) {
			v = readings[i]
		}
		edt = append(edt, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
	return edt
}

func TestDecodeEnergyLog(t *testing.T) {
	log, ok := DecodeEnergyLog(energyLogEDT(3, 1000, 1005))
	if !ok {
		t.Fatal("DecodeEnergyLog failed on a valid EDT")
	}
	if log.Day != 3 {
		t.Errorf("Day = %d, want 3", log.Day)
	}
	if !log.Valid[0] || log.Readings[0] != 1000 || !log.Valid[1] || log.Readings[1] != 1005 {
		t.Errorf("first readings = (%d %v, %d %v), want (1000 true, 1005 true)", log.Readings[0], log.Valid[0], log.Readings[1], log.Valid[1])
	}
	if log.Valid[2] {
		t.Error("slot with noData sentinel should be invalid")
	}

	if _, ok := DecodeEnergyLog(energyLogEDT(3)[:100]); ok {
		t.Error("DecodeEnergyLog with truncated EDT should fail")
	}
	if _, ok := DecodeEnergyLog(energyLogEDT(0xFF)); ok {
		t.Error("DecodeEnergyLog with day out of range should fail")
	}
}

func TestSlotEnergiesKWh(t *testing.T) {
	readings := make([]uint32, SlotsPerDay)
	for i := range readings {
		readings[i] = 1000 + uint32(i)*3
	}
	readings[10] = 0xFFFFFFFE // outage
	log, _ := DecodeEnergyLog(energyLogEDT(1, readings...))
	next, _ := DecodeEnergyLog(energyLogEDT(0, 1000+SlotsPerDay*3))

	got := SlotEnergiesKWh(log, &next, nil, 0.1)
	if got[0] == nil || *got[0] != 0.3 {
		t.Errorf("slot 0 = %v, want 0.3", got[0])
	}
	if got[9] != nil || got[10] != nil {
		t.Error("slots touching a no-data reading should be nil")
	}
	if got[SlotsPerDay-1] == nil || *got[SlotsPerDay-1] != 0.3 {
		t.Errorf("last slot = %v, want 0.3 (ends at next day's 00:00)", got[SlotsPerDay-1])
	}

	if got := SlotEnergiesKWh(log, nil, nil, 0.1); got[SlotsPerDay-1] != nil {
		t.Error("last slot without next day's log should be nil")
	}

	wrapped, _ := DecodeEnergyLog(energyLogEDT(0, 99999999, 2))
	if got := SlotEnergiesKWh(wrapped, nil, nil, 1); got[0] == nil || *got[0] != 3 {
		t.Errorf("wrapped slot = %v, want 3", got[0])
	}
}

func TestCumulativeEnergyKWh(t *testing.T) {
	if got := CumulativeEnergyKWh(1234, nil, 0.001); got != 1.234 {
		t.Errorf("CumulativeEnergyKWh(1234, nil, 0.001) = %v, want 1.234", got)
	}
	coef := uint32(10)
	if got := CumulativeEnergyKWh(1234, &coef, 0.001); got != 12.34 {
		t.Errorf("CumulativeEnergyKWh(1234, 10, 0.001) = %v, want 12.34", got)
	}
}
