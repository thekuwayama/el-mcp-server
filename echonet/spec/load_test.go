package spec

import (
	"encoding/json"
	"testing"
)

func TestResolveRef(t *testing.T) {
	plain := mraData{Type: "state", Size: 1}
	if got := resolveRef(plain); got.Type != "state" || got.Size != 1 {
		t.Errorf("resolveRef(no $ref) = %+v, want it unchanged", got)
	}

	got := resolveRef(mraData{Ref: "#/definitions/number_0-59minutes"})
	if got.Ref != "" || got.Type != "number" || got.Format != "uint8" || got.Unit != "minutes" {
		t.Errorf("resolveRef(number_0-59minutes) = %+v", got)
	}
}

func TestResolveRefUnknownDefinition(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("resolveRef did not panic for an unknown definition")
		}
	}()
	resolveRef(mraData{Ref: "#/definitions/no_such_definition"})
}

func TestOneOfWithRef(t *testing.T) {
	raw := json.RawMessage(`{"oneOf":[{"$ref":"#/definitions/number_0-59minutes"},{"type":"state","size":1}]}`)
	d := resolveData(raw)
	if got, want := dataTypeString(d), "number (uint8) | state (1 byte)"; got != want {
		t.Errorf("dataTypeString = %q, want %q", got, want)
	}
	if got, want := unitString(d), "minutes"; got != want {
		t.Errorf("unitString = %q, want %q", got, want)
	}
}

func findTestEPC(t *testing.T, eoj string, code byte) EPCDef {
	t.Helper()
	for _, c := range Classes {
		if c.EOJHex() != eoj {
			continue
		}
		for _, e := range c.EPCs {
			if e.Code == code {
				return e
			}
		}
		t.Fatalf("EOJ %s does not define EPC %02X", eoj, code)
	}
	t.Fatalf("EOJ %s not found", eoj)
	return EPCDef{}
}

// TestLoadedMRA checks values loaded from the embedded MRA, including
// properties whose data is a oneOf with $ref.
func TestLoadedMRA(t *testing.T) {
	if len(SuperClassEPCs) == 0 {
		t.Fatal("SuperClassEPCs is empty")
	}

	var aircon *DeviceClass
	for i := range Classes {
		if Classes[i].EOJHex() == "0130" {
			aircon = &Classes[i]
		}
	}
	if aircon == nil {
		t.Fatal("EOJ 0130 not found")
	}
	if aircon.NameJP != "家庭用エアコン" || aircon.NameEN != "Home air conditioner" {
		t.Errorf("0130 names = %q / %q", aircon.NameJP, aircon.NameEN)
	}

	cases := []struct {
		eoj      string
		code     byte
		dataType string
		unit     string
		access   string
	}{
		// oneOf: [$ref level_31-8, state]
		{"0130", 0xA0, "level | state (1 byte)", "", "Anno/Set/Get"},
		// oneOf: the unit comes from one of the alternatives
		{"0130", 0xB3, "number (uint8) | state (1 byte)", "℃", "Set/Get"},
		{"0288", 0xE7, "number (int32) | state (4 byte)", "W", "Get"},
	}
	for _, c := range cases {
		e := findTestEPC(t, c.eoj, c.code)
		if e.DataType != c.dataType || e.Unit != c.unit || e.AccessRules != c.access {
			t.Errorf("%s/%02X = {DataType: %q, Unit: %q, AccessRules: %q}, want {%q, %q, %q}",
				c.eoj, c.code, e.DataType, e.Unit, e.AccessRules, c.dataType, c.unit, c.access)
		}
	}
}
