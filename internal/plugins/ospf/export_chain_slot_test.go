// Design: docs/architecture/ospf/ospf-4-component-config.md -- one config transaction owns runtime and export undo.
package ospf

import (
	"testing"

	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestOSPFConfigTransactionRestoresExportOnce proves that the shared config
// transaction restores the exact prior export chain once, and that an absent
// candidate installs nothing and leaves rollback nothing to restore. Method:
// parse real accept/reject OSPF deliveries, install them through ConfigTx, count
// installs and read the chain the redistribution consumer actually uses.
func TestOSPFConfigTransactionRestoresExportOnce(t *testing.T) {
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	installs := 0
	tx := sdk.NewConfigTx(ospfTransactionConfig{}, parseOSPFTransactionSections, func(cfg ospfTransactionConfig) error {
		cfg.export.install()
		installs++
		return nil
	})
	sections := func(final string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: Namespace, Data: `{"ospf":{"router-id":"10.0.0.1","filter":{"export":{"final":"` + final + `"}}}}`}}
	}
	if err := tx.Configure(sections("accept")); err != nil {
		t.Fatal(err)
	}
	running := ospfredistribute.ExportChain()
	if running == nil {
		t.Fatal("running config installed no export chain")
	}
	if err := tx.Verify(sections("reject")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	next := ospfredistribute.ExportChain()
	if next == nil {
		t.Fatal("applied config installed no export chain")
	}
	if next == running {
		t.Fatal("applied reject config kept the running accept chain")
	}
	if installs != 2 {
		t.Fatalf("installs after apply = %d, want 2", installs)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := ospfredistribute.ExportChain(); got != running {
		t.Fatalf("after rollback the installed chain = %p, want running %p", got, running)
	}
	if installs != 3 {
		t.Fatalf("installs after rollback = %d, want 3", installs)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if installs != 3 {
		t.Fatalf("repeated rollback installed again: %d installs", installs)
	}
	if err := tx.Verify(nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if installs != 3 {
		t.Fatalf("absent delivery or its rollback installed again: %d installs", installs)
	}
	if got := ospfredistribute.ExportChain(); got != running {
		t.Fatalf("absent delivery left chain = %p, want running %p", got, running)
	}
}
