//go:build integration && linux

package firewallnft

import (
	"runtime"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/vishvananda/netns"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
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
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
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

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New: %v", err)
	}
	table := conn.AddTable(&nftables.Table{Name: tableName, Family: nftables.TableFamilyINet})
	if chainName != "" {
		conn.AddChain(&nftables.Chain{Name: chainName, Table: table})
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("add nft table %q: %v", tableName, err)
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
