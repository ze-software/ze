// Design: docs/research/vpp-deployment-reference.md -- runtime statistics ownership
// Related: runtime_stats_counters.go -- the snapshot index under test
package vpp

import (
	"testing"
	"time"

	"go.fd.io/govpp/api"
)

// Goal: the snapshot index answers each interface's own row and refuses a
// missing or duplicated index, in the default unit build.
// Method: index one dump directly, then index a second dump into the same map
// to prove rows from the earlier dump do not survive.
// MUTATION: letting a duplicate's last row win, answering a zero row for an
// absent index, or skipping the clear between dumps turns this red.
func TestInterfaceCounterSnapshotIndex(t *testing.T) {
	var snapshot interfaceCounterSnapshot
	if !snapshot.expired(time.Unix(1, 0)) {
		t.Fatal("an empty snapshot answered a bound")
	}
	snapshot.dump.Interfaces = []api.InterfaceCounters{
		{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 7}},
		{InterfaceIndex: 18, Rx: api.InterfaceCounterCombined{Bytes: 1}},
		{InterfaceIndex: 18, Rx: api.InterfaceCounterCombined{Bytes: 2}},
	}
	taken := time.Unix(1_700_000_000, 0)
	snapshot.index(taken)
	row, err := snapshot.lookup(17)
	if err != nil {
		t.Fatal(err)
	}
	if row.Rx.Bytes != 7 {
		t.Fatalf("index 17 rx bytes %d", row.Rx.Bytes)
	}
	if _, err := snapshot.lookup(18); err == nil {
		t.Fatal("a duplicated index answered one of its rows")
	}
	if _, err := snapshot.lookup(19); err == nil {
		t.Fatal("an absent index answered")
	}
	if snapshot.expired(taken) {
		t.Fatal("a snapshot expired for its own start time")
	}
	if !snapshot.expired(taken.Add(time.Nanosecond)) {
		t.Fatal("a snapshot answered a bound after it started")
	}
	snapshot.dump.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 18, Rx: api.InterfaceCounterCombined{Bytes: 3}}}
	snapshot.index(taken.Add(time.Minute))
	if _, err := snapshot.lookup(17); err == nil {
		t.Fatal("a row from the previous dump survived the next one")
	}
	row, err = snapshot.lookup(18)
	if err != nil {
		t.Fatalf("an index unique in the new dump stayed a duplicate: %v", err)
	}
	if row.Rx.Bytes != 3 {
		t.Fatalf("index 18 rx bytes %d", row.Rx.Bytes)
	}
}
