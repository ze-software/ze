// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- what the virtual-MAC macvlan receives in Backup
// Related: backupfilter.go -- the filter under test
// Related: vmac_state_integration_linux_test.go -- the same filter observed on the wire
package vrrp

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// TestBackupFilterTablesDropWhatTheMacvlanReceives checks the table the backup
// filter publishes.
//
// Method: two devices, one listed twice, give one inet table with one
// prerouting base chain that accepts by policy and drops, per device, what that
// device receives, sorted and deduplicated. No device gives no table.
func TestBackupFilterTablesDropWhatTheMacvlanReceives(t *testing.T) {
	if got := backupFilterTables(nil); got != nil {
		t.Fatalf("no device published %v, want no table", got)
	}

	tables := backupFilterTables([]string{"zv6-2-20", "zv4-2-10", "zv6-2-20"})
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	table := tables[0]
	if table.Name != backupFilterTableName || table.Family != firewall.FamilyInet {
		t.Fatalf("table %q family %v, want %q inet", table.Name, table.Family, backupFilterTableName)
	}
	if len(table.Chains) != 1 {
		t.Fatalf("got %d chains, want 1", len(table.Chains))
	}
	chain := table.Chains[0]
	if !chain.IsBase || chain.Hook != firewall.HookPrerouting || chain.Policy != firewall.PolicyAccept {
		t.Fatalf("chain base=%v hook=%v policy=%v, want a prerouting base chain accepting by policy",
			chain.IsBase, chain.Hook, chain.Policy)
	}
	var devices []string
	for _, term := range chain.Terms {
		if len(term.Matches) != 1 || len(term.Actions) != 1 {
			t.Fatalf("term %q has %d matches and %d actions, want one of each", term.Name, len(term.Matches), len(term.Actions))
		}
		match, ok := term.Matches[0].(firewall.MatchInputInterface)
		if !ok || match.Wildcard {
			t.Fatalf("term %q matches %#v, want an exact input interface", term.Name, term.Matches[0])
		}
		if _, ok := term.Actions[0].(firewall.Drop); !ok {
			t.Fatalf("term %q acts %#v, want drop", term.Name, term.Actions[0])
		}
		devices = append(devices, match.Name)
	}
	if want := []string{"zv4-2-10", "zv6-2-20"}; !slices.Equal(devices, want) {
		t.Fatalf("terms name %v, want %v", devices, want)
	}
}

// TestBackupFilterPublishesAndWithdraws checks the filter's reconcile calls.
//
// Method: the publisher is replaced by a recorder. Setting an entry publishes
// the table, setting the same entry again publishes nothing, and clearing the
// last entry publishes no table, so the kernel table is removed.
func TestBackupFilterPublishesAndWithdraws(t *testing.T) {
	var published [][]firewall.Table
	saved := backupFilterPublish
	backupFilterPublish = func(tables []firewall.Table) error {
		published = append(published, tables)
		return nil
	}
	t.Cleanup(func() { backupFilterPublish = saved })

	if err := setBackupFilter("vrrp:test-backup", "zv4-2-10"); err != nil {
		t.Fatal(err)
	}
	if err := setBackupFilter("vrrp:test-backup", "zv4-2-10"); err != nil {
		t.Fatal(err)
	}
	if err := clearBackupFilter("vrrp:test-backup"); err != nil {
		t.Fatal(err)
	}
	if err := clearBackupFilter("vrrp:test-backup"); err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 {
		t.Fatalf("published %d times, want 2 (set once, withdraw once)", len(published))
	}
	if len(published[0]) != 1 || len(published[1]) != 0 {
		t.Fatalf("published %v then %v, want one table then none", published[0], published[1])
	}
}

// TestBackupDiscardFollowsTheState checks when an instance drops what its
// virtual-MAC device receives.
//
// Method: a worker whose parent is never ready stays in Initialize; it sets the
// discard when it starts and withdraws it when it stops. A second instance is
// driven through the FSM: Backup keeps the discard (set by run, not repeated),
// promotion withdraws it after the addresses are installed, and demotion by a
// higher-priority advertisement sets it again BEFORE the addresses are removed,
// because the macvlan forwards with or without an address.
func TestBackupDiscardFollowsTheState(t *testing.T) {
	idle, fi, _ := newTestInstance(t, testSpec())
	idle.deps.parentReady = func(string, string) bool { return false }
	go idle.run()
	idle.shutdown()
	if got := fi.discardCalls(); !slices.Equal(got, []string{"on:zv4-2-10", "off"}) {
		t.Fatalf("an Initialize worker's discard calls are %v, want [on:zv4-2-10 off]", got)
	}

	in, f, clk := newTestInstance(t, testSpec())
	removesAtDiscard := -1
	set := in.deps.setBackupFilter
	in.deps.setBackupFilter = func(owner, device string) error {
		removesAtDiscard = len(f.snapshot().removes)
		return set(owner, device)
	}
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if got := f.discardCalls(); len(got) != 0 {
		t.Fatalf("entering Backup made discard calls %v, want none beyond the worker's own", got)
	}

	clk.Add(10 * time.Second)
	deadline := time.After(2 * time.Second)
	for in.machine.State() != fsm.StateMaster {
		select {
		case ev := <-in.events:
			in.dispatch(ev)
		case <-deadline:
			t.Fatal("the master-down timer never promoted the router")
		}
	}
	if got := f.discardCalls(); !slices.Equal(got, []string{"off"}) {
		t.Fatalf("promotion made discard calls %v, want [off]", got)
	}
	if len(f.snapshot().installs) != 1 {
		t.Fatal("promotion installed no address")
	}

	in.dispatch(fsm.AdvertReceived{Priority: 254, SrcIP: netip.MustParseAddr("192.0.2.99"), IntervalMs: 1000, VIPCount: 1})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v after a higher-priority advertisement, want Backup", in.machine.State())
	}
	if got := f.discardCalls(); !slices.Equal(got, []string{"off", "on:zv4-2-10"}) {
		t.Fatalf("demotion made discard calls %v, want [off on:zv4-2-10]", got)
	}
	if removesAtDiscard != 0 || len(f.snapshot().removes) != 1 {
		t.Fatalf("discard set with %d removals done, %d in all: want it set before the only removal",
			removesAtDiscard, len(f.snapshot().removes))
	}
}

// discardCalls returns the backup-filter calls recorded so far.
func (f *fakeDeps) discardCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.discards)
}
