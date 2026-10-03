// Design: docs/architecture/bgp/structural-forwarding.md -- next-hop self and withdrawal.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestDraftLinkLocalOneHopLostNextHopWithdraws is deliberately untagged until
// its real wire outcome is known. It does not exercise the multihop D6 carrier:
// the destination remains external and directly attached throughout.
// Draft Section 4, one-hop external default procedure: "If no next hops are
// included, the route MUST NOT be announced (treat-as-withdraw)."
func TestDraftLinkLocalOneHopLostNextHopWithdraws(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	delete(f.r.peers, f.destination.settings.PeerKey())
	dest, conn := newAnnouncePeer(t, "2001:db8:1::3")
	dest.settings.ProcessBindings = f.destination.settings.ProcessBindings
	dest.sendCtx.Store(f.destination.sendCtx.Load())
	dest.sendCtxID, dest.recvCtxID = f.ctxID, f.ctxID
	dest.negotiated.Store(f.destination.negotiated.Load())
	dest.session.sendCtxID = f.ctxID
	dest.session.negotiated = f.destination.session.negotiated
	f.destination, f.conn = dest, conn
	dest.settings.LocalAddress = netip.MustParseAddr("2001:db8:1::254")
	dest.settings.LinkLocal = netip.MustParseAddr("fe80::254")
	dest.settings.NextHopMode = NextHopSelf
	dest.settings.PeerAS = 65003
	dest.negotiated.Load().LinkLocalNextHop = true
	dest.negotiated.Load().families[family.IPv6Unicast] = true
	dest.llScope.Store(newLinkScopeFrom(llnhSegment, dest.settings.Address))
	dest.refreshForwardFacts()
	f.r.peers[dest.settings.PeerKey()] = dest
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: dest.settings.PeerKey()}, 4096)
	body, err := message.UnpackUpdate(llnhReflectedPayload("2001:db8:9::1"))
	if err != nil { t.Fatal(err) }
	_, _, mp, found := attribute.AttrFind(body.PathAttributes, attribute.AttrMPReachNLRI)
	if !found { t.Fatal("fixture has no MP_REACH") }
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x80, 14, byte(len(mp))}
	attrs = append(attrs, mp...)
	payload := buildUpdatePayload(attrs, nil)
	id := f.receive(t, payload)
	f.forward(t, id)
	f.waitBatch(t)
	initial := aigpSocketBodies(t, f.conn)
	if len(initial) != 1 { t.Fatalf("initial wire UPDATEs=%d", len(initial)) }
	advertised, err := message.UnpackUpdate(initial[0])
	if err != nil { t.Fatal(err) }
	next := llnhNextHopField(t, advertised)
	if !bytes.Contains(next, dest.settings.LocalAddress.AsSlice()) { t.Fatalf("first party next hop absent: %x", next) }

	// Loss of usable interface addresses leaves the established destination's
	// one-hop topology and negotiated capability intact. The recording socket
	// has no numeric endpoint, so resolution cannot invent a replacement.
	dest.settings.LocalAddress = netip.Addr{}
	dest.settings.LinkLocal = netip.Addr{}
	dest.refreshForwardFacts()
	if !dest.forwardFacts().nhSelfWithheld { t.Fatal("fixture did not reach the no-next-hop procedure") }
	id = f.receive(t, payload)
	sel, err := selector.Parse(dest.settings.Address.String())
	if err != nil { t.Fatal(err) }
	// A suppression return cannot stand in for the required wire withdrawal.
	_ = (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder"))
	f.drain(t)
	frames := aigpSocketBodies(t, f.conn)
	if len(frames) != 2 { t.Fatalf("one-hop no-next-hop replacement wrote %d UPDATEs total, want initial announcement then withdrawal", len(frames)) }
	withdrawn, err := message.UnpackUpdate(frames[1])
	if err != nil { t.Fatal(err) }
	if _, _, _, present := attribute.AttrFind(withdrawn.PathAttributes, attribute.AttrMPReachNLRI); present { t.Fatal("no-next-hop replacement was announced") }
	_, _, unreach, present := attribute.AttrFind(withdrawn.PathAttributes, attribute.AttrMPUnreachNLRI)
	want := append([]byte{0, 2, 1}, mp[21:]...)
	if !present || !bytes.Equal(unreach, want) { t.Fatalf("withdrawal=%x present=%v want %x", unreach, present, want) }
}
