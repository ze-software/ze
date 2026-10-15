// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package routingtable

import (
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func tableSection(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: pluginName, Data: data}}
}

// TestConfigTxAppliesReloadedTables proves AC-1 to AC-4 for routing-table: a
// reload's verify installs nothing, its apply publishes the new tables, and
// its rollback publishes the tables it replaced. Method: the registry is
// resolved after each step.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-1 to AC-4, row 21.
// PREVENTS: the acked-and-dropped reload, where a commit adding a table
// succeeded and the registry kept the boot tables.
// Not parallel: the registry is package state.
func TestConfigTxAppliesReloadedTables(t *testing.T) {
	t.Cleanup(func() { SetRegistry(nil) })
	tx := newRoutingTableConfigTx()
	if err := tx.Configure(tableSection(`{"routing-table":{"table":{"red":{"id":"100"}}}}`)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(tableSection(`{"routing-table":{"table":{"red":{"id":"100"},"blue":{"id":"200"}}}}`)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := GetRegistry().Resolve("blue"); err == nil {
		t.Fatal("verify installed the candidate table blue")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if id, err := GetRegistry().Resolve("blue"); err != nil || id != 200 {
		t.Fatalf("after apply blue = %d, %v; want 200", id, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := GetRegistry().Resolve("blue"); err == nil {
		t.Fatal("rollback kept the table blue")
	}
	if id, err := GetRegistry().Resolve("red"); err != nil || id != 100 {
		t.Fatalf("after rollback red = %d, %v; want 100", id, err)
	}
}

// TestConfigTxRefusedTablesInstallNothing proves a candidate with a reserved
// table ID is refused at verify and the following apply is refused too, so
// the registry keeps the running tables.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-2 for routing-table.
// PREVENTS: an apply after a refused verify publishing a half-checked map.
func TestConfigTxRefusedTablesInstallNothing(t *testing.T) {
	t.Cleanup(func() { SetRegistry(nil) })
	tx := newRoutingTableConfigTx()
	if err := tx.Configure(tableSection(`{"routing-table":{"table":{"red":{"id":"100"}}}}`)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	installed := GetRegistry()
	if err := tx.Verify(tableSection(`{"routing-table":{"table":{"red":{"id":"254"}}}}`)); err == nil {
		t.Fatal("verify accepted the reserved table ID 254")
	}
	if err := tx.Apply(); err == nil {
		t.Fatal("apply after a refused verify answered OK")
	}
	if GetRegistry() != installed {
		t.Fatal("a refused candidate replaced the registry")
	}
}
