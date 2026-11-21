package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thekuwayama/el-mcp-server/echonet"
	"github.com/thekuwayama/el-mcp-server/tools/ui"
)

// TestLockSmartmeterHistorySerializesSameMeter checks that concurrent holders
// of the lock for the same meter never overlap, so the get/set/restore
// sequence on EPC 0xE5 in fetchSmartmeterHistory can't interleave.
func TestLockSmartmeterHistorySerializesSameMeter(t *testing.T) {
	const goroutines = 8
	var active, maxActive atomic.Int32
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := lockSmartmeterHistory("192.0.2.1", 0x028801)
			defer unlock()
			n := active.Add(1)
			for {
				m := maxActive.Load()
				if n <= m || maxActive.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
		}()
	}
	wg.Wait()
	if got := maxActive.Load(); got != 1 {
		t.Errorf("max concurrent holders = %d, want 1", got)
	}
}

// TestLockSmartmeterHistoryIndependentMeters checks that meters with a
// different IP or EOJ don't block each other.
func TestLockSmartmeterHistoryIndependentMeters(t *testing.T) {
	// On failure the lock is left held; the keys are unique to this test.
	unlock := lockSmartmeterHistory("192.0.2.2", 0x028801)

	done := make(chan struct{})
	go func() {
		defer close(done)
		lockSmartmeterHistory("192.0.2.3", 0x028801)()
		lockSmartmeterHistory("192.0.2.2", 0x028802)()
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lock for a different meter blocked on a held lock")
	}

	// The same meter must wait until the held lock is released.
	acquired := make(chan struct{})
	go func() {
		lockSmartmeterHistory("192.0.2.2", 0x028801)()
		close(acquired)
	}()
	select {
	case <-acquired:
		t.Fatal("lock for the same meter was acquired while held")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("lock for the same meter was not acquired after release")
	}
}

func TestDashboardEnumTablesInjected(t *testing.T) {
	cases := []struct {
		name   string
		html   string
		tables map[string][]ui.EnumOption
	}{
		{"battery", batteryDashboardHTML, map[string][]ui.EnumOption{"OPERATION_MODES": ui.OperationModeOptions()}},
		{"v2h", v2hDashboardHTML, map[string][]ui.EnumOption{"OPERATION_MODES": ui.V2HOperationModeOptions()}},
		{"aircon", airconDashboardHTML, map[string][]ui.EnumOption{
			"OPERATION_MODES": ui.AirconOperationModeOptions(),
			"FAN_LEVELS":      ui.AirFlowLevelOptions(),
		}},
	}
	for _, c := range cases {
		if strings.Contains(c.html, "/*@enum:") {
			t.Errorf("%s dashboard still has an enum placeholder", c.name)
		}
		for name, opts := range c.tables {
			b, err := json.Marshal(opts)
			if err != nil {
				t.Fatal(err)
			}
			want := "var " + name + " = " + string(b) + ";"
			if !strings.Contains(c.html, want) {
				t.Errorf("%s dashboard does not contain %s", c.name, want)
			}
		}
	}

	for name, html := range map[string]string{"solar": solarDashboardHTML, "smartmeter": smartmeterDashboardHTML} {
		if strings.Contains(html, "/*@enum:") {
			t.Errorf("%s dashboard has an enum placeholder but no table is injected", name)
		}
	}
}

func TestInjectEnumTables(t *testing.T) {
	got := injectEnumTables("var A = /*@enum:A*/[];", map[string][]ui.EnumOption{"A": {{"42", "充電"}}})
	if want := `var A = [["42","充電"]];`; got != want {
		t.Errorf("injectEnumTables = %q, want %q", got, want)
	}

	defer func() {
		if recover() == nil {
			t.Error("injectEnumTables with a missing placeholder should panic")
		}
	}()
	injectEnumTables("var A = [];", map[string][]ui.EnumOption{"A": nil})
}

func TestDecodeEnum(t *testing.T) {
	cases := []struct {
		edt       []byte
		wantLabel string
		wantCode  string
	}{
		{[]byte{0x42}, "充電", "42"},
		{[]byte{0x99}, "不明(0x99)", "99"},
		{[]byte{0x42, 0x00}, "", ""},
		{nil, "", ""},
	}
	for _, c := range cases {
		label, code := decodeEnum(c.edt, ui.DecodeOperationMode)
		if label != c.wantLabel || code != c.wantCode {
			t.Errorf("decodeEnum(%v) = (%q, %q), want (%q, %q)", c.edt, label, code, c.wantLabel, c.wantCode)
		}
	}
}

