// Design: docs/architecture/edge-cases/as4.md -- the RFC 6793 Section 4.2.3 receive procedure
// RFC: rfc/short/rfc6793.md -- AS4_PATH reconstruction and the NEW-speaker discard
// Related: session_read.go -- processMessage, the ingest site under test
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server rail
//
// Every test in this file enters through processMessage, ForwardUpdate or
// reactorForwardRS. The collapse helper is never called directly: a test that
// drove it would prove the rule and say nothing about whether a received UPDATE
// reaches it.

package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"log/slog"
	"maps"
	"math"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The AS numbers one mixed-width relay is built from.
//
// collapseSourceAS is mappable, so an OLD speaker writes it into AS_PATH as
// itself. collapseRealAS is not, so the same speaker writes AS_TRANS in its
// place and carries the real value in AS4_PATH (RFC 6793 Section 4.2.2). Those
// two facts are what make the pair readable: a reconstruction that dropped the
// AS4_PATH keeps collapseSourceAS and loses only collapseRealAS.
const (
	collapseLocalAS  uint32 = 65000
	collapseSourceAS uint32 = 65100
	collapseRealAS   uint32 = 4200000123
	collapseASTrans  uint32 = 23456
)

const collapseSourceAddr = "10.9.0.1"

// collapseAttr builds one path attribute in wire form: flags, code, a one-octet
// length and the value. Every value in this file is well inside 255 octets, so
// the extended-length header never applies.
func collapseAttr(flags, code byte, value []byte) []byte {
	out := make([]byte, 0, 3+len(value))
	out = append(out, flags, code, byte(len(value)))
	return append(out, value...)
}

// collapseASPathValue builds one AS_SEQUENCE segment holding asns at the given
// AS number width, which is 2 octets toward an OLD speaker and 4 otherwise.
func collapseASPathValue(octets int, asns ...uint32) []byte {
	out := make([]byte, 0, 2+len(asns)*octets)
	out = append(out, byte(attribute.ASSequence), byte(len(asns)))
	for _, asn := range asns {
		if octets == 2 {
			out = binary.BigEndian.AppendUint16(out, uint16(asn)) //nolint:gosec // every 2-octet fixture ASN is mappable
			continue
		}
		out = binary.BigEndian.AppendUint32(out, asn)
	}
	return out
}

// collapseAggregatorValue builds an AGGREGATOR value at the given AS number
// width: the aggregating AS followed by its IPv4 address.
func collapseAggregatorValue(octets int, asn uint32) []byte {
	out := make([]byte, 0, octets+4)
	if octets == 2 {
		out = binary.BigEndian.AppendUint16(out, uint16(asn)) //nolint:gosec // callers pass a mappable ASN or AS_TRANS
	} else {
		out = binary.BigEndian.AppendUint32(out, asn)
	}
	return append(out, 192, 0, 2, 1)
}

// collapseMixedWidthAttrs is the attribute section an OLD speaker sends for a
// path whose second AS number does not fit two octets: AS_PATH carries AS_TRANS
// where the real AS is, and AS4_PATH carries the real one.
func collapseMixedWidthAttrs() []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS, collapseASTrans))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
		collapseASPathValue(4, collapseSourceAS, collapseRealAS))...)
	return attrs
}

// collapseAggregatorAttrs is the attribute section an OLD speaker sends for a
// path aggregated by aggAS. Its AS_PATH holds one mappable AS number, so the
// aggregator is the only thing the forward assertion has to narrow.
//
// The encoding is the one RFC 6793 Section 4.2.2 obliges the NEW speaker
// upstream to have used: a non-mappable aggAS travels as AS_TRANS in AGGREGATOR
// with the real value in AS4_AGGREGATOR, and a mappable one travels in
// AGGREGATOR alone.
func collapseAggregatorAttrs(aggAS uint32) []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	if aggAS <= math.MaxUint16 {
		return append(attrs, collapseAttr(0xC0, byte(attribute.AttrAggregator),
			collapseAggregatorValue(2, aggAS))...)
	}
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAggregator),
		collapseAggregatorValue(2, collapseASTrans))...)
	return append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Aggregator),
		collapseAggregatorValue(4, aggAS))...)
}

