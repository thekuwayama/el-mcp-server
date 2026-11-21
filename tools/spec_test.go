package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/echonet/spec"
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

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func mustFindClass(t *testing.T, eoj string) spec.DeviceClass {
	t.Helper()
	cls, ok := findClass(eoj)
	if !ok {
		t.Fatalf("findClass(%q) not found", eoj)
	}
	return cls
}

func classEPC(t *testing.T, cls spec.DeviceClass, code byte) spec.EPCDef {
	t.Helper()
	for _, e := range cls.EPCs {
		if e.Code == code {
			return e
		}
	}
	t.Fatalf("EOJ %s does not define EPC %02X", cls.EOJHex(), code)
	return spec.EPCDef{}
}

// redefinedEPCs are super class EPCs that the MRA redefines per device class.
var redefinedEPCs = []struct {
	eoj  string
	epcs []byte
}{
	{"0130", []byte{0x80, 0x8F}},
	{"027D", []byte{0x83, 0x89, 0x8C}},
	{"0279", []byte{0x83, 0x89, 0x8C}},
}

func TestGetEPCDetailPrefersClassDefinition(t *testing.T) {
	for _, rc := range redefinedEPCs {
		cls := mustFindClass(t, rc.eoj)
		for _, code := range rc.epcs {
			t.Run(fmt.Sprintf("%s/%02X", rc.eoj, code), func(t *testing.T) {
				res, _, err := getEPCDetail(context.Background(), nil, &getEPCDetailParams{EOJ: rc.eoj, EPC: fmt.Sprintf("%02X", code)})
				if err != nil {
					t.Fatal(err)
				}
				if res.IsError {
					t.Fatalf("IsError: %s", resultText(t, res))
				}
				var got epcDetail
				if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
					t.Fatal(err)
				}
				if want := toEPCDetail(classEPC(t, cls, code)); got != want {
					t.Errorf("got %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestGetEPCDetailAirconPowerSaving(t *testing.T) {
	// 0x8F of 0x0130 is announced on change, unlike the super class one.
	res, _, err := getEPCDetail(context.Background(), nil, &getEPCDetailParams{EOJ: "0130", EPC: "8F"})
	if err != nil {
		t.Fatal(err)
	}
	var got epcDetail
	if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if got.AccessRules != "Anno/Set/Get" {
		t.Errorf("AccessRules = %q, want %q", got.AccessRules, "Anno/Set/Get")
	}
}

func TestGetEPCDetailFallsBackToSuperClass(t *testing.T) {
	// 0x8A (maker code) is defined only in the super class for 0x0130.
	res, _, err := getEPCDetail(context.Background(), nil, &getEPCDetailParams{EOJ: "0130", EPC: "8A"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("IsError: %s", resultText(t, res))
	}
	var got epcDetail
	if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if got.EPC != "8A" {
		t.Errorf("EPC = %q, want %q", got.EPC, "8A")
	}
}

func TestGetEPCDetailUnknownEOJ(t *testing.T) {
	// A super class EPC must not succeed for an EOJ that does not exist.
	res, _, err := getEPCDetail(context.Background(), nil, &getEPCDetailParams{EOJ: "FFFF", EPC: "80"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("IsError = false, want true: %s", resultText(t, res))
	}
}

func TestListEPCExcludesRedefinedSuperClassEPCs(t *testing.T) {
	for _, rc := range redefinedEPCs {
		t.Run(rc.eoj, func(t *testing.T) {
			res, _, err := listEPC(context.Background(), nil, &listEPCParams{EOJ: rc.eoj})
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				SuperClass []epcSummary `json:"super_class_epcs"`
				ClassEPCs  []epcSummary `json:"class_epcs"`
			}
			if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, e := range got.ClassEPCs {
				seen[e.EPC] = true
			}
			for _, e := range got.SuperClass {
				if seen[e.EPC] {
					t.Errorf("EPC %s is listed in both super_class_epcs and class_epcs", e.EPC)
				}
				seen[e.EPC] = true
			}
			for _, code := range rc.epcs {
				if !seen[fmt.Sprintf("%02X", code)] {
					t.Errorf("EPC %02X is missing", code)
				}
			}
			if want := len(got.ClassEPCs) + len(got.SuperClass); want != len(seen) {
				t.Errorf("listed %d EPCs, want %d unique", want, len(seen))
			}
		})
	}
}

func TestSearchDeviceClassEPCCountHasNoDuplicates(t *testing.T) {
	for _, rc := range redefinedEPCs {
		t.Run(rc.eoj, func(t *testing.T) {
			cls := mustFindClass(t, rc.eoj)
			res, _, err := searchDeviceClass(context.Background(), nil, &searchDeviceClassParams{Query: rc.eoj})
			if err != nil {
				t.Fatal(err)
			}
			var got []deviceClassResult
			if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
				t.Fatal(err)
			}
			codes := map[byte]bool{}
			for _, e := range spec.SuperClassEPCs {
				codes[e.Code] = true
			}
			for _, e := range cls.EPCs {
				codes[e.Code] = true
			}
			for _, r := range got {
				if r.EOJ != rc.eoj {
					continue
				}
				if r.EPCCount != len(codes) {
					t.Errorf("EPCCount = %d, want %d", r.EPCCount, len(codes))
				}
				return
			}
			t.Fatalf("EOJ %s not found in results", rc.eoj)
		})
	}
}

func TestSearchDeviceClassTrimsQuery(t *testing.T) {
	cases := []struct {
		query string
		want  string // EOJ expected in the results
	}{
		{"エアコン", "0130"},
		{" エアコン", "0130"},
		{"エアコン\t", "0130"},
		{"　蓄電池　", "027D"}, // full-width spaces
		{" air conditioner ", "0130"},
		{" 0130 ", "0130"},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			res, _, err := searchDeviceClass(context.Background(), nil, &searchDeviceClassParams{Query: c.query})
			if err != nil {
				t.Fatal(err)
			}
			var got []deviceClassResult
			if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
				t.Fatalf("no match: %s", resultText(t, res))
			}
			for _, r := range got {
				if r.EOJ == c.want {
					return
				}
			}
			t.Errorf("EOJ %s not found in %+v", c.want, got)
		})
	}
}

func TestFormatEOJ(t *testing.T) {
	// get_property / set_property echo the EOJ back in the same 6-digit
	// uppercase form as discover_devices and render_*, whatever the input.
	cases := []struct {
		in   string
		want string
	}{
		{"013001", "013001"},
		{"0130", "013001"},
		{"027d01", "027D01"},
		{"0ef001", "0EF001"},
	}
	for _, c := range cases {
		eoj, errRes := parseEOJParam(c.in)
		if errRes != nil {
			t.Fatalf("parseEOJParam(%q) returned an error result", c.in)
		}
		if got := formatEOJ(eoj); got != c.want {
			t.Errorf("formatEOJ(parseEOJParam(%q)) = %q, want %q", c.in, got, c.want)
		}
	}
}
