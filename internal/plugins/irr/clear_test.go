package irr

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// VALIDATES: `clear irr as-set` of a set nothing stores answers that nothing
// was removed and that no consumer references it, and the command without its
// argument is refused with the usage it needs. Neither asks the IRR server or
// PeeringDB anything. (Replaces firewall-irr TestClearFirewallIRRPurgesEntry,
// whose clear moved to the owner; the removal of a stored set is
// TestClearNamesConsumers.)
// METHOD: run both through handleCommand on a running worker over an empty
// store, then stop the worker and read every question the fake resolver got.
// PREVENTS: a clear of an absent set reading as a removal, a null consumer
// list in the answer, or a clear that fetches.
func TestClearIRRAsSetAbsentAndMissing(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	stop := h.runWorker()
	status, data, err := h.owner.handleCommand(cmdClearIRRAsSet, []string{"AS-NEVER"})
	missingStatus, _, missingErr := h.owner.handleCommand(cmdClearIRRAsSet, nil)
	stop()

	if status != statusDone || err != nil {
		t.Fatalf("clear of an absent set answered %q %v, want %q", status, err, statusDone)
	}
	cleared, ok := data.(clearAnswer)
	if !ok {
		t.Fatalf("clear answered %T, want clearAnswer", data)
	}
	if cleared.Removed {
		t.Errorf("clear of an absent set answered removed")
	}
	if cleared.Consumers == nil || len(cleared.Consumers) != 0 {
		t.Errorf("consumers %#v, want an empty list, never null", cleared.Consumers)
	}

	if missingStatus != statusError || missingErr == nil {
		t.Fatalf("clear with no argument answered %q %v, want %q and an error", missingStatus, missingErr, statusError)
	}
	if !strings.Contains(missingErr.Error(), "needs one <as-set>") {
		t.Errorf("clear with no argument refused with %q, want it to name the missing <as-set>", missingErr)
	}
	if len(h.resolver.queries) != 0 || len(h.resolver.asked) != 0 {
		t.Errorf("clear asked IRR %v and PeeringDB %v, want nothing", h.resolver.queries, h.resolver.asked)
	}
}

// VALIDATES: `clear irr asn` reads the AS number in asplain and in asdot, and
// removes the binding and the stored copy of exactly that AS number; a dotted
// value whose low half is past 16 bits is refused and removes nothing.
// (Replaces firewall-irr TestClearASNReadsEveryNotation, whose clear moved to
// the owner.)
// METHOD: bind AS 65546 and AS 100 to stored sets through a PeeringDB fake,
// clear by each spelling on a running worker, then read the store.
// PREVENTS: "1.10" read as AS 1 or 110, or "0.65546" narrowed into another AS
// number whose stored list the operator never asked to remove.
func TestClearIRRAsnReadsEveryNotation(t *testing.T) {
	rows := []struct {
		spelling string
		want     uint32
	}{
		{"65546", 65546},
		{"1.10", 65546},
		{"0.100", 100},
	}
	for _, row := range rows {
		t.Run(row.spelling, func(t *testing.T) {
			h := boundASNHarness(t)
			stop := h.runWorker()
			status, data, err := h.owner.handleCommand(cmdClearIRRAsn, []string{row.spelling})
			stop()
			if status != statusDone || err != nil {
				t.Fatalf("clear irr asn %s answered %q %v, want %q", row.spelling, status, err, statusDone)
			}
			cleared, ok := data.(clearAnswer)
			if !ok || cleared.ASN != row.want || !cleared.Removed {
				t.Fatalf("clear irr asn %s answered %+v, want the removal of AS %d", row.spelling, data, row.want)
			}
			if h.state.has(store.BindingKey(row.want)) {
				t.Errorf("clear irr asn %s left the binding of AS %d", row.spelling, row.want)
			}
			if h.state.has(store.PointerKey(boundSets[row.want])) {
				t.Errorf("clear irr asn %s left the pointer of %s", row.spelling, boundSets[row.want])
			}
			for other, asSet := range boundSets {
				if other == row.want {
					continue
				}
				if !h.state.has(store.BindingKey(other)) || !h.state.has(store.PointerKey(asSet)) {
					t.Errorf("clear irr asn %s removed AS %d or %s, which it does not name", row.spelling, other, asSet)
				}
			}
		})
	}

	t.Run("0.65546", func(t *testing.T) {
		h := boundASNHarness(t)
		stop := h.runWorker()
		// A no-op on the worker: it returns after the worker's start-up load
		// and first pass, so the log below holds every write but the clear's.
		h.owner.do(func(context.Context) commandAnswer { return commandAnswer{} })
		before := h.state.log()
		status, _, err := h.owner.handleCommand(cmdClearIRRAsn, []string{"0.65546"})
		stop()
		if status != statusError || err == nil || !strings.Contains(err.Error(), "is not an AS number") {
			t.Fatalf("clear irr asn 0.65546 answered %q %v, want a refusal naming it as no AS number", status, err)
		}
		if after := h.state.log(); !slices.Equal(after, before) {
			t.Errorf("a refused clear wrote to the store: %v", after[len(before):])
		}
		for asn, asSet := range boundSets {
			if !h.state.has(store.BindingKey(asn)) || !h.state.has(store.PointerKey(asSet)) {
				t.Errorf("a refused clear removed AS %d or %s", asn, asSet)
			}
		}
	})
}

// boundSets is the AS-SET PeeringDB names for each AS number boundASNHarness
// binds.
var boundSets = map[uint32]string{65546: "AS-BIG", 100: "AS-SMALL"}

// boundASNHarness boots an owner whose BGP consumer names AS 65546 and AS
// 100, each bound through PeeringDB to a set the boot stored.
func boundASNHarness(t *testing.T) *ownerHarness {
	t.Helper()
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	for asn, asSet := range boundSets {
		h.resolver.sets[asn] = asSet
		h.resolver.answers[asSet] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	}
	h.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65546, 100}})
	h.boot()
	for asn, asSet := range boundSets {
		if !h.state.has(store.BindingKey(asn)) || !h.state.has(store.PointerKey(asSet)) {
			t.Fatalf("boot did not bind AS %d to a stored %s", asn, asSet)
		}
	}
	return h
}
