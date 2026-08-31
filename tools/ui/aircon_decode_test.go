package ui

import "testing"

func TestDecodeAirconOperationMode(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x41}, "自動", true},
		{[]byte{0x42}, "冷房", true},
		{[]byte{0x43}, "暖房", true},
		{[]byte{0x44}, "除湿", true},
		{[]byte{0x45}, "送風", true},
		{[]byte{0x40}, "その他", true},
		{[]byte{0x99}, "不明(0x99)", true},
		{[]byte{}, "", false},
		{[]byte{0x41, 0x42}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeAirconOperationMode(c.edt)
		if got != c.want || ok != c.ok {
			t.Errorf("DecodeAirconOperationMode(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeTargetTemperature(t *testing.T) {
	cases := []struct {
		edt  []byte
		want int
		ok   bool
	}{
		{[]byte{0x00}, 0, true},
		{[]byte{0x19}, 25, true},
		{[]byte{0x32}, 50, true},
		{[]byte{0xFD}, 0, false},
		{[]byte{}, 0, false},
		{[]byte{0x19, 0x00}, 0, false},
	}
	for _, c := range cases {
		got, ok := DecodeTargetTemperature(c.edt)
		if got != c.want || ok != c.ok {
			t.Errorf("DecodeTargetTemperature(%v) = (%d, %v), want (%d, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeRoomTemperature(t *testing.T) {
	cases := []struct {
		edt  []byte
		want int
		ok   bool
	}{
		{[]byte{0x19}, 25, true},
		{[]byte{0xFF}, -1, true},
		{[]byte{0x81}, -127, true},
		{[]byte{0x7E}, 0, false},
		{[]byte{}, 0, false},
	}
	for _, c := range cases {
		got, ok := DecodeRoomTemperature(c.edt)
		if got != c.want || ok != c.ok {
			t.Errorf("DecodeRoomTemperature(%v) = (%d, %v), want (%d, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeAirFlowLevel(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x31}, "風量1", true},
		{[]byte{0x38}, "風量8", true},
		{[]byte{0x41}, "自動", true},
		{[]byte{0x39}, "不明(0x39)", true},
		{[]byte{}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeAirFlowLevel(c.edt)
		if got != c.want || ok != c.ok {
			t.Errorf("DecodeAirFlowLevel(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}
