// Design: docs/architecture/irr-owner.md -- the one-time conversion of the firewall's stored lists
package irr

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// storeLegacyEntry writes entry under key as firewall-irr stored it before the
// owner: the JSON of a store.CachedEntry under meta/irr/{name}.
func storeLegacyEntry(t *testing.T, state *fakeState, key string, entry store.CachedEntry) {
	t.Helper()
	value, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.StatePut(context.Background(), key, value); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: AC-17 -- a meta/irr/{name} entry is converted whole: every IPv4
// and every IPv6 prefix in order, its refreshed-at and its stale-since, so the
// firewall enforces after the upgrade the list, and the staleness, it enforced
// before. (Replaces the per-prefix half of store TestMigrateMultiPrefix, whose
// legacy blob is no longer read.)
// PREVENTS: a conversion that keeps one prefix or one family, or that turns a
// last-known-good list into one that reads as fresh.
// METHOD: store one legacy entry holding three IPv4 and two IPv6 prefixes and a
// stale-since, boot the owner, decode the dataset its pointer names.
func TestLegacyEntryConvertedWhole(t *testing.T) {
	state := newFakeState()
	refreshed := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	staleSince := time.Date(2026, 11, 15, 9, 50, 0, 0, time.UTC)
	entry := store.CachedEntry{
		Name: "AS-CF", ASSet: "AS-CF",
		IPv4:        prefixes("1.1.1.0/24", "1.0.0.0/24", "1.2.3.0/24"),
		IPv6:        prefixes("2606:4700::/32", "2400:cb00::/32"),
		RefreshedAt: refreshed, StaleSince: staleSince,
	}
	storeLegacyEntry(t, state, "meta/irr/AS-CF", entry)
	h := newHarness(t, state, defaultOwnerConfig())
	h.writeInterest(consumerFirewall, store.Interest{ASSets: []string{"AS-CF"}})

	h.boot()

	if state.has("meta/irr/AS-CF") {
		t.Fatal("the legacy key was kept after conversion")
	}
	dataset := h.dataset("AS-CF")
	if !slices.Equal(dataset.IPv4(), entry.IPv4) || !slices.Equal(dataset.IPv6(), entry.IPv6) {
		t.Fatalf("converted dataset holds %v and %v, want %v and %v", dataset.IPv4(), dataset.IPv6(), entry.IPv4, entry.IPv6)
	}
	if !dataset.RefreshedAt().Equal(refreshed) || !dataset.CheckedAt().Equal(refreshed) {
		t.Fatalf("converted dataset refreshed at %s checked at %s, want both %s", dataset.RefreshedAt(), dataset.CheckedAt(), refreshed)
	}
	if !dataset.StaleSince().Equal(staleSince) {
		t.Fatalf("converted dataset stale since %s, want %s: the list was last-known-good before the upgrade", dataset.StaleSince(), staleSince)
	}
	if len(h.resolver.queries) != 0 {
		t.Fatalf("conversion queried IRR %v, want nothing", h.resolver.queries)
	}
}

// VALIDATES: a meta/irr/{name} value naming another name than its key is not
// converted: no pointer and no binding is written for the key's name, the
// value's name or the value's AS-SET, and the old key stays, so the list the
// firewall enforced is not lost. A well-formed entry beside it is still
// converted. (Replaces store TestPrefixStoreOpenNameKeyMismatch, whose reader
// is now the owner's conversion.)
// PREVENTS: a corrupt or tampered value poisoning another set's slot, or a
// failed conversion deleting the stored list.
// METHOD: store under meta/irr/AS13335 a value whose name is AS99999 and AS-SET
// AS-X, and under meta/irr/AS64500 a correct value for AS-GOOD; boot the owner
// with a consumer naming AS 64500; read the store.
func TestLegacyEntryNameMismatchKept(t *testing.T) {
	state := newFakeState()
	refreshed := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	storeLegacyEntry(t, state, "meta/irr/AS13335",
		store.CachedEntry{Name: "AS99999", ASSet: "AS-X", IPv4: prefixes("10.0.0.0/24"), RefreshedAt: refreshed})
	storeLegacyEntry(t, state, "meta/irr/AS64500",
		store.CachedEntry{Name: "AS64500", ASSet: "AS-GOOD", IPv4: prefixes("10.1.0.0/24"), RefreshedAt: refreshed})
	h := newHarness(t, state, defaultOwnerConfig())
	h.writeInterest(consumerFirewall, store.Interest{ASNs: []uint32{64500}})

	h.boot()

	if !state.has("meta/irr/AS13335") {
		t.Fatal("the mismatched legacy key was removed although it was not converted")
	}
	for _, set := range []string{"AS-X", "AS99999", "AS13335"} {
		if state.has(store.PointerKey(set)) {
			t.Errorf("a pointer for %s was written from a value whose name is not its key's", set)
		}
	}
	for _, asn := range []uint32{13335, 99999} {
		if state.has(store.BindingKey(asn)) {
			t.Errorf("a binding for AS %d was written from a value whose name is not its key's", asn)
		}
	}

	if state.has("meta/irr/AS64500") {
		t.Fatal("the well-formed legacy key was kept after conversion")
	}
	if dataset := h.dataset("AS-GOOD"); !slices.Equal(dataset.IPv4(), prefixes("10.1.0.0/24")) {
		t.Fatalf("AS-GOOD converted to %v, want 10.1.0.0/24", dataset.IPv4())
	}
	binding, err := store.DecodeBinding(state.value(store.BindingKey(64500)))
	if err != nil || binding.ASSet != "AS-GOOD" {
		t.Fatalf("binding of 64500: %+v %v, want AS-GOOD", binding, err)
	}
	if len(h.resolver.queries) != 0 || len(h.resolver.asked) != 0 {
		t.Fatalf("conversion queried IRR %v and PeeringDB %v, want nothing", h.resolver.queries, h.resolver.asked)
	}
}
