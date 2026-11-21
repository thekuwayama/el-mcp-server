package echonet

import (
	"bytes"
	"testing"
)

func TestEncode(t *testing.T) {
	f := NewSetCRequest(0x1234, 0x013001, 0xB0, []byte{0x41})
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	want := []byte{0x10, 0x81, 0x12, 0x34, 0x05, 0xFF, 0x01, 0x01, 0x30, 0x01, 0x61, 0x01, 0xB0, 0x01, 0x41}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode() = % X, want % X", got, want)
	}
}

func TestEncodeLengthLimits(t *testing.T) {
	// 255 bytes is the largest EDT that fits in PDC, and it must round-trip.
	edt := bytes.Repeat([]byte{0xAB}, MaxPDC)
	data, err := NewSetCRequest(1, 0x013001, 0xB0, edt).Encode()
	if err != nil {
		t.Fatalf("Encode(255-byte EDT) error = %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(decoded.Props) != 1 || !bytes.Equal(decoded.Props[0].EDT, edt) {
		t.Errorf("round-trip EDT mismatch: got %d props", len(decoded.Props))
	}

	if _, err := NewSetCRequest(1, 0x013001, 0xB0, make([]byte, MaxPDC+1)).Encode(); err == nil {
		t.Error("Encode(256-byte EDT) error = nil, want error")
	}

	epcs := make([]byte, MaxOPC)
	if _, err := NewGetRequest(1, 0x013001, epcs...).Encode(); err != nil {
		t.Errorf("Encode(255 properties) error = %v, want nil", err)
	}
	epcs = make([]byte, MaxOPC+1)
	if _, err := NewGetRequest(1, 0x013001, epcs...).Encode(); err == nil {
		t.Error("Encode(256 properties) error = nil, want error")
	}
}

func TestParseEOJHex(t *testing.T) {
	valid := []struct {
		in   string
		want uint32
	}{
		{"0130", 0x013001},
		{"013001", 0x013001},
		{"027d02", 0x027D02},
		{"0EF001", 0x0EF001},
	}
	for _, c := range valid {
		got, err := ParseEOJHex(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseEOJHex(%q) = (%06X, %v), want (%06X, nil)", c.in, got, err, c.want)
		}
	}

	invalid := []string{"", "013", "01300", "0130011", "0x30", "0x3001", "013G", "01300G", " 130", "1 30", "+130"}
	for _, in := range invalid {
		if got, err := ParseEOJHex(in); err == nil {
			t.Errorf("ParseEOJHex(%q) = %06X, want error", in, got)
		}
	}
}
