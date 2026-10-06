//go:build linux

package firewallvpp

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/firewall"
)

// The fake owns persistent external state; a fresh backend models only a Ze
// restart, not a VPP restart. Deleting a bound ACL is rejected by the fake.
func TestACLRestartAdoptsAndUpdatesSingleton(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 0, "eth1": 1})
	desired := oneChainTable()
	first := newOpsBackend()
	if err := applyWithOpsLocked(first, ops, desired); err != nil {
		t.Fatal(err)
	}
	index := first.aclIndexes["ze/wan/input"]
	allocated := ops.nextIdx
	ops.calls = nil
	restarted := newOpsBackend()
	if err := applyWithOpsLocked(restarted, ops, desired); err != nil {
		t.Fatal(err)
	}
	if len(ops.existingACL) != 1 || ops.existingACL[0].Index != index || ops.nextIdx != allocated {
		t.Fatalf("restart allocated a duplicate: ACLs=%v next=%d, want index=%d next=%d", ops.existingACL, ops.nextIdx, index, allocated)
	}
	if ops.countPrefix("del:") != 0 {
		t.Fatalf("restart deleted the desired ACL: %v", ops.calls)
	}
	for _, iface := range ops.ifaces {
		assertACLBinding(t, ops, iface, 1, []uint32{index})
	}

	// A new process must replace the adopted object's rules, not merely keep
	// its tag. Remove the term so only the chain's default verdict remains.
	desired[0].Chains[0].Terms = nil
	if err := applyWithOpsLocked(newOpsBackend(), ops, desired); err != nil {
		t.Fatal(err)
	}
	if len(ops.existingACL) != 1 || ops.existingACL[0].Index != index || len(ops.existingACL[0].Rules) != 1 {
		t.Fatalf("adopted ACL was not updated in place: %v", ops.existingACL)
	}
}

func TestACLRestartRetiresDuplicatesAndPreservesForeignDirections(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 0, "eth1": 1})
	// Dump order deliberately does not match index order.
	ops.existingACL = []aclDumpEntry{
		{Index: 7, Tag: "ze/wan/input"},
		{Index: 0, Tag: "ze/wan/input"},
		{Index: 8, Tag: "ze/stale/output"},
		{Index: 100, Tag: "foreign/input-a"},
		{Index: 101, Tag: "foreign/input-b"},
		{Index: 200, Tag: "foreign/output-a"},
		{Index: 201, Tag: "foreign/output-b"},
	}
	foreign := slices.Clone(ops.existingACL[3:])
	for _, iface := range ops.ifaces {
		ops.ifaceLists[iface] = ifaceACLList{nInput: 4, acls: []uint32{100, 7, 101, 0, 200, 8, 201, 7}}
	}
	b := newOpsBackend()
	if err := applyWithOpsLocked(b, ops, oneChainTable()); err != nil {
		t.Fatal(err)
	}
	if b.aclIndexes["ze/wan/input"] != 0 || len(ops.existingACL) != 5 {
		t.Fatalf("legacy duplicates not retired: indexes=%v ACLs=%v", b.aclIndexes, ops.existingACL)
	}
	for _, iface := range ops.ifaces {
		assertACLBinding(t, ops, iface, 3, []uint32{100, 101, 0, 200, 201})
	}
	if ops.countPrefix("del:0") != 0 || ops.countPrefix("del:7") != 1 || ops.countPrefix("del:8") != 1 {
		t.Fatalf("wrong owned objects deleted: %v", ops.calls)
	}

	// No process-local binding history is available during empty startup.
	if err := applyWithOpsLocked(newOpsBackend(), ops, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ops.existingACL, foreign) {
		t.Fatalf("empty restart modified foreign ACL objects: got=%v want=%v", ops.existingACL, foreign)
	}
	for _, iface := range ops.ifaces {
		assertACLBinding(t, ops, iface, 2, []uint32{100, 101, 200, 201})
	}
}

func TestACLEmptyRestartRemovesAllLegacyDuplicates(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 0, "eth1": 1})
	ops.existingACL = []aclDumpEntry{{Index: 0, Tag: "ze/wan/input"}, {Index: 1, Tag: "ze/wan/input"}}
	for _, iface := range ops.ifaces {
		ops.ifaceLists[iface] = ifaceACLList{nInput: 1, acls: []uint32{0, 1}}
	}
	if err := applyWithOpsLocked(newOpsBackend(), ops, nil); err != nil {
		t.Fatal(err)
	}
	if len(ops.existingACL) != 0 {
		t.Fatalf("owned objects survived empty restart: %v", ops.existingACL)
	}
	for _, iface := range ops.ifaces {
		assertACLBinding(t, ops, iface, 0, nil)
	}
}

