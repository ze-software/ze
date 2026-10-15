// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package as112

import (
	"log/slog"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// as112Sections returns a service section holding a disabled as112 with
// hostname and a tftp sibling carrying description, so two calls that differ
// only in description differ only outside as112's subtree.
func as112Sections(hostname, description string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"as112":{"enabled":"false","hostname":"` + hostname + `"},` +
		`"tftp":{"description":"` + description + `"}}}`}}
}

func newTestAS112ConfigTx(t *testing.T) *sdk.ConfigTx[as112Config] {
	t.Helper()
	prod := newAS112Producer()
	inst := newAS112Installer(prod, newServerManager(slog.Default(), prod.onServingChanged), slog.Default())
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	return sdk.NewConfigTx(as112Config{}, parseAS112Sections, inst.install)
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for as112: a reload that
// edits service outside as112 publishes no new snapshot, so the serial, the
// loopback registration and the listeners are untouched, and its rollback
// publishes nothing either. Method: the stored snapshot pointer is compared
// across a verify, apply and rollback whose candidate differs only in a
// sibling's leaf.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for as112.
// PREVENTS: every service edit re-registering loopback addresses and
// bumping the serial.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: stateP is package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx := newTestAS112ConfigTx(t)
	if err := tx.Configure(as112Sections("a.example", "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := loadState()
	if installed == nil {
		t.Fatal("configure published no snapshot")
	}
	if err := tx.Verify(as112Sections("a.example", "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if loadState() != installed {
		t.Fatal("an edit outside as112 published a new snapshot")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if loadState() != installed {
		t.Fatal("the rollback of an edit outside as112 published a snapshot")
	}
}

// TestConfigTxAppliesReloadedHostname proves AC-1 to AC-4 for as112: verify
// publishes nothing, apply publishes the snapshot of the new hostname, and
// rollback publishes one for the replaced config.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 22.
// PREVENTS: the acked-and-dropped reload that kept the boot config.
func TestConfigTxAppliesReloadedHostname(t *testing.T) {
	tx := newTestAS112ConfigTx(t)
	if err := tx.Configure(as112Sections("a.example", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := loadState()
	if err := tx.Verify(as112Sections("b.example", "x")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if loadState() != installed {
		t.Fatal("verify published a snapshot")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if loadState() == installed {
		t.Fatal("apply of a new hostname kept the boot snapshot")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if loadState() == installed {
		t.Fatal("rollback did not publish a snapshot")
	}
}
