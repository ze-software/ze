// Design: docs/guide/graceful-restart.md -- verdicts belong to state lifetimes.

package retention

import (
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestLedgerUnownedVerdictDoesNotCreateLifetime feeds valid retained and empty
// verdicts without an owning state UP. Neither may allocate peer lifetime state.
func TestLedgerUnownedVerdictDoesNotCreateLifetime(t *testing.T) {
	var ledger Ledger
	for _, families := range [][]family.Family{{family.IPv4Unicast}, nil} {
		payload, err := Encode("192.0.2.1", families)
		if err != nil {
			t.Fatal(err)
		}
		envelope, ok := Accepts(envelopeFor(t, payload))
		if !ok {
			t.Fatal("retention envelope not accepted")
		}
		if err := ledger.Record(envelope); err != nil {
			t.Fatal(err)
		}
		if len(ledger.verdicts) != 0 {
			t.Fatalf("unowned verdict created %d peer records", len(ledger.verdicts))
		}
	}
}

// TestLedgerLifetimeChurn keeps one active owner while dynamic owned peers come
// and go. Both populated and empty verdicts consume exactly their active slot;
// a recreated address starts without its previous lifetime's verdict.
func TestLedgerLifetimeChurn(t *testing.T) {
	var ledger Ledger
	const active = "192.0.2.1"
	ledger.Begin(active)
	record := func(peer string, families []family.Family) {
		t.Helper()
		payload, err := Encode(peer, families)
		if err != nil {
			t.Fatal(err)
		}
		envelope, ok := Accepts(envelopeFor(t, payload))
		if !ok {
			t.Fatal("retention envelope not accepted")
		}
		if err := ledger.Record(envelope); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 64 {
		peer := "198.51.100." + strconv.Itoa(i+1)
		ledger.Begin(peer)
		var families []family.Family
		if i%2 == 0 {
			families = []family.Family{family.IPv4Unicast}
		}
		record(peer, families)
		if len(ledger.verdicts) != 2 {
			t.Fatalf("active owner plus dynamic peer hold %d lifetimes", len(ledger.verdicts))
		}
		kept := ledger.Take(peer)
		if kept[family.IPv4Unicast] != (i%2 == 0) {
			t.Fatal("dynamic peer lost its family's retention decision")
		}
		record(peer, []family.Family{family.IPv4Unicast})
		if len(ledger.verdicts) != 1 {
			t.Fatal("dynamic DOWN or late verdict leaked a lifetime")
		}
	}
	record(active, []family.Family{family.IPv4Unicast})
	ledger.Begin(active)
	if kept := ledger.Take(active); len(kept) != 0 {
		t.Fatal("recreated peer inherited the previous lifetime's verdict")
	}
	if len(ledger.verdicts) != 0 {
		t.Fatal("final DOWN left active lifetime storage")
	}
}
