//go:build integration && linux

package firewallnft

import (
	"runtime"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/firewall"
)

func withNftNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := nftNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort cleanup
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})

	fn()
}

func nftNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 9 {
		name = name[len(name)-9:]
	}
	return "zenft_" + name
}

func newNftIntegrationBackend(t *testing.T) *backend {
	t.Helper()

	be, err := newBackend()
	if err != nil {
		t.Fatalf("newBackend: %v", err)
	}
	b, ok := be.(*backend)
	if !ok {
		t.Fatalf("newBackend returned %T, want *backend", be)
	}
	return b
}

func addNftIntegrationTable(t *testing.T, tableName string) {
	t.Helper()
	addNftIntegrationTableWithChain(t, tableName, "")
}

func addNftIntegrationTableWithChain(t *testing.T, tableName, chainName string) {
	t.Helper()
	addNftIntegrationTableInFamily(t, tableName, chainName, nftables.TableFamilyINet)
}

func addNftIntegrationTableInFamily(t *testing.T, tableName, chainName string, family nftables.TableFamily) {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	table := conn.AddTable(&nftables.Table{Name: tableName, Family: family})
	if chainName != "" {
		conn.AddChain(&nftables.Chain{Name: chainName, Table: table})
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("add nft table %q: %v", tableName, err)
	}
}

// requireLegacySweepPending fails with the reason rather than the symptom when
// an earlier test in this process has already driven firewall.ApplyAll. The
// one-time removal is gated on that flag, so a test that seeds a legacy table
// after it has been cleared would report "the table survived" and say nothing
// about why (internal/component/firewall/legacy_tables.go).
func requireLegacySweepPending(t *testing.T) {
	t.Helper()
	if !firewall.LegacySweepPending() {
		t.Fatal("the one-time removal has already run in this process: a test before this one drove firewall.ApplyAll, and the removal is gated on that")
	}
}

func nftIntegrationTableNames(t *testing.T) map[string]struct{} {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	tables, err := conn.ListTables()
	if err != nil {
		t.Fatalf("list nft tables: %v", err)
	}
	names := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		names[table.Name] = struct{}{}
	}
	return names
}

func requireNftTablePresent(t *testing.T, tableName string) {
	t.Helper()
	if _, ok := nftIntegrationTableNames(t)[tableName]; !ok {
		t.Fatalf("nft table %q is absent", tableName)
	}
}

func requireNftTableAbsent(t *testing.T, tableName string) {
	t.Helper()
	if _, ok := nftIntegrationTableNames(t)[tableName]; ok {
		t.Fatalf("nft table %q is present", tableName)
	}
}

