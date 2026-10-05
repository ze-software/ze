// Design: docs/architecture/route-selection.md -- AIGP session policy.
// Related: forward_aigp.go and aigp_readvertise.go -- resolved distance and replay.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// RFC 7311 Section 3.4.3 step 7: "If the route to XNH is a BGP-learned route
// that has an AIGP TLV value of Y, then set Xattr to Xattr+Y and set XNH to
// the next hop of this route."
// Step 5: "If D is below a configurable threshold, set the AIGP TLV value to
// Xattr. In either case, exit this procedure."
// RFC requirement: RFC7311-3.4.3-7 positive -- recursive metric recomputation may produce a zero increment and still announces the computed AIGP without a new source UPDATE.
// RFC requirement: RFC7311-3.4.3-7 negative -- a resolved recursive zero is not an unavailable direct-link increment and must not withdraw the route.
func TestRFC7311RecursiveZeroCostRemainsAnnounced(t *testing.T) {
	for _, prior := range []bool{false, true} {
		name := "initial-zero"
		if prior {
			name = "positive-to-zero"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			loc := locrib.Default()
			protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
			terminal := netip.MustParseAddr("198.18.233.19")
			terminalPrefix := netip.PrefixFrom(terminal, 32)
			loc.Insert(family.IPv4Unicast, terminalPrefix, locrib.Path{Source: protocol, Metric: 0})
			t.Cleanup(func() { loc.Remove(family.IPv4Unicast, terminalPrefix, protocol, 0) })
			prefix := netip.PrefixFrom(f.source.Settings().Address, 32)
			path := locrib.Path{Source: protocol, IsBGP: true, AIGPPresent: true, NextHop: terminal}
			if prior {
				path.AIGP = 7
			}
			loc.Insert(family.IPv4Unicast, prefix, path)
			f.forward(t, f.receive(t, f.body(t, 100)))
			forwardSocketBarrier(t, f.r)
			if prior {
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("initial UPDATE count=%d, want 1", len(bodies))
				}
				metric, present := aigpReceivedMetric(t, bodies[0])
				if !present || metric != 107 {
					t.Fatalf("initial recursive metric=%d present=%v, want 107", metric, present)
				}
				path.AIGP = 0
				loc.Insert(family.IPv4Unicast, prefix, path)
				f.r.readvertiseAIGP()
				forwardSocketBarrier(t, f.r)
			}
			distance := igpcost.Lookup(f.source.Settings().Address)
			if !distance.Resolved || distance.MissingAIGP || distance.Cost != 0 {
				t.Fatalf("fixture did not resolve recursive zero: %+v", distance)
			}
			bodies := aigpSocketBodies(t, f.conn)
			wantFrames := 1
			if prior {
				wantFrames++
			}
			if len(bodies) != wantFrames {
				t.Fatalf("recursive zero UPDATE count=%d, want %d", len(bodies), wantFrames)
			}
			body := bodies[len(bodies)-1]
			sections, err := wire.ParseUpdateSections(body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(sections.NLRI(body), []byte{24, 10, 20, 0}) || len(sections.Withdrawn(body)) != 0 {
				t.Fatalf("resolved recursive zero withdrew the usable route: %x", body)
			}
			metric, present := aigpReceivedMetric(t, body)
			if !present || metric != 100 {
				t.Fatalf("recursive zero metric=%d present=%v, want received 100 plus computed zero", metric, present)
			}
			_, _, hop, present := attribute.AttrFind(sections.Attrs(body), attribute.AttrNextHop)
			if !present || !bytes.Equal(hop, f.destination.Settings().LocalAddress.AsSlice()) {
				t.Fatalf("recursive zero next-hop-self=%x present=%v", hop, present)
			}
		})
	}
}

