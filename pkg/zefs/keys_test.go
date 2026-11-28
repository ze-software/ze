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

func TestFleetAuditKeysRegistered(t *testing.T) {
	// VALIDATES: All four fleet audit and device receipt key families are
	// registered as Private with their explicit ValueMax bounds, match concrete
	// paths via Lookup, and are classified as protected by
	// IsFleetAuditProtectedKey while unrelated runtime keys remain unprotected.
	// PREVENTS: Missing key registrations, wrong ValueMax caps, or live data
	// commands bypassing protection for newly created keys under
	// meta/fleet/audit/.
	cases := []struct {
		name     string
		entry    KeyEntry
		valueMax int
		concrete string
		params   []string
	}{
		{
			name:     "manifest",
			entry:    KeyFleetAuditManifest,
			valueMax: FleetAuditManifestValueMax,
			concrete: "meta/fleet/audit/manifest",
		},
		{
			name:     "page",
			entry:    KeyFleetAuditPage,
			valueMax: FleetAuditPageValueMax,
			concrete: "meta/fleet/audit/page/0000000000000001",
			params:   []string{"0000000000000001"},
		},
		{
			name:     "managed-receipts",
			entry:    KeyManagedAuditReceipts,
			valueMax: ManagedAuditReceiptsValueMax,
			concrete: "meta/managed/audit-receipts",
		},
		{
			name:     "managed-intent",
			entry:    KeyManagedAuditIntent,
			valueMax: ManagedAuditIntentValueMax,
			concrete: "meta/managed/audit-intent",
		},
	}

	if !strings.HasSuffix(FleetAuditPagePrefix, "/") {
		t.Fatalf("FleetAuditPagePrefix %q must end with '/'", FleetAuditPagePrefix)
	}
	if !strings.HasSuffix(FleetAuditPrefix, "/") {
		t.Fatalf("FleetAuditPrefix %q must end with '/'", FleetAuditPrefix)
	}
	if got := KeyFleetAuditPage.Prefix(); got != FleetAuditPagePrefix {
		t.Fatalf("KeyFleetAuditPage.Prefix() = %q, want %q", got, FleetAuditPagePrefix)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.entry.Private {
				t.Errorf("%s must be Private", tc.entry.Pattern)
			}
			if tc.entry.ValueMax != tc.valueMax {
				t.Errorf("%s ValueMax = %d, want %d", tc.entry.Pattern, tc.entry.ValueMax, tc.valueMax)
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
			if !IsFleetAuditProtectedKey(tc.concrete) {
				t.Errorf("IsFleetAuditProtectedKey(%q) = false, want true", tc.concrete)
			}
		})
	}

	for _, key := range []string{
		"meta/fleet/audit/page/9999999999999999",
		"meta/fleet/audit/cursor/edge-01",
		"meta/fleet/audit/arbitrary",
	} {
		if !IsFleetAuditProtectedKey(key) {
			t.Errorf("IsFleetAuditProtectedKey(%q) = false, want true", key)
		}
	}

	for _, key := range []string{
		"meta/fleet/device/edge-01/record",
		"meta/fleet/alarm/hub",
		"meta/managed/base-version",
		"meta/managed/chain-state",
		"meta/test/state",
	} {
		if IsFleetAuditProtectedKey(key) {
			t.Errorf("IsFleetAuditProtectedKey(%q) = true, want false", key)
		}
	}
}