// collapseRecvSession builds a session whose receive encoding context reports
// asn4, which is what every consumer of a received UPDATE reads the AS number
// width from.
func collapseRecvSession(t *testing.T, settings *PeerSettings, asn4 bool) (*Session, bgpctx.ContextID) {
	t.Helper()

	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	if asn4 {
		caps = append(caps, &capability.ASN4{ASN: settings.LocalAS})
	}

	s := NewSession(settings)
	s.negotiated = capability.Negotiate(caps, caps, settings.LocalAS, settings.PeerAS)
	require.Equal(t, asn4, s.negotiated.ASN4, "the fixture must negotiate the width it claims")

	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.negotiated))
	require.NoError(t, err, "receive encoding context must register")
	s.SetRecvCtxID(ctxID)
	return s, ctxID
}

// collapseSettings names the peer the fixture UPDATE arrives from.
func collapseSettings() *PeerSettings {
	addr := netip.MustParseAddr(collapseSourceAddr)
	return &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       collapseLocalAS,
		GlobalLocalAS: collapseLocalAS,
		PeerAS:        collapseSourceAS,
		RouterID:      0x01020301,
	}
}

// collapseReceive drives one UPDATE body through processMessage and answers the
// WireUpdate the dispatch step was handed.
//
// processMessage is called directly rather than through ReadAndProcess because
// the assertion is about what leaves the ingest step, and a socket round trip
// adds a goroutine without adding evidence.
func collapseReceive(t *testing.T, s *Session, body []byte) *wireu.WireUpdate {
	t.Helper()

	var dispatched *wireu.WireUpdate
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		dispatched = wu
		return false
	}

	hdr := message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE} //nolint:gosec // fixture bodies are small
	err, _ := s.processMessage(&hdr, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, err, "a well-formed UPDATE must reach dispatch")
	require.NotNil(t, dispatched, "the UPDATE must reach the dispatch step")
	return dispatched
}

// collapseAttrValue answers one attribute value of a dispatched WireUpdate, and
// whether the attribute is present at all.
func collapseAttrValue(t *testing.T, wu *wireu.WireUpdate, code attribute.AttributeCode) ([]byte, bool) {
	t.Helper()
	attrs, err := wu.Attrs()
	require.NoError(t, err, "the dispatched attribute section must parse")
	require.NotNil(t, attrs, "the dispatched UPDATE must carry an attribute section")
	value, err := attrs.GetRaw(code)
	require.NoError(t, err, "reading attribute %s", code)
	return value, value != nil
}

// TestReceiveCollapsesAS4PathIntoASPath is the wiring test for the ingest site:
// an UPDATE received from an OLD speaker reaches the collapse, and the AS path
// every later consumer reads is four-octet truth.
//
// VALIDATES: AC-1. The dispatched AS_PATH holds the real four-octet AS numbers,
// no AS4_PATH survives, and the payload is labeled with a context reporting
// ASN4 true so nothing downstream parses it at two octets.
// PREVENTS: AS 23456 reaching the RIB, the ingress filters and every forwarded
// destination, permanently, which is what RFC 6793 Section 4.2.3 exists to stop.
func TestReceiveCollapsesAS4PathIntoASPath(t *testing.T) {
	s, recvCtxID := collapseRecvSession(t, collapseSettings(), false)
	body := makeUpdateBody(nil, collapseMixedWidthAttrs(), fwdTestNLRI)

	wu := collapseReceive(t, s, body)

	asPath, ok := collapseAttrValue(t, wu, attribute.AttrASPath)
	require.True(t, ok, "the dispatched UPDATE must still carry an AS_PATH")
	assert.Equal(t, collapseASPathValue(4, collapseSourceAS, collapseRealAS), asPath,
		"RFC 6793 Section 4.2.3: the AS path information is the received AS4_PATH with the leading part of AS_PATH prepended")

	_, hasAS4Path := collapseAttrValue(t, wu, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path,
		"RFC 6793 Section 4.1: AS4_PATH MUST NOT be carried in an UPDATE between NEW speakers, so nothing survives ingest")

	ctx := bgpctx.Registry.Get(wu.SourceCtxID())
	require.NotNil(t, ctx, "the dispatched payload must carry a registered context")
	assert.True(t, ctx.ASN4(),
		"the collapsed payload is four-octet, so its context must say so or every consumer parses it at two")
	assert.NotEqual(t, recvCtxID, wu.SourceCtxID(),
		"the session's own receive context still describes the wire, so the collapse must relabel")
}

