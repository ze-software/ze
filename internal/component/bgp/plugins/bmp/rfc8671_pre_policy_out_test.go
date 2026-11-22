// RFC 8671 Section 5.2 conformance tests: pre-policy Adj-RIB-Out monitoring
// (L=0 with O=1, and zero/empty mandatory attributes when unknown at pre-policy
// phase completion).
//
// Related: bmp_pre_policy_out.go -- normalizePrePolicyAdjRIBOutBody
// Related: bmp_events.go -- handleSenderPrePolicyOut, peerHeaderFromEvent
// Related: bmp_replay.go -- cacheAdjUpdate, replayPeerLocked

package bmp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func prePolicyOutEvent(peerAddr string, body []byte, nextHopUnknown bool) *rpc.StructuredEvent {
	return &rpc.StructuredEvent{
		PeerAddress: peerAddr,
		PeerAS:      rpc.RemoteASOf(65001),
		EventType:   rpc.EventKindUpdatePrePolicyOut,
		Direction:   rpc.DirectionSent,
		RawMessage: &bgptypes.RawMessage{
			Type:                    msgtype.TypeUPDATE,
			RawBytes:                body,
			Direction:               rpc.DirectionSent,
			PrePolicyOut:            true,
			PrePolicyNextHopUnknown: nextHopUnknown,
		},
	}
}

// buildIPv4TestUpdate builds a BGP UPDATE body announcing 10.0.1.0/24 with
// ORIGIN=IGP, AS_PATH=[65001] (4-byte), and NEXT_HOP=198.51.100.1.
func buildIPv4TestUpdate() []byte {
	attrs := []byte{
		// ORIGIN (code 1): IGP (0)
		byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 1, 0,
		// AS_PATH (code 2): AS_SEQUENCE [65001] (4-octet ASN)
		byte(attribute.FlagTransitive), byte(attribute.AttrASPath), 6, 2, 1, 0x00, 0x00, 0xFD, 0xE9,
		// NEXT_HOP (code 3): 198.51.100.1
		byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), 4, 198, 51, 100, 1,
	}
	nlri := []byte{24, 10, 0, 1}
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)
	return body
}

// buildIPv6TestUpdate builds a BGP UPDATE body announcing 2001:db8:1::/48 via
// MP_REACH_NLRI with ORIGIN=IGP, AS_PATH=[65001], and next hop 2001:db8::1.
func buildIPv6TestUpdate() []byte {
	nh := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	nlri := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	mpVal := make([]byte, 0, 3+1+len(nh)+1+len(nlri))
	mpVal = append(mpVal, 0x00, 0x02, 0x01) // AFI=2 (IPv6), SAFI=1 (unicast)
	mpVal = append(mpVal, byte(len(nh)))
	mpVal = append(mpVal, nh...)
	mpVal = append(mpVal, 0x00) // reserved
	mpVal = append(mpVal, nlri...)

	attrs := make([]byte, 0, 32+len(mpVal))
	attrs = append(attrs,
		byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 1, 0,
		byte(attribute.FlagTransitive), byte(attribute.AttrASPath), 6, 2, 1, 0x00, 0x00, 0xFD, 0xE9,
		byte(attribute.FlagOptional), byte(attribute.AttrMPReachNLRI), byte(len(mpVal)),
	)
	attrs = append(attrs, mpVal...)

	body := make([]byte, 4+len(attrs))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	return body
}

