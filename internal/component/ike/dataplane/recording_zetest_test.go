//go:build zetest

package dataplane

import (
	"net"
	"testing"
)

// VALIDATES: in a zetest build the recording backend upserts a policy by its kernel
// identity, removes it by the same identity, and lists what it holds with the
// catch-all priority last.
// METHOD: install a wildcard entry twice with different actions and a narrower entry,
// list, remove the wildcard, list again.
// PREVENTS: a .ci assertion of the catch-all reading a list that duplicates an upsert
// or keeps an entry the engine removed.
func TestRecordingBackendRecordsTheSPD(t *testing.T) {
	backend, err := newRecordingBackend()
	if err != nil {
		t.Fatal(err)
	}
	_, anyNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatal(err)
	}
	_, narrow, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	catchAll := SPParams{Src: anyNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard, Priority: PriorityUnmatched}
	for _, action := range []SPAction{SPActionDiscard, SPActionBypass} {
		catchAll.Action = action
		if err := backend.InstallPolicy(catchAll); err != nil {
			t.Fatal(err)
		}
	}
	if err := backend.InstallPolicy(SPParams{Src: narrow, Dst: anyNet, Dir: SADirIn, Action: SPActionBypass, Priority: 100}); err != nil {
		t.Fatal(err)
	}
	listed, err := backend.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed %d policies, want 2 (the upsert replaces): %+v", len(listed), listed)
	}
	if listed[1].Priority != int(PriorityUnmatched) || listed[1].Action != SPActionBypass {
		t.Fatalf("last policy = %+v, want the catch-all with the action of the second install", listed[1])
	}
	if err := backend.RemovePolicyParams(catchAll); err != nil {
		t.Fatal(err)
	}
	listed, err = backend.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Priority != 100 {
		t.Fatalf("after removing the catch-all: %+v, want the narrow entry alone", listed)
	}
}

// VALIDATES: the recording backend lists ICMPErrorYield on the inbound discard of the
// node-wide and the interface catch-all alone, as the Linux readback does from the
// XFRM_POLICY_ICMP flag the backend sets there (catchAllYieldsToICMPErrorCheck).
// METHOD: install both catch-all inbound discards, the node-wide catch-all's outbound
// discard and an operator's inbound wildcard discard, then list and read the flag of
// each by its role and direction.
// PREVENTS: a .ci on the recording backend asserting icmp-error-yield false on a
// catch-all whose kernel entry yields to the ICMP reverse check, or true on an
// operator's discard that the check must honor.
func TestRecordingBackendListsTheCatchAllICMPErrorYield(t *testing.T) {
	backend, err := newRecordingBackend()
	if err != nil {
		t.Fatal(err)
	}
	_, anyNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatal(err)
	}
	_, operatorNet, err := net.ParseCIDR("198.51.100.0/24")
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		params SPParams
		yield  bool
	}
	entries := []entry{
		{SPParams{Src: anyNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard, Priority: PriorityUnmatched, Role: PolicyRoleCatchAll}, true},
		{SPParams{Src: anyNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard, Priority: PriorityUnmatched, IfID: 7, SPD: "xfrm7", Role: PolicyRoleInterfaceCatchAll}, true},
		{SPParams{Src: anyNet, Dst: anyNet, Dir: SADirOut, Action: SPActionDiscard, Priority: PriorityUnmatched, Role: PolicyRoleCatchAll}, false},
		{SPParams{Src: operatorNet, Dst: anyNet, Dir: SADirIn, Action: SPActionDiscard, Priority: 100, Role: PolicyRoleOperator}, false},
	}
	for _, e := range entries {
		if err := backend.InstallPolicy(e.params); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := backend.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(entries) {
		t.Fatalf("listed %d policies, want %d: %+v", len(listed), len(entries), listed)
	}
	for _, e := range entries {
		found := false
		for _, p := range listed {
			if p.Role != e.params.Role || p.Dir != e.params.Dir || p.IfID != e.params.IfID {
				continue
			}
			found = true
			if p.ICMPErrorYield != e.yield {
				t.Errorf("%s dir %d if_id %d: ICMPErrorYield %t, want %t", p.Role, p.Dir, p.IfID, p.ICMPErrorYield, e.yield)
			}
		}
		if !found {
			t.Errorf("%s dir %d if_id %d: not listed: %+v", e.params.Role, e.params.Dir, e.params.IfID, listed)
		}
	}
}

// VALIDATES: recordingDataplane implements SACounterChecker and answers nil so zetest
// fixtures can commit esp-group volume lifetimes.
// PREVENTS: recordingDataplane refusing volume lifetimes during commit verify in tests.
func TestRecordingSACountersSupported(t *testing.T) {
	var checker SACounterChecker = &recordingDataplane{}
	if err := checker.SACountersSupported(); err != nil {
		t.Errorf("recordingDataplane.SACountersSupported() = %v, want nil", err)
	}
}
