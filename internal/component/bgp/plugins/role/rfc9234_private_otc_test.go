// Design: docs/guide/bgp-role.md -- OTC preservation and the Internet-facing boundary.
// Related: otc.go -- OTCEgressFilter; otc_filter.go -- both transport readers.
// RFC 9234 Section 5 -- see rfc/short/rfc9234.md.

package role

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRFC9234PrivateOTCDoesNotLeavePublicEgress presents both Private Use
// ranges at the public-to-public eBGP boundary, directly and through the
// forked evaluator. Neither rejection may rewrite the received OTC.
// RFC requirement: RFC9234-5-9 negative -- an already-present private-domain OTC is rejected on public-to-public eBGP egress rather than sending that private ASN or rewriting the preserved attribute.
func TestRFC9234PrivateOTCDoesNotLeavePublicEgress(t *testing.T) {
	clearFilterState(t)
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 64497, PeerAS: 64498}
	setFilterState(map[string]*peerRoleConfig{"10.0.0.2": {role: roleProvider}}, nil)
	for _, private := range []uint32{64512, 65534, 4200000000, 4294967294} {
		t.Run(fmt.Sprint(private), func(t *testing.T) {
			payload := buildTestPayload(buildTestAttrs(private), []byte{24, 10, 1, 0})
			var mods filterapi.ModAccumulator
			if OTCEgressFilter(filterapi.PeerFilterInfo{}, dest, payload, nil, &mods) {
				t.Error("public eBGP accepted an existing Private Use OTC")
			}
			if mods.Len() != 0 {
				t.Error("the refused route's OTC was rewritten")
			}
			fields := `{"peer-address":"10.0.0.2","local-as":64497,"peer-as":64498,"ibgp":false}`
			out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
			if err != nil {
				t.Fatal(err)
			}
			if out.Action != sdk.FilterReject {
				t.Errorf("forked export verdict %v, want reject", out.Action)
			}
			if len(out.Sets) != 0 {
				t.Error("the forked refusal rewrote the received OTC")
			}
		})
	}
}

// TestPrivateOTCBoundaryPreservesOtherSessions distinguishes the boundary
// from unconditional rejection: public upstream OTC, interior/private
// sessions and withdrawal-only bodies keep their received OTC unchanged.
func TestPrivateOTCBoundaryPreservesOtherSessions(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{"10.0.0.2": {role: roleProvider}}, nil)
	for _, tc := range []struct {
		name    string
		local   uint32
		remote  uint32
		otc     uint32
		ibgp    bool
		declare bool
	}{
		{"public upstream OTC", 64497, 64498, 64496, false, true},
		{"private destination", 64497, 65001, 65000, false, true},
		{"private local AS", 65001, 64498, 65000, false, true},
		{"internal migration session", 64497, 64498, 65000, true, true},
		{"withdrawal only", 64497, 64498, 65000, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var nlri []byte
			if tc.declare {
				nlri = []byte{24, 10, 1, 0}
			}
			payload := buildTestPayload(buildTestAttrs(tc.otc), nlri)
			dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: tc.local, PeerAS: tc.remote, IsIBGP: tc.ibgp}
			var mods filterapi.ModAccumulator
			if !OTCEgressFilter(filterapi.PeerFilterInfo{}, dest, payload, nil, &mods) {
				t.Fatal("a case outside the public Private Use OTC boundary was refused")
			}
			if mods.Len() != 0 {
				t.Error("existing OTC was rewritten outside the boundary")
			}
			otc, found, malformed := findOTC(extractAttrsFromPayload(payload))
			if !found {
				t.Error("received OTC disappeared")
			}
			if malformed {
				t.Error("received OTC became malformed")
			}
			if otc != tc.otc {
				t.Errorf("received OTC changed: value=%d, want %d", otc, tc.otc)
			}
			fields := fmt.Sprintf(`{"peer-address":"10.0.0.2","local-as":%d,"peer-as":%d,"ibgp":%t}`, tc.local, tc.remote, tc.ibgp)
			out, err := evaluateRoleFilter(otcForkedRoute(otcEgressFilterName, payload, fields))
			if err != nil {
				t.Fatal(err)
			}
			if out.Action != sdk.FilterAccept {
				t.Errorf("forked outside-boundary verdict %v, want accept", out.Action)
			}
			if len(out.Sets) != 0 {
				t.Error("forked outside-boundary result rewrote the received OTC")
			}
		})
	}
}

// TestPublicOTCBoundaryStillStampsUnmarkedRoutes keeps the existing positive
// behavior as an explicit control for the new refusal.
func TestPublicOTCBoundaryStillStampsUnmarkedRoutes(t *testing.T) {
	clearFilterState(t)
	setFilterState(map[string]*peerRoleConfig{"10.0.0.2": {role: roleProvider}}, nil)
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), LocalAS: 64497, PeerAS: 64498}
	payload := buildTestPayload(buildTestAttrs(0), []byte{24, 10, 1, 0})
	var mods filterapi.ModAccumulator
	if !OTCEgressFilter(filterapi.PeerFilterInfo{}, dest, payload, nil, &mods) {
		t.Fatal("an unmarked route was refused")
	}
	ops := mods.Ops()
	if len(ops) != 1 {
		t.Fatalf("stamp operations %d, want one", len(ops))
	}
	if len(ops[0].Buf) != otcAttrLen {
		t.Fatalf("stamp length %d, want %d", len(ops[0].Buf), otcAttrLen)
	}
	if got := binary.BigEndian.Uint32(ops[0].Buf); got != dest.LocalAS {
		t.Errorf("stamp ASN %d, want Internet-facing ASN %d", got, dest.LocalAS)
	}
}
