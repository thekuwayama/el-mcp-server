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

func TestCumulativeEnergyKWh(t *testing.T) {
	if got := CumulativeEnergyKWh(1234, nil, 0.001); got != 1.234 {
		t.Errorf("CumulativeEnergyKWh(1234, nil, 0.001) = %v, want 1.234", got)
	}
	coef := uint32(10)
	if got := CumulativeEnergyKWh(1234, &coef, 0.001); got != 12.34 {
		t.Errorf("CumulativeEnergyKWh(1234, 10, 0.001) = %v, want 12.34", got)
	}
}
