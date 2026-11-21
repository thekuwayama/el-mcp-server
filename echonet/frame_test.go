package echonet

import "testing"

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
