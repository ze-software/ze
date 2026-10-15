// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package role

import (
	"reflect"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-role: a reload that
// edits bgp outside the role leaves installs nothing, so the filters and
// OnValidateOpen keep reading the very maps they read before, and the
// rollback of that transaction installs nothing either. Committed, it prunes no
// learned role.
// Method: the installed map identity is compared across a verify, apply and
// rollback, then a committed reload, whose candidate differs from the running config only in another
// peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-role.
// PREVENTS: every bgp peer edit rebuilding the role state.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the apply check fails.
// Not parallel: the filter state is package state.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	resetRoleState(t)
	sections := func(description string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{` +
			`"edge":{"connection":{"remote":{"ip":"192.0.2.1"}},"role":{"import":"customer"}},` +
			`"other":{"connection":{"remote":{"ip":"192.0.2.99"}},"description":"` + description + `"}}}}`}}
	}
	tx := newRoleConfigTx()
	if err := tx.configure(sections("first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := reflect.ValueOf(installedRoleConfig().peers).Pointer()
	if installed == 0 {
		t.Fatal("configure installed nothing")
	}

	if err := tx.tx.Verify(sections("second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if reflect.ValueOf(installedRoleConfig().peers).Pointer() != installed {
		t.Fatal("an edit outside the role leaves installed a new config")
	}
	if err := tx.tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if reflect.ValueOf(installedRoleConfig().peers).Pointer() != installed {
		t.Fatal("the rollback of an edit outside the role leaves installed a config")
	}

	// The same edit committed: the prune at config-committed runs, and peer
	// other, which has no role block, keeps the role its OPEN declared.
	setFilterRemoteRole("other", "", roleCustomer)
	commitRoleReload(t, tx, sections("third"))
	if reflect.ValueOf(installedRoleConfig().peers).Pointer() != installed {
		t.Fatal("a committed edit outside the role leaves installed a new config")
	}
	if _, learned := getFilterConfig("192.0.2.99", "other", ""); learned != roleCustomer {
		t.Fatalf("a committed edit outside the role dropped a learned role: %q", learned)
	}
}
