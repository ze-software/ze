// Design: docs/architecture/core-design.md -- BGP policy final disposition.

package filterapi

import (
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
)

// TestAbsentExportDispositionByRouteKind proves the ordinary learned-route
// rule and the surviving local/explicit-role rule on both session types.
// Method: exhaust an absent policy and one carrying only automatic members;
// missing and invalid engine provenance must never acquire the explicit default.
func TestAbsentExportDispositionByRouteKind(t *testing.T) {
	for _, snapshot := range []Policy{{}, InternalPolicy([]policy.Ref{{Name: "automatic"}})} {
		for _, tc := range []struct {
			name     string
			kind     ExportKind
			external bool
			want     bool
		}{
			{"internal learned", ExportOrdinaryLearned, false, false},
			{"internal explicit", ExportExplicit, false, true},
			{"internal unknown", ExportUnknown, false, false},
			{"internal invalid", ExportKind(255), false, false},
			{"external learned", ExportOrdinaryLearned, true, false},
			{"external explicit", ExportExplicit, true, false},
			{"external unknown", ExportUnknown, true, false},
			{"external invalid", ExportKind(255), true, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := snapshot.AcceptsExportAtEnd(tc.external, tc.kind); got != tc.want {
					t.Fatalf("export accept=%v, want %v", got, tc.want)
				}
				if got := UnconfiguredExportAccepts(tc.external, tc.kind); got != tc.want {
					t.Fatalf("unconfigured export accept=%v, want %v", got, tc.want)
				}
			})
		}
	}
}

// TestConfiguredExportFinalRequiresKnownKind proves an explicit final applies
// to either admitted export kind without turning absent provenance into authority.
// Method: exhaust each configured final on internal and external sessions.
func TestConfiguredExportFinalRequiresKnownKind(t *testing.T) {
	for _, final := range []policy.Final{policy.FinalAccept, policy.FinalReject} {
		snapshot, err := NewPolicy(nil, final)
		if err != nil {
			t.Fatal(err)
		}
		for _, external := range []bool{false, true} {
			for _, kind := range []ExportKind{ExportOrdinaryLearned, ExportExplicit, ExportUnknown, ExportKind(255)} {
				known := kind == ExportOrdinaryLearned || kind == ExportExplicit
				want := known && final == policy.FinalAccept
				if got := snapshot.AcceptsExportAtEnd(external, kind); got != want {
					t.Errorf("final=%d external=%v kind=%d: accept=%v, want %v", final, external, kind, got, want)
				}
			}
		}
	}
}
