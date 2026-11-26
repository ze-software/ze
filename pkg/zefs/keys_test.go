package zefs

import (
	"strings"
	"testing"
)

func TestFleetHistoryKeysRegistered(t *testing.T) {
	// VALIDATES: All three fleet history key families are registered as Private
	// with their explicit ValueMax bounds, match concrete device/shard paths via
	// Lookup, and agree with the exported trailing-slash ListKeys prefixes.
	// PREVENTS: Unregistered keys, missing ValueMax bounds, or missing trailing
	// slashes breaking storage.Storage.ListKeys enumeration.
	cases := []struct {
		name     string
		entry    KeyEntry
		prefix   string
		valueMax int
		concrete string
		params   []string
	}{
		{
			name:     "latest",
			entry:    KeyFleetHistoryLatest,
			prefix:   FleetHistoryLatestPrefix,
			valueMax: FleetHistoryLatestValueMax,
			concrete: "meta/fleet/history/latest/edge-01",
			params:   []string{"edge-01"},
		},
		{
			name:     "shard",
			entry:    KeyFleetHistoryShard,
			prefix:   FleetHistoryShardPrefix,
			valueMax: FleetHistoryShardValueMax,
			concrete: "meta/fleet/history/shard/edge-01/fine-20261126",
			params:   []string{"edge-01", "fine-20261126"},
		},
		{
			name:     "delete-intent",
			entry:    KeyFleetHistoryDeleteIntent,
			prefix:   FleetHistoryDeleteIntentPrefix,
			valueMax: FleetHistoryDeleteIntentValueMax,
			concrete: "meta/fleet/history/delete-intent/edge-01",
			params:   []string{"edge-01"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.entry.Private {
				t.Errorf("%s must be Private", tc.entry.Pattern)
			}
			if tc.entry.ValueMax != tc.valueMax {
				t.Errorf("%s ValueMax = %d, want %d", tc.entry.Pattern, tc.entry.ValueMax, tc.valueMax)
			}
			if !strings.HasSuffix(tc.prefix, "/") {
				t.Errorf("%s prefix %q must end with '/'", tc.name, tc.prefix)
			}
			if gotPrefix := tc.entry.Prefix(); gotPrefix != tc.prefix {
				t.Errorf("%s Prefix() = %q, want %q", tc.name, gotPrefix, tc.prefix)
			}
			if gotKey := tc.entry.Key(tc.params...); gotKey != tc.concrete {
				t.Errorf("%s Key() = %q, want %q", tc.name, gotKey, tc.concrete)
			}
			looked, ok := Lookup(tc.concrete)
			if !ok {
				t.Fatalf("Lookup(%q) returned false", tc.concrete)
			}
			if looked.Pattern != tc.entry.Pattern {
				t.Errorf("Lookup(%q).Pattern = %q, want %q", tc.concrete, looked.Pattern, tc.entry.Pattern)
			}
			if looked.ValueMax != tc.valueMax {
				t.Errorf("Lookup(%q).ValueMax = %d, want %d", tc.concrete, looked.ValueMax, tc.valueMax)
			}
		})
	}
}
