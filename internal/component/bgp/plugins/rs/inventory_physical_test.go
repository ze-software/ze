// Design: docs/architecture/plugin/rib-storage-design.md -- current physical RS memberships.
package rs

import (
	"github.com/ze-software/ze/internal/core/family"
	"net/netip"
	"testing"
)

// The real departure extractor selects one physical connection. Current rows
// of the same owner on a successor and a named sibling at the same address
// remain available for useful later withdrawal; zero is never a wildcard.
func TestRSPhysicalDepartureKeepsSuccessorAndNamedSibling(t *testing.T) {
	source := "192.0.2.73"
	first := withdrawalKey{owner: 7301, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.73.1.0/24")}
	next := withdrawalKey{owner: 7301, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.73.2.0/24")}
	sibling := withdrawalKey{owner: 7302, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.73.3.0/24")}
	server := &routeServer{withdrawals: map[string]map[withdrawalKey]withdrawalEntry{source: {first: {owner: 7301, connectionID: 7311}, next: {owner: 7301, connectionID: 7312}, sibling: {owner: 7302, connectionID: 7321}}}}
	if got := server.takeWithdrawalEntriesLocked(source, 7301, 0); len(got) != 0 {
		t.Fatal("unrecorded connection selected rows")
	}
	got := server.takeWithdrawalEntriesLocked(source, 7301, 7311)
	if len(got) != 1 {
		t.Fatalf("original departure took %d rows", len(got))
	}
	if _, present := got[first]; !present {
		t.Fatal("original current row was not selected")
	}
	for _, key := range []withdrawalKey{next, sibling} {
		if _, present := server.withdrawals[source][key]; !present {
			t.Fatalf("useful unrelated physical membership disappeared: %+v", key)
		}
	}
}
