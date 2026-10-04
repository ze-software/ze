// Design: docs/guide/route-reflection.md -- route-server transparency and per-client policy.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/selector"
)

// lowRSAttributes includes every transparency category, with nonzero ORIGIN
// and unequal opaque values so dropping or substituting an attribute is visible.
func lowRSAttributes() []byte {
	return []byte{
		0x40, 1, 1, 2,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea,
		0x40, 3, 4, 192, 0, 2, 1,
		0x80, 4, 4, 0, 0, 0, 123,
		0xc0, 8, 4, 0xfc, 0x00, 0, 9,
		0xc0, 241, 3, 0x11, 0x22, 0x33,
		0x80, 242, 3, 0x44, 0x55, 0x66,
	}
}

// TestRFC7947AllAttributesReachClient sends a complete UPDATE over the live
// session and compares each whole attribute (flags included) at the recipient.
// RFC 7947 Section 2.2: "Optional recognized and unrecognized BGP attributes, whether transitive or non-transitive, SHOULD NOT be updated by the route server (unless enforced by local IXP operator configuration) and SHOULD be passed on to other route server clients."
// RFC requirement: RFC7947-2.2-1 positive -- real ingress and recipient wire preserve ORIGIN, AS_PATH, NEXT_HOP, MED, community, unknown optional transitive and unknown optional non-transitive attributes, including their flags.
func TestRFC7947AllAttributesReachClient(t *testing.T) {
	source := lowLiveSettings("192.0.2.1", 65000, 65002)
	dest := lowLiveSettings("192.0.2.2", 65000, 65003)
	source.RSClient, dest.RSClient = true, true
	peers := lowLiveRouter(t, source, dest)
	attrs := lowRSAttributes()
	prefix := []byte{24, 203, 0, 113}
	peers[0].send(t, message.PackTo(&message.Update{PathAttributes: attrs, NLRI: prefix}, nil))
	lowEventually(t, func() bool { return peers[1].announcement(prefix) != nil }, "route-server client advertisement")
	got := peers[1].announcement(prefix)
	if !bytes.Equal(got, attrs) {
		t.Fatalf("route-server attribute transparency: got %x, want %x", got, attrs)
	}
}

// TestRouteServerTransparencyStopsAtOrdinaryPeer keeps the RFC 7947 exception
// confined to client redistribution when a reactor also serves ordinary BGP.
// RFC 7947 Section 2.2: "Optional recognized and unrecognized BGP attributes,
// whether transitive or non-transitive, SHOULD NOT be updated by the route
// server (unless enforced by local IXP operator configuration) and SHOULD be
// passed on to other route server clients."
// RFC requirement: RFC7947-2.2-1 negative -- transparency is confined to route-server clients: the same ingress preserves optional attributes for a client but strips unknown non-transitive attributes and sets unknown-transitive Partial for an ordinary peer.
func TestRouteServerTransparencyStopsAtOrdinaryPeer(t *testing.T) {
	source := lowLiveSettings("192.0.2.1", 65000, 65002)
	client := lowLiveSettings("192.0.2.2", 65000, 65003)
	ordinary := lowLiveSettings("192.0.2.3", 65000, 65000)
	source.RSClient, client.RSClient = true, true
	peers := lowLiveRouter(t, source, client, ordinary)
	attrs := lowRSAttributes()
	prefix := []byte{24, 203, 0, 114}
	peers[0].send(t, message.PackTo(&message.Update{PathAttributes: attrs, NLRI: prefix}, nil))
	lowEventually(t, func() bool {
		return peers[1].announcement(prefix) != nil && peers[2].announcement(prefix) != nil
	}, "both route-server and ordinary advertisements")
	if got := peers[1].announcement(prefix); !bytes.Equal(got, attrs) {
		t.Fatalf("client attributes changed: %x", got)
	}
	got := peers[2].announcement(prefix)
	if _, _, _, ok := attribute.AttrFind(got, attribute.AttributeCode(242)); ok {
		t.Fatal("unknown non-transitive attribute escaped the route-server client boundary")
	}
	if flags := rfc4271PublishedFlags(t, buildUpdatePayload(got, prefix), 241); flags != 0xe0 {
		t.Fatalf("ordinary peer unknown-transitive flags = %#x, want Partial set", flags)
	}
	lowAssertAttribute(t, got, attribute.AttributeCode(241), []byte{0x11, 0x22, 0x33})
}

