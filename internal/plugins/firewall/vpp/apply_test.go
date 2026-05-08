//go:build linux

package firewallvpp

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	govppacl "go.fd.io/govpp/binapi/acl"
	"go.fd.io/govpp/binapi/interface_types"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

type fakeOps struct {
	ifaces      map[string]interface_types.InterfaceIndex
	existingACL []aclDumpEntry
	ifaceLists  map[interface_types.InterfaceIndex]ifaceACLList
	calls       []string
	dumpErr     error
	addFailOn   map[string]error
	delFailOn   map[uint32]error
	bindFailOn  map[interface_types.InterfaceIndex]error
	nextIdx     uint32
}

func newFakeOps(ifaces map[string]interface_types.InterfaceIndex) *fakeOps {
	return &fakeOps{
		ifaces:     ifaces,
		ifaceLists: make(map[interface_types.InterfaceIndex]ifaceACLList),
		addFailOn:  map[string]error{},
		delFailOn:  map[uint32]error{},
		bindFailOn: map[interface_types.InterfaceIndex]error{},
	}
}

func (f *fakeOps) dumpInterfaces() (map[string]interface_types.InterfaceIndex, error) {
	f.calls = append(f.calls, "dumpIfaces")
	if f.dumpErr != nil {
		return nil, f.dumpErr
	}
	out := make(map[string]interface_types.InterfaceIndex, len(f.ifaces))
	maps.Copy(out, f.ifaces)
	return out, nil
}

func (f *fakeOps) aclAddReplace(req *govppacl.ACLAddReplace) (uint32, error) {
	f.calls = append(f.calls, "addReplace:"+req.Tag)
	if err, ok := f.addFailOn[req.Tag]; ok {
		return 0, err
	}
	f.nextIdx++
	return f.nextIdx, nil
}

func (f *fakeOps) aclDel(aclIndex uint32) error {
	f.calls = append(f.calls, fmt.Sprintf("del:%d", aclIndex))
	return f.delFailOn[aclIndex]
}

func (f *fakeOps) aclDump() ([]aclDumpEntry, error) {
	f.calls = append(f.calls, "aclDump")
	return append([]aclDumpEntry(nil), f.existingACL...), nil
}

func (f *fakeOps) aclInterfaceListDump(swIfIndex interface_types.InterfaceIndex) (ifaceACLList, error) {
	f.calls = append(f.calls, fmt.Sprintf("ifaceListDump:%d", swIfIndex))
	return f.ifaceLists[swIfIndex], nil
}

func (f *fakeOps) aclInterfaceSetACLList(swIfIndex interface_types.InterfaceIndex, nInput uint8, acls []uint32) error {
	f.calls = append(f.calls, fmt.Sprintf("bind:%d:nIn=%d:acls=%v", swIfIndex, nInput, acls))
	return f.bindFailOn[swIfIndex]
}

func (f *fakeOps) countPrefix(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func newOpsBackend() *backend {
	return &backend{
		aclIndexes: make(map[string]uint32),
		ifaceACLs:  make(map[interface_types.InterfaceIndex]ifaceACLBinding),
	}
}

func applyWithOpsLocked(b *backend, ops vppOps, desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.applyWithOps(ops, desired)
}

func oneChainTable() []firewall.Table {
	return []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:   "input",
			IsBase: true,
			Type:   firewall.ChainFilter,
			Hook:   firewall.HookInput,
			Policy: firewall.PolicyDrop,
			Terms: []firewall.Term{{
				Name:    "allow-ssh",
				Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}},
				Actions: []firewall.Action{firewall.Accept{}},
			}},
		}},
	}}
}

func TestApplyCreatesACL(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	if err := applyWithOpsLocked(b, fake, oneChainTable()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if fake.countPrefix("addReplace:") != 1 {
		t.Errorf("want 1 addReplace call, got %d", fake.countPrefix("addReplace:"))
	}
	if fake.countPrefix("bind:") != 1 {
		t.Errorf("want 1 bind call, got %d", fake.countPrefix("bind:"))
	}
	if _, ok := b.aclIndexes["ze/wan/input"]; !ok {
		t.Error("aclIndexes missing ze/wan/input")
	}
}

func TestApplyUpdatesACLInPlace(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, oneChainTable()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	firstIdx := b.aclIndexes["ze/wan/input"]

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, oneChainTable()); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if fake2.countPrefix("addReplace:") != 1 {
		t.Errorf("second apply: want 1 addReplace, got %d", fake2.countPrefix("addReplace:"))
	}
	if fake2.countPrefix("del:") != 0 {
		t.Errorf("update path should not delete, got %d del calls", fake2.countPrefix("del:"))
	}
	_ = firstIdx
}

func TestApplyUndoOnFailure(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	tables := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{
			{Name: "input", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookInput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t1", Actions: []firewall.Action{firewall.Accept{}}}}},
			{Name: "output", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookOutput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t2", Actions: []firewall.Action{firewall.Accept{}}}}},
		},
	}}
	fake.addFailOn["ze/wan/output"] = fmt.Errorf("scripted fail")

	err := applyWithOpsLocked(b, fake, tables)
	if err == nil {
		t.Fatal("expected error")
	}
	if fake.countPrefix("del:") < 1 {
		t.Errorf("undo should delete the successfully created ACL, got %d del calls", fake.countPrefix("del:"))
	}
}