// TestReceiveDiscardsAS4PathFromNewSpeaker is the wiring test for the Section
// 4.1 receive obligation: an AS4_PATH arriving from a NEW speaker is discarded
// rather than merged.
//
// VALIDATES: AC-3, and the receive half of RFC6793-4.1-7.
// PREVENTS: a peer lengthening or rewriting every path ze holds by sending an
// attribute the RFC forbids it to send, which today's ingest merges because the
// merge consults no width.
func TestReceiveDiscardsAS4PathFromNewSpeaker(t *testing.T) {
	s, recvCtxID := collapseRecvSession(t, collapseSettings(), true)

	asPathValue := collapseASPathValue(4, collapseSourceAS, collapseRealAS)
	aggregatorValue := collapseAggregatorValue(4, collapseSourceAS)
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath), asPathValue)...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAggregator), aggregatorValue)...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
		collapseASPathValue(4, 65200, 65201, 65202))...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Aggregator),
		collapseAggregatorValue(4, collapseRealAS))...)

	wu := collapseReceive(t, s, makeUpdateBody(nil, attrs, fwdTestNLRI))

	gotASPath, ok := collapseAttrValue(t, wu, attribute.AttrASPath)
	require.True(t, ok, "the dispatched UPDATE must still carry an AS_PATH")
	assert.Equal(t, asPathValue, gotASPath,
		"RFC 6793 Section 4.1: the attribute is DISCARDED, so nothing is merged into the AS_PATH")

	gotAggregator, ok := collapseAttrValue(t, wu, attribute.AttrAggregator)
	require.True(t, ok, "the dispatched UPDATE must still carry its AGGREGATOR")
	assert.Equal(t, aggregatorValue, gotAggregator,
		"the AS4_AGGREGATOR is discarded rather than chosen, so the received AGGREGATOR stands")

	_, hasAS4Path := collapseAttrValue(t, wu, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path, "RFC 6793 Section 4.1: a NEW speaker MUST discard a received AS4_PATH")
	_, hasAS4Agg := collapseAttrValue(t, wu, attribute.AttrAS4Aggregator)
	assert.False(t, hasAS4Agg, "RFC 6793 Section 4.1: a NEW speaker MUST discard a received AS4_AGGREGATOR")

	assert.Equal(t, recvCtxID, wu.SourceCtxID(),
		"a four-octet session needs no relabel, so the context ID is the one it received under")
}

// TestReceiveKeepsPayloadWhenNoAS4WorkIsOwed pins the fast path by identity: an
// ordinary UPDATE on a four-octet session owes no AS-path work, so the bytes
// that leave ingest are the bytes that arrived.
//
// VALIDATES: AC-9. The dispatched payload is the SAME slice and the context ID
// is the SAME value, which is what a four-octet fleet paying nothing means.
// PREVENTS: the transition machinery charging every UPDATE for a case only a
// mixed-width peering reaches. Equality would pass against a collapse that
// copies every payload; identity is the assertion that cannot.
func TestReceiveKeepsPayloadWhenNoAS4WorkIsOwed(t *testing.T) {
	s, recvCtxID := collapseRecvSession(t, collapseSettings(), true)

	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(4, collapseSourceAS, collapseRealAS))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	body := makeUpdateBody(nil, attrs, fwdTestNLRI)

	wu := collapseReceive(t, s, body)

	require.Equal(t, len(body), len(wu.Payload()), "the fast path must not resize the payload")
	assert.Same(t, &body[0], &wu.Payload()[0],
		"the fast path returns the received slice itself: a copy here is an allocation on every UPDATE")
	assert.Equal(t, recvCtxID, wu.SourceCtxID(),
		"nothing was rewritten, so nothing owes a relabel")
}

// collapseDest is one destination peer of a forward run and the AS-path family
// it must receive.
type collapseDest struct {
	name string
	addr string
	// asn2 makes this destination an OLD speaker: it negotiated no four-octet
	// AS number capability, so RFC 6793 Section 4.2.2 governs what it receives.
	asn2        bool
	rsClient    bool
	wantPath    []uint32
	wantAS4Path []uint32 // nil asserts the attribute is ABSENT
}

// collapseFrame is what one destination received: the AS numbers of its AS_PATH
// and of its AS4_PATH when it got one, and the aggregating AS of its AGGREGATOR
// and AS4_AGGREGATOR when it got those.
//
// Each aggregator carries a presence flag beside its value, because absence is
// what RFC 6793 Section 4.2.2 requires for a mappable aggregating AS, and a
// bare zero cannot be told from an AS number of 0.
type collapseFrame struct {
	asPath   []uint32
	as4Path  []uint32
	agg      uint32
	aggOK    bool
	as4Agg   uint32
	as4AggOK bool
}

