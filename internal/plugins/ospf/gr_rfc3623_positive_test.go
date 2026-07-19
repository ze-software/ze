// VALIDATES: the RFC 3623 positive Grace-LSA / graceful-restart properties that have a single
// meaningful polarity -- an originated Grace-LSA never freezes its age (sec A), a planned restart
// retains the FIB (sec 2.1), and an unplanned restart floods one Grace-LSA per active interface,
// encapsulated in a Link State Update, to AllSPFRouters 224.0.0.5 (sec 5).
// PREVENTS: a Grace-LSA that stops aging, a FIB torn down on graceful stop, or an unplanned
// restart that misses an interface or floods to the wrong destination.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	ospfpacket "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	ospftypes "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// countGraceLSAs returns how many of the given LSAs are Opaque Type 3 Grace-LSAs.
func countGraceLSAs(lsas []ospfpacket.LSA) int {
	n := 0
	for i := range lsas {
		if lsas[i].OpaqueType() == ospfpacket.GraceOpaqueType {
			n++
		}
	}
	return n
}

// TestGraceLSANeverSetsDoNotAge (RFC 3623 sec A): an originated Grace-LSA carries a normal, aging
// LS age -- the DoNotAge bit is never set. It drives the real OriginateOpaque producer with the
// exact Grace-LSA body the restarter emits and inspects the installed LSA header.
func TestGraceLSANeverSetsDoNotAge(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	db := ospflsdb.New(func() time.Time { return now })
	router := ospftypes.RouterID{10, 0, 0, 1}
	db.SetSelfRouterID(router)

	db.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     router,
		OpaqueType: ospfpacket.GraceOpaqueType,
		OpaqueID:   0,
		Scope:      ospftypes.LSTypeOpaqueLink,
		Interface:  "eth0",
		Options:    ospftypes.OptionO,
		Body:       grV4Body(120, grReasonReload, [4]byte{192, 0, 2, 1}, true),
	})

	lsas := db.LinkLSAs("eth0")
	var grace *ospfpacket.LSA
	for i := range lsas {
		if lsas[i].OpaqueType() == ospfpacket.GraceOpaqueType {
			grace = &lsas[i]
			break
		}
	}
	if grace == nil {
		t.Fatalf("expected an Opaque Type 3 Grace-LSA in the eth0 link store, got %d LSAs", len(lsas))
	}

	// RFC requirement: RFC3623-A-5 positive -- an originated Grace-LSA carries LS age 0 with the
	// DoNotAge bit (0x8000) clear: OpaqueOriginateInput has no DoNotAge field and OriginateOpaque
	// stamps LS age 0 with normal aging, so the frozen-age bit is never set (RFC 3623 sec A).
	if grace.Header.Age.DoNotAge() {
		t.Fatalf("Grace-LSA must never set the DoNotAge bit (RFC 3623 sec A)")
	}
	if got := grace.Header.Age.Age(); got != 0 {
		t.Fatalf("originated Grace-LSA LS age = %d, want 0", got)
	}
	if raw := grace.RawBytes; len(raw) < 2 || raw[0]&0x80 != 0 {
		t.Fatalf("Grace-LSA raw LS-age high bit (DoNotAge) must be clear; first age byte = %#x", raw[0])
	}
}

// TestPrepareRestartRetainsFIB (RFC 3623 sec 2.1): a planned restart raises the graceful-stop
// state so route install is suppressed; the ensuing engine stop's RemoveAll is a no-op and the
// pre-restart forwarding table stays in place across the restart.
func TestPrepareRestartRetainsFIB(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grPrepareEngine(t, now) // GR-enabled engine, router-id + one running interface

	// Before a graceful restart is prepared, SPF install/removal runs normally.
	if e.gr.suppressInstall() {
		t.Fatalf("install must not be suppressed before a graceful restart is prepared")
	}
	if err := e.gr.prepareRestart(grReasonReload); err != nil {
		t.Fatalf("prepareRestart: %v", err)
	}

	// RFC requirement: RFC3623-2.1-1 positive -- a planned restart raises the graceful-stop state
	// (prepareRestart sets gracefulStop, gr_restarter.go:49) so suppressInstall() reports true
	// (gr.go:235); that gate, wired into the SPF computer via SetInstallSuppress, makes the ensuing
	// engine stop's RemoveAll a no-op, so the pre-restart FIB is retained across the restart (§2.1).
	if !e.gr.suppressInstall() {
		t.Fatalf("after prepareRestart the graceful stop must suppress install so RemoveAll is skipped and the FIB is retained")
	}
}

