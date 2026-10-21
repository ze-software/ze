package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type validationCapture struct {
	mu     sync.Mutex
	bodies [][]byte
}

func validationForwardFixture(t *testing.T, addPath bool) (*reactorAPIAdapter, *Peer, *Peer, *validationCapture) {
	t.Helper()
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: addPath})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	src.recvCtxID = ctxID
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dst.settings.RSClient = true
	dst.refreshForwardFacts()
	capture := &validationCapture{}
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		capture.mu.Lock()
		defer capture.mu.Unlock()
		for index := range items {
			item := &items[index]
			for _, body := range item.rawBodies {
				capture.bodies = append(capture.bodies, bytes.Clone(body))
			}
			if len(item.updates) != 0 {
				t.Error("equal-context validation fixture unexpectedly required parsed updates")
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	return &reactorAPIAdapter{r: &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   newRecentUpdateCache(100),
		clock:           clock.RealClock{},
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: pool,
	}}, src, dst, capture
}

func validationBodies(t *testing.T, api *reactorAPIAdapter, capture *validationCapture) [][]byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := api.r.fwdPool.Barrier(ctx); err != nil {
		t.Fatal(err)
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	bodies := capture.bodies
	capture.bodies = nil
	return bodies
}

func validationParts(t *testing.T, bodies [][]byte) (announced, withdrawn []byte) {
	t.Helper()
	for _, body := range bodies {
		update, err := message.UnpackUpdate(body)
		if err != nil {
			t.Fatal(err)
		}
		announced = append(announced, update.NLRI...)
		withdrawn = append(withdrawn, update.WithdrawnRoutes...)
	}
	return announced, withdrawn
}

