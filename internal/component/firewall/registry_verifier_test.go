// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- every registered owner reaches the backend capability gate.

package firewall

import (
	"errors"
	"reflect"
	"testing"
)

// TestApplyAllVerifiesPluginTablesBeforeProgramming drives the real registry
// with an accepting initial table and an unsupported predicate from another
// owner. The registered backend verifier must reject the merged desired state
// before Apply, preserve the applied snapshot, and permit a later withdrawal.
func TestApplyAllVerifiesPluginTablesBeforeProgramming(t *testing.T) {
	for _, preloaded := range []bool{false, true} {
		name := "autoload"
		if preloaded {
			name = "preloaded"
		}
		t.Run(name, func(t *testing.T) {
			const backendName = "registry-verifier"
			b := snapshotBackend(t, backendName)
			applies := 0
			b.onApply = func([]Table) { applies++ }
			refused := errors.New("backend cannot represent link-layer unicast")
			verifications := 0
			if err := RegisterVerifier(backendName, func(tables []Table) error {
				verifications++
				for _, table := range tables {
					for _, chain := range table.Chains {
						for _, term := range chain.Terms {
							for _, match := range term.Matches {
								if _, ok := match.(MatchUnicast); ok {
									return refused
								}
							}
						}
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if preloaded {
				if err := LoadBackend(backendName); err != nil {
					t.Fatal(err)
				}
			}
			good := makeTable(FamilyInet)
			if err := RegisterTables("existing-owner", []Table{good}); err != nil {
				t.Fatal(err)
			}
			if err := ApplyAll(); err != nil {
				t.Fatal(err)
			}
			before := LastApplied()
			bad := makeTable(FamilyInet)
			bad.Chains[0].Name = "backup"
			bad.Chains[0].Terms[0].Matches = []Match{MatchUnicast{}}
			if err := RegisterTables("vrrp-backup", []Table{bad}); err != nil {
				t.Fatal(err)
			}
			if err := ApplyAll(); !errors.Is(err, refused) {
				t.Fatalf("plugin-only reconcile = %v, want verifier refusal", err)
			}
			if applies != 1 {
				t.Fatalf("backend Apply called %d times, want only the initial accepted apply", applies)
			}
			if !reflect.DeepEqual(LastApplied(), before) {
				t.Fatal("rejected reconcile changed the applied snapshot")
			}
			if err := RegisterTables("vrrp-backup", nil); err != nil {
				t.Fatal(err)
			}
			if err := ApplyAll(); err != nil {
				t.Fatalf("withdraw unsupported predicate: %v", err)
			}
			if applies != 2 {
				t.Fatalf("accepted apply count = %d, want 2", applies)
			}
			if verifications != 3 {
				t.Fatalf("verified %d reconciles, want all 3", verifications)
			}
		})
	}
}
