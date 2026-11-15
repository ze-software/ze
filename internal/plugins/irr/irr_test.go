package irr

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

const (
	consumerBGP      = "bgp-filter-irr"
	consumerFirewall = "firewall-irr"
)

// TestInterestDrivesFetch proves the owner fetches exactly the union of the
// consumers' interest keys, once per set, and fetches a set newly named after
// an interest notice. Method: two consumers name one shared set and one each;
// after boot the fake server has seen each set once; a rewritten interest
// plus interestChanged fetches only the new set.
func TestInterestDrivesFetch(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	for _, set := range []string{"AS-SHARED", "AS-BGP", "AS-FW", "AS-NEW"} {
		h.resolver.answers[set] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"as-shared", "AS-BGP"}})
	h.writeInterest(consumerFirewall, store.Interest{ASSets: []string{"AS-SHARED", "AS-FW"}})

	h.boot()

	if want := []string{"AS-BGP", "AS-FW", "AS-SHARED"}; !slices.Equal(sorted(h.resolver.queries), want) {
		t.Fatalf("boot fetched %v, want each referenced set once: %v", h.resolver.queries, want)
	}
	h.resolver.queries = nil
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-SHARED", "AS-BGP", "AS-NEW"}})
	h.owner.interestChanged()
	h.pass()
	if want := []string{"AS-NEW"}; !slices.Equal(h.resolver.queries, want) {
		t.Fatalf("interest notice fetched %v, want only %v", h.resolver.queries, want)
	}
}

// TestPointerFlipsAfterWrite proves the order of every store write: the
// dataset, then the generation, then the pointer carrying that generation,
// and that a refused dataset write leaves the pointer unwritten.
func TestPointerFlipsAfterWrite(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})

	h.boot()

	datasetKey := "meta/irr-data/whois.radb.net/AS-TEST"
	order := indexOf(state.log(), "put "+datasetKey, "increment "+store.GenerationKey(), "put "+store.PointerKey("AS-TEST"))
	if !slices.IsSorted(order) || slices.Contains(order, -1) {
		t.Fatalf("write order %v in %v, want dataset, generation, pointer", order, state.log())
	}
	pointer, _ := h.pointer("AS-TEST")
	if pointer.Generation != 1 || h.notices.count() != 1 {
		t.Fatalf("pointer generation %d, %d notices, want 1 and 1", pointer.Generation, h.notices.count())
	}

	state.failPut["meta/irr-data/whois.radb.net/AS-OTHER"] = errors.New("store full")
	h.resolver.answers["AS-OTHER"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST", "AS-OTHER"}})
	h.owner.interestChanged()
	h.pass()
	if state.has(store.PointerKey("AS-OTHER")) {
		t.Fatal("the pointer of AS-OTHER was written although its dataset write was refused")
	}
	if at, ok := h.owner.schedule.at("AS-OTHER"); !ok || !at.Equal(h.clock.at.Add(retryInterval)) {
		t.Fatalf("refused write re-armed at %s ok=%v, want a retry in a minute", at, ok)
	}
}

// TestFailedFetchKeepsPointer proves a fetch the server does not answer keeps
// the dataset, the pointer and checked-at, and is retried in one minute
// (AC-7).
func TestFailedFetchKeepsPointer(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	before, _ := h.pointer("AS-TEST")
	checkedAt := h.dataset("AS-TEST").CheckedAt()

	h.clock.at = h.clock.at.Add(time.Hour)
	h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("i/o timeout")}
	h.pass()

	after, _ := h.pointer("AS-TEST")
	if after != before {
		t.Fatalf("pointer changed on a failed fetch: %+v, was %+v", after, before)
	}
	if got := h.dataset("AS-TEST").CheckedAt(); !got.Equal(checkedAt) {
		t.Fatalf("checked-at %s after a failed fetch, want unchanged %s", got, checkedAt)
	}
	if at, ok := h.owner.schedule.at("AS-TEST"); !ok || !at.Equal(h.clock.at.Add(retryInterval)) {
		t.Fatalf("failed fetch re-armed at %s ok=%v, want %s", at, ok, h.clock.at.Add(retryInterval))
	}
}

