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
