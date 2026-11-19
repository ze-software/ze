// Design: docs/architecture/plugin/rib-storage-design.md -- retained receive provenance.
package rr

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestReceiveOwnerInventoryKeepsAliases proves the text RECEIVE producer retains
// both owners of one prefix. These passive DTO identities grant no send authority.
func TestReceiveOwnerInventoryKeepsAliases(t *testing.T) {
	rr := receiveOwnerInventory(t)
	entries := rr.withdrawals["192.0.2.1"]
	if len(entries) != 2 {
		t.Fatalf("same-address aliases retained %d entries, want 2", len(entries))
	}
	want := map[uint64][2]uint64{41: {141, 5}, 42: {142, 6}}
	for _, entry := range entries {
		if got := [2]uint64{entry.SourceReceipt, entry.SourcePolicyEpoch}; got != want[entry.SourceOwner] {
			t.Fatalf("receive provenance changed: %+v", entry)
		}
		delete(want, entry.SourceOwner)
	}
	if len(want) != 0 {
		t.Fatalf("missing original owners: %v", want)
	}
}

// TestReceiveOwnerDownPreservesAlias proves taking one DOWN leaves the same
// prefix from the other original owner available for its own later withdrawal.
func TestReceiveOwnerDownPreservesAlias(t *testing.T) {
	rr := receiveOwnerInventory(t)
	taken := rr.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241)
	if len(taken) != 1 {
		t.Fatalf("owner 41 DOWN took %d routes, want 1", len(taken))
	}
	for _, entry := range taken {
		if entry.SourceOwner != 41 {
			t.Fatalf("DOWN borrowed alias owner: %+v", entry)
		}
		if entry.SourceReceipt != 141 {
			t.Fatalf("DOWN changed original receipt: %+v", entry)
		}
	}
	survivors := rr.takeWithdrawalEntriesLocked("192.0.2.1", 42, 242)
	if len(survivors) != 1 {
		t.Fatalf("alias survivor lost: got %d routes", len(survivors))
	}
}

// TestReceiveOwnerLateDownIsIdempotent proves an old owner's repeated DOWN
// cannot take the newer alias after the original owner's routes were taken.
func TestReceiveOwnerLateDownIsIdempotent(t *testing.T) {
	rr := receiveOwnerInventory(t)
	rr.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241)
	if taken := rr.takeWithdrawalEntriesLocked("192.0.2.1", 41, 241); len(taken) != 0 {
		t.Fatalf("late DOWN took %d foreign routes", len(taken))
	}
	if got := len(rr.withdrawals["192.0.2.1"]); got != 1 {
		t.Fatalf("late DOWN left %d survivors, want 1", got)
	}
}

func receiveOwnerInventory(t *testing.T) *routeReflector {
	t.Helper()
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	ops := map[family.Family][]familyOperation{family.IPv4Unicast: {{action: actionAdd, nlris: []string{"prefix 203.0.113.0/24"}}}}
	rr.updateWithdrawalMapText("192.0.2.1", ops, 5, 141, 41, 241, 51, false)
	rr.updateWithdrawalMapText("192.0.2.1", ops, 6, 142, 42, 242, 52, false)
	return rr
}
