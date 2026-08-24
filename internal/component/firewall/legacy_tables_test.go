package firewall

import (
	"slices"
	"testing"
)

// VALIDATES: IsLegacyTable answers yes only for a kernel table an older ze build
// wrote: the right name, the family that build used, and no chain but the ones
// that build wrote.
// PREVENTS: the one-time sweep destroying a table somebody else owns. These
// names are ordinary words, and another tool that programs nftables from
// FlowSpec can use one of them. Deleting that tool's table is a worse failure
// than the stale rule the sweep exists to clear, so name and family alone must
// not be enough to decide.
func TestIsLegacyTableRequiresNameFamilyAndChainShape(t *testing.T) {
	cases := []struct {
		name   string
		table  string
		family TableFamily
		chains []string
		want   bool
	}{
		{"both chains the plugin wrote", "flowspec", FamilyInet, []string{"flowspec-fwd", "flowspec-in"}, true},
		{"one chain the plugin wrote", "flowspec", FamilyInet, []string{"flowspec-fwd"}, true},
		{"another tool's chain under the same name and family", "flowspec", FamilyInet, []string{"forward"}, false},
		{"ours plus one that is not", "flowspec", FamilyInet, []string{"flowspec-fwd", "forward"}, false},
		{"no chain at all", "flowspec", FamilyInet, nil, false},
		{"the right chain in another family", "flowspec", FamilyIP, []string{"flowspec-fwd"}, false},
		{"anomaly-shape in the family the plugin used", "anomaly-shape", FamilyIP, []string{"ingress"}, true},
		{"anomaly-shape6 in the family the plugin used", "anomaly-shape6", FamilyIP6, []string{"ingress"}, true},
		{"anomaly-shape6 in the v4 family", "anomaly-shape6", FamilyIP, []string{"ingress"}, false},
		{"copp as translatePolicy wrote it", "copp", FamilyInet, []string{"input"}, true},
		{"copp in a family it never used", "copp", FamilyIP, []string{"input"}, false},
		{"copp with a chain it never wrote", "copp", FamilyInet, []string{"forward"}, false},
		{"ddos-local hooked on forward, v4", "ddos-local", FamilyIP, []string{"forward"}, true},
		{"ddos-local hooked on input, v4", "ddos-local", FamilyIP, []string{"ingress"}, true},
		{"ddos-local hooked on forward, v6", "ddos-local", FamilyIP6, []string{"forward"}, true},
		{"ddos-local hooked on input, v6", "ddos-local", FamilyIP6, []string{"ingress"}, true},
		{"ddos-local in the family it never used", "ddos-local", FamilyInet, []string{"forward"}, false},
		{"ddos-local with a chain it never wrote", "ddos-local", FamilyIP, []string{"input"}, false},
		{"a name ze never wrote", "operator-table", FamilyInet, []string{"ingress"}, false},
		{"the renamed table is owned by its prefix, not by this list", "ze_flowspec", FamilyInet, []string{"flowspec-fwd"}, false},
		{"the empty name", "", FamilyInet, []string{"flowspec-fwd"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsLegacyTable(tc.table, tc.family, tc.chains); got != tc.want {
				t.Fatalf("IsLegacyTable(%q, %v, %v) = %v, want %v",
					tc.table, tc.family, tc.chains, got, tc.want)
			}
		})
	}
}