func TestParseHistoryDate(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	// Just after midnight, so a naive "now minus 24h" calculation would be off by one.
	now := time.Date(2026, 11, 15, 0, 5, 0, 0, jst)

	tests := []struct {
		date        string
		wantDaysAgo int
		wantErr     bool
	}{
		{date: "", wantDaysAgo: 0},
		{date: "2026-11-15", wantDaysAgo: 0},
		{date: "2026-11-14", wantDaysAgo: 1},
		{date: "2026-10-31", wantDaysAgo: 15},
		{date: "2026-08-08", wantDaysAgo: 99},
		{date: "2026-08-07", wantErr: true}, // 100 days ago: beyond EPC 0xE5's range
		{date: "2026-11-16", wantErr: true}, // future
		{date: "2026/11/14", wantErr: true}, // wrong layout
		{date: "2026-02-30", wantErr: true}, // not a real date
	}
	for _, tt := range tests {
		got, err := parseHistoryDate(tt.date, now)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseHistoryDate(%q) = %d, want error", tt.date, got)
			}
			continue
		}
		if err != nil || got != tt.wantDaysAgo {
			t.Errorf("parseHistoryDate(%q) = (%d, %v), want (%d, nil)", tt.date, got, err, tt.wantDaysAgo)
		}
	}
}

// Between 0:00 and 9:00 JST the UTC date is still the previous day; with now
// taken in jst, "today" follows the meter's JST date.
func TestParseHistoryDateInJST(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 5, 0, 0, time.UTC).In(jst) // 2026-11-15 00:05 JST
	if got, err := parseHistoryDate("2026-11-15", now); err != nil || got != 0 {
		t.Errorf("parseHistoryDate(2026-11-15) = (%d, %v), want (0, nil)", got, err)
	}
}

