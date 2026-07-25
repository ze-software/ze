// VALIDATES: the RFC 3623 sec 3.1/3.2 helper state machine -- entry only when ALL checks
// pass (AC-16) and each failing check blocks entry (AC-17); a re-received Grace-LSA updates
// the grace period without churn (AC-18); while helping, X's adjacency stays advertised and X
// stays DR (AC-16); exit on flush / grace-expiry / strict-checking topology change with the
// stub-area external exception (AC-19/20); and a malformed IPv6 Grace-LSA is ignored (AC-21).
// PREVENTS: a helper dropping X's link on a transient regression, or exiting early on a benign
// stub-area external change, or crashing on a malformed Grace-LSA.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// TestHelperEntryAllChecksPass (AC-16): entry allowed when every RFC 3623 sec 3.1 check passes.
func TestHelperEntryAllChecksPass(t *testing.T) {
	ok, reason := helperEntryAllowed(helperEntry{
		policyEnabled: true, selfRestarting: false, graceRemaining: true,
		fullAdjacency: true, lsdbUnchanged: true,
	})
	if !ok {
		t.Fatalf("all checks pass but entry denied: %q", reason)
	}
}

// TestHelperEntryRejectedPerCheck (AC-17): each failing check blocks entry (table-driven).
func TestHelperEntryRejectedPerCheck(t *testing.T) {
	base := helperEntry{policyEnabled: true, selfRestarting: false, graceRemaining: true, fullAdjacency: true, lsdbUnchanged: true}
	cases := []struct {
		name   string
		mutate func(*helperEntry)
	}{
		{"policy-disabled", func(e *helperEntry) { e.policyEnabled = false }},
		{"self-restarting", func(e *helperEntry) { e.selfRestarting = true }},
		{"grace-expired", func(e *helperEntry) { e.graceRemaining = false }},
		{"no-full-adjacency", func(e *helperEntry) { e.fullAdjacency = false }},
		{"lsdb-changed", func(e *helperEntry) { e.lsdbUnchanged = false }},
	}
	for _, tc := range cases {
		e := base
		tc.mutate(&e)
		if ok, _ := helperEntryAllowed(e); ok {
			t.Fatalf("%s: entry must be blocked", tc.name)
		}
	}
}

func grHelperManager(t *testing.T, now time.Time) *grManager {
	t.Helper()
	e := grEnableEngine(t, false, now)
	return e.gr
}

// TestHelperKeepsAdjacencyAdvertisedV4 / V6 (AC-16, A-9, R-3): after entry the helper reports
// X as a Full neighbor so the topology builder keeps X's link advertised.
func TestHelperKeepsAdjacencyAdvertisedV4(t *testing.T) { testHelperKeepsAdjacency(t, false) }
func TestHelperKeepsAdjacencyAdvertisedV6(t *testing.T) { testHelperKeepsAdjacency(t, true) }

func testHelperKeepsAdjacency(t *testing.T, v6 bool) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, v6, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	g := graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}
	e.gr.helperEnter(helperKey{iface: "eth0", router: x}, g, true, netip.MustParseAddr("10.0.0.9"), 5)
	if !e.gr.isHelping("eth0", x) {
		t.Fatalf("v6=%v: expected to be helping X after entry", v6)
	}
	nbrs := e.gr.helpingNeighbors("eth0")
	if len(nbrs) != 1 || nbrs[0].RouterID != x || nbrs[0].State != "full" {
		t.Fatalf("v6=%v: helper must surface X as a Full neighbor: %+v", v6, nbrs)
	}
}

// TestHelperKeepsXAsDR (AC-16, A-14): X stays DR while helping.
func TestHelperKeepsXAsDR(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := grHelperManager(t, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	m.helperEnter(helperKey{iface: "eth0", router: x}, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}, true, netip.Addr{}, 0)
	if dr, ok := m.helperDR("eth0"); !ok || dr != x {
		t.Fatalf("X must stay DR while helping (got %v ok=%v)", dr, ok)
	}
}