func TestApplyReconcileRemovesStaleACLs(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	twoChains := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{
			{Name: "input", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookInput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t1", Actions: []firewall.Action{firewall.Accept{}}}}},
			{Name: "forward", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookForward, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t2", Actions: []firewall.Action{firewall.Drop{}}}}},
		},
	}}
	if err := applyWithOpsLocked(b, fake1, twoChains); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if len(b.aclIndexes) != 2 {
		t.Fatalf("want 2 ACL indexes after first apply, got %d", len(b.aclIndexes))
	}

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, oneChainTable()); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if fake2.countPrefix("del:") != 1 {
		t.Errorf("reconcile should delete 1 stale ACL, got %d del calls", fake2.countPrefix("del:"))
	}
	if len(b.aclIndexes) != 1 {
		t.Errorf("want 1 ACL index after second apply, got %d", len(b.aclIndexes))
	}
}

func TestStartupOrphanCleanup(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.existingACL = []aclDumpEntry{
		{Index: 10, Tag: "ze/old/chain"},
		{Index: 11, Tag: "ze/wan/input"},
		{Index: 99, Tag: "foreign/acl"},
	}

	if err := applyWithOpsLocked(b, fake, oneChainTable()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	delCalls := 0
	for _, c := range fake.calls {
		if c == "del:10" {
			delCalls++
		}
		if c == "del:99" {
			t.Error("should not delete foreign ACL index 99")
		}
	}
	if delCalls != 1 {
		t.Errorf("want 1 orphan deletion (ze/old/chain), got %d", delCalls)
	}
}

func TestStartupOrphanPreservesDesired(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.existingACL = []aclDumpEntry{
		{Index: 11, Tag: "ze/wan/input"},
	}

	if err := applyWithOpsLocked(b, fake, oneChainTable()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, c := range fake.calls {
		if c == "del:11" {
			t.Error("should not delete desired ACL ze/wan/input during orphan cleanup")
		}
	}
}

func TestBindPreservesForeignACLs(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.ifaceLists[5] = ifaceACLList{
		nInput: 1,
		acls:   []uint32{100, 200},
	}

	if err := applyWithOpsLocked(b, fake, oneChainTable()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	found := false
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "bind:5:") {
			found = true
			if !strings.Contains(c, "100") || !strings.Contains(c, "200") {
				t.Errorf("bind should preserve foreign ACLs 100 and 200, got %s", c)
			}
		}
	}
	if !found {
		t.Error("no bind call for interface 5")
	}
}

func TestBindMergesInputAndOutput(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	tables := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{
			{Name: "input", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookInput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t1", Actions: []firewall.Action{firewall.Accept{}}}}},
			{Name: "output", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookOutput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t2", Actions: []firewall.Action{firewall.Drop{}}}}},
		},
	}}

	if err := applyWithOpsLocked(b, fake, tables); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	found := false
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "bind:5:") {
			found = true
			if !strings.Contains(c, "nIn=1") {
				t.Errorf("want nIn=1 (one input ACL), got %s", c)
			}
		}
	}
	if !found {
		t.Error("no bind call for interface 5")
	}
}

func TestBindUnbindsPreviouslyBoundInterfaces(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, oneChainTable()); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, []firewall.Table{}); err != nil {
		t.Fatalf("empty apply: %v", err)
	}

	found := false
	for _, c := range fake2.calls {
		if strings.HasPrefix(c, "bind:5:") {
			found = true
			if !strings.Contains(c, "nIn=0") || !strings.Contains(c, "acls=[]") {
				t.Errorf("empty apply should clear bindings, got %s", c)
			}
		}
	}
	if !found {
		t.Error("empty apply should issue bind call to clear previously-bound interface 5")
	}
}

func TestApplyEmptyDesired(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	if err := applyWithOpsLocked(b, fake, []firewall.Table{}); err != nil {
		t.Fatalf("empty apply: %v", err)
	}
	if len(b.aclIndexes) != 0 {
		t.Errorf("want 0 ACL indexes, got %d", len(b.aclIndexes))
	}
}

func TestSplitForeignPreservesOrder(t *testing.T) {
	zeACLs := map[uint32]bool{10: true, 20: true}
	existing := ifaceACLList{
		nInput: 3,
		acls:   []uint32{5, 10, 15, 20, 25},
	}
	fIn, fOut := splitForeign(existing, zeACLs)
	wantIn := []uint32{5, 15}
	wantOut := []uint32{25}
	if !equalU32(fIn, wantIn) {
		t.Errorf("foreignInput = %v, want %v", fIn, wantIn)
	}
	if !equalU32(fOut, wantOut) {
		t.Errorf("foreignOutput = %v, want %v", fOut, wantOut)
	}
}

func TestSplitForeignEmptyExisting(t *testing.T) {
	fIn, fOut := splitForeign(ifaceACLList{}, map[uint32]bool{10: true})
	if fIn != nil || fOut != nil {
		t.Errorf("want nil/nil, got %v/%v", fIn, fOut)
	}
}