// collapseForwardEnv holds the reactor one forward run drives, with the source
// session already wired into it.
type collapseForwardEnv struct {
	reactor   *Reactor
	api       *reactorAPIAdapter
	source    *Peer
	session   *Session
	updateID  uint64
	frames    map[string]collapseFrame
	framesMu  *sync.Mutex
	seen      chan struct{}
	octetsFor map[string]int
}

// collapseForwardIngest builds a reactor holding the source peer and every
// destination, drives one mixed-width UPDATE through the source session's
// processMessage, and answers the environment the forward rails run in.
//
// The UPDATE enters through the receive path rather than being placed in the
// cache directly: the whole claim of this spec is that the forward rails become
// correct because of what ingest did, so a fixture that seeded the cache would
// prove nothing about either.
func collapseForwardIngest(t *testing.T, attrs []byte, dests []collapseDest) *collapseForwardEnv {
	t.Helper()

	ctx4 := bgpctx.EncodingContextForASN4(true)
	ctxID4, err := bgpctx.Registry.Register(ctx4)
	require.NoError(t, err)
	ctx2 := bgpctx.EncodingContextForASN4(false)
	ctxID2, err := bgpctx.Registry.Register(ctx2)
	require.NoError(t, err)

	settings := collapseSettings()
	sourcePeer := NewPeer(settings)
	sourcePeer.state.Store(int32(PeerStateEstablished))
	sourcePeer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	sourcePeer.sendCtx.Store(ctx2)
	sourcePeer.sendCtxID = ctxID2
	sourcePeer.refreshForwardFacts()

	peers := map[netip.AddrPort]*Peer{settings.PeerKey(): sourcePeer}
	octetsFor := make(map[string]int, len(dests))
	for _, dest := range dests {
		destCtx, destCtxID := ctx4, ctxID4
		octets := 4
		if dest.asn2 {
			destCtx, destCtxID, octets = ctx2, ctxID2, 2
		}
		peer := collapseDestPeer(t, dest, destCtx, destCtxID)
		peers[peer.Settings().PeerKey()] = peer
		octetsFor[dest.addr] = octets
	}

	framesMu := &sync.Mutex{}
	frames := make(map[string]collapseFrame, len(dests))
	seen := make(chan struct{}, len(dests)+1)
	pool := newFwdPool(func(key fwdKey, items []fwdItem) {
		framesMu.Lock()
		for i := range items {
			addr := key.peerAddr.Addr().String()
			// A destination is reached on one of two rails: the zero-copy raw
			// split hands back rawBodies, and a re-encode for a differing
			// destination context hands back structured updates. Both are the
			// bytes that peer receives, so both are read here rather than the
			// one the fixture happens to take today.
			for _, body := range collapseDispatchedBodies(&items[i]) {
				value, ok := bodyPathAttr(t, body, byte(attribute.AttrASPath))
				if !ok {
					continue
				}
				frame := collapseFrame{asPath: collapseReadASNs(t, value, octetsFor[addr])}
				// AS4_PATH is four-octet by definition (RFC 6793 Section 3), so
				// its width never depends on what this destination negotiated.
				if as4, hasAS4 := bodyPathAttr(t, body, byte(attribute.AttrAS4Path)); hasAS4 {
					frame.as4Path = collapseReadASNs(t, as4, 4)
				}
				if agg, hasAgg := bodyPathAttr(t, body, byte(attribute.AttrAggregator)); hasAgg {
					frame.agg = collapseReadAggregator(t, agg, octetsFor[addr])
					frame.aggOK = true
				}
				if as4Agg, hasAS4Agg := bodyPathAttr(t, body, byte(attribute.AttrAS4Aggregator)); hasAS4Agg {
					// AS4_AGGREGATOR is four-octet by definition (RFC 6793 Section 3).
					frame.as4Agg = collapseReadAggregator(t, as4Agg, 4)
					frame.as4AggOK = true
				}
				frames[addr] = frame
			}
		}
		framesMu.Unlock()
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
			seen <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 16, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	cache := newRecentUpdateCache(100)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("test-plugin")

	r := &Reactor{
		clock:           clock.RealClock{},
		config:          &Config{LocalAS: collapseLocalAS},
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         pool,
		attrModHandlers: attrModHandlersWithDefaults(),
	}

	var updateID uint64
	r.setMessageReceiver(&testDeliveryReceiver{
		consumerCount: 1,
		onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			updateID = msg.MessageID
		},
	})

	session, _ := collapseRecvSession(t, settings, false)
	collapseEstablish(t, session)
	sourcePeer.mu.Lock()
	sourcePeer.session = session
	sourcePeer.mu.Unlock()
	session.onMessageReceived = r.notifyMessageReceiver

	body := makeUpdateBody(nil, attrs, fwdTestNLRI)
	hdr := message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE} //nolint:gosec // fixture bodies are small
	processErr, _ := session.processMessage(&hdr, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, processErr, "the received UPDATE must reach the forward cache")
	require.NotZero(t, updateID, "the receive path must hand the forward cache a message id")

	return &collapseForwardEnv{
		reactor:   r,
		api:       &reactorAPIAdapter{r: r},
		source:    sourcePeer,
		session:   session,
		updateID:  updateID,
		frames:    frames,
		framesMu:  framesMu,
		seen:      seen,
		octetsFor: octetsFor,
	}
}

