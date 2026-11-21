package tools

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestParseHexByte(t *testing.T) {
	valid := []struct {
		in   string
		want byte
	}{
		{"B0", 0xB0},
		{"b0", 0xB0},
		{"00", 0x00},
		{"FF", 0xFF},
		{" E7 ", 0xE7},
	}
	for _, c := range valid {
		got, err := parseHexByte(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseHexByte(%q) = (%02X, %v), want (%02X, nil)", c.in, got, err, c.want)
		}
	}

	invalid := []string{"", "0", "123", "0G", "G0", "0x", "0x80", "+1", "-1", "8 0"}
	for _, in := range invalid {
		if got, err := parseHexByte(in); err == nil {
			t.Errorf("parseHexByte(%q) = %02X, want error", in, got)
		}
	}
}

func TestParseHexBytes(t *testing.T) {
	valid := []struct {
		in   string
		want []byte
	}{
		{"30", []byte{0x30}},
		{"4100", []byte{0x41, 0x00}},
		{"FF 01", []byte{0xFF, 0x01}},
		{"ff 01 a0", []byte{0xFF, 0x01, 0xA0}},
		{" 30 ", []byte{0x30}},
	}
	for _, c := range valid {
		got, err := parseHexBytes(c.in)
		if err != nil || !bytes.Equal(got, c.want) {
			t.Errorf("parseHexBytes(%q) = (% X, %v), want (% X, nil)", c.in, got, err, c.want)
		}
	}

	invalid := []string{"", "   ", "3", "0x30", "0G", "G0", "30 0G", "+1", "-1"}
	for _, in := range invalid {
		if got, err := parseHexBytes(in); err == nil {
			t.Errorf("parseHexBytes(%q) = % X, want error", in, got)
		}
	}
}

func TestParseHexBytesLengthLimit(t *testing.T) {
	maxLen := strings.Repeat("AB", 255)
	got, err := parseHexBytes(maxLen)
	if err != nil || len(got) != 255 {
		t.Errorf("parseHexBytes(255 bytes) = (%d bytes, %v), want (255 bytes, nil)", len(got), err)
	}

	spaced := strings.TrimSpace(strings.Repeat("AB ", 255))
	if got, err := parseHexBytes(spaced); err != nil || len(got) != 255 {
		t.Errorf("parseHexBytes(255 space-separated bytes) = (%d bytes, %v), want (255 bytes, nil)", len(got), err)
	}

	for _, in := range []string{strings.Repeat("AB", 256), strings.Repeat("AB ", 256), strings.Repeat("AB", 1000)} {
		if _, err := parseHexBytes(in); !errors.Is(err, errHexTooLong) {
			t.Errorf("parseHexBytes(%d chars) error = %v, want errHexTooLong", len(in), err)
		}
	}
}
