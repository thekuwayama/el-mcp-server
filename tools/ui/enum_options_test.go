package ui

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestEnumCode(t *testing.T) {
	cases := []struct {
		edt  []byte
		want string
		ok   bool
	}{
		{[]byte{0x42}, "42", true},
		{[]byte{0x0A}, "0A", true},
		{[]byte{}, "", false},
		{[]byte{0x42, 0x00}, "", false},
	}
	for _, c := range cases {
		got, ok := EnumCode(c.edt)
		if ok != c.ok || got != c.want {
			t.Errorf("EnumCode(%v) = (%q, %v), want (%q, %v)", c.edt, got, ok, c.want, c.ok)
		}
	}
}

func TestEnumOptionMarshalJSON(t *testing.T) {
	b, err := json.Marshal([]EnumOption{{"42", "充電"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `[["42","充電"]]`; got != want {
		t.Errorf("json.Marshal = %s, want %s", got, want)
	}
}

// Every option must decode back to its own label, so the dashboards' select
// and the tools' operation_mode/air_flow_level fields can't drift apart.
func TestEnumOptionsRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		opts   []EnumOption
		decode func([]byte) (string, bool)
		want   []string // codes in display order
	}{
		{"OperationModeOptions", OperationModeOptions(), DecodeOperationMode,
			[]string{"40", "41", "42", "43", "44", "45", "46", "48"}},
		{"V2HOperationModeOptions", V2HOperationModeOptions(), DecodeV2HOperationMode,
			[]string{"40", "42", "43", "44", "46", "47", "48", "49"}},
		{"AirconOperationModeOptions", AirconOperationModeOptions(), DecodeAirconOperationMode,
			[]string{"40", "41", "42", "43", "44", "45"}},
		{"AirFlowLevelOptions", AirFlowLevelOptions(), DecodeAirFlowLevel,
			[]string{"41", "31", "32", "33", "34", "35", "36", "37", "38"}},
	}
	for _, c := range cases {
		if len(c.opts) != len(c.want) {
			t.Errorf("%s has %d options, want %d", c.name, len(c.opts), len(c.want))
			continue
		}
		for i, opt := range c.opts {
			if opt[0] != c.want[i] {
				t.Errorf("%s[%d] code = %q, want %q", c.name, i, opt[0], c.want[i])
			}
			v, err := strconv.ParseUint(opt[0], 16, 8)
			if err != nil {
				t.Errorf("%s[%d] code %q is not a 1-byte hex value", c.name, i, opt[0])
				continue
			}
			label, ok := c.decode([]byte{byte(v)})
			if !ok || label != opt[1] {
				t.Errorf("%s[%d] = %v, but decoding %s gives (%q, %v)", c.name, i, opt, opt[0], label, ok)
			}
		}
	}
}

func TestAirFlowLevelOptionsLabels(t *testing.T) {
	opts := AirFlowLevelOptions()
	if opts[0] != (EnumOption{"41", "自動"}) {
		t.Errorf("AirFlowLevelOptions()[0] = %v, want [41 自動]", opts[0])
	}
	if last := opts[len(opts)-1]; last != (EnumOption{"38", "風量8"}) {
		t.Errorf("AirFlowLevelOptions() last = %v, want [38 風量8]", last)
	}
}
