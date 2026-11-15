// Design: docs/architecture/irr-owner.md -- what the owner stores of a refresh
package irr

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// mergingResolver answers every fetch with one fixed entry and no error: the
// shape store.PrefixStore.Refresh returns when one family answered and the
// cached prefixes of the other were kept (store
// TestRefreshKeepsLastGoodPerFamily). Every other call goes to the embedded
// fake. Called from the worker only.
type mergingResolver struct {
	*fakeResolver
	merged store.CachedEntry
}

func (r *mergingResolver) refresh(_ context.Context, asSet string) (*store.CachedEntry, error) {
	r.queries = append(r.queries, asSet)
	entry := r.merged
	return &entry, nil
}

// VALIDATES: AC-1 -- an answer that learned one family, merged with the kept
// prefixes of the other and marked stale, is stored with both families, its
// refreshed-at and its stale-since, and a restarted owner seeds that copy back
// as the next merge base with both families and its staleness. (Replaces the
// reopen halves of store TestRefreshKeepsLastGoodPerFamily and
// TestRefreshKeepsLastGoodOnEmptyAnswer, "staleness must survive a restart".)
// PREVENTS: a family kept in memory but lost from the stored copy, so the
// firewall loses that family's accept term at the next start; or staleness lost
// across a restart, so a list nobody confirmed reads as fresh.
// METHOD: an owner whose resolver returns a stale two-family entry fetches the
// set at boot; decode the stored dataset; boot a second owner over the same
// store ten minutes later and read what it seeded.
func TestOneFamilyAnswerStoredStaleAcrossRestart(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	merged := store.CachedEntry{
		Name: "AS-TEST", ASSet: "AS-TEST",
		IPv4:        prefixes("192.0.2.0/24", "198.51.100.0/24"),
		IPv6:        prefixes("2001:db8::/32"),
		RefreshedAt: h.clock.at.Add(-2 * time.Hour),
		StaleSince:  h.clock.at.Add(-time.Hour),
	}
	merging := &mergingResolver{fakeResolver: h.resolver, merged: merged}
	h.owner = newOwner(state, h.notices, func(ownerConfig) resolver { return merging })
	h.owner.clock = h.clock.now
	if err := h.owner.configure(defaultOwnerConfig()); err != nil {
		t.Fatal(err)
	}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})

	h.boot()

	if !slices.Equal(merging.queries, []string{"AS-TEST"}) {
		t.Fatalf("boot fetched %v, want AS-TEST once: without the fetch the test proves nothing", merging.queries)
	}
	dataset := h.dataset("AS-TEST")
	if !slices.Equal(dataset.IPv4(), merged.IPv4) || !slices.Equal(dataset.IPv6(), merged.IPv6) {
		t.Fatalf("stored %v and %v, want both families %v and %v", dataset.IPv4(), dataset.IPv6(), merged.IPv4, merged.IPv6)
	}
	if !dataset.RefreshedAt().Equal(merged.RefreshedAt) || !dataset.StaleSince().Equal(merged.StaleSince) {
		t.Fatalf("stored refreshed-at %s stale-since %s, want %s and %s",
			dataset.RefreshedAt(), dataset.StaleSince(), merged.RefreshedAt, merged.StaleSince)
	}

	second := newHarness(t, state, defaultOwnerConfig())
	second.clock.at = h.clock.at.Add(10 * time.Minute)
	second.boot()

	if len(second.resolver.queries) != 0 {
		t.Fatalf("restart queried %v, want nothing", second.resolver.queries)
	}
	seeded, ok := second.resolver.seeded["AS-TEST"]
	if !ok {
		t.Fatal("restart did not seed the stored list as the merge base")
	}
	if !slices.Equal(seeded.IPv4, merged.IPv4) || !slices.Equal(seeded.IPv6, merged.IPv6) {
		t.Fatalf("restart seeded %v and %v, want both families %v and %v", seeded.IPv4, seeded.IPv6, merged.IPv4, merged.IPv6)
	}
	if !seeded.StaleSince.Equal(merged.StaleSince) || !seeded.RefreshedAt.Equal(merged.RefreshedAt) {
		t.Fatalf("restart seeded refreshed-at %s stale-since %s, want %s and %s: staleness must survive a restart",
			seeded.RefreshedAt, seeded.StaleSince, merged.RefreshedAt, merged.StaleSince)
	}
}
