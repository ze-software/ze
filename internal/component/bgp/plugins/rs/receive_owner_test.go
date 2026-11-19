// Design: docs/architecture/plugin/rib-storage-design.md -- retained receive provenance.
package rs

import "testing"

// TestReceiveOwnerInventoryKeepsAliases proves the text RECEIVE producer retains
// both owners of one prefix. These passive DTO identities grant no send authority.
func TestReceiveOwnerInventoryKeepsAliases(t *testing.T) {
	rs := receiveOwnerInventory(t)
	entries := rs.withdrawals["192.0.2.1"]
	if len(entries) != 2 {
		t.Fatalf("same-address aliases retained %d entries, want 2", len(entries))
	}
	want := map[uint64][3]uint64{41: {141, 5, 241}, 42: {142, 6, 242}}
	for _, entry := range entries {
		if got := [3]uint64{entry.sourceReceipt, entry.sourcePolicyEpoch, entry.connectionID}; got != want[entry.owner] {
			t.Fatalf("receive provenance changed: %+v", entry)
		}
		delete(want, entry.owner)
	}
	if len(want) != 0 {
		t.Fatalf("missing original owners: %v", want)
	}
}

// TestReceiveOwnerDownPreservesAlias proves taking one DOWN leaves the same
// prefix from the other original owner available for its own later withdrawal.
func TestReceiveOwnerDownPreservesAlias(t *testing.T) {
	rs := receiveOwnerInventory(t)
	taken := rs.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241)
	if len(taken) != 1 {
		t.Fatalf("owner 41 DOWN took %d routes, want 1", len(taken))
	}
	for _, entry := range taken {
		if entry.owner != 41 {
			t.Fatalf("DOWN borrowed alias owner: %+v", entry)
		}
		if entry.sourceReceipt != 141 {
			t.Fatalf("DOWN changed original receipt: %+v", entry)
		}
	}
	survivors := rs.takeWithdrawalEntriesLocked("192.0.2.1", 42, 242)
	if len(survivors) != 1 {
		t.Fatalf("alias survivor lost: got %d routes", len(survivors))
	}
}

// TestReceiveOwnerLateDownIsIdempotent proves an old owner's repeated DOWN
// cannot take the newer alias after the original owner's routes were taken.
func TestReceiveOwnerLateDownIsIdempotent(t *testing.T) {
	rs := receiveOwnerInventory(t)
	rs.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241)
	if taken := rs.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241); len(taken) != 0 {
		t.Fatalf("late DOWN took %d foreign routes", len(taken))
	}
	if got := len(rs.withdrawals["192.0.2.1"]); got != 1 {
		t.Fatalf("late DOWN left %d survivors, want 1", got)
	}
}

func receiveOwnerInventory(t *testing.T) *routeServer {
	t.Helper()
	rs := &routeServer{withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry)}
	ops := map[string][]FamilyOperation{"ipv4/unicast": {{Action: actionAdd, NLRIs: []any{"prefix 203.0.113.0/24"}}}}
	rs.updateWithdrawalMapText("192.0.2.1", ops, 5, 141, 41, 241)
	rs.updateWithdrawalMapText("192.0.2.1", ops, 6, 142, 42, 242)
	return rs
}
