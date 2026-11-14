package tools

import (
	"testing"
	"time"
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
