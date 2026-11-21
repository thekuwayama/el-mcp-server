package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