func TestSplitForeignAllZe(t *testing.T) {
	zeACLs := map[uint32]bool{1: true, 2: true}
	existing := ifaceACLList{nInput: 1, acls: []uint32{1, 2}}
	fIn, fOut := splitForeign(existing, zeACLs)
	if fIn != nil || fOut != nil {
		t.Errorf("want nil/nil when all are ze, got %v/%v", fIn, fOut)
	}
}

func TestDesiredACLTags(t *testing.T) {
	tables := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{
			{Name: "input", IsBase: true},
			{Name: "helper", IsBase: false},
		},
	}}
	tags := desiredACLTags(tables)
	if !tags["ze/wan/input"] {
		t.Error("missing ze/wan/input")
	}
	if tags["ze/wan/helper"] {
		t.Error("non-base chain should not be in desired tags")
	}
}

func TestHookIsInput(t *testing.T) {
	inputHooks := []firewall.ChainHook{firewall.HookInput, firewall.HookForward, firewall.HookPrerouting, firewall.HookIngress}
	outputHooks := []firewall.ChainHook{firewall.HookOutput, firewall.HookPostrouting, firewall.HookEgress}
	for _, h := range inputHooks {
		if !hookIsInput(h) {
			t.Errorf("%s should be input", h)
		}
	}
	for _, h := range outputHooks {
		if hookIsInput(h) {
			t.Errorf("%s should be output", h)
		}
	}
}

func TestApplyDumpInterfacesFailure(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(nil)
	fake.dumpErr = fmt.Errorf("VPP not connected")

	err := applyWithOpsLocked(b, fake, oneChainTable())
	if err == nil {
		t.Fatal("expected error when dumpInterfaces fails")
	}
	if !strings.Contains(err.Error(), "VPP not connected") {
		t.Errorf("want VPP not connected in error, got %v", err)
	}
}

func TestApplyBindFailureTriggersUndo(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.bindFailOn[5] = fmt.Errorf("bind rejected")

	err := applyWithOpsLocked(b, fake, oneChainTable())
	if err == nil {
		t.Fatal("expected error when bind fails")
	}
	if fake.countPrefix("del:") < 1 {
		t.Error("bind failure should trigger undo (delete created ACLs)")
	}
}

func TestReconcileRemovalsDeleteFailureContinues(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	twoChains := []firewall.Table{{
		Name:   "wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{
			{Name: "input", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookInput, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t1", Actions: []firewall.Action{firewall.Accept{}}}}},
			{Name: "forward", IsBase: true, Type: firewall.ChainFilter, Hook: firewall.HookForward, Policy: firewall.PolicyDrop,
				Terms: []firewall.Term{{Name: "t2", Actions: []firewall.Action{firewall.Drop{}}}}},
		},
	}}
	if err := applyWithOpsLocked(b, fake1, twoChains); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	for _, idx := range b.aclIndexes {
		fake2.delFailOn[idx] = fmt.Errorf("delete failed")
	}
	err := applyWithOpsLocked(b, fake2, oneChainTable())
	if err != nil {
		t.Fatalf("reconcile delete failure should not fail Apply: %v", err)
	}
}

func TestCleanupStartupOrphansDeleteFailure(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.existingACL = []aclDumpEntry{
		{Index: 10, Tag: "ze/old/chain"},
	}
	fake.delFailOn[10] = fmt.Errorf("delete orphan failed")

	err := applyWithOpsLocked(b, fake, oneChainTable())
	if err == nil {
		t.Fatal("expected error when orphan cleanup delete fails")
	}
}

func TestListTablesReturnsLastApplied(t *testing.T) {
	b := newOpsBackend()
	tables, err := b.ListTables()
	if err != nil {
		t.Fatalf("ListTables on fresh backend: %v", err)
	}
	if tables != nil {
		t.Errorf("fresh backend should return nil, got %v", tables)
	}

	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake, oneChainTable()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	tables, err = b.ListTables()
	if err != nil {
		t.Fatalf("ListTables after apply: %v", err)
	}
	if len(tables) != 1 {
		t.Errorf("want 1 table, got %d", len(tables))
	}
}

func TestGetCountersReturnsNil(t *testing.T) {
	b := newOpsBackend()
	counters, err := b.GetCounters("wan")
	if err != nil {
		t.Fatalf("GetCounters: %v", err)
	}
	if counters != nil {
		t.Errorf("want nil counters, got %v", counters)
	}
}

func TestApplyUpdateUsesReplaceIndex(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, oneChainTable()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if len(b.aclIndexes) != 1 {
		t.Fatalf("want 1 ACL index, got %d", len(b.aclIndexes))
	}

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, oneChainTable()); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if fake2.countPrefix("addReplace:") != 1 {
		t.Errorf("update should call addReplace once, got %d", fake2.countPrefix("addReplace:"))
	}
	if fake2.countPrefix("del:") != 0 {
		t.Errorf("update should not delete, got %d del calls", fake2.countPrefix("del:"))
	}
}

func equalU32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