// RFC requirement: RFC8671-5.2-1 positive -- "The L flag MUST be set to 0 to
// indicate pre-policy." With adj-rib-out-pre-policy enabled through the reload
// rail, both live pre-policy Adj-RIB-Out Route Monitoring messages and replayed
// pre-policy Adj-RIB-Out routes and End-of-RIB markers carry O=1 and L=0 on the
// collector wire.
func TestRFC8671PrePolicyAdjRIBOutSetsLFlagZero(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:      newBMPState(),
		openCache:  make(map[string]*openPair),
		dedupState: make(map[string]map[uint64]struct{}),
		peerUps:    map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
		stopCh:     make(chan struct{}),
	}
	engine := startReloadEngine(t, bp, "")
	liveCollectorSession(t, bp, engine, client, map[string]any{
		"route-monitoring-policy": policyAll,
		"collector":               testCollectorEntry(),
	})

	// Enabling adj-rib-out-pre-policy alters session behavior, so Section 7.2
	// bounces the peer and sends the pre-policy Adj-RIB-Out EOR (O=1, L=0)
	// alongside the Adj-RIB-In (O=0, L=0) and post-policy Adj-RIB-Out (O=1, L=1) EORs.
	done := make(chan error, 1)
	go func() {
		done <- engine.reloadBGP(senderJSON(t, map[string]any{
			"route-monitoring-policy": policyAll,
			"adj-rib-out-pre-policy":  yangTrue,
			"collector":               testCollectorEntry(),
		}))
	}()

	var sawPrePolicyOutEOR bool
	for range 5 { // PeerDown, PeerUp, 3 EORs (Adj-RIB-In, Post-Policy Out, Pre-Policy Out)
		msg, err := readBMPFromPipe(server)
		require.NoError(t, err)
		if rm, ok := msg.(*RouteMonitoring); ok {
			if rm.Peer.Flags&PeerFlagO != 0 && rm.Peer.Flags&PeerFlagL == 0 {
				requireReloadEOR(t, rm, PeerFlagO)
				sawPrePolicyOutEOR = true
			}
		}
	}
	awaitReload(t, done)
	require.True(t, sawPrePolicyOutEOR, "enabling adj-rib-out-pre-policy must emit a pre-policy Adj-RIB-Out End-of-RIB with O=1, L=0")

	// Live pre-policy Adj-RIB-Out UPDATE must carry O=1 and L=0.
	body := buildIPv4TestUpdate()
	res := asyncRead(server)
	bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, false))
	got := <-res
	require.NoError(t, got.err)
	rm, ok := got.msg.(*RouteMonitoring)
	require.True(t, ok, "expected *RouteMonitoring, got %T", got.msg)
	require.Equal(t, uint8(PeerFlagO), rm.Peer.Flags&(PeerFlagO|PeerFlagL|peerFlagsReserved),
		"pre-policy Adj-RIB-Out must set O=1, L=0, reserved=0; got flags %#x", rm.Peer.Flags)
	require.False(t, rm.Peer.isPostPolicy(), "L flag must be 0 for pre-policy Adj-RIB-Out")
	require.True(t, rm.Peer.isAdjRIBOut(), "O flag must be 1 for pre-policy Adj-RIB-Out")

	// Replaying the cached Adj-RIB snapshot to a newly primed collector must
	// also emit the cached pre-policy Adj-RIB-Out route with O=1, L=0.
	replaySrv, replayCli := net.Pipe()
	defer closeLog(replaySrv, "replay-server")
	defer closeLog(replayCli, "replay-client")

	ss := &senderSession{name: "replay", conn: replayCli, stopCh: make(chan struct{})}
	go func() {
		bp.eventMu.Lock()
		ss.writeMu.Lock()
		bp.primeSender(ss)
		ss.writeMu.Unlock()
		bp.eventMu.Unlock()
	}()

	// Expect PeerUp + 1 cached pre-policy Adj-RIB-Out route + 3 EORs.
	var replayedPrePolicyRoute bool
	for range 5 {
		msg, err := readBMPFromPipe(replaySrv)
		require.NoError(t, err)
		if rm, ok := msg.(*RouteMonitoring); ok {
			if len(rm.BGPUpdate) > message.HeaderLen+4 {
				require.Equal(t, uint8(PeerFlagO), rm.Peer.Flags&(PeerFlagO|PeerFlagL),
					"replayed pre-policy Adj-RIB-Out route must carry O=1, L=0; got %#x", rm.Peer.Flags)
				replayedPrePolicyRoute = true
			}
		}
	}
	require.True(t, replayedPrePolicyRoute, "primeSender must replay cached pre-policy Adj-RIB-Out route with O=1, L=0")
}