// TestRFC7947ClientPolicyControlsWire continues the fast path's skipped list
// through ForwardUpdate and the actual socket worker. Each destination policy
// is independently selected, and a denial must emit neither announcement nor
// fallback copy. The second round reverses the decisions on the same clients.
// RFC 7947 Section 2.1: "The route server SHOULD forward UPDATE messages from its Loc-RIB or Loc-RIBs to its clients as determined by local policy."
// RFC requirement: RFC7947-x-4 positive -- a route-server client deferred by the fast path receives the actual UPDATE when its own export policy accepts it, including after that client's policy changes from reject to accept.
// RFC requirement: RFC7947-x-4 negative -- the same deferred client receives no announcement when its own policy rejects it, only the withdrawal of the prefix; acceptance by a different client does not bypass that rejection.
func TestRFC7947ClientPolicyControlsWire(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	other, otherConn := newAnnouncePeer(t, "198.18.232.3")
	other.settings.GlobalLocalAS = 65000
	other.settings.ProcessBindings = f.destination.settings.ProcessBindings
	other.sendCtx.Store(f.destination.sendCtx.Load())
	other.sendCtxID, other.recvCtxID = f.ctxID, f.ctxID
	other.negotiated.Store(f.destination.negotiated.Load())
	other.session.sendCtxID = f.ctxID
	other.session.negotiated = f.destination.session.negotiated
	f.r.peers[other.settings.PeerKey()] = other
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: other.settings.PeerKey()}, 4096)
	for _, p := range []*Peer{f.destination, other} {
		p.settings.RSClient = true
		p.settings.NextHopMode = NextHopUnchanged
		p.settings.ExportFilters = []filterapi.FilterRef{{Name: "client-policy"}}
		p.refreshForwardFacts()
	}
	f.r.api = &pluginserver.Server{}
	f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
	for round := range 2 {
		calls := make(map[string]int)
		acceptedAddr := f.destination.settings.Address.String()
		if round == 1 {
			acceptedAddr = other.settings.Address.String()
		}
		f.r.policyFilterSeam = func(_, _, _, address string, _ uint32, text string) PolicyResponse {
			calls[address]++
			if address == acceptedAddr {
				return PolicyResponse{Action: PolicyAccept}
			}
			return PolicyResponse{Action: PolicyReject}
		}
		before := []int{len(aigpSocketBodies(t, f.conn)), len(aigpSocketBodies(t, otherConn))}
		prefix := []byte{24, 203, 0, byte(113 + round)}
		id := f.receive(t, buildUpdatePayload(lowRSAttributes(), prefix))
		update, ok := f.r.recentUpdates.Get(id)
		if !ok {
			t.Fatal("source UPDATE missing from cache")
		}
		// RFC 7947 Section 2.1: no policy-agnostic send before fallback.
		skipped, sent := reactorForwardRS(f.r, update, id, f.source.settings.Address, f.source)
		if sent != 0 || len(skipped) != 2 {
			t.Fatalf("policy fallback = %v, sent=%d", skipped, sent)
		}
		if err := (&reactorAPIAdapter{r: f.r}).ForwardUpdate(selector.All(), id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")); err != nil {
			t.Fatal(err)
		}
		forwardSocketBarrier(t, f.r)
		for i, conn := range []*recordingConn{f.conn, otherConn} {
			p := f.destination
			if i == 1 {
				p = other
			}
			if calls[p.settings.Address.String()] != 1 {
				t.Fatalf("policy calls by client = %v", calls)
			}
			bodies := aigpSocketBodies(t, conn)
			if i != round {
				// A rejected client is announced nothing. It is written the
				// withdrawal of the prefix instead (RFC 7606 Section 2
				// treat-as-withdraw), because it may hold an earlier generation.
				if len(bodies) != before[i]+1 {
					t.Fatalf("policy-rejected client received %d additional UPDATEs, want the one withdrawal", len(bodies)-before[i])
				}
				w, err := message.UnpackUpdate(bodies[len(bodies)-1])
				if err != nil {
					t.Fatal(err)
				}
				if len(w.NLRI) != 0 {
					t.Fatalf("policy-rejected client was announced %x", w.NLRI)
				}
				if !bytes.Equal(w.WithdrawnRoutes, prefix) {
					t.Fatalf("policy-rejected client withdrawal %x, want %x", w.WithdrawnRoutes, prefix)
				}
				continue
			}
			if len(bodies) != before[i]+1 {
				t.Fatalf("accepted client received %d additional UPDATEs, want 1", len(bodies)-before[i])
			}
			u, err := message.UnpackUpdate(bodies[len(bodies)-1])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(u.NLRI, prefix) {
				t.Fatalf("wrong NLRI %x", u.NLRI)
			}
			lowAssertAttribute(t, u.PathAttributes, attribute.AttrOrigin, []byte{2})
		}
	}
}
