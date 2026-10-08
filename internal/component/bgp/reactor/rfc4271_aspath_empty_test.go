// Design: docs/architecture/wire/attributes.md -- present zero-length AS_PATH.
// Related: forward_as_override_composition_test.go -- Session-written forwarding proof.
package reactor

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestForwardPresentEmptyASPath forwards a present zero-length attribute through
// both forwarding rails and checks the actual recipient UPDATE. Internal and
// route-server recipients retain the empty attribute rather than gaining an AS.
// RFC 4271 Section 5.1.2: "if the AS_PATH is empty, the local system creates a path
// segment of type AS_SEQUENCE, places its own AS into that segment, and places
// that segment into the AS_PATH."
// MUTATION: Return without prepending when ASPath.Prepend sees zero segments;
// external recipients must fail while the internal and route-server controls hold.
// RFC requirement: RFC4271-5.1.2-3 positive -- a present zero-length AS_PATH
// forwarded to an external peer leaves as exactly one AS_SEQUENCE containing only
// the local AS, on both forwarding rails and at both negotiated ASN widths.
func TestForwardPresentEmptyASPath(t *testing.T) {
	for _, fast := range []bool{false, true} {
		for _, asn4 := range []bool{false, true} {
			for _, tc := range []struct {
				name   string
				peerAS uint32
				rs     bool
			}{
				{name: "external", peerAS: 65002},
				{name: "internal", peerAS: 65000},
				{name: "route-server", peerAS: 65002, rs: true},
			} {
				t.Run(fmt.Sprintf("fast=%v/asn4=%v/%s", fast, asn4, tc.name), func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					f.source.settings.RSClient = true
					f.source.refreshForwardFacts()
					peer := f.destination
					peer.settings.LocalAS = 65000
					peer.settings.PeerAS = tc.peerAS
					peer.settings.RSClient = tc.rs
					peer.settings.NextHopMode = NextHopUnchanged
					ctx := bgpctx.EncodingContextForASN4(asn4)
					ctxID, err := bgpctx.Registry.Register(ctx)
					if err != nil {
						t.Fatal(err)
					}
					peer.sendCtx.Store(ctx)
					peer.sendCtxID = ctxID
					peer.session.sendCtxID = ctxID
					peer.negotiated.Load().ASN4 = asn4
					peer.session.negotiated.ASN4 = asn4
					peer.refreshForwardFacts()

					attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
					// RFC 4271 Section 5.1.2 defines empty by the attribute length,
					// not by absence or by an AS_SEQUENCE with no members.
					attrs = append(attrs, 0x40, byte(attribute.AttrASPath), 0)
					attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
					body := buildUpdatePayload(attrs, a2InlinePrefix)
					original := slices.Clone(body)
					id := f.receive(t, body)
					// RFC 4271 Section 5.1.2: exercise real forwarding, not only a generator.
					if fast {
						update, ok := f.r.recentUpdates.Get(id)
						if !ok {
							t.Fatal("received UPDATE missing")
						}
						skipped, sent := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
						if len(skipped) != 0 {
							t.Fatalf("skipped recipients=%v", skipped)
						}
						if sent != 1 {
							t.Fatalf("forwarded=%d, want one", sent)
						}
						f.source.session.flushFwdDirty()
					} else {
						f.forward(t, id)
					}
					forwardSocketBarrier(t, f.r)
					bodies := aigpSocketBodies(t, f.conn)
					if len(bodies) != 1 {
						t.Fatalf("recipient UPDATEs=%d, want one", len(bodies))
					}
					update, err := message.UnpackUpdate(bodies[0])
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(update.NLRI, a2InlinePrefix) {
						t.Fatalf("NLRI=%x, want %x", update.NLRI, a2InlinePrefix)
					}
					_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
					if !found {
						t.Fatal("recipient AS_PATH missing")
					}
					var want []byte
					if tc.name == "external" {
						want = []byte{byte(attribute.ASSequence), 1, 0xfd, 0xe8}
						if asn4 {
							want = []byte{byte(attribute.ASSequence), 1, 0, 0, 0xfd, 0xe8}
						}
					}
					if !slices.Equal(value, want) {
						t.Errorf("recipient AS_PATH=%x, want %x", value, want)
					}
					if !slices.Equal(body, original) {
						t.Error("forwarding changed shared received bytes")
					}
				})
			}
		}
	}
}
