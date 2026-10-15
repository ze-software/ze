// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package rib

import (
	"strings"
	"testing"
)

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-rib: a reload that
// edits bgp outside the leaves the rib reads installs nothing, so the
// selection rules keep the very state they held, no reselect walk runs, and
// the rollback of that transaction installs nothing either.
// Method: the stored blackhole pointer and the multipath values are compared
// across a verify, apply and rollback whose candidate differs from the running
// config only in another peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-rib.
// PREVENTS: every bgp peer edit re-running the rib's install and its walk
// over every stored route.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	r := newRIBManager(nil)
	tx := r.newConfigTx(false)
	running := `"multipath":{"maximum-paths":2,"relax-as-path":true},"peer":{"other":{"connection":{"remote":{"ip":"192.0.2.99"}},"description":"%s"}}`
	if err := tx.Configure(ribSections(strings.Replace(running, "%s", "first", 1))); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := r.blackholeCfg.Load()
	if installed == nil {
		t.Fatal("configure installed nothing")
	}

	if err := tx.Verify(ribSections(strings.Replace(running, "%s", "second", 1))); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if r.blackholeCfg.Load() != installed {
		t.Fatal("an edit outside the rib's leaves installed a new config")
	}
	if r.maximumPaths.Load() != 2 || !r.relaxASPath.Load() {
		t.Fatalf("multipath changed: maximum-paths %d relax %v", r.maximumPaths.Load(), r.relaxASPath.Load())
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if r.blackholeCfg.Load() != installed {
		t.Fatal("the rollback of an edit outside the rib's leaves installed a config")
	}
}