// TestEmptyAnswerAdvancesCheckedAt proves an answer naming no prefix keeps the
// stored list, marks it stale and advances checked-at, so the next fetch
// waits a full interval rather than a minute (AC-7).
func TestEmptyAnswerAdvancesCheckedAt(t *testing.T) {
	h := newHarness(t, newFakeState(), defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()

	h.clock.at = h.clock.at.Add(time.Hour)
	h.resolver.answers["AS-TEST"] = fakeAnswer{}
	h.pass()

	dataset := h.dataset("AS-TEST")
	if len(dataset.IPv4()) != 1 || dataset.StaleSince().IsZero() {
		t.Fatalf("empty answer stored %d prefixes, stale-since %s; want the kept list, stale", len(dataset.IPv4()), dataset.StaleSince())
	}
	if !dataset.CheckedAt().Equal(h.clock.at) {
		t.Fatalf("checked-at %s, want the answer time %s", dataset.CheckedAt(), h.clock.at)
	}
	if at, _ := h.owner.schedule.at("AS-TEST"); !at.Equal(h.clock.at.Add(time.Hour)) {
		t.Fatalf("next fetch %s, want a full interval later", at)
	}
}

// TestRestartReadsStoredNoFetch proves a second owner over the same store
// sends no query at start and plans the stored set at checked-at plus the
// interval (AC-1, AC-3, owner half).
func TestRestartReadsStoredNoFetch(t *testing.T) {
	state := newFakeState()
	first := newHarness(t, state, defaultOwnerConfig())
	first.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	first.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	first.boot()
	checkedAt := first.dataset("AS-TEST").CheckedAt()

	second := newHarness(t, state, defaultOwnerConfig())
	second.clock.at = checkedAt.Add(40 * time.Minute)
	second.boot()

	if len(second.resolver.queries) != 0 {
		t.Fatalf("restart queried %v, want nothing", second.resolver.queries)
	}
	if at, _ := second.owner.schedule.at("AS-TEST"); !at.Equal(checkedAt.Add(time.Hour)) {
		t.Fatalf("restart armed AS-TEST at %s, want %s", at, checkedAt.Add(time.Hour))
	}
	if _, seeded := second.resolver.seeded["AS-TEST"]; !seeded {
		t.Fatal("restart did not seed the stored list as the merge base")
	}
}

// TestServerChangeKeepsOldUntilNew proves a new server is asked at once, the
// old server's dataset stays the enforced one while the new server fails, and
// is removed once the new copy's pointer flips (AC-9).
func TestServerChangeKeepsOldUntilNew(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()
	oldKey := "meta/irr-data/whois.radb.net/AS-TEST"

	cfg := defaultOwnerConfig()
	cfg.server = "rr.ntt.net"
	h.resolver.answers["AS-TEST"] = fakeAnswer{err: errors.New("connection refused")}
	h.resolver.queries = nil
	if err := h.owner.configure(cfg); err != nil {
		t.Fatal(err)
	}
	h.pass()
	if len(h.resolver.queries) != 1 {
		t.Fatalf("server change sent %v, want one query at once", h.resolver.queries)
	}
	if pointer, _ := h.pointer("AS-TEST"); pointer.Server != "whois.radb.net" || !state.has(oldKey) {
		t.Fatalf("pointer names %q while the new server fails, want the old copy kept", pointer.Server)
	}

	h.clock.at = h.clock.at.Add(retryInterval)
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("198.51.100.0/24")}
	h.pass()
	if pointer, _ := h.pointer("AS-TEST"); pointer.Server != "rr.ntt.net" {
		t.Fatalf("pointer names %q after the new server answered, want rr.ntt.net", pointer.Server)
	}
	if state.has(oldKey) {
		t.Fatal("the old server's dataset was kept after the pointer flipped")
	}
}

// TestLegacyEntryConverted proves a meta/irr/{name} entry is converted under
// the configured server with its pointer and binding, then removed, with no
// query (AC-17).
func TestLegacyEntryConverted(t *testing.T) {
	state := newFakeState()
	refreshed := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	legacy, err := json.Marshal(store.CachedEntry{Name: "AS65001", ASSet: "AS-CUSTOMER", IPv4: prefixes("203.0.113.0/24"), RefreshedAt: refreshed})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.StatePut(context.Background(), "meta/irr/AS65001", legacy); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, state, defaultOwnerConfig())
	h.writeInterest(consumerFirewall, store.Interest{ASNs: []uint32{65001}})

	h.boot()

	if state.has("meta/irr/AS65001") {
		t.Fatal("the legacy key was kept after conversion")
	}
	dataset := h.dataset("AS-CUSTOMER")
	if len(dataset.IPv4()) != 1 || !dataset.CheckedAt().Equal(refreshed) {
		t.Fatalf("converted dataset holds %v checked at %s", dataset.IPv4(), dataset.CheckedAt())
	}
	binding, err := store.DecodeBinding(state.value(store.BindingKey(65001)))
	if err != nil || binding.ASSet != "AS-CUSTOMER" {
		t.Fatalf("binding of 65001: %+v %v, want AS-CUSTOMER", binding, err)
	}
	if len(h.resolver.queries) != 0 || len(h.resolver.asked) != 0 {
		t.Fatalf("conversion queried IRR %v and PeeringDB %v, want nothing", h.resolver.queries, h.resolver.asked)
	}
}