// VALIDATES: IsLegacyTableName matches on name and family alone, which is what
// makes it a pre-filter rather than the decision.
// PREVENTS: a future edit turning the pre-filter into the answer. It exists to
// keep a netlink round trip off every kernel table, and it says nothing about
// who wrote the one it matches.
func TestIsLegacyTableNameIsOnlyThePreFilter(t *testing.T) {
	if !IsLegacyTableName("flowspec", FamilyInet) {
		t.Fatal("the pre-filter must admit the name and family ze wrote")
	}
	if IsLegacyTableName("flowspec", FamilyIP) {
		t.Fatal("the pre-filter must reject another family")
	}
	if IsLegacyTableName("operator-table", FamilyInet) {
		t.Fatal("the pre-filter must reject a name ze never wrote")
	}
	// One producer wrote its name in two families, so the pre-filter admits
	// both and neither is the decision.
	if !IsLegacyTableName("ddos-local", FamilyIP) || !IsLegacyTableName("ddos-local", FamilyIP6) {
		t.Fatal("the pre-filter must admit every family the producer wrote")
	}
	if IsLegacyTableName("ddos-local", FamilyInet) {
		t.Fatal("the pre-filter must reject a family the producer never wrote")
	}
	// The pre-filter admits what the full test then refuses on chain shape.
	if IsLegacyTable("flowspec", FamilyInet, []string{"forward"}) {
		t.Fatal("the pre-filter must not be sufficient on its own")
	}
}

// VALIDATES: the ledger holds every producer that shipped a bare name, and no
// other. Four did: copp, ddos-local, the FlowSpec bridge and the anomaly-shape
// responder, the last of which wrote two names.
// PREVENTS: an upgraded box stranding a table nobody put in the ledger. The
// removal reads this map and nothing else, so a producer left out of it keeps
// enforcing the rules its previous build installed for the life of the box.
func TestEveryProducerThatShippedABareNameIsInTheLedger(t *testing.T) {
	want := []string{"anomaly-shape", "anomaly-shape6", "copp", "ddos-local", "flowspec"}
	got := make([]string, 0, len(legacyTables))
	for name := range legacyTables {
		got = append(got, name)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("legacyTables holds %v, want %v", got, want)
	}
}

// VALIDATES: every legacy name is one no current producer registers. A name
// still in use would be deleted on every reconcile, right after the backend
// added it.
// PREVENTS: an entry outliving the rename that made it legacy.
func TestLegacyTableNamesAreNoLongerRegistrable(t *testing.T) {
	for name := range legacyTables {
		if err := RegisterTables("legacy-check", []Table{{Name: name, Family: FamilyInet}}); err == nil {
			t.Fatalf("legacy table %q is still a name a producer can register", name)
		}
	}
}

// VALIDATES: an EMPTY reconcile loads a backend and reaches Apply while the
// one-time removal is pending, and stops doing so once it has run.
// PREVENTS: the case this whole removal exists for going unfixed. A box that
// holds a legacy table, has no firewall {} section and has no owner registering
// anything never loads a backend, so ApplyAll used to return before it reached
// one and the table an older build wrote kept enforcing for the life of the
// box. Both integration proofs of the removal call Apply with a non-empty set,
// so neither of them can see this hole.
func TestAnEmptyReconcileReachesABackendWhileTheSweepIsPending(t *testing.T) {
	resetBackends()
	resetTables()
	pending := legacySweepPending.Load()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
		legacySweepPending.Store(pending)
	})

	loaded := 0
	applied := 0
	var lastApplied []Table
	if err := RegisterBackend("legacy-sweep-test", func() (Backend, error) {
		loaded++
		return &countingBackend{onApply: func(d []Table) { applied++; lastApplied = d }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "legacy-sweep-test"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	// No owner has registered anything, and no firewall {} section loaded a
	// backend. This is the upgraded box the removal is for.
	legacySweepPending.Store(true)
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll with an empty set and the sweep pending: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("backend loaded %d times, want 1: the empty reconcile never reached one", loaded)
	}
	if applied != 1 {
		t.Fatalf("Apply called %d times, want 1: the sweep runs inside Apply", applied)
	}
	if len(lastApplied) != 0 {
		t.Fatalf("Apply got %v, want an empty desired set", lastApplied)
	}
	if LegacySweepPending() {
		t.Fatal("the sweep is still pending after a reconcile reached a backend")
	}

	// Once it has run, an empty reconcile with nothing loaded is a no-op again.
	if err := CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if err := ApplyAll(); err != nil {
		t.Fatalf("second ApplyAll: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("backend loaded %d times, want 1: an empty set must not spin one up once the sweep has run", loaded)
	}
}