// RFC 8950 Section 3: "When the AFI/SAFI is <1/1>, <1/2>, or <1/4> and when
// the Length of Next Hop Address field is equal to 16 or 32, the next-hop
// address is of type IPv6."
// RFC requirement: RFC7311-3.4.3-7 positive -- a retained IPv4 route with a negotiated IPv6 next hop recovers with recomputed AIGP after a metric change, without a new source UPDATE.
// RFC requirement: RFC7311-3.4.3-7 negative -- replay must not silently lose a cost-withheld route by forcing an extended next hop into legacy IPv4 framing.
func TestRFC7311ExtendedNextHopCostRecovery(t *testing.T) {
	for _, paired := range []bool{false, true} {
		name := "global-16"
		if paired {
			name = "global-and-link-local-32"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			ctx := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
				ASN4: true,
				ExtendedNextHop: map[capability.Family]capability.AFI{
					{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}: capability.AFIIPv6,
				},
			}, bgpctx.DirectionSend)
			ctxID, err := bgpctx.Registry.Register(ctx)
			if err != nil {
				t.Fatal(err)
			}
			f.ctxID = ctxID
			for _, peer := range []*Peer{f.source, f.destination} {
				peer.sendCtx.Store(ctx)
				peer.sendCtxID, peer.recvCtxID = ctxID, ctxID
				peer.session.sendCtxID = ctxID
			}
			f.destination.settings.LocalAddress = netip.MustParseAddr("2001:db8:7311::1")
			f.destination.refreshForwardFacts()
			f.source.refreshForwardFacts()
			hop := netip.MustParseAddr("2001:db8:7311::2")
			nextHop := hop.AsSlice()
			if paired {
				nextHop = append(nextHop, netip.MustParseAddr("fe80::2").AsSlice()...)
			}
			mp := []byte{0, 1, 1, byte(len(nextHop))}
			mp = append(mp, nextHop...)
			mp = append(mp, 0, 24, 10, 20, 0)
			attrs := []byte{
				0x40, 1, 1, 0,
				0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
				0x80, 26, 11, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 100,
				0x80, 14, byte(len(mp)),
			}
			attrs = append(attrs, mp...)
			f.forward(t, f.receive(t, makeUpdateBody(nil, attrs, nil)))
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			if len(bodies) != 1 {
				t.Fatalf("withheld UPDATE count=%d, want 1", len(bodies))
			}
			legacyWithdraw := []byte{0, 4, 24, 10, 20, 0, 0, 0}
			mpWithdraw := []byte{0, 0, 0, 10, 0x80, 15, 7, 0, 1, 1, 24, 10, 20, 0}
			if !bytes.Equal(bodies[0], legacyWithdraw) && !bytes.Equal(bodies[0], mpWithdraw) {
				t.Fatalf("unavailable extended-next-hop cost did not withdraw the exact route: %x", bodies[0])
			}
			protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
			prefix := netip.PrefixFrom(hop, 128)
			locrib.Default().Insert(family.IPv6Unicast, prefix, locrib.Path{Source: protocol, Metric: 11})
			t.Cleanup(func() { locrib.Default().Remove(family.IPv6Unicast, prefix, protocol, 0) })
			f.r.readvertiseAIGP()
			forwardSocketBarrier(t, f.r)
			bodies = aigpSocketBodies(t, f.conn)
			if len(bodies) != 2 {
				t.Fatalf("extended-next-hop recovery UPDATE count=%d, want 2; retained MP route was not restored", len(bodies))
			}
			body := bodies[1]
			sections, err := wire.ParseUpdateSections(body)
			if err != nil {
				t.Fatal(err)
			}
			if len(sections.NLRI(body)) != 0 || len(sections.Withdrawn(body)) != 0 {
				t.Fatalf("recovery changed extended-next-hop route to legacy framing: %x", body)
			}
			outAttrs := sections.Attrs(body)
			_, _, receivedMP, present := attribute.AttrFind(outAttrs, attribute.AttrMPReachNLRI)
			wantMP := []byte{0, 1, 1, 16}
			wantMP = append(wantMP, f.destination.Settings().LocalAddress.AsSlice()...)
			wantMP = append(wantMP, 0, 24, 10, 20, 0)
			if !present || !bytes.Equal(receivedMP, wantMP) {
				t.Fatalf("recovered MP_REACH=%x present=%v, want negotiated IPv6 self hop and original NLRI %x", receivedMP, present, wantMP)
			}
			_, flags, value, present := attribute.AttrFind(outAttrs, attribute.AttrAIGP)
			wantMetric := [11]byte{1, 0, 11}
			binary.BigEndian.PutUint64(wantMetric[3:], 111)
			if !present || flags != 0x80 || !bytes.Equal(value, wantMetric[:]) {
				t.Fatalf("recovered AIGP=%x flags=%x present=%v, want 111", value, flags, present)
			}
		})
	}
}