// collapseDispatchedBodies answers the UPDATE bodies one dispatched fwdItem
// carries, whichever of the two forward rails produced it.
func collapseDispatchedBodies(item *fwdItem) [][]byte {
	bodies := make([][]byte, 0, len(item.rawBodies)+len(item.updates))
	bodies = append(bodies, item.rawBodies...)
	for _, update := range item.updates {
		bodies = append(bodies, fwdPackUpdateBody(update))
	}
	return bodies
}

// collapseDestPeer builds one established destination peer.
func collapseDestPeer(t *testing.T, dest collapseDest, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	addr := netip.MustParseAddr(dest.addr)
	peer := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       collapseLocalAS,
		GlobalLocalAS: collapseLocalAS,
		PeerAS:        65500 + uint32(addr.As4()[3]),
		RouterID:      0x01020300 | uint32(addr.As4()[3]),
		RSFastPath:    dest.rsClient,
		RSClient:      dest.rsClient,
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.refreshForwardFacts()
	return peer
}

// collapseEstablish drives one session to Established with no socket, which is
// what processMessage's FSM step needs and all it needs.
func collapseEstablish(t *testing.T, s *Session) {
	t.Helper()
	require.NoError(t, s.fsm.Event(fsm.EventManualStart))
	require.NoError(t, s.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, s.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, s.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, s.fsm.State())

	server, client := net.Pipe()
	t.Cleanup(func() {
		server.Close() //nolint:errcheck // test cleanup
		client.Close() //nolint:errcheck // test cleanup
	})
	s.mu.Lock()
	s.conn = server
	s.bufWriter = bufio.NewWriterSize(server, 16384)
	s.mu.Unlock()
	t.Cleanup(s.timers.StopAll)
}

// collapseReadASNs reads an AS_PATH or AS4_PATH value holding one AS_SEQUENCE
// and answers its AS numbers, outermost first.
//
// It refuses any other shape rather than answering short: a segment count or a
// segment type these fixtures did not produce means the rail wrote something
// this test is not reading, and a silent partial decode would pass as an
// AS_PATH assertion.
func collapseReadASNs(t *testing.T, value []byte, octets int) []uint32 {
	t.Helper()
	require.GreaterOrEqual(t, len(value), 2, "an AS path attribute must carry a segment header")
	require.Equal(t, byte(attribute.ASSequence), value[0], "the fixture path is one AS_SEQUENCE segment")
	count := int(value[1])
	require.Len(t, value, 2+count*octets, "the segment must hold exactly count AS numbers at this width")

	asns := make([]uint32, count)
	for i := range asns {
		off := 2 + i*octets
		if octets == 2 {
			asns[i] = uint32(binary.BigEndian.Uint16(value[off:]))
			continue
		}
		asns[i] = binary.BigEndian.Uint32(value[off:])
	}
	return asns
}

// collapseReadAggregator reads an AGGREGATOR or AS4_AGGREGATOR value and answers
// the aggregating AS number.
//
// It refuses any other length rather than answering short, for the reason
// collapseReadASNs gives: a value this test cannot read means the rail wrote
// something the assertion is not looking at.
func collapseReadAggregator(t *testing.T, value []byte, octets int) uint32 {
	t.Helper()
	require.Len(t, value, octets+4, "an aggregator value is one AS number and one IPv4 address")
	if octets == 2 {
		return uint32(binary.BigEndian.Uint16(value))
	}
	return binary.BigEndian.Uint32(value)
}

// await waits for every destination of a forward run to be dispatched, and
// answers what each one received.
func (e *collapseForwardEnv) await(t *testing.T, count int) map[string]collapseFrame {
	t.Helper()
	for range count {
		select {
		case <-e.seen:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for every destination to receive the forwarded UPDATE")
		}
	}
	e.framesMu.Lock()
	defer e.framesMu.Unlock()
	out := make(map[string]collapseFrame, len(e.frames))
	maps.Copy(out, e.frames)
	return out
}

