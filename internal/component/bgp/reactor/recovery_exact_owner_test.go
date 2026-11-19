// Design: docs/architecture/update-building.md -- source exclusions retain exact owners.
package reactor

import (
	"slices"
	"testing"
)

func TestRecoveryProducerDoesNotLiftSameAddressOwnerExclusion(t *testing.T) {
	original, _ := newAnnouncePeer(t, "192.0.2.1")
	alias, _ := newAnnouncePeer(t, "192.0.2.1")
	if original.sourceOwner == 0 || alias.sourceOwner == original.sourceOwner {
		t.Fatal("real Peer constructor did not allocate distinct original owners")
	}
	record := adjOutOwed{}
	record.exclude(adjOutExclusion{source: original.sourceOwner, generation: original.forwardGeneration.Load()})
	record.noteAnnouncer(adjOutPath{source: alias, generation: alias.forwardGeneration.Load()}, false)
	excluded := record.electableExcluded()
	if !slices.Contains(excluded, original.sourceOwner) || slices.Contains(excluded, alias.sourceOwner) {
		t.Fatalf("actual owed producer confused aliases: got %v", excluded)
	}
	// The positive control uses the original owner and its current receipt.
	record.noteAnnouncer(adjOutPath{source: original, generation: original.forwardGeneration.Load()}, false)
	if slices.Contains(record.electableExcluded(), original.sourceOwner) {
		t.Fatal("original owner's current announcement did not lift its exclusion")
	}
}

func TestRecoverySourceFactsMatchOriginalReceiptAndOwner(t *testing.T) {
	original, facts := sourceReceiptFixture(t)
	alias, aliasFacts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(live.close)
	other, err := store.issue(aliasFacts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.close)
	got := store.recoverySource(live.ID(), original.sourceOwner)
	if !got.ok || got.info.peer != original || got.receipt != live.ID() || got.ctxID != facts.source.ctxID {
		t.Fatalf("recovery changed original receipt facts: %+v", got)
	}
	if store.recoverySource(live.ID(), alias.sourceOwner).ok || store.recoverySource(other.ID(), original.sourceOwner).ok {
		t.Fatal("same-address owner accepted another owner's original receipt")
	}
	if store.recoverySource(0, original.sourceOwner).ok || store.recoverySource(live.ID(), 0).ok {
		t.Fatal("zero identity granted recovery facts")
	}
}

func TestRecoveryWorkerPublishesOriginalDestinationOwner(t *testing.T) {
	rail := newOwedRail(t)
	prefix := []byte{24, 192, 0, 2}
	rail.forward(t, false, prefix)
	rail.forward(t, true, prefix)
	request := rail.request(t)
	if request.DestinationOwner != rail.destination.sourceOwner || request.DestinationOwner == 0 {
		t.Fatalf("actual recovery producer lost destination owner: got %d, want %d", request.DestinationOwner, rail.destination.sourceOwner)
	}
	if request.Destination != rail.destination.Settings().Address {
		t.Fatal("recovery producer lost destination display address")
	}
	rail.release <- nil
}
