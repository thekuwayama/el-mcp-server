package ui

import "testing"

func TestDecodeV2HOperationMode(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x42}, "充電", true},
		{[]byte{0x46}, "充放電", true},
		{[]byte{0x48}, "準備", true},
		{[]byte{0x99}, "不明(0x99)", true},
		{[]byte{0x42, 0x00}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeV2HOperationMode(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("DecodeV2HOperationMode(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodeVehicleConnectionStatus(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x30}, "車両未接続", true},
		{[]byte{0x43}, "車両接続・充電可・放電可", true},
		{[]byte{0x99}, "不明(0x99)", true},
		{[]byte{0x30, 0x00}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeVehicleConnectionStatus(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("DecodeVehicleConnectionStatus(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}
