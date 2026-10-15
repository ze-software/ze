// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package role

import (
	"slices"
	"sync"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// resetRoleState clears the package filter state before and after a test.
func resetRoleState(t *testing.T) {
	t.Helper()
	clearState := func() {
		filterMu.Lock()
		filterPeerConfigs, filterNameToIP, filterRemoteRoles = nil, nil, nil
		filterMu.Unlock()
	}
	clearState()
	t.Cleanup(clearState)
}

// roleTxSections is a bgp root with peer edge (192.0.2.1, role customer,
// the given strict and export) and, when withOther, peer other (192.0.2.2,
// role peer); dynamic group ix (role provider) when withGroup.
func roleTxSections(strict, export string, withOther, withGroup bool) []sdk.ConfigSection {
	body := `"peer":{"edge":{"connection":{"remote":{"ip":"192.0.2.1"}},"role":{"import":"customer","strict":` + strict + `,"export":` + export + `}}`
	if withOther {
		body += `,"other":{"connection":{"remote":{"ip":"192.0.2.2"}},"role":{"import":"peer"}}`
	}
	body += `}`
	if withGroup {
		body += `,"group":{"ix":{"connection":{"remote":{"ip":"dynamic","range":"198.51.100.0/24"}},"role":{"import":"provider"}}}`
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{` + body + `}}`}}
}

// TestRoleExportReloadTakesEffect proves AC-1 and AC-7 for bgp-role: a
// reload that edits a peer's strict and export installs both at apply, and
// the rollback installs the previous ones back.
// Method: getFilterConfig, which the Section 5 filters read for every UPDATE,
// is read after configure, apply and rollback.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1, AC-4, AC-7 for bgp-role.
// PREVENTS: a commit that reports success while the egress filter keeps the
// startup export set.
func TestRoleExportReloadTakesEffect(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, false, false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	before, _ := getFilterConfig("192.0.2.1", "", "")
	if before == nil || before.strict {
		t.Fatalf("startup config not installed: %+v", before)
	}

	if err := tx.tx.Verify(roleTxSections("true", `["default","unknown"]`, false, false)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	after, _ := getFilterConfig("192.0.2.1", "", "")
	if !after.strict || !slices.Contains(after.export, "unknown") {
		t.Fatalf("the reload did not install strict and export: %+v", after)
	}

	if err := tx.tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	restored, _ := getFilterConfig("192.0.2.1", "", "")
	if restored.strict || slices.Contains(restored.export, "unknown") {
		t.Fatalf("the rollback did not restore the previous config: %+v", restored)
	}

	if err := tx.tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":`}}); err == nil {
		t.Fatal("a bgp section that is not JSON was accepted")
	}
}

// TestRoleRollbackKeepsLearnedRoles proves AC-17 and R-14: an apply that
// removes a peer prunes nothing, so the rollback of that transaction leaves
// the role the peer's OPEN declared in place, and its config back.
// Method: learn a role for peer other, apply a candidate without it, read
// the learned role; roll back and read it again.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-17, R-14, A-8 for bgp-role.
// PREVENTS: a refused commit dropping a learned role a live session never re-sends.
// MUTATION: call pruneLearnedRoles from installRoleConfig and the apply check fails.
func TestRoleRollbackKeepsLearnedRoles(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, true, false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("192.0.2.2", "", rolePeer)

	if err := tx.tx.Verify(roleTxSections("false", `["default"]`, false, false)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, learned := getFilterConfig("192.0.2.2", "", ""); learned != rolePeer {
		t.Fatalf("the apply pruned the learned role: %q", learned)
	}
	if err := tx.tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	cfg, learned := getFilterConfig("192.0.2.2", "", "")
	if learned != rolePeer {
		t.Fatalf("the learned role did not survive the rollback: %q", learned)
	}
	if cfg == nil {
		t.Fatal("the rollback did not restore the removed peer's config")
	}
}

// TestRoleCommitPrunesLearnedRoles proves AC-17's other half: once the
// removal commits, the removed peer's learned role is gone, while a dynamic
// member keyed by its name whose group stays configured keeps its role.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-17, R-14 for bgp-role.
// PREVENTS: a learned role outliving the peer the config removed, or the
// prune dropping a dynamic member's role (remoteRoleLocked name fallback).
func TestRoleCommitPrunesLearnedRoles(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, true, true)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("192.0.2.2", "", rolePeer)
	setFilterRemoteRole("dyn-198.51.100.7", "ix", roleCustomer)

	if err := tx.tx.Verify(roleTxSections("false", `["default"]`, false, true)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.committed("tx-1"); err != nil {
		t.Fatalf("committed: %v", err)
	}
	if _, learned := getFilterConfig("192.0.2.2", "", ""); learned != "" {
		t.Fatalf("the committed removal kept the learned role %q", learned)
	}
	if _, learned := getFilterConfig("", "dyn-198.51.100.7", "ix"); learned != roleCustomer {
		t.Fatalf("the dynamic member whose group stays lost its role: %q", learned)
	}
}

// TestRoleSnapshotSwapRace proves R-4: OnValidateOpen and the filters read
// the installed config while a reload installs a new one, with no data race.
// Method: run under -race, with OPEN validations, filter reads and apply or
// rollback swaps in parallel.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-4 for bgp-role.
// PREVENTS: OnValidateOpen reading closure variables a reload reassigns.
func TestRoleSnapshotSwapRace(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, true, false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			installed := installedRoleConfig()
			applyValidateOpen(installed.peers, installed.nameToIP, &sdk.ValidateOpenInput{Peer: "192.0.2.1"})
			getFilterConfig("192.0.2.2", "", "")
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			strict := "false"
			if i%2 == 0 {
				strict = "true"
			}
			if err := tx.tx.Verify(roleTxSections(strict, `["default"]`, i%3 == 0, false)); err != nil {
				t.Errorf("verify: %v", err)
				return
			}
			if err := tx.tx.Apply(); err != nil {
				t.Errorf("apply: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}
