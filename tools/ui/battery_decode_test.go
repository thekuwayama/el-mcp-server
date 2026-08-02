package ui

import "testing"

func TestDecodeOperationMode(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x42}, "充電", true},
		{[]byte{0x43}, "放電", true},
		{[]byte{0x99}, "不明(0x99)", true},
		{[]byte{0x42, 0x00}, "", false},
	}
	for _, c := range cases {
		got, ok := DecodeOperationMode(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("DecodeOperationMode(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestDecodePercent(t *testing.T) {
	if v, ok := DecodePercent([]byte{72}); !ok || v != 72 {
		t.Errorf("DecodePercent(72) = (%d, %v), want (72, true)", v, ok)
	}
	if _, ok := DecodePercent([]byte{0x99, 0x01}); ok {
		t.Error("DecodePercent with 2 bytes should fail")
	}
}

func TestDecodeSignedPowerW(t *testing.T) {
	// -320 W as a 4-byte big-endian two's complement value.
	edt := []byte{0xFF, 0xFF, 0xFE, 0xC0}
	v, ok := DecodeSignedPowerW(edt)
	if !ok || v != -320 {
		t.Errorf("DecodeSignedPowerW(-320) = (%d, %v), want (-320, true)", v, ok)
	}
}
