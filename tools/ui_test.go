package tools

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/thekuwayama/el-mcp-server/tools/ui"
)

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
