// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package role

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// setFilterState installs role configs and a name-to-address map the way a
// Stage 2 configure does, then prunes the learned roles.
//
// The configured-peer set is built by configuredPeerKeys, the function the
// production parse uses, over a bgp subtree derived from the arguments: a peer
// for each name-to-address entry, a peer named by its own address for each
// other address key, and a dynamic group for each group selector. A test
// therefore exercises the key set production installs, and setFilterState(nil,
// nil) installs an empty one, whose prune clears every learned role.
func setFilterState(configs map[string]*peerRoleConfig, n2ip map[string]string) {
	keys := configuredPeerKeys(filterStateSubtree(configs, n2ip))
	installRoleConfig(roleConfig{peers: configs, nameToIP: n2ip, peerKeys: keys}) //nolint:errcheck // installRoleConfig cannot fail
	pruneLearnedRoles()
}

// filterStateSubtree is the bgp subtree whose configured peers are the ones
// setFilterState's arguments name.
func filterStateSubtree(configs map[string]*peerRoleConfig, n2ip map[string]string) map[string]any {
	peers := make(map[string]any)
	groups := make(map[string]any)
	named := make(map[string]struct{}, len(n2ip))
	for name, ip := range n2ip {
		peers[name] = map[string]any{"connection": map[string]any{"remote": map[string]any{"ip": ip}}}
		named[ip] = struct{}{}
	}
	for key := range configs {
		if group, isGroup := strings.CutPrefix(key, configjson.GroupKeyPrefix); isGroup {
			groups[group] = map[string]any{"connection": map[string]any{"remote": map[string]any{"ip": configjson.DynamicPeerIP}}}
			continue
		}
		if _, isNamed := named[key]; isNamed {
			continue
		}
		peers[key] = map[string]any{}
	}
	return map[string]any{"peer": peers, "group": groups}
}

// resetRoleState clears the package filter state before and after a test.
func resetRoleState(t *testing.T) {
	t.Helper()
	clearState := func() {
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
		// An empty install then a prune also clears the configured-peer set
		// the prune diffs against.
		installRoleConfig(roleConfig{}) //nolint:errcheck // installRoleConfig cannot fail
		pruneLearnedRoles()
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

// rolelessPeerSections is a bgp root with peer edge (192.0.2.1, role
// customer) and, when withPlain, peer plain (192.0.2.3) with NO role block,
// carrying description. A role-less peer's OPEN can still declare a role, and
// the OTC export set reads it as that destination's role.
func rolelessPeerSections(withPlain bool, description string) []sdk.ConfigSection {
	body := `"edge":{"connection":{"remote":{"ip":"192.0.2.1"}},"role":{"import":"customer","export":["customer"]}}`
	if withPlain {
		body += `,"plain":{"connection":{"remote":{"ip":"192.0.2.3"}},"description":"` + description + `"}`
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"peer":{` + body + `}}}`}}
}

// commitRoleReload runs one reload's verify, apply and committed.
func commitRoleReload(t *testing.T, tx *roleConfigTx, sections []sdk.ConfigSection) {
	t.Helper()
	if err := tx.tx.Verify(sections); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.committed("tx"); err != nil {
		t.Fatalf("committed: %v", err)
	}
}

// TestRoleUnrelatedCommitKeepsRolelessPeerRole proves R-14 and AC-5 for a
// configured peer with no role block: a commit that removes no peer keeps the
// role its OPEN declared.
// Method: the role is learned under the peer's name (no name-to-address entry
// exists for a role-less peer), a reload edits only that peer's description,
// then getFilterConfig, which OTCEgressFilter reads for the export set, is read.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14, AC-5 for bgp-role.
// PREVENTS: the prune at config-committed dropping a role-less peer's learned
// role on any edit, which withdraws every route the export set sends it.
func TestRoleUnrelatedCommitKeepsRolelessPeerRole(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(rolelessPeerSections(true, "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("plain", "", roleCustomer)

	commitRoleReload(t, tx, rolelessPeerSections(true, "second"))
	if _, learned := getFilterConfig("192.0.2.3", "plain", ""); learned != roleCustomer {
		t.Fatalf("an unrelated commit dropped the role-less peer's learned role: %q", learned)
	}
}

// TestRoleCommitRemovingRolelessPeerPrunesRole proves R-14's other half for
// a role-less peer: the commit that removes it drops its learned role.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14 for bgp-role.
// PREVENTS: a learned role outliving the peer the config removed.
func TestRoleCommitRemovingRolelessPeerPrunesRole(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(rolelessPeerSections(true, "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("plain", "", roleCustomer)

	commitRoleReload(t, tx, rolelessPeerSections(false, ""))
	if _, learned := getFilterConfig("192.0.2.3", "plain", ""); learned != "" {
		t.Fatalf("the commit removing the peer kept its learned role %q", learned)
	}
}

// groupWithStaticPeerSections is a bgp root with group ix holding static peer
// member (192.0.2.9, no role block). When dynamic, ix also carries a listen
// range, so it is a dynamic group too and its template is configured.
func groupWithStaticPeerSections(dynamic bool) []sdk.ConfigSection {
	group := `"peer":{"member":{"connection":{"remote":{"ip":"192.0.2.9"}}}}`
	if dynamic {
		group = `"connection":{"remote":{"ip":"dynamic","range":"198.51.100.0/24"}},` + group
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{"group":{"ix":{` + group + `}}}}`}}
}