// TestClearNamesConsumers proves clear irr as-set removes the pointer and the
// dataset, names every consumer still referencing the set, tells them, and
// does not fetch it again until update irr (AC-16).
func TestClearNamesConsumers(t *testing.T) {
	state := newFakeState()
	h := newHarness(t, state, defaultOwnerConfig())
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.writeInterest(consumerFirewall, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()

	answer := h.owner.clearASSet(context.Background(), "AS-TEST")
	cleared, ok := answer.data.(clearAnswer)
	if answer.err != nil || !ok || !cleared.Removed {
		t.Fatalf("clear answered %+v %v", answer.data, answer.err)
	}
	if want := []string{consumerBGP, consumerFirewall}; !slices.Equal(cleared.Consumers, want) {
		t.Fatalf("clear named %v, want %v", cleared.Consumers, want)
	}
	if state.has(store.PointerKey("AS-TEST")) || state.has("meta/irr-data/whois.radb.net/AS-TEST") {
		t.Fatal("clear left the pointer or the dataset")
	}
	if !strings.Contains(h.notices.sent[len(h.notices.sent)-1], `"removed":true`) {
		t.Fatalf("last notice %s, want removed", h.notices.sent[len(h.notices.sent)-1])
	}
	h.resolver.queries = nil
	h.clock.at = h.clock.at.Add(2 * time.Hour)
	h.pass()
	if len(h.resolver.queries) != 0 {
		t.Fatalf("a cleared set was fetched again: %v", h.resolver.queries)
	}
	if answer := h.owner.updateASSet(context.Background(), "AS-TEST"); answer.err != nil {
		t.Fatalf("update after clear: %v", answer.err)
	}
	if _, ok := h.pointer("AS-TEST"); !ok {
		t.Fatal("update irr as-set did not store the cleared set again")
	}
}

// TestBoundASNNotAskedAtStart proves a stored binding is used at start with no
// PeeringDB question, and PeeringDB is asked again when the set is due, a
// failure then keeping the stored set (AC-11).
func TestBoundASNNotAskedAtStart(t *testing.T) {
	state := newFakeState()
	first := newHarness(t, state, defaultOwnerConfig())
	first.resolver.sets[65001] = "AS-Y"
	first.resolver.answers["AS-Y"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	first.writeInterest(consumerBGP, store.Interest{ASNs: []uint32{65001}})
	first.boot()

	second := newHarness(t, state, defaultOwnerConfig())
	second.clock.at = first.clock.at.Add(10 * time.Minute)
	second.resolver.pdbErr = errors.New("peeringdb timeout")
	second.boot()
	if len(second.resolver.asked) != 0 || len(second.resolver.queries) != 0 {
		t.Fatalf("restart asked PeeringDB %v and IRR %v, want nothing", second.resolver.asked, second.resolver.queries)
	}

	second.clock.at = first.clock.at.Add(time.Hour)
	second.resolver.answers["AS-Y"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	second.pass()
	if len(second.resolver.asked) != 1 || !slices.Equal(second.resolver.queries, []string{"AS-Y"}) {
		t.Fatalf("due pass asked PeeringDB %v and IRR %v, want one question and AS-Y", second.resolver.asked, second.resolver.queries)
	}
}

// TestManualShowsSchedule proves show irr reports the schedule as manual with
// refresh-interval 0 and a stored set (AC-10).
func TestManualShowsSchedule(t *testing.T) {
	cfg := defaultOwnerConfig()
	cfg.refreshInterval = 0
	h := newHarness(t, newFakeState(), cfg)
	h.resolver.answers["AS-TEST"] = fakeAnswer{ipv4: prefixes("192.0.2.0/24")}
	h.writeInterest(consumerBGP, store.Interest{ASSets: []string{"AS-TEST"}})
	h.boot()

	show, err := h.owner.showIRR()
	if err != nil {
		t.Fatalf("show irr after boot: %v", err)
	}
	if show.Schedule != nextManual || len(show.Datasets) != 1 || show.Datasets[0].NextRefresh != nextManual {
		t.Fatalf("show irr %+v, want a manual schedule", show)
	}
	if show.Datasets[0].IPv4Count != 1 || !slices.Equal(show.Datasets[0].Consumers, []string{consumerBGP}) {
		t.Fatalf("show irr row %+v", show.Datasets[0])
	}
}

func sorted(names []string) []string {
	out := slices.Clone(names)
	slices.Sort(out)
	return out
}

// indexOf returns the position of each op in log, -1 when absent.
func indexOf(log []string, ops ...string) []int {
	out := make([]int, 0, len(ops))
	for _, op := range ops {
		out = append(out, slices.Index(log, op))
	}
	return out
}
