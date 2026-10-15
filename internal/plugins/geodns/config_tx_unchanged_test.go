// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package geodns

import (
	"log/slog"
	"reflect"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// geodnsSections returns a service section holding geodns with one zone and
// a tftp sibling carrying description, so two calls that differ only in
// description differ only outside geodns's subtree.
func geodnsSections(zone, description string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"geodns":{"enabled":"false","zone":["` + zone + `"]},` +
		`"tftp":{"description":"` + description + `"}}}`}}
}

func newTestGeoDNSConfigTx(t *testing.T) *sdk.ConfigTx[geodnsConfig] {
	t.Helper()
	inst := &geodnsInstaller{mgr: newServerManager(slog.Default()), log: slog.Default()}
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	return sdk.NewConfigTx(geodnsConfig{}, parseGeoDNSSections, inst.install)
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for geodns: a reload that
// edits service outside geodns publishes no new resolver snapshot, so the SOA
// serial and the listeners are untouched, and its rollback publishes nothing
// either. Method: the stored snapshot pointer is compared across a verify,
// apply and rollback whose candidate differs only in a sibling's leaf.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for geodns.
// PREVENTS: every service edit bumping the SOA serial and counting a reload.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: stateP is package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx := newTestGeoDNSConfigTx(t)
	if err := tx.Configure(geodnsSections("a.example.", "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := loadState()
	if installed == nil {
		t.Fatal("configure published no snapshot")
	}
	if err := tx.Verify(geodnsSections("a.example.", "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if loadState() != installed {
		t.Fatal("an edit outside geodns published a new snapshot")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if loadState() != installed {
		t.Fatal("the rollback of an edit outside geodns published a snapshot")
	}
}

// TestConfigTxAppliesReloadedZones proves AC-1 to AC-4 for geodns: verify
// publishes nothing, apply publishes the snapshot of the new zone set, and
// rollback publishes the replaced one.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 23.
// PREVENTS: the acked-and-dropped reload that kept the boot zones.
func TestConfigTxAppliesReloadedZones(t *testing.T) {
	tx := newTestGeoDNSConfigTx(t)
	if err := tx.Configure(geodnsSections("a.example.", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := loadState()
	if err := tx.Verify(geodnsSections("b.example.", "x")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if loadState() != installed {
		t.Fatal("verify published a snapshot")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if loadState() == installed {
		t.Fatal("apply of a new zone kept the boot snapshot")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if loadState() == installed {
		t.Fatal("rollback did not publish a snapshot")
	}
	if !reflect.DeepEqual(loadState().names, installed.names) {
		t.Fatalf("rollback names = %v, want the boot names %v", loadState().names, installed.names)
	}
}