// RFC requirement: RFC8671-5.2-1 negative -- L=0 with O=1 is reserved strictly
// for pre-policy Adj-RIB-Out when enabled. The YANG leaf defaults to false so
// disabled senders emit no pre-policy Adj-RIB-Out messages, while post-policy
// Adj-RIB-Out still carries O=1, L=1 and Adj-RIB-In still carries O=0, L=0.
func TestRFC8671PrePolicyAdjRIBOutDistinguishesFromPostPolicyAndDefaultOff(t *testing.T) {
	// Verify YANG schema declares adj-rib-out-pre-policy as boolean default false.
	leaf := yangLeafEntry(t, "ze-bgp-conf", "bgp", "bmp", "sender", "adj-rib-out-pre-policy")
	require.Equal(t, gyang.Ybool, leaf.Type.Kind)
	def, ok := leaf.SingleDefaultValue()
	require.True(t, ok, "adj-rib-out-pre-policy must declare a default")
	require.Equal(t, "false", def)

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)
	body := buildIPv4TestUpdate()

	// Default (adjRIBOutPrePolicy=false): pre-policy Adj-RIB-Out event emits nothing.
	bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, false))

	// Post-policy Adj-RIB-Out (EventKindUpdate, DirectionSent) MUST still set O=1 AND L=1.
	resPost := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body))
	gotPost := <-resPost
	require.NoError(t, gotPost.err)
	rmPost, ok := gotPost.msg.(*RouteMonitoring)
	require.True(t, ok, "expected *RouteMonitoring, got %T", gotPost.msg)
	require.Equal(t, uint8(PeerFlagO|PeerFlagL), rmPost.Peer.Flags&(PeerFlagO|PeerFlagL),
		"post-policy Adj-RIB-Out must keep O=1 and L=1; got %#x", rmPost.Peer.Flags)

	// Even with adjRIBOutPrePolicy=true, post-policy sent and pre-policy received
	// keep their own O/L flags (O=1,L=1 and O=0,L=0 respectively).
	bp.setSenderPolicy(policyAll, true, false)

	resIn := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, body))
	gotIn := <-resIn
	require.NoError(t, gotIn.err)
	rmIn, ok := gotIn.msg.(*RouteMonitoring)
	require.True(t, ok)
	require.Equal(t, uint8(0), rmIn.Peer.Flags&(PeerFlagO|PeerFlagL),
		"Adj-RIB-In must keep O=0 and L=0; got %#x", rmIn.Peer.Flags)

	body2 := buildIPv4TestUpdate()
	body2[len(body2)-1] = 2 // 10.0.2.0/24 so dedup does not suppress
	resPost2 := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body2))
	gotPost2 := <-resPost2
	require.NoError(t, gotPost2.err)
	rmPost2, ok := gotPost2.msg.(*RouteMonitoring)
	require.True(t, ok)
	require.Equal(t, uint8(PeerFlagO|PeerFlagL), rmPost2.Peer.Flags&(PeerFlagO|PeerFlagL),
		"post-policy Adj-RIB-Out must not lose L=1 when pre-policy Adj-RIB-Out is enabled; got %#x", rmPost2.Peer.Flags)
}