// TestForwardUpdateCarriesReconstructedPathToNewSpeaker is the wiring test for
// the general forward rail: a route learned from an OLD speaker reaches a
// four-octet destination carrying the real AS numbers.
//
// VALIDATES: AC-10 for a four-octet eBGP destination, entered through
// ForwardUpdate.
// PREVENTS: ze relaying AS 23456 to a modern core, which is D-1: the widening
// rails re-encode AS_PATH without ever reading the AS4_PATH beside it.
func TestForwardUpdateCarriesReconstructedPathToNewSpeaker(t *testing.T) {
	dests := []collapseDest{{
		name: "four-octet eBGP destination", addr: "10.9.0.2",
		wantPath: []uint32{collapseLocalAS, collapseSourceAS, collapseRealAS},
	}}
	env := collapseForwardIngest(t, collapseMixedWidthAttrs(), dests)

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, env.api.ForwardUpdate(sel, env.updateID, "test-plugin", plugin.OperatorSender()))

	frames := env.await(t, len(dests))
	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, frames[dest.addr].asPath, dest.name)
		assert.Nil(t, frames[dest.addr].as4Path,
			"RFC 6793 Section 4.1: no AS4_PATH is carried between NEW speakers")
	}
}

// TestForwardUpdateNarrowsReconstructedPathToOldSpeaker is the wiring test for
// the other direction: the reconstructed path is re-narrowed for a destination
// that speaks two octets, and the real AS numbers ride the AS4_PATH ze derives.
//
// VALIDATES: AC-11, entered through ForwardUpdate.
// PREVENTS: a two-octet destination receiving AS_TRANS with no AS4_PATH to
// recover the real AS from, which is the same loss one hop further out.
func TestForwardUpdateNarrowsReconstructedPathToOldSpeaker(t *testing.T) {
	dests := []collapseDest{{
		name: "two-octet eBGP destination", addr: "10.9.0.3", asn2: true,
		wantPath:    []uint32{collapseLocalAS, collapseSourceAS, collapseASTrans},
		wantAS4Path: []uint32{collapseLocalAS, collapseSourceAS, collapseRealAS},
	}}
	env := collapseForwardIngest(t, collapseMixedWidthAttrs(), dests)

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, env.api.ForwardUpdate(sel, env.updateID, "test-plugin", plugin.OperatorSender()))

	frames := env.await(t, len(dests))
	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, frames[dest.addr].asPath, dest.name)
		assert.Equal(t, dest.wantAS4Path, frames[dest.addr].as4Path,
			"RFC 6793 Section 4.2.2: the real four-octet AS numbers ride the AS4_PATH")
	}
}

// TestForwardUpdateNarrowsAggregatorToOldSpeaker proves the AGGREGATOR half of
// the same narrowing. The collapse hands the forward path one four-octet
// aggregating AS, and the rail encodes it for the width the destination
// negotiated, without the forward rail needing an arm of its own.
//
// VALIDATES: AC-12, entered through ForwardUpdate. Both polarities run, because
// RFC 6793 Section 4.2.2 states two obligations and only the pair pins them:
// "if the aggregating Autonomous System's AS number is a non-mappable
// four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute
// and set the AS number field in the existing AGGREGATOR attribute to the
// reserved AS number, AS_TRANS. Note that if the AS number is mappable, then
// the AS4_AGGREGATOR attribute MUST NOT be sent."
// PREVENTS: a two-octet neighbor losing the aggregating AS behind AS_TRANS with
// nothing to recover it from, and a spurious AS4_AGGREGATOR beside a mappable
// one. A single-polarity test passes over the second failure.
func TestForwardUpdateNarrowsAggregatorToOldSpeaker(t *testing.T) {
	cases := []struct {
		name         string
		aggAS        uint32
		wantAgg      uint32
		wantAS4Agg   uint32
		wantAS4AggOK bool
	}{
		{
			name: "non-mappable aggregating AS", aggAS: collapseRealAS,
			wantAgg: collapseASTrans, wantAS4Agg: collapseRealAS, wantAS4AggOK: true,
		},
		{
			name: "mappable aggregating AS", aggAS: collapseSourceAS,
			wantAgg: collapseSourceAS,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dest := collapseDest{
				name: "two-octet eBGP destination", addr: "10.9.0.3", asn2: true,
				wantPath: []uint32{collapseLocalAS, collapseSourceAS},
			}
			env := collapseForwardIngest(t, collapseAggregatorAttrs(tc.aggAS), []collapseDest{dest})

			sel, err := selector.Parse("*")
			require.NoError(t, err)
			require.NoError(t, env.api.ForwardUpdate(sel, env.updateID, "test-plugin", plugin.OperatorSender()))

			frame := env.await(t, 1)[dest.addr]
			assert.Equal(t, dest.wantPath, frame.asPath, dest.name)
			assert.Nil(t, frame.as4Path,
				"RFC 6793 Section 4.2.2: a mappable-only AS path carries no AS4_PATH")

			require.True(t, frame.aggOK, "the AGGREGATOR must reach a two-octet destination")
			assert.Equal(t, tc.wantAgg, frame.agg, "the AGGREGATOR is written at two octets")
			assert.Equal(t, tc.wantAS4AggOK, frame.as4AggOK,
				"an AS4_AGGREGATOR is owed for a non-mappable aggregating AS and forbidden for a mappable one")
			if tc.wantAS4AggOK {
				assert.Equal(t, tc.wantAS4Agg, frame.as4Agg,
					"the AS4_AGGREGATOR carries the real four-octet aggregating AS")
			}
		})
	}
}

