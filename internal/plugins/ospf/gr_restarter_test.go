// VALIDATES: the RFC 3623 sec 2/2.2/2.3 restarter state machine -- enter suppresses self-LSA
// origination and route install (AC-7/AC-8) for both families, the three exit triggers
// (all-adjacencies / inconsistent-LSA / grace-expiry, AC-12/13/14) each run the exit actions
// and clear suppression (AC-15), and Grace-LSA origination keeps LS age at 0 without reset on
// retransmit (AC-5).
// PREVENTS: a self-LSA leaking during restart, routes churning, or the grace window never
// closing (a stuck restarter).
package ospf

import (
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestRestarterSuppressesSelfLSAsV4(t *testing.T) { testRestarterSuppresses(t, false) }
func TestRestarterSuppressesSelfLSAsV6(t *testing.T) { testRestarterSuppresses(t, true) }

// testRestarterSuppresses (AC-7, A-7, R-2): while in restart, originateSelfLSAs is a no-op
// (the shared chokepoint returns early) for both families.
func testRestarterSuppresses(t *testing.T, v6 bool) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, v6, now)
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, nil)
	if !e.gr.suppressOrigination() {
		t.Fatalf("v6=%v: origination must be suppressed while in restart", v6)
	}
	// originateSelfLSAs must short-circuit (no panic, no work) while suppressed.
	e.originateSelfLSAs()
	if !e.gr.inRestart() {
		t.Fatalf("v6=%v: still expected to be in restart", v6)
	}
}

// TestRestarterExitAllAdjacencies (AC-12): the restarter exits once every pre-restart
// adjacency re-reaches Full.
func TestRestarterExitAllAdjacencies(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	a := ospftypes.RouterID{10, 0, 0, 2}
	b := ospftypes.RouterID{10, 0, 0, 3}
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, []ospftypes.RouterID{a, b})
	e.gr.noteAdjacencyFull(a)
	if !e.gr.inRestart() {
		t.Fatalf("still one adjacency outstanding; must stay in restart")
	}
	e.gr.noteAdjacencyFull(b)
	if e.gr.inRestart() {
		t.Fatalf("all adjacencies Full; must exit restart")
	}
}

// TestRestarterExitInconsistentLSA (AC-13): an inconsistent LSA exits immediately.
func TestRestarterExitInconsistentLSA(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, []ospftypes.RouterID{{10, 0, 0, 2}})
	e.gr.noteInconsistentLSA()
	if e.gr.inRestart() {
		t.Fatalf("inconsistent LSA must exit restart")
	}
}

// TestRestarterExitGraceExpiry (AC-14): the grace timer exits the restart.
func TestRestarterExitGraceExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, []ospftypes.RouterID{{10, 0, 0, 2}})
	e.gr.graceExpired()
	if e.gr.inRestart() {
		t.Fatalf("grace expiry must exit restart")
	}
}

// TestRestarterExitActions (AC-15): exit clears the in-restart flag and re-enables install.
func TestRestarterExitActions(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, nil)
	if !e.gr.suppressInstall() {
		t.Fatalf("install must be suppressed while in restart")
	}
	e.gr.exitRestart(grExitGraceExpiry)
	if e.gr.suppressInstall() || e.gr.suppressOrigination() {
		t.Fatalf("exit must clear origination + install suppression")
	}
}

// TestGRDisabledPrepareRefused mirrors AC-25 on the restarter path.
func TestGRDisabledPrepareRefused(t *testing.T) {
	e := newEngine(nil)
	if err := e.gr.prepareRestart(grReasonReload); err == nil {
		t.Fatalf("prepareRestart must refuse when disabled")
	}
}

// TestGraceLSAv4OriginatedViaCarrier (A-2, AC-3): an IPv4 Grace-LSA originated through the
// ext-1 opaque link-store carrier lands in the link store as an Opaque Type 3 LSA.
func TestGraceLSAv4OriginatedViaCarrier(t *testing.T) {
	e := newEngine(nil)
	router := ospftypes.RouterID{10, 0, 0, 1}
	e.lsdb.SetSelfRouterID(router)
	body := grV4Body(120, grReasonReload, [4]byte{192, 0, 2, 1}, true)
	e.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     router,
		OpaqueType: ospfpacket.GraceOpaqueType,
		OpaqueID:   0,
		Scope:      ospftypes.LSTypeOpaqueLink,
		Interface:  "eth0",
		Options:    ospftypes.OptionO,
		Body:       body,
	})
	lsas := e.lsdb.LinkLSAs("eth0")
	found := false
	for i := range lsas {
		if lsas[i].OpaqueType() == ospfpacket.GraceOpaqueType {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an Opaque Type 3 Grace-LSA in the eth0 link store, got %d LSAs", len(lsas))
	}
}

// TestGraceLSAv6Originated (A-12, AC-4): an IPv6 Grace-LSA originates via OriginateLinkSelf
// with LS Type 0x000B (neutral sentinel) and LS ID = Interface ID.
func TestGraceLSAv6Originated(t *testing.T) {
	e := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	router := ospftypes.RouterID{10, 0, 0, 1}
	e.lsdb.SetSelfRouterID(router)
	info := &ospflsdb.InterfaceInfo{Name: "eth0", InterfaceID: 5}
	if !e.v6OriginateGraceLSA(router, info, 120, grReasonReload, false) {
		t.Fatalf("v6OriginateGraceLSA returned false")
	}
	lsas := e.lsdb.LinkLSAs("eth0")
	found := false
	for i := range lsas {
		if lsas[i].Header.Type == ospftypes.LSTypeGraceV6 && lsas[i].Header.LinkStateID == v6SummaryLSID(5) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a Grace-LSA (LS ID = Interface ID 5) in the eth0 link store, got %d LSAs", len(lsas))
	}
}

// TestGraceLSAAgeNotResetOnRetransmit (AC-5, R-5): re-originating the SAME Grace-LSA body does
// not create a newer instance (LS age is not re-stamped); the LSDB retransmits the original.
func TestGraceLSAAgeNotResetOnRetransmit(t *testing.T) {
	e := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	router := ospftypes.RouterID{10, 0, 0, 1}
	e.lsdb.SetSelfRouterID(router)
	info := &ospflsdb.InterfaceInfo{Name: "eth0", InterfaceID: 5}
	if !e.v6OriginateGraceLSA(router, info, 120, grReasonReload, false) {
		t.Fatalf("first origination should install a new instance")
	}
	if e.v6OriginateGraceLSA(router, info, 120, grReasonReload, false) {
		t.Fatalf("re-originating the same Grace-LSA must NOT create a newer instance (no LS-age reset)")
	}
}

// TestRestarterReElectsSelfDR (AC-11, A-14): the restarter keeps its DR role when a
// Waiting-state Hello lists it as DR during restart.
func TestRestarterReElectsSelfDR(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	// Not in restart: no re-election.
	if e.gr.shouldReElectSelfDR(true, true) {
		t.Fatalf("must not re-elect self DR when not in restart")
	}
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, nil)
	if !e.gr.shouldReElectSelfDR(true, true) {
		t.Fatalf("in restart + Waiting + Hello lists self as DR -> must re-elect self DR")
	}
	if e.gr.shouldReElectSelfDR(false, true) {
		t.Fatalf("not in Waiting -> no forced re-election")
	}
	if e.gr.shouldReElectSelfDR(true, false) {
		t.Fatalf("Hello does not list self as DR -> no re-election")
	}
}
