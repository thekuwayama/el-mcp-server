package ui

import "testing"

func TestDecodeOperatingStatus(t *testing.T) {
	if v, ok := DecodeOperatingStatus([]byte{0x30}); !ok || v != "稼働中" {
		t.Errorf("DecodeOperatingStatus(0x30) = (%q, %v), want (稼働中, true)", v, ok)
	}
	if v, ok := DecodeOperatingStatus([]byte{0x31}); !ok || v != "停止" {
		t.Errorf("DecodeOperatingStatus(0x31) = (%q, %v), want (停止, true)", v, ok)
	}
	if _, ok := DecodeOperatingStatus([]byte{0x30, 0x00}); ok {
		t.Error("DecodeOperatingStatus with 2 bytes should fail")
	}
}

func TestDecodeStateEnum(t *testing.T) {
	names := map[byte]string{0x01: "a", 0x02: "b"}

	if v, ok := decodeStateEnum([]byte{0x01}, names); !ok || v != "a" {
		t.Errorf("decodeStateEnum(0x01) = (%q, %v), want (a, true)", v, ok)
	}
	if v, ok := decodeStateEnum([]byte{0x99}, names); !ok || v != "不明(0x99)" {
		t.Errorf("decodeStateEnum(0x99) = (%q, %v), want (不明(0x99), true)", v, ok)
	}
	if _, ok := decodeStateEnum([]byte{0x01, 0x02}, names); ok {
		t.Error("decodeStateEnum with 2 bytes should fail")
	}
}
