package ui

import "testing"

func TestDecodeSystemInterconnectionStatus(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x00}, "系統連系(逆潮流可)", true},
		{[]byte{0x02}, "系統連系(逆潮流不可)", true},
		{[]byte{0x99}, "不明(0x99)", true},
		{[]byte{0x00, 0x01}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeSystemInterconnectionStatus(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("DecodeSystemInterconnectionStatus(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeOutputPowerRestraintStatus(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x44}, "抑制未実施", true},
		{[]byte{0x41}, "抑制中(出力制御)", true},
		{[]byte{0x99}, "不明(0x99)", true},
	}
	for _, c := range cases {
		got, ok := DecodeOutputPowerRestraintStatus(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("DecodeOutputPowerRestraintStatus(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeUnsignedPowerW(t *testing.T) {
	if v, ok := DecodeUnsignedPowerW([]byte{0x01, 0x2C}); !ok || v != 300 {
		t.Errorf("DecodeUnsignedPowerW(300) = (%d, %v), want (300, true)", v, ok)
	}
	if _, ok := DecodeUnsignedPowerW([]byte{0xFF, 0xFF}); ok {
		t.Error("DecodeUnsignedPowerW with 0xFFFF should fail (out of range)")
	}
	if _, ok := DecodeUnsignedPowerW([]byte{0x01}); ok {
		t.Error("DecodeUnsignedPowerW with 1 byte should fail")
	}
}

func TestDecodeCumulativeEnergyKWh(t *testing.T) {
	// 12345 raw (0.001kWh units) => 12.345 kWh
	edt := []byte{0x00, 0x00, 0x30, 0x39}
	v, ok := DecodeCumulativeEnergyKWh(edt)
	if !ok || v != 12.345 {
		t.Errorf("DecodeCumulativeEnergyKWh(12345) = (%v, %v), want (12.345, true)", v, ok)
	}
	if _, ok := DecodeCumulativeEnergyKWh([]byte{0x00, 0x00, 0x30}); ok {
		t.Error("DecodeCumulativeEnergyKWh with 3 bytes should fail")
	}
}

func TestSelfConsumptionPercent(t *testing.T) {
	if v, ok := SelfConsumptionPercent(141.303, 13.298); !ok || v != 90 {
		t.Errorf("SelfConsumptionPercent(141.303, 13.298) = (%d, %v), want (90, true)", v, ok)
	}
	if _, ok := SelfConsumptionPercent(0, 0); ok {
		t.Error("SelfConsumptionPercent with 0 generated should fail")
	}
	if _, ok := SelfConsumptionPercent(10, 20); ok {
		t.Error("SelfConsumptionPercent with sold > generated should fail")
	}
}
