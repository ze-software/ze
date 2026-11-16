// Design: docs/architecture/irr-owner.md -- a refused clear, a reverted server, findings that clear, a binding that moves
package irr

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// familyResolver merges each answer per family into its seeded copy, as
// store.PrefixStore.Refresh does (commit): the IPv4 answer replaces the seeded
// IPv4, and with no IPv6 answered the seeded IPv6 is kept and the entry marked
// stale. The IPv6 answer of a set is in ipv6. Called from the worker only.
type familyResolver struct {
	*fakeResolver
	ipv6 map[string][]netip.Prefix
}

func (r *familyResolver) refresh(ctx context.Context, asSet string) (*store.CachedEntry, error) {
	kept, had := r.seeded[asSet]
	entry, err := r.fakeResolver.refresh(ctx, asSet)
	if err != nil {
		return entry, err
	}
	entry.IPv6 = r.ipv6[asSet]
	if len(entry.IPv6) == 0 && had && len(kept.IPv6) > 0 {
		entry.IPv6, entry.StaleSince = kept.IPv6, r.clock()
	}
	r.seeded[asSet] = *entry
	return entry, nil
}

// newFamilyHarness is a harness whose owner gets a fresh familyResolver each
// time it builds one, as newStoreResolver does, so a copy seeded into one
// resolver is not in the next. Every resolver answers from one table, and
// h.resolver is the last one built. It answers the IPv6 table and the number
// of resolvers built.
func newFamilyHarness(t *testing.T, state *fakeState) (*ownerHarness, map[string][]netip.Prefix, *int) {
	t.Helper()
	h := newHarness(t, state, defaultOwnerConfig())
	ipv6 := make(map[string][]netip.Prefix)
	built := 0
	h.owner = newOwner(state, h.notices, func(ownerConfig) resolver {
		built++
		fresh := newFakeResolver(h.clock.now)
		fresh.answers = h.resolver.answers
		h.resolver = fresh
		return &familyResolver{fakeResolver: fresh, ipv6: ipv6}
	})
	h.owner.clock = h.clock.now
	if err := h.owner.configure(defaultOwnerConfig()); err != nil {
		t.Fatal(err)
	}
	return h, ipv6, &built
}

// bootBothFamilies stores AS-TEST with one IPv4 and one IPv6 prefix.
func bootBothFamilies(t *testing.T, h *ownerHarness, ipv6 map[string][]netip.Prefix) {
	t.Helper()
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	ipv6["AS-TEST"] = prefixes("2001:db8::/32")
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	if got := h.dataset("AS-TEST").IPv6(); !slices.Equal(got, prefixes("2001:db8::/32")) {
		t.Fatalf("boot stored IPv6 %v, want 2001:db8::/32: without both families the test proves nothing", got)
	}
}

// answerIPv4Only makes the next fetch of AS-TEST answer a new IPv4 prefix
// and nothing for IPv6, an hour later.
func answerIPv4Only(h *ownerHarness, ipv6 map[string][]netip.Prefix) {
	delete(ipv6, "AS-TEST")
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	h.resolver.queries = nil
	h.clock.at = h.clock.at.Add(time.Hour)
}

// requireBothFamilies fails unless the fetch ran and the stored AS-TEST holds
// the new IPv4 prefix and the IPv6 prefix stored before.
func requireBothFamilies(t *testing.T, h *ownerHarness) {
	t.Helper()
	if !slices.Equal(h.resolver.queries, []string{"AS-TEST"}) {
		t.Fatalf("the pass fetched %v, want AS-TEST once: without the fetch the test proves nothing", h.resolver.queries)
	}
	dataset := h.dataset("AS-TEST")
	if !slices.Equal(dataset.IPv4(), prefixes("198.51.100.0/24")) {
		t.Fatalf("stored IPv4 %v, want the new answer 198.51.100.0/24", dataset.IPv4())
	}
	if !slices.Equal(dataset.IPv6(), prefixes("2001:db8::/32")) {
		t.Fatalf("stored IPv6 %v, want the enforced 2001:db8::/32 kept: the answer named no IPv6 (AC-7)", dataset.IPv6())
	}
}