func TestACLReconcilePropagatesAPIFailures(t *testing.T) {
	for _, stage := range []string{"discovery", "replace", "binding-read", "binding-write", "delete"} {
		t.Run(stage, func(t *testing.T) {
			ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
			b := newOpsBackend()
			if err := applyWithOpsLocked(b, ops, oneChainTable()); err != nil {
				t.Fatal(err)
			}
			before, err := b.ListTables()
			if err != nil {
				t.Fatal(err)
			}
			cause := fmt.Errorf("scripted %s failure", stage)
			desired := []firewall.Table(nil)
			switch stage {
			case "discovery":
				ops.aclDumpErr = cause
			case "replace":
				ops.addFailOn["ze/wan/input"] = cause
				desired = oneChainTable()
			case "binding-read":
				ops.listFailOn[5] = cause
			case "binding-write":
				ops.bindFailOn[5] = cause
			case "delete":
				ops.delFailOn[b.aclIndexes["ze/wan/input"]] = cause
			}
			ops.calls = nil
			if err := applyWithOpsLocked(b, ops, desired); !errors.Is(err, cause) {
				t.Fatalf("Apply error=%v, want wrapped %v", err, cause)
			}
			after, err := b.ListTables()
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatalf("failed apply changed last successful tables: %v, %v", after, err)
			}
			if len(ops.existingACL) != 1 {
				t.Fatalf("failed operation lost owned ACL: %v", ops.existingACL)
			}
			if stage != "delete" && ops.countPrefix("del:") != 0 {
				t.Fatalf("deleted before detachment succeeded: %v", ops.calls)
			}
			ops.aclDumpErr = nil
			clear(ops.addFailOn)
			clear(ops.listFailOn)
			clear(ops.bindFailOn)
			clear(ops.delFailOn)
			if err := applyWithOpsLocked(b, ops, nil); err != nil {
				t.Fatalf("subsequent reconcile: %v", err)
			}
			if len(ops.existingACL) != 0 {
				t.Fatalf("subsequent reconcile leaked owned state: %v", ops.existingACL)
			}
		})
	}
}

func TestACLPartialBindingFailureRetainsOwnership(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(fmt.Sprintf("restart=%t", restart), func(t *testing.T) {
			ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 0, "eth1": 1})
			ops.bindFailAt = 2
			b := newOpsBackend()
			if err := applyWithOpsLocked(b, ops, oneChainTable()); err == nil {
				t.Fatal("partial binding should fail")
			}
			if len(ops.existingACL) != 1 || len(b.aclIndexes) != 1 || ops.countPrefix("del:") != 0 {
				t.Fatalf("partial binding lost ownership: objects=%v indexes=%v calls=%v", ops.existingACL, b.aclIndexes, ops.calls)
			}
			bound := 0
			for _, binding := range ops.ifaceLists {
				if len(binding.acls) != 0 {
					bound++
				}
			}
			if bound != 1 {
				t.Fatalf("fixture did not leave exactly one interface bound: %v", ops.ifaceLists)
			}
			if restart {
				b = newOpsBackend()
			}
			ops.bindFailAt = 0
			if err := applyWithOpsLocked(b, ops, nil); err != nil {
				t.Fatal(err)
			}
			if len(ops.existingACL) != 0 {
				t.Fatalf("failed-apply object was treated as foreign: %v", ops.existingACL)
			}
			for _, iface := range ops.ifaces {
				assertACLBinding(t, ops, iface, 0, nil)
			}
		})
	}
}

func TestACLProgrammingRollbackFailureRemainsOwned(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	desired := oneChainTable()
	desired[0].Chains = append(desired[0].Chains, firewall.Chain{Name: "output", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookOutput, Policy: firewall.PolicyDrop})
	addErr := errors.New("create rejected")
	delErr := errors.New("rollback rejected")
	ops.addFailOn["ze/wan/output"] = addErr
	ops.delFailOn[1] = delErr
	b := newOpsBackend()
	err := applyWithOpsLocked(b, ops, desired)
	if !errors.Is(err, addErr) || !errors.Is(err, delErr) {
		t.Fatalf("rollback did not preserve both errors: %v", err)
	}
	if len(ops.existingACL) != 1 || b.aclIndexes["ze/wan/input"] != 1 {
		t.Fatalf("failed rollback lost ownership: %v / %v", ops.existingACL, b.aclIndexes)
	}
	clear(ops.addFailOn)
	clear(ops.delFailOn)
	if err := applyWithOpsLocked(b, ops, nil); err != nil {
		t.Fatal(err)
	}
	if len(ops.existingACL) != 0 {
		t.Fatalf("failed rollback left an orphan: %v", ops.existingACL)
	}
}

