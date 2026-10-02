// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv2 Grace-LSA origination and helper receive.
// Related: gr_restarter.go -- grOriginateGraceLSAs, the OSPFv2 origination this drives.
// Related: gr_helper.go -- graceOnReceive, the OSPFv2 helper receive this drives.
//
// VALIDATES: RFC 3623 sec A on the OSPFv2 path Ze runs. The Grace-LSA grOriginateGraceLSAs
// installs carries the Grace Period TLV (Type 1, Length 4) and the Graceful restart reason
// TLV (Type 2, Length 1) as literal octets, plus the IP interface address TLV (Type 3,
// Length 4) on a broadcast segment. A peer's OSPFv2 Grace-LSA that lacks the Grace Period
// or the reason TLV is ignored by the helper (graceOnReceive): the running helper session's
// grace window neither moves nor ends.
// PREVENTS: an OSPFv2 Grace-LSA with a wrong TLV type number (the codec units compare the
// constants with themselves), and a helper acting on a grace period it never received.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3623InstalledGraceBody returns the raw opaque body of the one Grace-LSA the engine
// holds for iface, read from the link-scope LSDB without the codec.
func rfc3623InstalledGraceBody(t *testing.T, e *engine, iface string) []byte {
	t.Helper()
	var bodies [][]byte
	for _, l := range e.lsdb.LinkLSAs(iface) {
		if l.OpaqueType() == ospfpacket.GraceOpaqueType {
			bodies = append(bodies, l.Body)
		}
	}
	if len(bodies) != 1 {
		t.Fatalf("interface %s holds %d Grace-LSAs, want 1", iface, len(bodies))
	}
	return bodies[0]
}

// RFC requirement: RFC3623-A-2 positive -- the Grace-LSA grOriginateGraceLSAs installs on a
// point-to-point and on a broadcast interface opens with the Grace Period TLV as the literal
// octets 00 01 00 04 00 00 00 78 (Type 1, Length 4, 120 seconds).
// RFC requirement: RFC3623-A-3 positive -- the same Grace-LSAs carry the Graceful restart
// reason TLV as the literal octets 00 02 00 01 02 00 00 00 (Type 2, Length 1, reason 2
// software reload/upgrade, padded to 32 bits); on the point-to-point segment the body is
// exactly these 16 octets.
// RFC requirement: RFC3623-A-4 positive -- on the broadcast segment the body continues with
// the IP interface address TLV header as the literal octets 00 03 00 04 and four octets of
// value, 24 octets in all.
func TestRFC3623OriginatedGraceLSATLVOctets(t *testing.T) {
	// Goal: the type numbers on the wire, read without the codec. Method: one running
	// point-to-point and one running broadcast interface, grOriginateGraceLSAs, compare the
	// installed bodies with literal octets.
	e := rfc3623RunningEngine(t, map[string]networkType{
		"eth0": ospftypes.NetworkPointToPoint,
		"eth1": ospftypes.NetworkBroadcast,
	})
	if ifs := e.grOriginateGraceLSAs(120, grReasonReload, false); len(ifs) != 2 {
		t.Fatalf("grOriginateGraceLSAs touched %v, want 2 interfaces", ifs)
	}
	mandatory := []byte{
		0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00, 0x78,
		0x00, 0x02, 0x00, 0x01, 0x02, 0x00, 0x00, 0x00,
	}
	p2p := rfc3623InstalledGraceBody(t, e, "eth0")
	if !bytes.Equal(p2p, mandatory) {
		t.Fatalf("eth0 (point-to-point) Grace-LSA body = % x, want % x", p2p, mandatory)
	}
	broadcast := rfc3623InstalledGraceBody(t, e, "eth1")
	if len(broadcast) != 24 {
		t.Fatalf("eth1 (broadcast) Grace-LSA body is %d octets (% x), want 24", len(broadcast), broadcast)
	}
	if !bytes.Equal(broadcast[:16], mandatory) {
		t.Fatalf("eth1 (broadcast) Grace-LSA mandatory TLVs = % x, want % x", broadcast[:16], mandatory)
	}
	if want := []byte{0x00, 0x03, 0x00, 0x04}; !bytes.Equal(broadcast[16:20], want) {
		t.Fatalf("eth1 (broadcast) IP interface address TLV header = % x, want % x", broadcast[16:20], want)
	}
}

// RFC requirement: RFC3623-A-2 negative -- RFC 3623 prescribes no handling for a grace-LSA
// without the Grace Period TLV, and the helper cannot run a grace window it was never
// given, so Ze ignores it: a peer's OSPFv2 Grace-LSA carrying only the reason TLV, received
// through graceOnReceive while Ze helps that peer, leaves the helper session and its grace
// end unchanged (a control Grace-LSA with both TLVs and period 600 moves it).
// RFC requirement: RFC3623-A-3 negative -- likewise a peer's Grace-LSA carrying only the
// Grace Period TLV (period 600, no reason TLV) is ignored: same session, same grace end.
func TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV(t *testing.T) {
	// Goal: the handling of absence (owner ruling 2). Method: a helper session for X, then
	// X's Grace-LSA body through the OSPFv2 opaque receive hook, compare the grace end.
	periodOnly := []byte{0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x02, 0x58}
	reasonOnly := []byte{0x00, 0x02, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00}
	complete := []byte{0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x02, 0x58, 0x00, 0x02, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00}
	cases := []struct {
		name    string
		body    []byte
		applied bool
	}{
		{name: "missing-grace-period", body: reasonOnly, applied: false},
		{name: "missing-restart-reason", body: periodOnly, applied: false},
		{name: "control-both-tlvs", body: complete, applied: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000_000, 0)
			e := grEnableEngine(t, false, now)
			x := ospftypes.RouterID{10, 0, 0, 9}
			key := helperKey{iface: "eth0", router: x}
			e.gr.helperEnter(key, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}, false, netip.Addr{}, 0)
			before, ok := e.gr.helperGraceEnd(key)
			if !ok {
				t.Fatal("setup: no helper session for X")
			}
			e.graceOnReceive(opaqueReceived{
				OpaqueType:        ospfpacket.GraceOpaqueType,
				Interface:         "eth0",
				AdvertisingRouter: x,
				Body:              tc.body,
			})
			after, ok := e.gr.helperGraceEnd(key)
			if !ok {
				t.Fatal("the Grace-LSA ended the helper session; it must be ignored or applied, never a flush")
			}
			if tc.applied {
				if want := now.Add(600 * time.Second); !after.Equal(want) {
					t.Fatalf("control: grace end %v, want %v (the complete Grace-LSA must reach the helper)", after, want)
				}
				return
			}
			if !after.Equal(before) {
				t.Fatalf("grace end moved from %v to %v: a Grace-LSA missing a mandatory TLV was acted on", before, after)
			}
		})
	}
}