// TestRoleCommitRemovingGroupRangeKeepsStaticMemberRole proves that a commit
// which turns a dynamic group static drops its dynamic members' learned roles
// and keeps the learned role of a static peer the group still lists.
// Method: the static peer and a dynamic member both record a role with group
// ix, a reload removes ix's listen range, then getFilterConfig, which
// OTCEgressFilter reads for the export set, is read for both.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14 for bgp-role.
// PREVENTS: the prune reading a static peer's group as its identity, which
// withdrew every route to that peer as role "unknown" until its session
// bounced, because an established peer sends no second OPEN.
func TestRoleCommitRemovingGroupRangeKeepsStaticMemberRole(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(groupWithStaticPeerSections(true)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("member", "ix", rolePeer)
	setFilterRemoteRole("dyn-198.51.100.7", "ix", roleCustomer)

	commitRoleReload(t, tx, groupWithStaticPeerSections(false))
	if _, learned := getFilterConfig("192.0.2.9", "member", "ix"); learned != rolePeer {
		t.Fatalf("the commit removing the group's range dropped the static member's role: %q", learned)
	}
	if remoteRoleRecorded("198.51.100.7", "dyn-198.51.100.7") {
		t.Fatalf("a member of a group that is no longer dynamic kept its learned role")
	}
}

// TestRoleCommitDropsRoleLearnedForMemberOfUnconfiguredGroup proves that the
// prune drops a role learned for a member of a group no config holds, such as
// an OPEN validated while the commit removing that group was in flight.
// Method: a role is recorded for a dynamic member of group gone, which neither
// config holds, then an unrelated commit runs.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14 for bgp-role.
// PREVENTS: an entry keyed outside both configured-peer sets surviving every
// later prune, since a diff of the two sets can never name it.
func TestRoleCommitDropsRoleLearnedForMemberOfUnconfiguredGroup(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(rolelessPeerSections(true, "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("plain", "", roleCustomer)
	setFilterRemoteRole("dyn-198.51.100.8", "gone", rolePeer)

	commitRoleReload(t, tx, rolelessPeerSections(true, "second"))
	if remoteRoleRecorded("198.51.100.8", "dyn-198.51.100.8") {
		t.Fatalf("a role learned for a member of a never-configured group survived the prune")
	}
	if _, learned := getFilterConfig("192.0.2.3", "plain", ""); learned != roleCustomer {
		t.Fatalf("the configured peer lost its learned role: %q", learned)
	}
}

// TestRoleUnrelatedCommitKeepsCreatedPeerRole proves that a commit keeps the
// learned role of a peer made at runtime with `create bgp peer <addr>`.
// Method: the role is recorded the way that peer's OPEN stores it, under the
// generated name peer-10.0.0.1 with no group (reactor peerConfigNameFor
// replaces only an IPv6 colon), which no config ever names; an unrelated commit runs, then getFilterConfig, which
// OTCEgressFilter reads for the export set, is read.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14 for bgp-role.
// PREVENTS: the prune dropping a created peer's role on any commit, which
// withdrew every route the export set sends it as role "unknown" until its
// session bounced, because the created peer survives a commit that does not
// declare it and an established peer sends no second OPEN.
func TestRoleUnrelatedCommitKeepsCreatedPeerRole(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(rolelessPeerSections(true, "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("peer-10.0.0.1", "", roleCustomer)

	commitRoleReload(t, tx, rolelessPeerSections(true, "second"))
	if _, learned := getFilterConfig("10.0.0.1", "peer-10.0.0.1", ""); learned != roleCustomer {
		t.Fatalf("an unrelated commit dropped the created peer's learned role: %q", learned)
	}
}

// TestRoleOpenBeforeInstallOfAddedPeerKeepsRole proves that a peer a reload
// adds keeps the role its OPEN declared when that OPEN was validated before the
// role plugin installed the config naming the peer.
// Method: the OPEN of peer other is recorded while the installed config does
// not hold other, so its name has no address and the role is stored under the
// name; the reload adding other (192.0.2.2, with a role block, so its name now
// maps to its address) commits, then getFilterConfig is read by address and
// name, as OTCEgressFilter reads it.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-14 for bgp-role.
// PREVENTS: the reader translating the name to the address, which holds no
// entry, and reading the added peer's role as unknown until its session
// bounced.
func TestRoleOpenBeforeInstallOfAddedPeerKeepsRole(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, false, false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("other", "", roleCustomer)

	commitRoleReload(t, tx, roleTxSections("false", `["default"]`, true, false))
	if _, learned := getFilterConfig("192.0.2.2", "other", ""); learned != roleCustomer {
		t.Fatalf("the added peer's role, learned before the install, was not read: %q", learned)
	}
}

// TestRoleAddressEntryBeatsStaleNameEntry proves that the reader takes the
// learned role stored under a peer's address over an older one stored under
// its bare name.
// Method: peer other's first OPEN is recorded before the config naming it is
// installed, so that role is stored under the name; the reload adding other
// (192.0.2.2, so its name now maps to its address) commits, and other's next
// OPEN is recorded under the address with a different role. getFilterConfig is
// then read by address and name, as OTCEgressFilter reads it.
// VALIDATES: remoteRoleLocked's order: address, translated name, bare name.
// PREVENTS: a read that tries the bare name first answering with the role of a
// session that ended, when a later OPEN already wrote the address key.
func TestRoleAddressEntryBeatsStaleNameEntry(t *testing.T) {
	resetRoleState(t)
	tx := newRoleConfigTx()
	if err := tx.configure(roleTxSections("false", `["default"]`, false, false)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	setFilterRemoteRole("other", "", roleCustomer)

	commitRoleReload(t, tx, roleTxSections("false", `["default"]`, true, false))
	setFilterRemoteRole("other", "", roleProvider)
	if _, learned := getFilterConfig("192.0.2.2", "other", ""); learned != roleProvider {
		t.Fatalf("the reader took the stale role under the name over the address entry: %q", learned)
	}
}