func TestACLProgrammingFailureKeepsAdoptedProtection(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	ops.existingACL = []aclDumpEntry{{Index: 0, Tag: "ze/wan/input"}}
	ops.ifaceLists[5] = ifaceACLList{nInput: 1, acls: []uint32{0}}
	desired := oneChainTable()
	desired[0].Chains = append(desired[0].Chains, firewall.Chain{Name: "output", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookOutput, Policy: firewall.PolicyDrop})
	cause := errors.New("create rejected")
	ops.addFailOn["ze/wan/output"] = cause
	b := newOpsBackend()
	if err := applyWithOpsLocked(b, ops, desired); !errors.Is(err, cause) {
		t.Fatalf("Apply error=%v, want %v", err, cause)
	}
	if len(ops.existingACL) != 1 || ops.existingACL[0].Index != 0 || ops.countPrefix("del:") != 0 {
		t.Fatalf("failed apply removed adopted protection: %v / %v", ops.existingACL, ops.calls)
	}
	if index, ok := b.aclIndexes["ze/wan/input"]; !ok || index != 0 {
		t.Fatalf("failed apply lost adopted ownership: %v", b.aclIndexes)
	}
	assertACLBinding(t, ops, 5, 1, []uint32{0})
}

func TestACLDiscoveryDoesNotOwnReusedForeignIndex(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	b := newOpsBackend()
	if err := applyWithOpsLocked(b, ops, oneChainTable()); err != nil {
		t.Fatal(err)
	}
	index := b.aclIndexes["ze/wan/input"]
	// Another controller replaced the object between reconciliations.
	ops.existingACL = []aclDumpEntry{{Index: index, Tag: "foreign/reused"}}
	if err := applyWithOpsLocked(b, ops, nil); err != nil {
		t.Fatal(err)
	}
	if len(ops.existingACL) != 1 || ops.existingACL[0].Tag != "foreign/reused" {
		t.Fatalf("cached ownership deleted a foreign object: %v", ops.existingACL)
	}
	assertACLBinding(t, ops, 5, 1, []uint32{index})
}

func TestACLRestartPreservesBothDesiredDirections(t *testing.T) {
	ops := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	ops.existingACL = []aclDumpEntry{
		{Index: 1, Tag: "ze/wan/input"},
		{Index: 2, Tag: "ze/wan/output"},
		{Index: 3, Tag: "ze/wan/input"},
		{Index: 4, Tag: "ze/old/input"},
		{Index: 100, Tag: "foreign/input-a"},
		{Index: 101, Tag: "foreign/input-b"},
		{Index: 200, Tag: "foreign/output-a"},
		{Index: 201, Tag: "foreign/output-b"},
	}
	ops.ifaceLists[5] = ifaceACLList{nInput: 5, acls: []uint32{100, 1, 101, 3, 4, 200, 2, 201, 3}}
	desired := oneChainTable()
	desired[0].Chains = append(desired[0].Chains, firewall.Chain{Name: "output", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookOutput, Policy: firewall.PolicyDrop})
	if err := applyWithOpsLocked(newOpsBackend(), ops, desired); err != nil {
		t.Fatal(err)
	}
	assertACLBinding(t, ops, 5, 3, []uint32{100, 101, 1, 200, 201, 2})
	if len(ops.existingACL) != 6 || ops.countPrefix("del:1") != 0 || ops.countPrefix("del:2") != 0 {
		t.Fatalf("desired directional ACLs were not adopted: %v / %v", ops.existingACL, ops.calls)
	}
}

func assertACLBinding(t *testing.T, ops *fakeOps, iface interface_types.InterfaceIndex, nInput uint8, acls []uint32) {
	t.Helper()
	got := ops.ifaceLists[iface]
	if got.nInput != nInput || !slices.Equal(got.acls, acls) {
		t.Fatalf("interface %d binding=%v, want nInput=%d ACLs=%v", iface, got, nInput, acls)
	}
}
