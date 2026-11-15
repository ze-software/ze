package irr

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// storedBinding reads the binding the owner stored for asn.
func (h *ownerHarness) storedBinding(asn uint32) (store.Binding, bool) {
	h.t.Helper()
	if !h.state.has(store.BindingKey(asn)) {
		return store.Binding{}, false
	}
	binding, err := store.DecodeBinding(h.state.value(store.BindingKey(asn)))
	if err != nil {
		h.t.Fatalf("stored binding of %d: %v", asn, err)
	}
	return binding, true
}

// VALIDATES: an update the servers do not answer keeps what is stored: `update
// irr as-set` answers an error, leaves the pointer and checked-at as they
// were, and records the failure (a retry a minute later, the show row
// failing); `update irr asn` with PeeringDB failing and nothing bound queries
// the AS number itself and leaves that set's stored pointer and checked-at as
// they were. (Replaces filter_irr TestUpdateASNFailurePreservesState, whose
// command moved to the owner.)
// METHOD: one owner stores AS-TEST and AS65010; a second owner over the same
// store, whose IRR server and PeeringDB fail, answers both commands on its
// running worker; its schedule and show irr are read once it stopped.
// PREVENTS: a failed update replacing or dropping the last good copy, a failed
// update that is never retried or shows as stored, or an AS number with no
// answer being queried under a guessed set.
func TestUpdateFailurePreservesState(t *testing.T) {
	state := newFakeState()
	first := newHarness(t, state, defaultOwnerConfig())
	first.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	first.resolver.answers["AS65010"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	first.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST", "AS65010"}})
	first.boot()
	before, ok := first.pointer("AS-TEST")
	if !ok {
		t.Fatal("the first owner stored no pointer for AS-TEST")
	}
	checkedAt := first.dataset("AS-TEST").CheckedAt()
	asnBefore, ok := first.pointer("AS65010")
	if !ok {
		t.Fatal("the first owner stored no pointer for AS65010")
	}
	asnCheckedAt := first.dataset("AS65010").CheckedAt()

	second := newHarness(t, state, defaultOwnerConfig())
	second.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
	second.resolver.answers["AS65010"] = fakeAnswer{err: errors.New("connection refused")}
	second.resolver.pdbErr = errors.New("peeringdb timeout")
	stop := second.runWorker()
	setStatus, _, setErr := second.owner.handleCommand(cmdUpdateIRRAsSet, []string{"AS-TEST"})
	asnStatus, _, asnErr := second.owner.handleCommand(cmdUpdateIRRAsn, []string{"65010"})
	stop()

	if setStatus != statusError || setErr == nil {
		t.Fatalf("update irr as-set AS-TEST: status %q err %v, want %q and an error", setStatus, setErr, statusError)
	}
	if after, _ := second.pointer("AS-TEST"); after != before {
		t.Fatalf("pointer %+v after a failed update, want unchanged %+v", after, before)
	}
	if got := second.dataset("AS-TEST").CheckedAt(); !got.Equal(checkedAt) {
		t.Fatalf("checked-at %s after a failed update, want unchanged %s", got, checkedAt)
	}
	retry, scheduled := second.owner.schedule.at("AS-TEST")
	if want := second.clock.at.Add(retryInterval); !scheduled || !retry.Equal(want) {
		t.Fatalf("AS-TEST scheduled at %s (scheduled %v) after a failed update, want the retry at %s", retry, scheduled, want)
	}
	var row *datasetRow
	show, err := second.owner.showIRR()
	if err != nil {
		t.Fatalf("show irr after a failed update: %v", err)
	}
	for i := range show.Datasets {
		if show.Datasets[i].ASSet == "AS-TEST" {
			row = &show.Datasets[i]
		}
	}
	if row == nil || row.Status != rowFailing {
		t.Fatalf("show irr row of AS-TEST %+v after a failed update, want status %q", row, rowFailing)
	}

	if asnStatus != statusError || asnErr == nil {
		t.Fatalf("update irr asn 65010: status %q err %v, want %q and an error", asnStatus, asnErr, statusError)
	}
	if !slices.Equal(second.resolver.asked, []uint32{65010}) {
		t.Fatalf("PeeringDB asked %v, want [65010]", second.resolver.asked)
	}
	if want := []string{"AS-TEST", "AS65010"}; !slices.Equal(second.resolver.queries, want) {
		t.Fatalf("IRR queried %v, want %v: with PeeringDB failing and nothing bound the AS number itself is the set", second.resolver.queries, want)
	}
	if binding, ok := second.storedBinding(65010); !ok || binding.ASSet != "AS65010" {
		t.Fatalf("binding of 65010 = %+v (stored %v), want AS65010", binding, ok)
	}
	if after, _ := second.pointer("AS65010"); after != asnBefore {
		t.Fatalf("AS65010 pointer %+v after a failed update, want unchanged %+v", after, asnBefore)
	}
	if got := second.dataset("AS65010").CheckedAt(); !got.Equal(asnCheckedAt) {
		t.Fatalf("AS65010 checked-at %s after a failed update, want unchanged %s", got, asnCheckedAt)
	}
	// The asn path records its failure on the set it fetched, as the as-set
	// path does: the deleted filter_irr test proved lastErr for this ASN.
	asnRetry, asnScheduled := second.owner.schedule.at("AS65010")
	if want := second.clock.at.Add(retryInterval); !asnScheduled || !asnRetry.Equal(want) {
		t.Fatalf("AS65010 scheduled at %s (scheduled %v) after a failed update irr asn, want the retry at %s", asnRetry, asnScheduled, want)
	}
	var asnRow *datasetRow
	for i := range show.Datasets {
		if show.Datasets[i].ASSet == "AS65010" {
			asnRow = &show.Datasets[i]
		}
	}
	if asnRow == nil || asnRow.Status != rowFailing {
		t.Fatalf("show irr row of AS65010 %+v after a failed update irr asn, want status %q", asnRow, rowFailing)
	}
}

// VALIDATES: a bare AS number follows PeeringDB at each due time: with
// PeeringDB failing and nothing bound the AS number itself is bound and
// fetched; a PeeringDB answer rewrites the binding and the named set is
// fetched; a later change of PeeringDB's answer is followed the same way at
// the next due time; and a PeeringDB failure after that keeps the bound set
// (AC-11). (Replaces the filter_irr test of the same name, whose discovery
// moved to the owner.)
// METHOD: one consumer names AS 65001; each step sets the fake PeeringDB,
// moves the clock to the due time and runs the worker's passes; the stored
// binding, the PeeringDB questions and the IRR queries are read per step.
// PREVENTS: the first answer, or the AS<asn> literal an outage produced,
// becoming the set for the owner's life, and an outage dropping a binding.
func TestDiscoveryFollowsPeeringDB(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS65001"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	h.resolver.answers["AS-ONE"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.resolver.answers["AS-TWO"] = fakeAnswer{ipv4: prefixes("203.0.113.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65001}})

	steps := []struct {
		name   string
		pdbErr error
		answer string
		want   string
	}{
		{name: "PeeringDB down, nothing bound", pdbErr: errors.New("peeringdb timeout"), want: "AS65001"},
		{name: "PeeringDB names AS-ONE", answer: "AS-ONE", want: "AS-ONE"},
		{name: "PeeringDB names AS-TWO", answer: "AS-TWO", want: "AS-TWO"},
		{name: "PeeringDB down, AS-TWO bound", pdbErr: errors.New("peeringdb timeout"), want: "AS-TWO"},
	}
	for i, step := range steps {
		h.resolver.pdbErr = step.pdbErr
		delete(h.resolver.sets, 65001)
		if step.answer != "" {
			h.resolver.sets[65001] = step.answer
		}
		h.resolver.queries, h.resolver.asked = nil, nil
		if i == 0 {
			h.boot()
		} else {
			// The bound set is due a full interval after it was stored. The
			// first pass asks PeeringDB; a binding that moved is fetched by the
			// pass the worker runs next.
			h.clock.at = h.clock.at.Add(time.Hour)
			h.pass()
			h.pass()
		}
		binding, ok := h.storedBinding(65001)
		if !ok || binding.ASSet != step.want {
			t.Fatalf("step %d (%s): binding %+v (stored %v), want %s", i, step.name, binding, ok, step.want)
		}
		if !slices.Equal(h.resolver.asked, []uint32{65001}) {
			t.Fatalf("step %d (%s): PeeringDB asked %v, want once", i, step.name, h.resolver.asked)
		}
		if !slices.Equal(h.resolver.queries, []string{step.want}) {
			t.Fatalf("step %d (%s): IRR queried %v, want [%s]", i, step.name, h.resolver.queries, step.want)
		}
		if _, stored := h.pointer(step.want); !stored {
			t.Fatalf("step %d (%s): no pointer stored for %s", i, step.name, step.want)
		}
	}
}