// VALIDATES: a `clear irr as-set` whose pointer removal the store refuses
// leaves the set as it was, merge base included: the next answer that names
// IPv4 alone is stored with the IPv6 prefixes enforced before (AC-7).
// PREVENTS: the clear forgetting the resolver's copy before the refusal, so
// the one-family answer replaces the enforced list without its other family.
// METHOD: store AS-TEST with both families, refuse the pointer removal, clear,
// then answer IPv4 only an hour later and decode the stored dataset.
func TestRefusedClearKeepsMergeBase(t *testing.T) {
	state := newFakeState()
	h, ipv6, _ := newFamilyHarness(t, state)
	bootBothFamilies(t, h, ipv6)

	state.failRemove[store.PointerKey("AS-TEST")] = errors.New("store busy")
	answer := h.owner.clearASSet(context.Background(), "AS-TEST")
	if answer.err == nil {
		t.Fatal("a clear whose pointer removal was refused answered no error")
	}
	if _, ok := h.pointer("AS-TEST"); !ok {
		t.Fatal("the refused clear removed the pointer")
	}
	delete(state.failRemove, store.PointerKey("AS-TEST"))

	answerIPv4Only(h, ipv6)
	h.pass()
	requireBothFamilies(t, h)
}

// VALIDATES: a `clear irr as-set` that removed the pointer and the dataset
// and could not raise the generation answers that the copy was removed and
// the consumers were not told, holds the set, raises
// doctor-irr-dataset-unpublished with the clear as its fix, and the same clear
// run again tells the consumers and clears the finding.
// PREVENTS: an answer saying the copy could not be removed when it was, no
// hold so the next pass fetches the cleared set again, and a removal no
// consumer ever hears of.
// METHOD: store AS-TEST, refuse every increment, clear, run a pass two hours
// later, then lift the refusal and clear again; read the answer, the hold,
// the findings, the queries and the notices.
func TestUnannouncedClearHoldsAndTells(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	sent := h.notices.count()

	h.state.failIncrement = errors.New("store busy")
	answer := h.owner.clearASSet(context.Background(), "AS-TEST")
	if answer.err == nil {
		t.Fatal("a clear whose generation was refused answered no error")
	}
	if text := answer.err.Error(); !strings.Contains(text, "removed the stored copy") || !strings.Contains(text, "were not told") {
		t.Errorf("clear answered %q, want it to say the copy was removed and the consumers were not told", text)
	}
	if h.state.has(store.PointerKey("AS-TEST")) {
		t.Fatal("the pointer is still stored: without the removal the test proves nothing")
	}
	if _, held := h.owner.heldSets["AS-TEST"]; !held {
		t.Error("the cleared set is not held")
	}
	if !hasFinding(h, codeIRRDatasetUnpublished, "clear irr as-set AS-TEST") {
		t.Errorf("findings %+v, want %s fixed by clear irr as-set AS-TEST", h.owner.findings(), codeIRRDatasetUnpublished)
	}
	if h.notices.count() != sent {
		t.Errorf("a notice was sent with no generation raised")
	}

	h.resolver.queries = nil
	h.clock.at = h.clock.at.Add(2 * time.Hour)
	h.pass()
	if len(h.resolver.queries) != 0 {
		t.Fatalf("the cleared set was fetched again: %v", h.resolver.queries)
	}

	h.state.failIncrement = nil
	if retry := h.owner.clearASSet(context.Background(), "AS-TEST"); retry.err != nil {
		t.Fatalf("the second clear answered %v", retry.err)
	}
	if h.notices.count() != sent+1 || !strings.Contains(h.notices.sent[sent], `"removed":true`) {
		t.Fatalf("notices %v after the second clear, want one removal of AS-TEST", h.notices.sent[sent:])
	}
	if hasFinding(h, codeIRRDatasetUnpublished, "") {
		t.Errorf("findings %+v after the consumers were told, want none", h.owner.findings())
	}
}

// hasFinding reports whether the owner reports a finding with code, and with
// fix when fix is not empty.
func hasFinding(h *ownerHarness, code, fix string) bool {
	for _, finding := range h.owner.findings() {
		if finding.Code == code && (fix == "" || finding.Fix == fix) {
			return true
		}
	}
	return false
}

// VALIDATES: a server changed and changed back seeds the new resolver with
// the stored copy of the server it names, so the next answer naming IPv4
// alone is stored with the IPv6 prefixes enforced before (AC-7).
// PREVENTS: the resolver built for the reverted server holding no copy while
// the stored dataset counts as fresh, so a one-family answer overwrites the
// enforced list without its other family.
// METHOD: store AS-TEST from whois.radb.net with both families, change the
// server to rr.ntt.net whose fetch fails, change it back, then answer IPv4
// only an hour later and decode the stored dataset.
func TestServerRevertKeepsMergeBase(t *testing.T) {
	state := newFakeState()
	h, ipv6, built := newFamilyHarness(t, state)
	bootBothFamilies(t, h, ipv6)

	other := defaultOwnerConfig()
	other.server = "rr.ntt.net"
	h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
	if err := h.owner.configure(other); err != nil {
		t.Fatal(err)
	}
	h.pass()
	if pointer, _ := h.pointer("AS-TEST"); pointer.Server != "whois.radb.net" {
		t.Fatalf("pointer names %q while rr.ntt.net fails, want whois.radb.net", pointer.Server)
	}
	if err := h.owner.configure(defaultOwnerConfig()); err != nil {
		t.Fatal(err)
	}
	h.pass()
	if *built != 3 {
		t.Fatalf("%d resolvers built, want 3: without a new resolver for the revert the test proves nothing", *built)
	}

	answerIPv4Only(h, ipv6)
	h.pass()
	requireBothFamilies(t, h)
}

