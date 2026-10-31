package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
)

// TestExportChainSlotRestoresOnRollback proves a rolled-back config apply
// takes its export chain back: after the running chain is adopted and a new
// one applied, restore installs the running chain again, once, and an apply
// that installed nothing leaves a rollback nothing to restore. Method: drive
// the slot as OnConfigApply and OnConfigRollback do and read the chain the
// redistribution consumer judges with (ospfredistribute.ExportChain).
func TestExportChainSlotRestoresOnRollback(t *testing.T) {
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	running := exportChainConfig{chain: &policy.RouteChain{}}
	next := exportChainConfig{chain: &policy.RouteChain{}}
	var slot exportChainSlot

	slot.disarm()
	slot.adopt(running)
	slot.disarm()
	slot.adopt(next)
	if got := ospfredistribute.ExportChain(); got != next.chain {
		t.Fatalf("after apply the installed chain = %p, want the applied %p", got, next.chain)
	}
	if !slot.restore() {
		t.Fatal("restore after an apply answered false, want true")
	}
	if got := ospfredistribute.ExportChain(); got != running.chain {
		t.Fatalf("after rollback the installed chain = %p, want the running %p", got, running.chain)
	}
	if slot.restore() {
		t.Fatal("a second restore answered true, want false: the rollback already ran")
	}

	// An apply with no pending config installs nothing, so its rollback keeps
	// the chain in place.
	slot.disarm()
	if slot.restore() {
		t.Fatal("restore after an apply that installed nothing answered true, want false")
	}
	if got := ospfredistribute.ExportChain(); got != running.chain {
		t.Fatalf("the installed chain = %p, want the running %p kept", got, running.chain)
	}
}