// TestForwardRSCarriesReconstructedPathToClient is the wiring test for the
// route-server rail, which never reaches ASPathEdit for a non-eBGP client and
// resolves the AS-path family through its own encoder.
//
// VALIDATES: AC-10 for a four-octet route-server client, entered through
// reactorForwardRS.
// PREVENTS: the rail split hiding the defect on one rail: a fix proven on the
// general rail alone would leave every RS client reading AS 23456.
func TestForwardRSCarriesReconstructedPathToClient(t *testing.T) {
	dests := []collapseDest{{
		name: "four-octet route-server client", addr: "10.9.0.4", rsClient: true,
		wantPath: []uint32{collapseSourceAS, collapseRealAS},
	}}
	env := collapseForwardIngest(t, collapseMixedWidthAttrs(), dests)

	update, ok := env.reactor.recentUpdates.Get(env.updateID)
	require.True(t, ok, "the received UPDATE must be in the forward cache")
	skipped, dispatched := reactorForwardRS(env.reactor, update, env.updateID,
		netip.MustParseAddr(collapseSourceAddr), env.source)
	require.Empty(t, skipped, "no destination carries an export filter")
	require.Equal(t, len(dests), dispatched, "every client must be dispatched to")

	frames := env.await(t, len(dests))
	for _, dest := range dests {
		assert.Equal(t, dest.wantPath, frames[dest.addr].asPath, dest.name)
		assert.Nil(t, frames[dest.addr].as4Path,
			"RFC 7947 Section 2.2.2 keeps the path, and RFC 6793 Section 4.1 keeps the AS4_PATH off the wire")
	}
}

// TestReceivedBytesReachTheObserversUncollapsed pins the constraint A-7 became:
// an archive records what the PEER sent, never what ze normalized.
//
// VALIDATES: AC-17. processMessage hands onMessageReceived the socket's own
// body beside the collapsed WireUpdate, and notifyObservers (reactor_notify.go)
// passes that same argument to every observer and to r.rawCapture. So the two
// assertions below are the whole of the constraint at the site the collapse
// touches: the raw argument is the received slice ITSELF, and the WireUpdate
// beside it is not.
// PREVENTS: an MRT archive or a pcap recording ze's reconstruction, which an
// operator meets as an AS_PATH no router on the wire ever sent.
func TestReceivedBytesReachTheObserversUncollapsed(t *testing.T) {
	s, _ := collapseRecvSession(t, collapseSettings(), false)
	body := makeUpdateBody(nil, collapseMixedWidthAttrs(), fwdTestNLRI)

	var observed []byte
	var dispatched *wireu.WireUpdate
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, rawBytes []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		observed = rawBytes
		dispatched = wu
		return false
	}

	hdr := message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE} //nolint:gosec // fixture bodies are small
	err, _ := s.processMessage(&hdr, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, err)
	require.NotNil(t, observed, "the observers must be handed the received bytes")
	require.NotNil(t, dispatched)

	require.NotEmpty(t, observed)
	assert.Same(t, &body[0], &observed[0],
		"the observers get the socket's own slice: a collapse in place would rewrite every archive")

	rawASPath, hasRaw := bodyPathAttr(t, observed, byte(attribute.AttrASPath))
	require.True(t, hasRaw, "the recorded bytes must still carry the AS_PATH the peer sent")
	assert.Equal(t, collapseASPathValue(2, collapseSourceAS, collapseASTrans), rawASPath,
		"the archive keeps the two-octet AS_PATH, AS_TRANS and all")
	_, hasRawAS4 := bodyPathAttr(t, observed, byte(attribute.AttrAS4Path))
	assert.True(t, hasRawAS4, "the archive keeps the AS4_PATH the peer sent beside it")

	_, hasCollapsedAS4 := collapseAttrValue(t, dispatched, attribute.AttrAS4Path)
	assert.False(t, hasCollapsedAS4,
		"the dispatched payload is the collapse, so the two views must differ or one of them is wrong")
}