// RFC requirement: RFC8671-5.2-2 positive -- "All mandatory attributes, such as
// next hop, MUST be either zero or have an empty length if they are unknown at
// the pre-policy phase completion." Verifies IPv4 NEXT_HOP zeroing (0.0.0.0),
// missing ORIGIN/AS_PATH/NEXT_HOP synthesis (empty length / zero), IPv6
// MP_REACH_NLRI empty next-hop length (0), and replay normalization over the
// live BMP sender pipe.
func TestRFC8671PrePolicyAdjRIBOutNormalizesUnknownMandatoryAttrs(t *testing.T) {
	t.Run("ipv4 unknown next hop zeroed to 0.0.0.0", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		body := buildIPv4TestUpdate() // has NEXT_HOP 198.51.100.1
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, true))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)
		require.Equal(t, uint8(PeerFlagO), rm.Peer.Flags&(PeerFlagO|PeerFlagL))

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		sections, err := wire.ParseUpdateSections(wireBody)
		require.NoError(t, err)
		attrs := sections.Attrs(wireBody)

		_, _, nh, foundNH := attribute.AttrFind(attrs, attribute.AttrNextHop)
		require.True(t, foundNH, "NEXT_HOP must be present")
		require.Equal(t, []byte{0, 0, 0, 0}, nh, "unknown IPv4 NEXT_HOP must be zero (0.0.0.0)")

		_, _, orig, foundOrig := attribute.AttrFind(attrs, attribute.AttrOrigin)
		require.True(t, foundOrig)
		require.Equal(t, []byte{0}, orig, "known ORIGIN must stay intact")

		_, _, asPath, foundAS := attribute.AttrFind(attrs, attribute.AttrASPath)
		require.True(t, foundAS)
		require.Equal(t, []byte{2, 1, 0x00, 0x00, 0xFD, 0xE9}, asPath, "known AS_PATH must stay intact")
		require.Equal(t, []byte{24, 10, 0, 1}, sections.NLRI(wireBody), "NLRI must stay intact")
	})

	t.Run("ipv4 missing ORIGIN AS_PATH NEXT_HOP encoded empty or zero", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		// Inline IPv4 NLRI 10.0.9.0/24 with no mandatory attributes present yet.
		bareBody := []byte{0x00, 0x00, 0x00, 0x00, 24, 10, 0, 9}
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", bareBody, true))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		sections, err := wire.ParseUpdateSections(wireBody)
		require.NoError(t, err)
		attrs := sections.Attrs(wireBody)

		_, _, orig, foundOrig := attribute.AttrFind(attrs, attribute.AttrOrigin)
		require.True(t, foundOrig, "missing ORIGIN must be emitted with empty length")
		require.Empty(t, orig, "unknown ORIGIN must have empty length 0")

		_, _, asPath, foundAS := attribute.AttrFind(attrs, attribute.AttrASPath)
		require.True(t, foundAS, "missing AS_PATH must be emitted with empty length")
		require.Empty(t, asPath, "unknown AS_PATH must have empty length 0")

		_, _, nh, foundNH := attribute.AttrFind(attrs, attribute.AttrNextHop)
		require.True(t, foundNH, "missing NEXT_HOP must be emitted as zero")
		require.Equal(t, []byte{0, 0, 0, 0}, nh, "unknown NEXT_HOP must be 0.0.0.0")
	})

	t.Run("ipv6 MP_REACH_NLRI unknown next hop encoded with empty length", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		body := buildIPv6TestUpdate() // has MP next hop 2001:db8::1
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, true))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)
		require.Equal(t, uint8(PeerFlagO), rm.Peer.Flags&(PeerFlagO|PeerFlagL))

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		sections, err := wire.ParseUpdateSections(wireBody)
		require.NoError(t, err)
		_, _, mpVal, foundMP := attribute.AttrFind(sections.Attrs(wireBody), attribute.AttrMPReachNLRI)
		require.True(t, foundMP, "MP_REACH_NLRI must remain present")
		require.GreaterOrEqual(t, len(mpVal), 5)
		require.Equal(t, []byte{0x00, 0x02, 0x01}, mpVal[:3], "AFI/SAFI must remain IPv6 unicast")
		require.Equal(t, byte(0), mpVal[3], "unknown MP_REACH_NLRI next hop must have empty length 0")
		require.Equal(t, byte(0), mpVal[4], "reserved byte must be 0")
		require.Equal(t, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, mpVal[5:], "IPv6 NLRI must stay intact")
	})

	t.Run("replay snapshot preserves normalized zero/empty next hop", func(t *testing.T) {
		bp := &BMPPlugin{
			state:              newBMPState(),
			openCache:          make(map[string]*openPair),
			dedupState:         make(map[string]map[uint64]struct{}),
			peerUps:            map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
			routeMonitorPolicy: policyPrePolicy,
			adjRIBOutPrePolicy: true,
			stopCh:             make(chan struct{}),
		}
		// Cache while no collector is connected.
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", buildIPv4TestUpdate(), true))

		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		ss := &senderSession{name: "replay", conn: client, stopCh: make(chan struct{})}
		go func() {
			bp.eventMu.Lock()
			ss.writeMu.Lock()
			bp.primeSender(ss)
			ss.writeMu.Unlock()
			bp.eventMu.Unlock()
		}()

		var sawNormalized bool
		for range 4 { // PeerUp + 1 pre-policy route + 2 EORs (Adj-RIB-In, Pre-Policy Out)
			msg, err := readBMPFromPipe(server)
			require.NoError(t, err)
			if rm, ok := msg.(*RouteMonitoring); ok && len(rm.BGPUpdate) > message.HeaderLen+4 {
				wireBody := rm.BGPUpdate[message.HeaderLen:]
				sections, err := wire.ParseUpdateSections(wireBody)
				require.NoError(t, err)
				_, _, nh, found := attribute.AttrFind(sections.Attrs(wireBody), attribute.AttrNextHop)
				require.True(t, found)
				require.Equal(t, []byte{0, 0, 0, 0}, nh, "replayed pre-policy Adj-RIB-Out route must keep zeroed NEXT_HOP")
				sawNormalized = true
			}
		}
		require.True(t, sawNormalized)
	})
}