// TestUnplannedGraceLSAFloodsToAllSPFRouters (RFC 3623 sec 5): a Grace-LSA originated on a
// broadcast segment is encapsulated in a Link State Update and flooded to AllSPFRouters
// (224.0.0.5) via the standard link-scope flood path the unplanned restart rides.
func TestUnplannedGraceLSAFloodsToAllSPFRouters(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	db := ospflsdb.New(func() time.Time { return now })
	self := ospftypes.RouterID{1, 1, 1, 1}
	db.SetSelfRouterID(self)
	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name:          "eth0",
			AreaID:        ospftypes.BackboneArea,
			AreaType:      ospflsdb.AreaTypeNormal,
			NetworkType:   ospflsdb.NetworkBroadcast,
			State:         ospflsdb.InterfaceStateDR,
			RouterID:      self,
			DR:            self,
			TransmitDelay: 1,
			Neighbors: []ospflsdb.NeighborInfo{{
				RouterID:      ospftypes.RouterID{2, 2, 2, 2},
				Address:       netip.MustParseAddr("10.0.0.2"),
				State:         ospflsdb.NeighborStateFull,
				OpaqueCapable: true,
			}},
		}}
	})
	type send struct {
		dst netip.Addr
		pkt ospfpacket.Packet
	}
	var sends []send
	db.SetTx(func(_ string, dst netip.Addr, payload []byte) error {
		p, err := ospfpacket.DecodePacket(payload)
		if err != nil {
			return err
		}
		sends = append(sends, send{dst: dst, pkt: p})
		return nil
	})

	// Originate the Grace-LSA exactly as the unplanned restart does for this interface.
	db.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router:     self,
		OpaqueType: ospfpacket.GraceOpaqueType,
		OpaqueID:   0,
		Scope:      ospftypes.LSTypeOpaqueLink,
		Interface:  "eth0",
		Options:    ospftypes.OptionO,
		Body:       grV4Body(120, grUnplannedReason(), [4]byte{10, 0, 0, 1}, true),
	})

	// RFC requirement: RFC3623-5-2 positive -- a Grace-LSA originated on a broadcast segment is
	// encapsulated in a Link State Update and flooded to AllSPFRouters (224.0.0.5), the destination
	// RFC 3623 sec 5 mandates for the unplanned-restart Grace-LSA flood.
	found := false
	var floodDst netip.Addr
	for i := range sends {
		if sends[i].pkt.LSUpdate == nil {
			continue
		}
		for _, l := range sends[i].pkt.LSUpdate.LSAs {
			if l.OpaqueType() == ospfpacket.GraceOpaqueType {
				found = true
				floodDst = sends[i].dst
			}
		}
	}
	if !found {
		t.Fatalf("Grace-LSA was not flooded in a Link State Update; sends = %+v", sends)
	}
	if floodDst != transport.AllSPFRouters {
		t.Fatalf("Grace-LSA flooded to %v, want AllSPFRouters %v (224.0.0.5)", floodDst, transport.AllSPFRouters)
	}
}

// TestUnplannedGraceLSAPerActiveInterface (RFC 3623 sec 5): the unplanned-restart origination pass
// walks every active interface and originates exactly one Grace-LSA per active (non-passive)
// interface; passive interfaces originate none.
func TestUnplannedGraceLSAPerActiveInterface(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	rid := ospftypes.RouterID{10, 0, 0, 1}
	e.mu.Lock()
	e.cfg.RouterID = rid
	e.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkPointToPoint}
	e.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkPointToPoint}
	e.running["passive0"] = interfaceConfig{Name: "passive0", AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, Passive: true}
	e.mu.Unlock()
	e.lsdb.SetSelfRouterID(rid)

	// maybeUnplannedRestart originates the Grace-LSAs through grOriginateGraceLSAs; drive that pass.
	ifs := e.grOriginateGraceLSAs(120, grUnplannedReason(), false)

	// RFC requirement: RFC3623-5-3 positive -- on an unplanned restart the restarter walks every
	// active interface and originates exactly one Grace-LSA per active (non-passive) interface,
	// each installed and flooded through the standard link-scope Link State Update path; the
	// passive interface originates none (RFC 3623 sec 5).
	got := map[string]bool{}
	for _, name := range ifs {
		got[name] = true
	}
	if len(ifs) != 2 || !got["eth0"] || !got["eth1"] || got["passive0"] {
		t.Fatalf("grOriginateGraceLSAs touched %v, want exactly {eth0, eth1} (passive excluded)", ifs)
	}
	for _, iface := range []string{"eth0", "eth1"} {
		if n := countGraceLSAs(e.lsdb.LinkLSAs(iface)); n != 1 {
			t.Fatalf("interface %s has %d Grace-LSAs, want exactly 1", iface, n)
		}
	}
	if n := countGraceLSAs(e.lsdb.LinkLSAs("passive0")); n != 0 {
		t.Fatalf("passive interface originated %d Grace-LSAs, want 0", n)
	}
}