// TestForwardValidationRetainedRecovery observes the received-cache and stored
// replay rails across pending, valid, invalid and recovery states. ADD-PATH must
// withdraw the identifier previously advertised, and recovery must preserve the
// attributes without a replacement received UPDATE.
func TestForwardValidationRetainedRecovery(t *testing.T) {
	api, src, dst, capture := validationForwardFixture(t, true)
	route := storedAddPathRoute(7)
	attrs, err := hex.DecodeString(route.AttrHex)
	if err != nil {
		t.Fatal(err)
	}
	nlri := binary.BigEndian.AppendUint32(nil, route.PathID)
	nlri = append(nlri, 24, 10, 0, 0)
	payload := buildUpdatePayload(attrs, nlri)
	before := bytes.Clone(payload)
	update, id := newLeakTestUpdate(t, api.r.recentUpdates, payload, src.recvCtxID)
	update.WireUpdate.SetSourceID(src.SourceID())
	route.MsgID = id
	eligible := false
	present := true
	generation := id
	key := ribevents.ValidationRoute{Peer: src.Settings().Address, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24"), PathID: 7}
	ribevents.RegisterValidationLookup(func(got ribevents.ValidationRoute, msgID uint64) bool {
		return got == key && eligible && (msgID == 0 || msgID == generation)
	}, func(got ribevents.ValidationRoute) bool {
		return got == key && present
	})
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })

	if skipped, delivered := reactorForwardRS(api.r, update, id, src.Settings().Address, src); len(skipped) != 0 || delivered != 0 {
		t.Fatalf("pending fast path delivered: skipped=%v delivered=%d", skipped, delivered)
	}
	if err := api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{dst.Settings().PeerKey()}, "test-plugin", plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	announced, _ := validationParts(t, validationBodies(t, api, capture))
	if len(announced) != 0 {
		t.Fatalf("pending received route advertised: %x", announced)
	}
	if !bytes.Equal(payload, before) {
		t.Fatal("validation modified received bytes")
	}

	relay := func() [][]byte {
		t.Helper()
		if err := api.RelayStoredRoute(dst.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		return validationBodies(t, api, capture)
	}
	eligible = true
	valid := relay()
	announced, withdrawn := validationParts(t, valid)
	if len(announced) != 8 || len(withdrawn) != 0 || !bytes.Equal(announced[4:], nlri[4:]) {
		t.Fatalf("accepted route = announce %x withdraw %x", announced, withdrawn)
	}
	if len(valid) != 1 {
		t.Fatalf("one retained path produced %d UPDATE bodies", len(valid))
	}
	decoded, err := message.UnpackUpdate(valid[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.PathAttributes, attrs) {
		t.Fatalf("retained attributes = %x, received %x", decoded.PathAttributes, attrs)
	}
	advertised := bytes.Clone(announced)
	eligible = false
	announced, withdrawn = validationParts(t, relay())
	if len(announced) != 0 || !bytes.Equal(withdrawn, advertised) {
		t.Fatalf("invalid transition = announce %x withdraw %x; previous %x", announced, withdrawn, advertised)
	}
	eligible = true
	repaired := relay()
	if len(valid) != 1 || len(repaired) != 1 || !bytes.Equal(valid[0], repaired[0]) {
		t.Fatalf("recovery changed retained advertisement: %x -> %x", valid, repaired)
	}
	generation++
	if bodies := relay(); len(bodies) != 0 {
		t.Fatalf("stale retained generation reached destination: %x", bodies)
	}
	route.Withdraw = true
	route.MsgID = 0
	if bodies := relay(); len(bodies) != 0 {
		t.Fatalf("stale removal withdrew current eligible route: %x", bodies)
	}
	eligible = false
	present = false
	announced, withdrawn = validationParts(t, relay())
	if len(announced) != 0 || !bytes.Equal(withdrawn, advertised) {
		t.Fatalf("removed route withdrawal = %x/%x, want %x", announced, withdrawn, advertised)
	}
	if n := api.r.recentUpdates.Len(); n != 0 {
		t.Fatalf("forward/replay retained %d cache entries after drain", n)
	}
}

// TestForwardValidationMixedNLRI proves one rejected path cannot suppress an
// eligible sibling or swallow a withdrawal already carried by the UPDATE.
func TestForwardValidationMixedNLRI(t *testing.T) {
	api, src, dst, capture := validationForwardFixture(t, false)
	attrs := mustHex(t, storedIPv4Route("10.0.0.1").AttrHex)
	payload := buildUpdatePayload(attrs, []byte{24, 10, 0, 0, 24, 10, 0, 1})
	payload = append([]byte{0, 4, 24, 10, 0, 2}, payload[2:]...)
	update, id := newLeakTestUpdate(t, api.r.recentUpdates, payload, src.recvCtxID)
	update.WireUpdate.SetSourceID(src.SourceID())
	ribevents.RegisterValidationLookup(func(key ribevents.ValidationRoute, generation uint64) bool {
		return key.Prefix == netip.MustParsePrefix("10.0.0.0/24") && (generation == 0 || generation == id)
	}, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	sel, err := selector.Parse(dst.Settings().Address.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := api.ForwardUpdate(sel, id, "test-plugin", plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	announced, withdrawn := validationParts(t, validationBodies(t, api, capture))
	if !bytes.Equal(announced, []byte{24, 10, 0, 0}) {
		t.Fatalf("mixed announcement = %x", announced)
	}
	if !bytes.Equal(withdrawn, []byte{24, 10, 0, 1, 24, 10, 0, 2}) {
		t.Fatalf("mixed withdrawals = %x", withdrawn)
	}
}

// TestForwardValidationMPPathIdentity keeps one IPv6 ADD-PATH path while denying
// another path for the same prefix, retaining the full 32-octet next-hop field.
func TestForwardValidationMPPathIdentity(t *testing.T) {
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv6Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	nlri := binary.BigEndian.AppendUint32(nil, 7)
	nlri = append(nlri, prefix...)
	nlri = binary.BigEndian.AppendUint32(nlri, 8)
	nlri = append(nlri, prefix...)
	nh := make([]byte, 32)
	nh[0], nh[1], nh[16], nh[17] = 0x20, 1, 0xfe, 0x80
	var buf [256]byte
	n := writeMPReach(buf[:], 4, family.IPv6Unicast, nh, nlri)
	binary.BigEndian.PutUint16(buf[2:4], uint16(n))
	wu := wireu.NewWireUpdate(buf[:4+n], ctxID)
	wu.SetMessageID(17)
	update := &ReceivedUpdate{WireUpdate: wu, SourcePeerIP: netip.MustParseAddr("192.0.2.1")}
	defer update.returnFwdHandles()
	ribevents.RegisterValidationLookup(func(key ribevents.ValidationRoute, generation uint64) bool {
		return key.Family == family.IPv6Unicast && key.PathID == 7 && (generation == 0 || generation == 17)
	}, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	selected, denied, refused, err := forwardValidationWire(update)
	if err != nil {
		t.Fatal(err)
	}
	if refused != nil {
		t.Fatal("unicast paths produced a redirect-refused copy")
	}
	if selected == nil || denied == nil {
		t.Fatal("mixed MP paths did not produce both selected and withdrawn routes")
	}
	reach, err := selected.MPReach()
	if err != nil || reach == nil || !bytes.Equal(reach.NextHopBytes(), nh) || !bytes.Equal(reach[37:], nlri[:11]) {
		t.Fatalf("selected MP_REACH = %x, err %v", reach, err)
	}
	unreach, err := denied.MPUnreach()
	if err != nil || unreach == nil || !bytes.Equal(unreach[3:], nlri[11:]) {
		t.Fatalf("denied MP_UNREACH = %x, err %v", unreach, err)
	}
}

// TestForwardValidationSplitsRedirectRefusedRules proves the redirect-to-IP
// verdict is applied rule by rule on the wire, the way the RIB strips it rule
// by rule from the local install. Four flow-vpn rules of one UPDATE carry four
// Route Distinguishers; the RIB refused the action of the first and the third,
// so the selected UPDATE carries the second and the fourth, the refused copy
// the first and the third, each in received order and each with the received
// attributes, and the forward rail sends the refused copy without 0x0800.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "the BGP speaker SHOULD
// discard the redirect-to-ip extended community (and not propagate it further
// with the flow-spec route) if the last AS in the AS_PATH or AS4_PATH
// attribute of the longest prefix match for X does not match the AS of the
// EBGP peer." The longest match is looked up in the rule's own domain, so the
// verdict differs per Route Distinguisher.
//
// MUTATION: answer the first rule's verdict for every rule of the UPDATE in
// flowSpecSection (every rule lands in one copy).
func TestForwardValidationSplitsRedirectRefusedRules(t *testing.T) {
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}
	rule := func(rd byte) []byte {
		// length(1) | RD(8) | destination component 10.0.0.0/24
		return []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, rd, 1, 24, 10, 0, 0}
	}
	var nlri []byte
	for rd := byte(1); rd <= 4; rd++ {
		nlri = append(nlri, rule(rd)...)
	}
	hop := []byte{198, 51, 100, 1}
	attrs := mustHex(t, "40010100c010080800000000000000")
	mp := append([]byte{0, 1, byte(fam.SAFI), byte(len(hop))}, hop...)
	mp = append(mp, 0)
	mp = append(mp, nlri...)
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	body = append(body, attrs...)
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetMessageID(21)
	source := netip.MustParseAddr("192.0.2.1")
	update := &ReceivedUpdate{WireUpdate: wu, SourcePeerIP: source}
	defer update.returnFwdHandles()
	key := func(rd byte) ribevents.ValidationRoute {
		return ribevents.ValidationRoute{Peer: source, Family: fam, NLRI: ribevents.FlowSpecKey(rule(rd))}
	}
	ribevents.RegisterFlowSpecLookup(func(ribevents.ValidationRoute, uint64) bool { return true }, nil, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	ribevents.RegisterFlowSpecRedirectLookup(func(got ribevents.ValidationRoute, msgID uint64) bool {
		return msgID == 21 && (got == key(1) || got == key(3))
	})
	t.Cleanup(func() { ribevents.RegisterFlowSpecRedirectLookup(nil) })
	selected, denied, refused, err := forwardValidationWire(update)
	if err != nil {
		t.Fatal(err)
	}
	if denied != nil {
		t.Fatal("an eligible rule was withdrawn")
	}
	for _, tc := range []struct {
		name string
		wire *wireu.WireUpdate
		want []byte
	}{
		{"selected", selected, append(rule(2), rule(4)...)},
		{"refused", refused, append(rule(1), rule(3)...)},
	} {
		if tc.wire == nil {
			t.Fatalf("%s copy missing", tc.name)
		}
		reach, err := tc.wire.MPReach()
		if err != nil || reach == nil || !bytes.Equal(reach.NextHopBytes(), hop) || !bytes.Equal(reach[5+len(hop):], tc.want) {
			t.Fatalf("%s MP_REACH = %x, err %v; want rules %x", tc.name, reach, err, tc.want)
		}
		if !bytes.Contains(tc.wire.Payload(), mustHex(t, "c010080800000000000000")) {
			t.Fatalf("%s copy lost the received communities: %x", tc.name, tc.wire.Payload())
		}
	}
}