// RFC requirement: RFC8671-5.2-2 negative -- zeroing or emptying applies ONLY
// when a mandatory attribute is unknown at pre-policy completion. When mandatory
// attributes (IPv4 NEXT_HOP, IPv6 MP_REACH_NLRI next hop, ORIGIN, AS_PATH) are
// known, they are conveyed verbatim, and withdrawal-only UPDATEs receive no
// fabricated mandatory attributes.
func TestRFC8671PrePolicyAdjRIBOutPreservesKnownMandatoryAttrsAndWithdrawals(t *testing.T) {
	t.Run("known ipv4 mandatory attributes preserved verbatim", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		body := buildIPv4TestUpdate() // NEXT_HOP = 198.51.100.1, ORIGIN = 0, AS_PATH = [65001]
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, false))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		require.True(t, bytes.Equal(wireBody, body), "known IPv4 pre-policy UPDATE must be conveyed verbatim without zeroing")

		sections, err := wire.ParseUpdateSections(wireBody)
		require.NoError(t, err)
		_, _, nh, found := attribute.AttrFind(sections.Attrs(wireBody), attribute.AttrNextHop)
		require.True(t, found)
		require.Equal(t, []byte{198, 51, 100, 1}, nh, "known IPv4 NEXT_HOP must not be zeroed")
	})

	t.Run("known ipv6 MP_REACH_NLRI next hop preserved verbatim", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		body := buildIPv6TestUpdate()
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", body, false))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		require.True(t, bytes.Equal(wireBody, body), "known IPv6 pre-policy UPDATE must be conveyed verbatim")

		sections, err := wire.ParseUpdateSections(wireBody)
		require.NoError(t, err)
		_, _, mpVal, found := attribute.AttrFind(sections.Attrs(wireBody), attribute.AttrMPReachNLRI)
		require.True(t, found)
		require.Equal(t, byte(16), mpVal[3], "known IPv6 next hop length must remain 16")
	})

	t.Run("withdrawal-only UPDATE gains no invented mandatory attributes", func(t *testing.T) {
		server, client := net.Pipe()
		defer closeLog(server, "server")
		defer closeLog(client, "client")

		bp := newPipeSender(client, false)
		bp.setSenderPolicy(policyAll, true, false)

		// Withdraw 10.0.1.0/24, 0 path attributes, 0 NLRI.
		withdrawBody := []byte{0x00, 0x04, 24, 10, 0, 1, 0x00, 0x00}
		res := asyncRead(server)
		bp.handleStructuredEvent(prePolicyOutEvent("10.0.0.1", withdrawBody, true))
		got := <-res
		require.NoError(t, got.err)
		rm := got.msg.(*RouteMonitoring)

		wireBody := rm.BGPUpdate[message.HeaderLen:]
		require.Equal(t, withdrawBody, wireBody, "withdrawal-only pre-policy UPDATE must not gain mandatory attributes")
	})
}