// TestReceiveCollapseRunsAfterRFC7606 pins the order of the two rewrites: RFC
// 7606 judges what the PEER sent, and only then is the AS-path family
// reconciled.
//
// The fixture is both malformed and mixed-width. RFC 7607 Section 2 makes AS 0
// in an AS4_PATH malformed, and RFC 6793 Section 6 chooses attribute discard
// for it, so the attribute is gone before the reconciliation looks. Run the
// other way round, the reconciliation would merge AS 0 into the AS path
// information and RFC 7606 would then be judging ze's own bytes.
//
// VALIDATES: R-7.
// PREVENTS: a peer injecting AS 0 into every path ze stores and relays, by
// sending it in the attribute the receive path is obliged to discard.
func TestReceiveCollapseRunsAfterRFC7606(t *testing.T) {
	s, _ := collapseRecvSession(t, collapseSettings(), false)

	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS, collapseASTrans))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
		collapseASPathValue(4, collapseSourceAS, 0))...)

	wu := collapseReceive(t, s, makeUpdateBody(nil, attrs, fwdTestNLRI))

	asPath, ok := collapseAttrValue(t, wu, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, collapseASPathValue(4, collapseSourceAS, collapseASTrans), asPath,
		"RFC 7607 Section 2: the AS4_PATH was discarded before the reconciliation, so the AS_PATH stands as the AS path information")
	_, hasAS4Path := collapseAttrValue(t, wu, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path, "nothing carries an AS4_PATH past ingest, discarded or reconciled")
}

// TestReceiveCollapseLogsDiscardedMalformedAS4Path is the other half of RFC 6793
// Section 6: the attribute is discarded, the UPDATE keeps being processed, and
// the error is logged locally for analysis.
//
// VALIDATES: AC-8. One line under the session subsystem, naming the peer and
// the reason, and a dispatched UPDATE carrying the widened AS_PATH.
// PREVENTS: a silent drop. An operator whose peer sends a broken AS4_PATH sees
// a shortened AS path and nothing that says why.
func TestReceiveCollapseLogsDiscardedMalformedAS4Path(t *testing.T) {
	var sink bytes.Buffer
	restore := swapSessionLogger(func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	t.Cleanup(restore)

	s, _ := collapseRecvSession(t, collapseSettings(), false)

	// One AS_SEQUENCE claiming two AS numbers and carrying one. RFC 7607's AS 0
	// validator has nothing to say about it, so it reaches the reconciliation
	// exactly as the peer sent it.
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS, collapseASTrans))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
		[]byte{byte(attribute.ASSequence), 2, 0, 0, 0, 1})...)

	wu := collapseReceive(t, s, makeUpdateBody(nil, attrs, fwdTestNLRI))

	asPath, ok := collapseAttrValue(t, wu, attribute.AttrASPath)
	require.True(t, ok, "RFC 6793 Section 6: the UPDATE continues to be processed")
	assert.Equal(t, collapseASPathValue(4, collapseSourceAS, collapseASTrans), asPath,
		"the received AS_PATH is the AS path information when the AS4_PATH cannot be read")

	line := sink.String()
	assert.Contains(t, line, "discarded a received AS4 attribute")
	assert.Contains(t, line, collapseSourceAddr, "the line must name the peer whose attribute it was")
	assert.Contains(t, line, "AS4_PATH", "the line must name the attribute that went")

	// The control: a well-formed UPDATE says nothing at all, so the line above
	// is a signal rather than noise every mixed-width UPDATE carries.
	sink.Reset()
	collapseReceive(t, s, makeUpdateBody(nil, collapseMixedWidthAttrs(), fwdTestNLRI))
	assert.Empty(t, sink.String(), "a reconciliation that dropped nothing writes nothing")
}
