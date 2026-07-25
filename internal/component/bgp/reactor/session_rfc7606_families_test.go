// Overview: session_read.go — processMessage treat-as-withdraw synthesis + dispatch
// Overview: session_validation.go — mpFamilyDispatchable negotiation predicate (D-5)
// RFC: rfc/short/rfc7606.md — Section 2 (treat-as-withdraw), Section 3.g (one MP_UNREACH/UPDATE)

package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// setupEstablishedSessionTwoFamilies brings up an eBGP session negotiating BOTH IPv4 unicast
// and IPv6 unicast, so a treat-as-withdraw UPDATE can carry two different MP families and
// have both survive the negotiation predicate. Mirrors setupEstablishedSessionEBGP.
func setupEstablishedSessionTwoFamilies(t *testing.T) (*Session, net.Conn, func()) {
	t.Helper()

	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	startSession(t, session)

	client, server := net.Pipe()
	cleanup := func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	}

	readOpen(t, session, server, client)

	// Peer OPEN advertising ASN4 + IPv4 unicast + IPv6 unicast.
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 18,
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4 unicast
			1, 4, 0, 2, 0, 1, // Multiprotocol IPv6 unicast
		},
	}
	openBytes := message.PackTo(peerOpen, nil)
	go func() {
		client.Write(openBytes) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		client.Read(buf) //nolint:errcheck // drain KEEPALIVE
	}()
	require.NoError(t, session.ReadAndProcess())

	keepaliveBytes := message.PackTo(message.NewKeepalive(), nil)
	go func() {
		client.Write(keepaliveBytes) //nolint:errcheck // test goroutine
	}()
	require.NoError(t, session.ReadAndProcess())

	require.Equal(t, fsm.StateEstablished, session.State())
	neg := session.Negotiated()
	require.NotNil(t, neg)
	require.True(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}))
	require.True(t, neg.SupportsFamily(capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}))

	return session, client, cleanup
}

// TestSessionRFC7606TreatAsWithdrawTwoFamiliesDispatchesBoth verifies AC-8: a treat-as-
// withdraw UPDATE carrying MP_REACH (family Y) and MP_UNREACH (family X, X != Y) withdraws
// BOTH families end to end.
//
// RFC 7606 Section 3.g allows only one MP_UNREACH per UPDATE, and the RIB reads only the
// first, so the two families are dispatched as two withdraw-only UPDATEs (D-3/D-8). Both must
// reach the receiver; the second rides a noPoolBufID BufHandle carrying its own heap body, so
// it is entered in the forward cache and reaches both the RIB and route-server clients exactly
// like the primary (see TestSessionRFC7606TreatAsWithdrawTwoFamiliesEntersForwardCache).
//
// VALIDATES: both MP families in a two-family treat-as-withdraw UPDATE are dispatched as
// withdrawals, each in its own UPDATE the RIB's first-match MP_UNREACH accessor can read.
// PREVENTS: the second family being merged into one body where the RIB never sees it (AC-8).
func TestSessionRFC7606TreatAsWithdrawTwoFamiliesDispatchesBoth(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionTwoFamilies(t)
	defer cleanup()

	var dispatched [][]byte
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			dispatched = append(dispatched, append([]byte(nil), wu.Payload()...))
		}
		return false
	}

	update := buildTwoFamilyTreatAsWithdrawUpdate()

	go func() {
		sendUpdateAndDrain(client, buildUpdateMsg(update))
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err, "treat-as-withdraw must not error")
	require.Equal(t, fsm.StateEstablished, session.State(), "session must survive")

	require.Len(t, dispatched, 2, "each MP family rides its own withdraw-only UPDATE (RFC 7606 3.g)")

	// Read each dispatched body through the exact first-match accessor the RIB uses; the two
	// families must be exactly {IPv6 unicast, IPv4 unicast}, with one MP_UNREACH per body.
	got := map[family.Family]bool{}
	for _, payload := range dispatched {
		wu := wireu.NewWireUpdate(payload, 0)
		mu, muErr := wu.MPUnreach()
		require.NoError(t, muErr)
		require.NotNil(t, mu, "each dispatched body must carry an MP_UNREACH the RIB can read")
		got[mu.Family()] = true
	}
	assert.True(t, got[family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}],
		"IPv6 unicast (from MP_REACH) must be withdrawn")
	assert.True(t, got[family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}],
		"IPv4 unicast (from MP_UNREACH) must be withdrawn")
}

