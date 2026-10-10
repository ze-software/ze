package filter_irr

import (
	"errors"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// irrSection builds a bgp config delivery naming one IRR-filtered peer of AS
// 65001 with the given as-set. No IRR server is reachable and the plugin holds
// no state store, so handleConfigure enrolls the ASN and starts no resolution.
func irrSection(asSet string) []sdk.ConfigSection {
	data := `{"bgp":{"peer":{"10.0.0.1":{` +
		`"session":{"asn":{"remote":"65001"},"irr":{"as-set":"` + asSet + `"}},` +
		`"filter":{"import":["bgp-filter-irr:65001"]}}}}}`
	return []sdk.ConfigSection{{Root: configRootBGP, Data: data}}
}

func newTestTx(t *testing.T) *irrConfigTx {
	t.Helper()
	plug := &irrPlugin{byASN: make(map[uint32]*asnState), stopCh: make(chan struct{})}
	t.Cleanup(func() { close(plug.stopCh) })
	return &irrConfigTx{plug: plug}
}

func enrolledASSet(t *testing.T, tx *irrConfigTx) string {
	t.Helper()
	tx.plug.mu.RLock()
	defer tx.plug.mu.RUnlock()
	st := tx.plug.byASN[65001]
	if st == nil {
		t.Fatal("ASN 65001 is not enrolled")
	}
	return st.asSet
}

// VALIDATES: spec-terminal-demo-showcase AC-1. A reload's verify holds the
// candidate without applying it, apply makes it the filter's config, rollback
// puts the replaced one back, and a verify that fails leaves the running
// config untouched.
// PREVENTS: the fail-open reload: OnConfigure alone made every live IRR edit
// an accepted no-op.
func TestFilterIRRAppliesAReloadedAsSet(t *testing.T) {
	tx := newTestTx(t)
	if err := tx.configure(irrSection("AS-OLD")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-OLD" {
		t.Fatalf("after boot: as-set %q, want AS-OLD", got)
	}

	if err := tx.verify(irrSection("AS-NEW")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-OLD" {
		t.Fatalf("verify applied the candidate: as-set %q, want AS-OLD until apply", got)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-NEW" {
		t.Fatalf("after apply: as-set %q, want AS-NEW", got)
	}

	if err := tx.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-OLD" {
		t.Fatalf("after rollback: as-set %q, want AS-OLD", got)
	}

	bad := []sdk.ConfigSection{{Root: configRootBGP, Data: "{not json"}}
	if err := tx.verify(bad); !errors.Is(err, errFilterIrrInvalidBgpConfigJson) {
		t.Fatalf("verify of invalid JSON: err %v, want errFilterIrrInvalidBgpConfigJson", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-OLD" {
		t.Fatalf("a failed verify changed the filter: as-set %q, want AS-OLD", got)
	}
}

// VALIDATES: spec-terminal-demo-showcase AC-1, fail closed. An apply with no
// verified candidate, or a second apply of one candidate, is an error, never
// an OK that changed nothing. A verify that carries no bgp section is a
// verified no-op and its apply succeeds.
func TestFilterIRRRefusesApplyWithoutVerify(t *testing.T) {
	tx := newTestTx(t)
	if err := tx.configure(irrSection("AS-OLD")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.apply(); !errors.Is(err, errFilterIrrApplyUnverified) {
		t.Fatalf("apply without verify: err %v, want errFilterIrrApplyUnverified", err)
	}

	if err := tx.verify(irrSection("AS-NEW")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.apply(); !errors.Is(err, errFilterIrrApplyUnverified) {
		t.Fatalf("second apply of one candidate: err %v, want errFilterIrrApplyUnverified", err)
	}

	if err := tx.verify(nil); err != nil {
		t.Fatalf("verify with no bgp section: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply of a transaction that leaves bgp alone: %v", err)
	}
	if got := enrolledASSet(t, tx); got != "AS-NEW" {
		t.Fatalf("a transaction without bgp changed the filter: as-set %q, want AS-NEW", got)
	}
}

// VALIDATES: a reload that changes a peer's as-set does not carry the list
// resolved for the old one. The new state waits for its own first resolution.
// PREVENTS: an as-set edit filtering against the old set, with firstDone
// already closed, until the next periodic refresh.
func TestFilterIRRDropsTheListOfAReplacedAsSet(t *testing.T) {
	tx := newTestTx(t)
	if err := tx.configure(irrSection("AS-OLD")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	tx.plug.mu.Lock()
	old := tx.plug.byASN[65001]
	old.list = &irrPrefixList{}
	old.signalFirstResolved()
	tx.plug.mu.Unlock()

	if err := tx.verify(irrSection("AS-NEW")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	tx.plug.mu.RLock()
	st := tx.plug.byASN[65001]
	tx.plug.mu.RUnlock()
	if st.list != nil {
		t.Fatal("the list resolved for AS-OLD was carried over to AS-NEW")
	}
	if isClosed(st.firstDone) {
		t.Fatal("firstDone closed for AS-NEW before any resolution")
	}
}
