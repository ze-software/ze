// Design: docs/architecture/mpls/mpls-kernel.md -- forwarding reader registry tests
package mplsfib

import (
	"errors"
	"maps"
	"testing"
)

// withForwardingReaders snapshots the reader registry and restores it after
// the test, so a test that registers never poisons the next one.
func withForwardingReaders(t *testing.T) {
	t.Helper()
	forwardingMu.Lock()
	saved := maps.Clone(forwardingReaders)
	forwardingReaders = map[string]ForwardingReader{}
	forwardingMu.Unlock()
	t.Cleanup(func() {
		forwardingMu.Lock()
		forwardingReaders = saved
		forwardingMu.Unlock()
	})
}

// TestForwardingReaderRegistry proves a data plane's reader is found by the
// name it registered under, a second reader for that name is refused, and an
// unregistered name answers false rather than a reader that reads nothing.
//
// Method: register a reader for "vpp", look it up, register again, look up
// "netlink".
func TestForwardingReaderRegistry(t *testing.T) {
	withForwardingReaders(t)
	want := []ForwardingEntry{{InLabel: 100, Operation: "pop"}}
	reader := func(int) ([]ForwardingEntry, error) { return want, nil }

	if err := RegisterForwardingReader("vpp", reader); err != nil {
		t.Fatalf("first registration refused: %v", err)
	}
	got, ok := ForwardingReaderFor("vpp")
	if !ok {
		t.Fatal("registered reader not found")
	}
	entries, err := got(0)
	if err != nil || len(entries) != 1 || entries[0].InLabel != 100 {
		t.Fatalf("lookup answered another reader: %v %v", entries, err)
	}

	err = RegisterForwardingReader("vpp", reader)
	if !errors.Is(err, errForwardingDuplicate) {
		t.Fatalf("second reader for one data plane: got %v, want errForwardingDuplicate", err)
	}
	if _, ok := ForwardingReaderFor("netlink"); ok {
		t.Fatal("an unregistered data plane answered a reader")
	}
}

// TestForwardingReaderRegistryRefusesEmpty proves an empty name and a nil
// reader are refused, so the lookup can never answer a reader for "" or one
// that panics on call.
func TestForwardingReaderRegistryRefusesEmpty(t *testing.T) {
	withForwardingReaders(t)
	if err := RegisterForwardingReader("", func(int) ([]ForwardingEntry, error) { return nil, nil }); !errors.Is(err, errForwardingNoDataPlane) {
		t.Fatalf("empty data plane: got %v", err)
	}
	if err := RegisterForwardingReader("vpp", nil); !errors.Is(err, errForwardingNilReader) {
		t.Fatalf("nil reader: got %v", err)
	}
}

// TestLabelOperation proves the classification every reader shares: no
// outgoing label and Implicit NULL alone are a pop, anything else a swap.
func TestLabelOperation(t *testing.T) {
	cases := []struct {
		out  []int
		want string
	}{
		{nil, "pop"},
		{[]int{3}, "pop"},
		{[]int{16001}, "swap"},
		{[]int{16001, 16002}, "swap"},
		{[]int{3, 16001}, "swap"},
	}
	for _, c := range cases {
		if got := LabelOperation(c.out); got != c.want {
			t.Errorf("LabelOperation(%v) = %q, want %q", c.out, got, c.want)
		}
	}
}