// TestRFC7606TreatAsWithdrawNonNegotiatedFamilyDrops verifies AC-9: a treat-as-withdraw
// UPDATE whose only routes belong to an MP family that was never negotiated is dropped, with
// no NOTIFICATION and no teardown.
//
// RFC 7606 treat-as-withdraw synthesis converts an MP_REACH into an MP_UNREACH so the
// announced routes are withdrawn. If that conversion ran blindly for a non-negotiated family,
// the synthesized MP_UNREACH would reach the strict RFC 4760 Section 6 family check and tear
// the session down -- a teardown path the pre-synthesis "drop the malformed UPDATE" behavior
// never had. D-5 skips the non-negotiated family at synthesis: nothing was installed for it,
// so there is nothing to withdraw, and the UPDATE is dropped exactly as before.
//
// VALIDATES: a non-negotiated MP_REACH in a treat-as-withdraw UPDATE is dropped without a
// teardown, not converted into a withdrawal that trips the strict-mode family check.
// PREVENTS: a malformed UPDATE for an un-negotiated family becoming a peer-triggerable reset.
func TestRFC7606TreatAsWithdrawNonNegotiatedFamilyDrops(t *testing.T) {
	// The helper negotiates only IPv4 unicast.
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// MP_REACH for IPv6 unicast (NOT negotiated), well-formed.
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01, // SAFI = 1 (Unicast)
		0x10, // Next-hop length = 16
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01, // Next-hop = ::1
		0x00,                         // Reserved
		0x20, 0x20, 0x01, 0x0d, 0xb8, // NLRI = 2001:db8::/32
	}
	// A malformed ORIGIN (length 2) makes this treat-as-withdraw (RFC 7606 Section 7.1); the
	// only routes it carries are in the non-negotiated MP_REACH.
	pathAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN with length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	pathAttrs = append(pathAttrs, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)

	update := make([]byte, 0, 4+len(pathAttrs))
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)

	go func() {
		sendUpdateAndDrain(client, buildUpdateMsg(update))
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err,
		"a non-negotiated MP family in a treat-as-withdraw UPDATE must be dropped, not error out")
	require.Equal(t, fsm.StateEstablished, session.State(),
		"session must survive: the non-negotiated family gains no teardown path")
	require.Equal(t, 0, *callbackCount,
		"nothing is dispatched: the non-negotiated family had nothing installed to withdraw")
}

// buildTwoFamilyTreatAsWithdrawUpdate builds an UPDATE body carrying MP_REACH (IPv6 unicast,
// announcing 2001:db8::/32) followed by MP_UNREACH (IPv4 unicast, withdrawing 10.0.0.0/8),
// with a malformed ORIGIN (length 2) that makes it treat-as-withdraw (RFC 7606 Section 7.1).
// Synthesis splits the two MP families across two withdraw-only UPDATEs (D-3/D-8): the primary
// carries the IPv6 MP_UNREACH, the extra body carries the IPv4 MP_UNREACH.
func buildTwoFamilyTreatAsWithdrawUpdate() []byte {
	// MP_REACH IPv6 unicast (AFI=2/SAFI=1): announce 2001:db8::/32.
	mpReach := []byte{0x00, 0x02, 0x01, 0x10}
	mpReach = append(mpReach, make([]byte, 16)...)
	mpReach = append(mpReach, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)
	// MP_UNREACH IPv4 unicast (AFI=1/SAFI=1): withdraw 10.0.0.0/8.
	mpUnreach := []byte{0x00, 0x01, 0x01, 0x08, 0x0a}

	pathAttrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	pathAttrs = append(pathAttrs, 0x80, 0x0e, byte(len(mpReach)))
	pathAttrs = append(pathAttrs, mpReach...)
	pathAttrs = append(pathAttrs, 0x80, 0x0f, byte(len(mpUnreach)))
	pathAttrs = append(pathAttrs, mpUnreach...)

	update := make([]byte, 0, 4+len(pathAttrs))
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)
	return update
}