// VALIDATES: an unreadable pointer or dataset finding clears once the set is
// fetched and stored again (AC-22); the findings of a set no consumer names
// and no pointer names clear at the next plan; and a cleared set's findings
// clear with it.
// PREVENTS: `ze doctor` reporting a WARN about a copy already replaced, a set
// nobody references or a set the operator removed, until a restart.
// METHOD: per row, reach the finding, then the step that must clear it, and
// read the owner's recorded problems.
func TestFindingsClearWithTheirCause(t *testing.T) {
	for _, row := range []string{"dataset", "pointer"} {
		t.Run("unreadable "+row+" fetched again", func(t *testing.T) {
			state := newFakeState()
			first := newHarness(t, state, defaultOwnerConfig())
			first.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
			first.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
			first.boot()
			pointer, _ := first.pointer("AS-TEST")
			key := pointer.DatasetKey("AS-TEST")
			if row == "pointer" {
				key = store.PointerKey("AS-TEST")
			}
			corrupt := state.value(key)
			corrupt[len(corrupt)-1] ^= 0xff
			state.values[key] = corrupt

			second := newHarness(t, state, defaultOwnerConfig())
			second.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
			second.owner.absorb(context.Background())
			second.owner.load(context.Background())
			if found, ok := second.owner.problems[key]; !ok || found.code != codeIRRDatasetUnreadable {
				t.Fatalf("load recorded %+v for %s, want %s: without it the test proves nothing", found, key, codeIRRDatasetUnreadable)
			}
			second.pass()
			if !slices.Equal(second.resolver.queries, []string{"AS-TEST"}) {
				t.Fatalf("the pass fetched %v, want AS-TEST", second.resolver.queries)
			}
			if found, ok := second.owner.problems[key]; ok {
				t.Fatalf("%s still reported %+v after AS-TEST was stored again", key, found)
			}
		})
	}

	t.Run("set no consumer names", func(t *testing.T) {
		h := newHarness(t, newFakeState(), defaultOwnerConfig())
		h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
		h.resolver.answers["AS-OTHER"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
		h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
		h.boot()
		if _, ok := h.owner.problems["AS-TEST"]; !ok {
			t.Fatal("the failed fetch raised nothing: without it the test proves nothing")
		}
		h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-OTHER"}})
		h.owner.interestChanged()
		h.pass()
		if found, ok := h.owner.problems["AS-TEST"]; ok {
			t.Fatalf("AS-TEST, which no consumer names and nothing stores, still reports %+v", found)
		}
	})

	t.Run("cleared set", func(t *testing.T) {
		h := newHarness(t, newFakeState(), defaultOwnerConfig())
		h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
		h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
		h.boot()
		h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
		h.clock.at = h.clock.at.Add(time.Hour)
		h.pass()
		if _, ok := h.owner.problems["AS-TEST"]; !ok {
			t.Fatal("the failed fetch raised nothing: without it the test proves nothing")
		}
		if answer := h.owner.clearASSet(context.Background(), "AS-TEST"); answer.err != nil {
			t.Fatal(answer.err)
		}
		h.pass()
		if found, ok := h.owner.problems["AS-TEST"]; ok {
			t.Fatalf("the cleared AS-TEST still reports %+v", found)
		}
	})
}

// VALIDATES: a PeeringDB question that fails, or panics, is a
// doctor-irr-refresh-overdue finding under the binding's key, fixed by `update
// irr asn`, and the next answer clears it; `update irr asn` after a panic says
// the question failed.
// PREVENTS: a PeeringDB outage only the log shows, and an answer blaming the
// store for a question that never answered.
// METHOD: one consumer names AS 65001; per row PeeringDB fails or panics at
// boot; read the findings, then answer and run update irr asn.
func TestPeeringDBFailureIsAFinding(t *testing.T) {
	for _, panics := range []bool{false, true} {
		name := "failure"
		if panics {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, newFakeState(), defaultOwnerConfig())
			h.resolver.answers["AS65001"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
			h.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65001}})
			if panics {
				h.resolver.panicASN = 65001
			} else {
				h.resolver.pdbErr = errors.New("peeringdb timeout")
			}
			h.boot()
			if !hasFinding(h, codeIRRRefreshOverdue, "update irr asn 65001") {
				t.Fatalf("findings %+v, want %s fixed by update irr asn 65001", h.owner.findings(), codeIRRRefreshOverdue)
			}
			if panics {
				answer := h.owner.updateASN(context.Background(), 65001)
				if answer.err == nil || !strings.Contains(answer.err.Error(), "PeeringDB question about the AS number failed") {
					t.Fatalf("update irr asn after a panic answered %v, want it to say the PeeringDB question failed", answer.err)
				}
				h.resolver.panicASN = 0
			}
			h.resolver.pdbErr = nil
			if answer := h.owner.updateASN(context.Background(), 65001); answer.err != nil {
				t.Fatalf("update irr asn answered %v", answer.err)
			}
			if found, ok := h.owner.problems[store.BindingKey(65001)]; ok {
				t.Fatalf("PeeringDB answered and the binding still reports %+v", found)
			}
		})
	}
}