// VALIDATES: after a consumer stops naming an AS-SET and names the AS number
// instead, a PeeringDB outage binds the AS number itself, never the set the
// consumer dropped. (Replaces the filter_irr test of the same name.)
// METHOD: the consumer names AS-OLD and the owner stores it; the consumer's
// interest then names AS 65001 alone, PeeringDB fails, and one pass runs.
// PREVENTS: the dropped set coming back as the discovery fallback.
func TestRemovedASSetIsNotThePeeringDBFallback(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-OLD"] = fakeAnswer{ipv4: prefixes("10.0.0.0/24")}
	h.resolver.answers["AS65001"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-OLD"}})
	h.boot()
	if _, ok := h.pointer("AS-OLD"); !ok {
		t.Fatal("AS-OLD was not stored")
	}

	h.resolver.queries = nil
	h.resolver.pdbErr = errors.New("peeringdb timeout")
	h.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65001}})
	h.owner.interestChanged()
	h.pass()

	if !slices.Equal(h.resolver.queries, []string{"AS65001"}) {
		t.Fatalf("IRR queried %v, want [AS65001]: the dropped set must not be the fallback", h.resolver.queries)
	}
	if binding, ok := h.storedBinding(65001); !ok || strings.EqualFold(binding.ASSet, "AS-OLD") || binding.ASSet != "AS65001" {
		t.Fatalf("binding of 65001 = %+v (stored %v), want AS65001", binding, ok)
	}
}
