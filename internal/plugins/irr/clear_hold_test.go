// Design: docs/architecture/irr-owner.md -- `clear irr` and the hold it leaves
package irr

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// VALIDATES: AC-16 -- `clear irr as-set` of a set whose dataset write was
// refused, with its retry armed, ends that retry: no later pass fetches the set
// or writes its dataset or pointer. (Replaces store TestPurgeAfterFailedPersist
// and TestPurgeDropsPendingPersist, whose pending write is now the owner's
// retry.)
// PREVENTS: a retry armed before the clear resurrecting the list the operator
// removed.
// METHOD: refuse the dataset write at boot so the retry is armed, clear the
// set, lift the refusal, run a pass two hours later, read the store and the
// resolver.
func TestClearAfterRefusedWriteStoresNothing(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	datasetPrefix := store.DatasetPrefix + "whois.radb.net/AS-TEST/"
	state.failPutPrefix[datasetPrefix] = errors.New("store full")
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	if at, ok := h.owner.schedule.at("AS-TEST"); !ok || !at.Equal(h.clock.at.Add(retryInterval)) {
		t.Fatalf("refused write armed %s ok=%v, want a retry in a minute: without it the test proves nothing", at, ok)
	}

	answer := h.owner.clearASSet(context.Background(), "AS-TEST")
	cleared, ok := answer.data.(clearAnswer)
	if answer.err != nil || !ok {
		t.Fatalf("clear answered %+v %v", answer.data, answer.err)
	}
	if cleared.Removed {
		t.Error("clear answered removed, but nothing was stored")
	}
	if !slices.Equal(cleared.Consumers, []string{consumerBGP}) {
		t.Errorf("clear named %v, want %v", cleared.Consumers, []string{consumerBGP})
	}

	delete(state.failPutPrefix, datasetPrefix)
	h.resolver.queries = nil
	h.clock.at = h.clock.at.Add(2 * time.Hour)
	h.pass()

	if len(h.resolver.queries) != 0 {
		t.Fatalf("a cleared set was fetched again: %v", h.resolver.queries)
	}
	if len(state.keysUnder(datasetPrefix)) != 0 || state.has(store.PointerKey("AS-TEST")) {
		t.Fatal("a pass after the clear stored the set the operator removed")
	}
}