// VALIDATES: a binding PeeringDB moves to a set nothing stores yet is written
// only after that set's pointer: while the new set's fetch fails, a consumer
// reading the binding and then the set it names reads the old set's dataset,
// and once the new set is stored the binding names it.
// PREVENTS: the binding naming a set with no pointer, which a consumer reads
// as no list (ErrNotStored): BGP drops the list and holds the peer, the
// firewall holds its table back, for as long as the new set fails.
// METHOD: AS 65001 is bound to the stored AS-X; PeeringDB then names AS-Y,
// whose fetch fails; at the due time read through store.ReadBinding and
// store.ReadSet as a consumer does; then let AS-Y answer and read the write
// order.
func TestMovedBindingFlipsAfterNewSetStored(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.sets[65001] = "AS-X"
	h.resolver.answers["AS-X"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65001}})
	h.boot()
	if binding, ok := h.storedBinding(65001); !ok || binding.ASSet != "AS-X" {
		t.Fatalf("boot bound %+v, want AS-X", binding)
	}

	h.resolver.sets[65001] = "AS-Y"
	h.resolver.answers["AS-Y"] = fakeAnswer{err: errors.New("connection refused")}
	h.resolver.queries = nil
	h.clock.at = h.clock.at.Add(time.Hour)
	h.pass()
	h.pass()
	if !slices.Contains(h.resolver.queries, "AS-Y") {
		t.Fatalf("the pass fetched %v, want AS-Y tried: without it the test proves nothing", h.resolver.queries)
	}
	binding, err := store.ReadBinding(ctx, h.state, 65001)
	if err != nil || binding.ASSet != "AS-X" {
		t.Fatalf("a consumer reads binding %+v (%v) while AS-Y fails, want AS-X", binding, err)
	}
	enforced, err := store.ReadSet(ctx, h.state, binding.ASSet)
	if err != nil {
		t.Fatalf("a consumer reading %s gets %v, want the stored dataset", binding.ASSet, err)
	}
	if !slices.Equal(enforced.Dataset.IPv4(), prefixes("192.0.2.0/24")) {
		t.Fatalf("a consumer reads %v, want AS-X's 192.0.2.0/24", enforced.Dataset.IPv4())
	}

	h.resolver.answers["AS-Y"] = fakeAnswer{ipv4: prefixes("203.0.113.0/24")}
	h.clock.at = h.clock.at.Add(retryInterval)
	h.pass()
	binding, err = store.ReadBinding(ctx, h.state, 65001)
	if err != nil || binding.ASSet != "AS-Y" {
		t.Fatalf("binding %+v (%v) once AS-Y is stored, want AS-Y", binding, err)
	}
	if _, err := store.ReadSet(ctx, h.state, "AS-Y"); err != nil {
		t.Fatalf("a consumer reading AS-Y gets %v", err)
	}
	log := h.state.log()
	pointerAt := slices.Index(log, "put "+store.PointerKey("AS-Y"))
	bindingAt := -1
	for i, entry := range slices.Backward(log) {
		if entry == "put "+store.BindingKey(65001) {
			bindingAt = i
			break
		}
	}
	if pointerAt < 0 || bindingAt < pointerAt {
		t.Fatalf("write order %v: the binding must be written after the pointer of AS-Y", log)
	}
}