// TestSessionRFC7606TreatAsWithdrawExtraFamilyForwardCacheEligible verifies AC-8 at the
// dispatch boundary: the extra MP family's synthesized withdraw is handed to the receive path
// with a cache-eligible BufHandle (the noPoolBufID sentinel over its own body), NOT an empty
// one. reactor_notify.go only enters an UPDATE in the recentUpdates forward cache when
// buf.Buf != nil; an empty BufHandle would leave the second family's withdraw uncached, so a
// route server forwarding it would miss the cache and log a false "BUG: ForwardUpdatesDirect:
// msgID missing from cache" while dropping the withdrawal.
//
// VALIDATES: the extra body rides a non-nil, non-pool (noPoolBufID) BufHandle carrying its own
// bytes, so it enters the forward cache exactly like the primary and its eviction is a no-op.
// PREVENTS: regressing to the empty-BufHandle dispatch that orphaned the second family (AC-8).
func TestSessionRFC7606TreatAsWithdrawExtraFamilyForwardCacheEligible(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionTwoFamilies(t)
	defer cleanup()

	type capture struct {
		payload []byte
		buf     BufHandle
	}
	var captured []capture
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		buf BufHandle, _ map[string]any, _ string,
	) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			captured = append(captured, capture{
				payload: append([]byte(nil), wu.Payload()...),
				buf:     buf,
			})
		}
		return false
	}

	update := buildTwoFamilyTreatAsWithdrawUpdate()
	go func() {
		sendUpdateAndDrain(client, buildUpdateMsg(update))
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	require.Len(t, captured, 2, "each MP family rides its own withdraw-only UPDATE (RFC 7606 3.g)")

	// Exactly the extra body (not the primary) rides a noPoolBufID handle; the primary rides
	// the real session pool buffer. The extra handle must be cache-eligible (Buf != nil) and
	// carry the extra body's own bytes so the forward cache holds the right payload.
	var extra *capture
	for i := range captured {
		if captured[i].buf.ID == noPoolBufID {
			require.Nil(t, extra, "only the extra MP family rides a noPoolBufID handle")
			extra = &captured[i]
		}
	}
	require.NotNil(t, extra,
		"the extra MP family's withdraw must ride a cache-eligible noPoolBufID BufHandle, not an empty one")
	require.NotNil(t, extra.buf.Buf,
		"the extra body's BufHandle must carry a non-nil buffer so reactor_notify.go enters it in the forward cache")
	require.Equal(t, extra.payload, extra.buf.Buf,
		"the BufHandle must carry the extra body's own bytes (the payload the route server forwards)")
}

// TestSessionRFC7606TreatAsWithdrawTwoFamiliesEntersForwardCache verifies AC-8 end to end
// through the real reactor delivery path (the stub-based sibling tests cannot observe the
// forward cache). A two-family treat-as-withdraw UPDATE must enter BOTH synthesized withdraw
// UPDATEs in recentUpdates, so a route server forwarding the SECOND family finds its cache
// entry via ForwardCached/ForwardUpdatesDirect instead of missing it and logging a false
// "BUG: ForwardUpdatesDirect: msgID missing from cache" while dropping the withdrawal.
//
// VALIDATES: both bodies are cached and both reach the delivery receiver.
// PREVENTS: the second family's withdraw being orphaned from the forward cache (AC-8).
func TestSessionRFC7606TreatAsWithdrawTwoFamiliesEntersForwardCache(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionTwoFamilies(t)
	defer cleanup()

	// Wire the session to a REAL reactor so notifyMessageReceiver runs and the forward cache
	// is exercised. The peer address matches the session's settings.Address so findPeerByAddr
	// resolves it; consumerCount > 0 keeps cache entries retained (Activate does not evict).
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	peerAddr := netip.MustParseAddr("192.0.2.1")
	require.NoError(t, reactor.AddPeer(NewPeerSettings(peerAddr, 65001, 65002, 0x01020301)))
	received := 0
	reactor.setMessageReceiver(&testDeliveryReceiver{
		consumerCount: 1,
		onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {
			received++
		},
	})
	session.onMessageReceived = reactor.notifyMessageReceiver

	update := buildTwoFamilyTreatAsWithdrawUpdate()
	go func() {
		sendUpdateAndDrain(client, buildUpdateMsg(update))
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())

	require.Equal(t, 2, received,
		"both synthesized withdraw UPDATEs must reach the delivery receiver")
	require.Equal(t, 2, reactor.recentUpdates.Len(),
		"both bodies must be entered in the forward cache (AC-8): the second family's withdraw is not orphaned")
}