// TestHelperAlreadyHelpingUpdatesGrace (AC-18): a re-received Grace-LSA updates the grace
// period; no second session.
func TestHelperAlreadyHelpingUpdatesGrace(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := grHelperManager(t, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	key := helperKey{iface: "eth0", router: x}
	m.helperEnter(key, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 60}, false, netip.Addr{}, 0)
	first, _ := m.helperGraceEnd(key)
	m.onGraceReceived(graceReceived{iface: "eth0", advRouter: x, gracePeriod: 600})
	second, ok := m.helperGraceEnd(key)
	if !ok {
		t.Fatalf("session must still exist after re-receipt")
	}
	if !second.After(first) {
		t.Fatalf("grace end must extend on re-receipt: first=%v second=%v", first, second)
	}
	if n := m.helperSessionCount(); n != 1 {
		t.Fatalf("re-receipt must not create a second session: count=%d", n)
	}
}

// TestHelperExitOnFlush (AC-19): a flushed Grace-LSA exits the helper.
func TestHelperExitOnFlush(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := grHelperManager(t, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	m.helperEnter(helperKey{iface: "eth0", router: x}, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}, false, netip.Addr{}, 0)
	m.onGraceReceived(graceReceived{iface: "eth0", advRouter: x, withdrawn: true})
	if m.isHelping("eth0", x) {
		t.Fatalf("a flushed Grace-LSA must exit the helper")
	}
}

// TestHelperExitOnGraceExpiry (AC-19): the grace timer exits the helper.
func TestHelperExitOnGraceExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := grHelperManager(t, now)
	x := ospftypes.RouterID{10, 0, 0, 9}
	key := helperKey{iface: "eth0", router: x}
	m.helperEnter(key, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}, false, netip.Addr{}, 0)
	m.helperGraceExpired(key)
	if m.isHelping("eth0", x) {
		t.Fatalf("grace expiry must exit the helper")
	}
}

// TestHelperStrictExitOnTopologyChange (AC-19): a strict-checking content change that would
// flood to X exits the helper.
func TestHelperStrictExitOnTopologyChange(t *testing.T) {
	if !helperShouldExitOnChange(true, "normal", ospftypes.LSTypeRouter, true) {
		t.Fatalf("strict checking must exit on a Router-LSA change that floods to X")
	}
	if helperShouldExitOnChange(false, "normal", ospftypes.LSTypeRouter, true) {
		t.Fatalf("with strict checking off, a change must not exit")
	}
}

// TestHelperStubAreaExternalDoesNotExit (AC-20, R-4): a changed AS-external in a stub area
// would not flood to X, so helping does not terminate.
func TestHelperStubAreaExternalDoesNotExit(t *testing.T) {
	if helperShouldExitOnChange(true, "stub", ospftypes.LSTypeASExternal, false) {
		t.Fatalf("a stub-area external change must NOT exit the helper")
	}
	if helperShouldExitOnChange(true, "nssa", ospftypes.LSTypeASExternal, false) {
		t.Fatalf("an nssa-area external change must NOT exit the helper")
	}
}

// TestHelperRejectsGraceLSAMissingTLV (AC-21): a malformed IPv6 Grace-LSA (missing a mandatory
// TLV) is ignored, not crashed on.
func TestHelperRejectsGraceLSAMissingTLV(t *testing.T) {
	// Build a Grace-LSA with only the Grace Period TLV (no Restart Reason) by hand-encoding
	// a header + a single TLV, then confirm the decoder rejects it.
	lsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{Type: ospfv3types.LSTypeGrace, LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 5}},
		Body:   []byte{0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00, 0x78}, // only Grace Period TLV
	}
	raw := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(raw, 0)
	if _, ok := v3GraceFromLSA(raw); ok {
		t.Fatalf("a Grace-LSA missing the mandatory Reason TLV must be rejected")
	}
}