// A single-phase two-wire meter reports no data for the T phase, so the state
// must keep the R phase and omit the T phase instead of dropping both.
func TestSmartmeterUIStateSinglePhaseCurrents(t *testing.T) {
	currents, ok := ui.DecodeInstantaneousCurrents([]byte{0x00, 0x7B, 0x7F, 0xFE})
	if !ok {
		t.Fatal("DecodeInstantaneousCurrents failed for a single-phase two-wire reading")
	}
	state := smartmeterUIState{
		InstantaneousRPhaseA: currents.RPhaseA,
		InstantaneousTPhaseA: currents.TPhaseA,
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"instantaneous_r_phase_a":12.3`) {
		t.Errorf("state JSON %s lacks the R phase", got)
	}
	if strings.Contains(got, "instantaneous_t_phase_a") {
		t.Errorf("state JSON %s should omit the T phase", got)
	}
}

func TestDashboardHTMLInjectsCommonJS(t *testing.T) {
	for name, html := range dashboardHTMLs() {
		if strings.Contains(html, commonJSPlaceholder) {
			t.Errorf("%s: placeholder %q remains", name, commonJSPlaceholder)
		}
		if strings.Count(html, dashboardCommonJS) != 1 {
			t.Errorf("%s: common script is not inlined exactly once", name)
		}
		scripts := extractScripts(html)
		if len(scripts) != 2 {
			t.Fatalf("%s: got %d <script> elements, want 2 (common + device)", name, len(scripts))
		}
		// The device script calls ElMcpApp.start, so the common script must
		// come first.
		if !strings.Contains(scripts[0], "var ElMcpApp =") {
			t.Errorf("%s: first <script> is not the common script", name)
		}
		if !strings.Contains(scripts[1], "ElMcpApp.start(") {
			t.Errorf("%s: device <script> does not call ElMcpApp.start", name)
		}
	}
}

func TestInjectCommonJS(t *testing.T) {
	const js = "var x = 1;\n"
	got, err := injectCommonJS("<body>"+commonJSPlaceholder+"</body>", js)
	if err != nil {
		t.Fatalf("injectCommonJS: %v", err)
	}
	if want := "<body><script>\nvar x = 1;\n</script></body>"; got != want {
		t.Errorf("injectCommonJS = %q, want %q", got, want)
	}

	errCases := []struct {
		name, tmpl, js string
	}{
		{name: "no placeholder", tmpl: "<body></body>", js: js},
		{name: "duplicate placeholder", tmpl: commonJSPlaceholder + commonJSPlaceholder, js: js},
		{name: "closing script tag in js", tmpl: commonJSPlaceholder, js: "var s = '</SCRIPT>';"},
	}
	for _, tt := range errCases {
		if _, err := injectCommonJS(tt.tmpl, tt.js); err == nil {
			t.Errorf("%s: injectCommonJS succeeded, want error", tt.name)
		}
	}
}

// TestDashboardScriptsSyntax runs `node --check` on every inline script of the
// served dashboards. It is skipped when node is not installed.
func TestDashboardScriptsSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH")
	}
	dir := t.TempDir()
	for name, html := range dashboardHTMLs() {
		for i, script := range extractScripts(html) {
			path := filepath.Join(dir, fmt.Sprintf("%s-%d.js", name, i))
			if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(node, "--check", path).CombinedOutput(); err != nil {
				t.Errorf("%s script #%d: node --check failed: %v\n%s", name, i, err, out)
			}
		}
	}
}

func dashboardHTMLs() map[string]string {
	return map[string]string{
		"battery":    batteryDashboardHTML,
		"solar":      solarDashboardHTML,
		"v2h":        v2hDashboardHTML,
		"aircon":     airconDashboardHTML,
		"smartmeter": smartmeterDashboardHTML,
	}
}

// extractScripts returns the bodies of the <script> elements in html.
func extractScripts(html string) []string {
	var scripts []string
	for {
		start := strings.Index(html, "<script>")
		if start < 0 {
			return scripts
		}
		html = html[start+len("<script>"):]
		end := strings.Index(html, "</script>")
		if end < 0 {
			return scripts
		}
		scripts = append(scripts, html[:end])
		html = html[end+len("</script>"):]
	}
}

func TestGetStringAndGetPtr(t *testing.T) {
	props := map[byte][]byte{
		ui.OperatingStatusEPC:          {0x30},
		ui.RemainingCapacityPercentEPC: {0xFF}, // out of range: decode fails
		ui.ChargeDischargePowerEPC:     {0x00, 0x00, 0x01, 0xF4},
	}

	if got := getString(props, ui.OperatingStatusEPC, ui.DecodeOperatingStatus); got != "稼働中" {
		t.Errorf("getString(0x80) = %q, want %q", got, "稼働中")
	}
	if got := getString(props, ui.OperationModeEPC, ui.DecodeOperationMode); got != "" {
		t.Errorf("getString(missing EPC) = %q, want empty", got)
	}
	if got := getPtr(props, ui.ChargeDischargePowerEPC, ui.DecodeSignedPowerW); got == nil || *got != 500 {
		t.Errorf("getPtr(0xD3) = %v, want 500", got)
	}
	if got := getPtr(props, ui.RemainingCapacityPercentEPC, ui.DecodePercent); got != nil {
		t.Errorf("getPtr(undecodable) = %v, want nil", *got)
	}

	// A failed Get yields a nil map; every field must then be left empty.
	if got := getString(nil, ui.OperatingStatusEPC, ui.DecodeOperatingStatus); got != "" {
		t.Errorf("getString(nil props) = %q, want empty", got)
	}
	if got := getPtr(nil, ui.ChargeDischargePowerEPC, ui.DecodeSignedPowerW); got != nil {
		t.Errorf("getPtr(nil props) = %v, want nil", *got)
	}
}

func TestGetEnum(t *testing.T) {
	props := map[byte][]byte{ui.OperationModeEPC: {0x42}}
	if label, code := getEnum(props, ui.OperationModeEPC, ui.DecodeOperationMode); label == "" || code != "42" {
		t.Errorf("getEnum(0x42) = (%q, %q), want a label and code %q", label, code, "42")
	}
	if label, code := getEnum(props, ui.AirFlowLevelEPC, ui.DecodeAirFlowLevel); label != "" || code != "" {
		t.Errorf("getEnum(missing EPC) = (%q, %q), want empty", label, code)
	}
	if label, code := getEnum(nil, ui.OperationModeEPC, ui.DecodeOperationMode); label != "" || code != "" {
		t.Errorf("getEnum(nil props) = (%q, %q), want empty", label, code)
	}
}

func TestEnergyLogFromGet(t *testing.T) {
	// edt builds an EPC 0xE2/0xE4 EDT for collection day d whose slots all
	// hold the reading 100.
	edt := func(d int) []byte {
		b := []byte{byte(d >> 8), byte(d)}
		for range ui.SlotsPerDay {
			b = append(b, 0x00, 0x00, 0x00, 0x64)
		}
		return b
	}
	getSNA := fmt.Errorf("device returned Get_SNA (EPC E4 %w)", echonet.ErrPropertyNotAvailable)

	tests := []struct {
		name    string
		edt     []byte
		err     error
		wantLog bool
		wantIs  error // the returned error must wrap this
		wantErr bool
	}{
		{name: "valid log", edt: edt(2), wantLog: true},
		{name: "Get_SNA is treated as not supported", err: getSNA},
		{name: "timeout is an error", err: os.ErrDeadlineExceeded, wantIs: os.ErrDeadlineExceeded, wantErr: true},
		{name: "canceled ctx is an error", err: context.Canceled, wantIs: context.Canceled, wantErr: true},
		{name: "unexpected ESV is an error", err: errors.New("unexpected ESV: 71"), wantErr: true},
		{name: "malformed EDT", edt: edt(2)[:10], wantErr: true},
		{name: "day mismatch", edt: edt(3), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := energyLogFromGet(0xE4, 2, tt.edt, tt.err)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("energyLogFromGet() = %v, want error", log)
				}
				if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
					t.Errorf("energyLogFromGet() error = %v, want it to wrap %v", err, tt.wantIs)
				}
				if log != nil {
					t.Errorf("energyLogFromGet() log = %v, want nil with an error", log)
				}
				return
			}
			if err != nil {
				t.Fatalf("energyLogFromGet() error = %v", err)
			}
			if (log != nil) != tt.wantLog {
				t.Fatalf("energyLogFromGet() log = %v, want non-nil %v", log, tt.wantLog)
			}
			if log != nil && (log.Day != 2 || !log.Valid[0] || log.Readings[0] != 100) {
				t.Errorf("energyLogFromGet() = day %d, slot 0 (%d %v), want day 2, slot 0 (100 true)", log.Day, log.Readings[0], log.Valid[0])
			}
		})
	}
}
