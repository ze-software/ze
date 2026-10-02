// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv3 Grace-LSA origination and helper receive.
//
// VALIDATES: RFC 5187 sec 2.2 on the OSPFv3 path Ze actually runs. The Grace-LSA the engine
// originates (v6OriginateGraceLSA, the restarter's OSPFv3 origination) carries the Grace
// Period TLV (Type 1, Length 4) and the Graceful restart reason TLV (Type 2, Length 1),
// read as literal octets of the installed LSA, including for zero values. A peer's
// Grace-LSA that lacks either TLV is ignored by the helper (grInspectV6Update): it neither
// moves a running helper session's grace window nor ends it.
// PREVENTS: an OSPFv3 Grace-LSA with a wrong or missing TLV type number (the codec tests
// compare the constants with themselves), and a helper acting on a grace period it never
// received.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v3LSAHeaderOctets is the OSPFv3 LSA header length (RFC 5340 sec A.4.2).
const v3LSAHeaderOctets = 20

// RFC requirement: RFC5187-2.2-1 positive -- the Grace-LSA the engine originates on an
// OSPFv3 interface (v6OriginateGraceLSA) starts its body with the literal Grace Period TLV
// header 0x0001 0x0004 followed by the 4-octet period, for a nonzero and a zero period.
// RFC requirement: RFC5187-2.2-2 positive -- the same body carries, next, the literal
// Graceful restart reason TLV header 0x0002 0x0001, the reason octet and three octets of
// zero padding, for reason 2 and reason 0 (unknown); the body is exactly these 16 octets.
func TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs(t *testing.T) {
	cases := []struct {
		name   string
		period uint32
		reason uint8
		want   []byte
	}{
		{
			name:   "period-120-reload",
			period: 120,
			reason: grReasonReload,
			want:   []byte{0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00, 0x78, 0x00, 0x02, 0x00, 0x01, 0x02, 0x00, 0x00, 0x00},
		},
		{
			name:   "period-0-unknown",
			period: 0,
			reason: 0,
			want:   []byte{0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
		},
	}
	if grReasonReload != 2 {
		t.Fatalf("grReasonReload = %d, the vector assumes 2 (software reload/upgrade)", grReasonReload)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
			router := ospftypes.RouterID{10, 0, 0, 1}
			e.lsdb.SetSelfRouterID(router)
			info := &ospflsdb.InterfaceInfo{Name: "eth0", InterfaceID: 5}
			if !e.v6OriginateGraceLSA(router, info, tc.period, tc.reason, false) {
				t.Fatal("v6OriginateGraceLSA installed nothing")
			}
			raw := installedV6GraceLSA(t, e, "eth0")
			if len(raw) < v3LSAHeaderOctets {
				t.Fatalf("installed Grace-LSA is %d octets, shorter than its header", len(raw))
			}
			if got := raw[v3LSAHeaderOctets:]; !bytes.Equal(got, tc.want) {
				t.Fatalf("Grace-LSA body = % x, want % x (TLV 1 len 4, then TLV 2 len 1)", got, tc.want)
			}
		})
	}
}

// installedV6GraceLSA returns the wire octets of the one OSPFv3 Grace-LSA in iface's link store.
func installedV6GraceLSA(t *testing.T, e *engine, iface string) []byte {
	t.Helper()
	var found [][]byte
	for _, lsa := range e.lsdb.LinkLSAs(iface) {
		if lsa.Header.Type == ospftypes.LSTypeGraceV6 {
			found = append(found, lsa.RawBytes)
		}
	}
	if len(found) != 1 {
		t.Fatalf("link store of %s holds %d Grace-LSAs, want 1", iface, len(found))
	}
	return found[0]
}

// v6GraceUpdate builds an LS Update carrying one OSPFv3 Grace-LSA from router with the given
// TLV body, through the same OSPFv3 encoder the engine uses for its own LSAs.
func v6GraceUpdate(router ospftypes.RouterID, body []byte) ospfpacket.LSUpdate {
	lsa := v6SelfLSA(ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Type:              ospfv3types.LSTypeGrace,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 7},
			AdvertisingRouter: ospfv3types.RouterID(router),
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		Body: body,
	})
	return ospfpacket.LSUpdate{LSAs: []ospfpacket.LSA{lsa}}
}

// RFC requirement: RFC5187-2.2-1 negative -- RFC 5187 prescribes no handling for a
// grace-LSA without the Grace Period TLV, and the helper cannot run a grace window it was
// never given, so Ze ignores it: a peer's OSPFv3 Grace-LSA carrying only the reason TLV,
// received through grInspectV6Update while Ze helps that peer, leaves the helper session
// and its grace end unchanged (a control Grace-LSA with both TLVs and period 600 moves it).
// RFC requirement: RFC5187-2.2-2 negative -- likewise a peer's Grace-LSA carrying only the
// Grace Period TLV (period 600, no reason TLV) is ignored: same session, same grace end.
func TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV(t *testing.T) {
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
			e := grEnableEngine(t, true, now)
			x := ospftypes.RouterID{10, 0, 0, 9}
			key := helperKey{iface: "eth0", router: x}
			e.gr.helperEnter(key, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}, false, netip.Addr{}, 0)
			before, ok := e.gr.helperGraceEnd(key)
			if !ok {
				t.Fatal("setup: no helper session for X")
			}
			e.grInspectV6Update("eth0", v6GraceUpdate(x, tc.body))
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