func requireNftChainAbsent(t *testing.T, tableName, chainName string) {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	tables, err := conn.ListTables()
	if err != nil {
		t.Fatalf("list nft tables: %v", err)
	}
	var family nftables.TableFamily
	found := false
	for _, table := range tables {
		if table.Name == tableName {
			family = table.Family
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("nft table %q is absent", tableName)
	}

	chains, err := conn.ListChainsOfTableFamily(family)
	if err != nil {
		t.Fatalf("list nft chains: %v", err)
	}
	for _, chain := range chains {
		if chain.Table != nil && chain.Table.Name == tableName && chain.Name == chainName {
			t.Fatalf("nft chain %q still exists in table %q", chainName, tableName)
		}
	}
}

// VALIDATES: P0-8 -- nft apply deletes same-process owned tables only.
// PREVENTS: dropping an unknown ze_* table when reconciling a changed config.
func TestNftIntegration_ApplyDeletesOnlySameInstanceOwnedTables(t *testing.T) {
	withNftNetNS(t, func() {
		addNftIntegrationTable(t, "ze_fw_foreign")

		b := newNftIntegrationBackend(t)
		if err := b.Apply([]firewall.Table{{Name: "ze_fw_old", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		requireNftTablePresent(t, "ze_fw_old")
		requireNftTablePresent(t, "ze_fw_foreign")

		if err := b.Apply([]firewall.Table{{Name: "ze_fw_next", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("second apply: %v", err)
		}
		requireNftTableAbsent(t, "ze_fw_old")
		requireNftTablePresent(t, "ze_fw_next")
		requireNftTablePresent(t, "ze_fw_foreign")
	})
}

// VALIDATES: P0-8 -- restart reapply replaces desired tables without prefix sweeping.
// PREVENTS: crash recovery deleting unknown ze_* tables or leaving stale rules in desired tables.
func TestNftIntegration_RestartReapplyPreservesUnknownZeTables(t *testing.T) {
	withNftNetNS(t, func() {
		addNftIntegrationTableWithChain(t, "ze_fw_live", "stale_rule_chain")
		addNftIntegrationTable(t, "ze_fw_foreign")

		b := newNftIntegrationBackend(t)
		if err := b.Apply([]firewall.Table{{Name: "ze_fw_live", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("restart apply: %v", err)
		}

		requireNftTablePresent(t, "ze_fw_live")
		requireNftTablePresent(t, "ze_fw_foreign")
		requireNftChainAbsent(t, "ze_fw_live", "stale_rule_chain")
	})
}

// VALIDATES: AC-2 -- every owner's desired table is deleted before applyTable
// re-adds it, so a table that survives two reconciles holds the rules the
// current desired state names and no duplicates.
// PREVENTS: the FlowSpec accumulation. Its table was named without the
// ownership prefix, so the sweep skipped it, Apply added to a table it never
// deleted, and one announced route left two identical drop rules after a second
// reconcile.
func TestApplySweepsEveryOwnersTable(t *testing.T) {
	withNftNetNS(t, func() {
		b := newNftIntegrationBackend(t)

		desired := []firewall.Table{{
			Name:   "ze_flowspec",
			Family: firewall.FamilyInet,
			Chains: []firewall.Chain{{
				Name:     "flowspec-fwd",
				IsBase:   true,
				Type:     firewall.ChainFilter,
				Hook:     firewall.HookForward,
				Priority: -1,
				Policy:   firewall.PolicyAccept,
				Terms: []firewall.Term{{
					Name:    "fs-1",
					Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}}},
					Actions: []firewall.Action{firewall.Drop{}},
				}},
			}},
		}}

		if err := b.Apply(desired); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		if got := nftIntegrationRuleCount(t, "ze_flowspec", "flowspec-fwd"); got != 1 {
			t.Fatalf("after one reconcile the chain holds %d rules, want 1", got)
		}

		// The same desired state again. The table must be deleted and rebuilt,
		// never added to.
		if err := b.Apply(desired); err != nil {
			t.Fatalf("second apply: %v", err)
		}
		if got := nftIntegrationRuleCount(t, "ze_flowspec", "flowspec-fwd"); got != 1 {
			t.Fatalf("after two reconciles the chain holds %d rules, want 1", got)
		}

		// The route is withdrawn: the owner registers nothing, and the table
		// must leave the kernel.
		if err := b.Apply(nil); err != nil {
			t.Fatalf("withdraw apply: %v", err)
		}
		requireNftTableAbsent(t, "ze_flowspec")
	})
}

// VALIDATES: AC-3 -- a table an older ze build wrote without the ownership
// prefix is deleted once, and the second reconcile has nothing left to delete.
// PREVENTS: an in-place upgrade leaving the old table enforcing forever. The
// renamed producer asks for ze_flowspec, and nothing else reaches the table the
// previous build installed.
func TestApplyRemovesTheLegacyUnprefixedTableOnce(t *testing.T) {
	withNftNetNS(t, func() {
		requireLegacySweepPending(t)
		// What an older build left behind: the name, the inet family, and a
		// chain that build wrote. All three are the test, so the seed carries
		// the chain rather than the bare table.
		addNftIntegrationTableWithChain(t, "flowspec", "flowspec-fwd")
		addNftIntegrationTable(t, "operator-table")

		b := newNftIntegrationBackend(t)
		if err := b.Apply([]firewall.Table{{Name: "ze_flowspec", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		requireNftTableAbsent(t, "flowspec")
		requireNftTablePresent(t, "ze_flowspec")
		requireNftTablePresent(t, "operator-table")

		if err := b.Apply([]firewall.Table{{Name: "ze_flowspec", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("second apply: %v", err)
		}
		requireNftTableAbsent(t, "flowspec")
		requireNftTablePresent(t, "ze_flowspec")
		requireNftTablePresent(t, "operator-table")
	})
}

// VALIDATES: a table another tool wrote under a name ze used to use survives
// the one-time removal. The name and the inet family match; the chain does not.
// PREVENTS: ze destroying somebody else's kernel state. These names are
// ordinary words -- another FlowSpec-to-nftables tool can use "flowspec" -- and
// deleting its table is a worse failure than the stale rule the removal exists
// to clear. Name and family alone would delete it.
func TestApplyLeavesAnotherToolsTableAlone(t *testing.T) {
	withNftNetNS(t, func() {
		requireLegacySweepPending(t)
		// Same name, same family, a chain ze never wrote.
		addNftIntegrationTableWithChain(t, "flowspec", "forward")

		b := newNftIntegrationBackend(t)
		if err := b.Apply([]firewall.Table{{Name: "ze_flowspec", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		requireNftTablePresent(t, "flowspec")
		requireNftTablePresent(t, "ze_flowspec")

		// And it keeps surviving while the removal is still pending, so the
		// chain test is what saves it rather than the one-shot gate.
		if err := b.Apply([]firewall.Table{{Name: "ze_flowspec", Family: firewall.FamilyInet}}); err != nil {
			t.Fatalf("second apply: %v", err)
		}
		requireNftTablePresent(t, "flowspec")
	})
}

// VALIDATES: an EMPTY desired set still removes the table an older build left.
// PREVENTS: the upgrade this removal exists for going unfixed on the box that
// needs it most -- one with no firewall {} section and no owner registering
// anything, where the only reconcile that will ever run carries nothing.
func TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable(t *testing.T) {
	withNftNetNS(t, func() {
		requireLegacySweepPending(t)
		addNftIntegrationTableWithChain(t, "flowspec", "flowspec-fwd")
		addNftIntegrationTable(t, "operator-table")

		b := newNftIntegrationBackend(t)
		if err := b.Apply(nil); err != nil {
			t.Fatalf("empty apply: %v", err)
		}
		requireNftTableAbsent(t, "flowspec")
		requireNftTablePresent(t, "operator-table")
	})
}

// VALIDATES: every entry in the legacy ledger is removed against a real
// kernel, in the address family its producer wrote it in.
// PREVENTS: an entry that reads right and never fires. The decision raises the
// kernel family and lists that family's chains, so an entry in ip or ip6 goes
// down a path no inet-only proof ever walks: copp and the FlowSpec bridge wrote
// inet, ddos-local wrote ip and ip6, and the anomaly-shape responder wrote one
// name in each.
func TestApplyRemovesEveryLegacyTableInItsOwnFamily(t *testing.T) {
	withNftNetNS(t, func() {
		requireLegacySweepPending(t)

		addNftIntegrationTableInFamily(t, "copp", "input", nftables.TableFamilyINet)
		addNftIntegrationTableInFamily(t, "flowspec", "flowspec-in", nftables.TableFamilyINet)
		addNftIntegrationTableInFamily(t, "ddos-local", "forward", nftables.TableFamilyIPv4)
		addNftIntegrationTableInFamily(t, "anomaly-shape", "ingress", nftables.TableFamilyIPv4)
		addNftIntegrationTableInFamily(t, "anomaly-shape6", "ingress", nftables.TableFamilyIPv6)
		addNftIntegrationTable(t, "operator-table")

		b := newNftIntegrationBackend(t)
		if err := b.Apply(nil); err != nil {
			t.Fatalf("apply: %v", err)
		}
		for _, name := range []string{"copp", "flowspec", "ddos-local", "anomaly-shape", "anomaly-shape6"} {
			requireNftTableAbsent(t, name)
		}
		requireNftTablePresent(t, "operator-table")
	})
}

// VALIDATES: the removal is a migration with an end. It runs on the first
// reconcile of the process and on no later one, so a table written under a
// legacy name after ze started is left alone.
// PREVENTS: a standing deletion policy shipping under the word "once". Every
// current producer carries the ownership prefix and RegisterTables refuses a
// bare name, so a bare table appearing later is somebody else's by
// construction. Deleting it would be ze destroying another tool's kernel state
// in every release from here on.
//
// It drives firewall.ApplyAll rather than backend.Apply, because ApplyAll is
// what clears the pending flag. It therefore leaves the flag cleared for the
// rest of this process: keep it last among the legacy tests, and note that
// requireLegacySweepPending is what makes a wrong order say so.
func TestTheLegacyRemovalRunsOnceInTheProcess(t *testing.T) {
	withNftNetNS(t, func() {
		requireLegacySweepPending(t)

		addNftIntegrationTableWithChain(t, "flowspec", "flowspec-fwd")

		if err := firewall.LoadBackend("nft"); err != nil {
			t.Fatalf("LoadBackend: %v", err)
		}
		t.Cleanup(func() { _ = firewall.CloseBackend() })

		if err := firewall.ApplyAll(); err != nil {
			t.Fatalf("first reconcile: %v", err)
		}
		requireNftTableAbsent(t, "flowspec")
		if firewall.LegacySweepPending() {
			t.Fatal("the removal is still pending after a reconcile reached the backend")
		}

		// Written while ze was running, so it is not what an older build left.
		addNftIntegrationTableWithChain(t, "flowspec", "flowspec-fwd")
		if err := firewall.ApplyAll(); err != nil {
			t.Fatalf("second reconcile: %v", err)
		}
		requireNftTablePresent(t, "flowspec")
	})
}

func nftIntegrationRuleCount(t *testing.T, tableName, chainName string) int {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	tables, err := conn.ListTables()
	if err != nil {
		t.Fatalf("list nft tables: %v", err)
	}
	for _, table := range tables {
		if table.Name != tableName {
			continue
		}
		rules, err := conn.GetRules(table, &nftables.Chain{Name: chainName, Table: table})
		if err != nil {
			t.Fatalf("get rules of %q/%q: %v", tableName, chainName, err)
		}
		return len(rules)
	}
	t.Fatalf("nft table %q is absent", tableName)
	return 0
}
