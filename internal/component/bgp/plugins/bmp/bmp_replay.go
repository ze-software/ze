// Design: docs/guide/bmp.md -- per-peer initial Adj-RIB snapshots
// Related: bmp_events.go -- serialized live events and collector priming
// RFC 7854 Sections 3.3 and 5 -- see rfc/short/rfc7854.md

package bmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The snapshot is current route state, never an UPDATE history. Both limits
// cover the whole BMP plugin. Conservative byte accounting charges shared
// attributes to each route, so retained storage cannot exceed the byte budget.
const (
	adjReplayBytesMax  = 512 << 20
	adjReplayRoutesMax = 2_000_000
)

var errAdjReplayLimit = errors.New("bmp: Adj-RIB replay storage exhausted")

type adjRouteKey struct {
	family family.Family
	pathID uint32
	prefix string // Normalized opaque NLRI identity, not a printable prefix.
	sent   bool
}

// adjRoute owns NLRI bytes and shares immutable attributes with routes learned
// in the same UPDATE. All references are owned before event delivery returns.
type adjRoute struct {
	peer    PeerHeader
	attrs   []byte
	nextHop []byte
	nlri    []byte
	mp      bool
}

func (r *adjRoute) storageBytes() int {
	return len(r.attrs) + len(r.nextHop) + len(r.nlri)
}

// cacheAdjUpdate maintains both directions even with no connected collectors.
// Caller MUST hold eventMu, which primeSender's caller also holds. A collector
// therefore sees either the state before this event followed by its live UPDATE,
// or the state after it, never an old snapshot after a newer withdrawal.
func (bp *BMPPlugin) cacheAdjUpdate(se *rpc.StructuredEvent) error {
	bp.mu.RLock()
	st := bp.peerUps[se.PeerAddress]
	bp.mu.RUnlock()
	if st == nil {
		return nil
	}
	if st.replayErr != nil {
		return st.replayErr
	}
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.Type != msgtype.TypeUPDATE {
		return nil
	}
	wu := msg.WireUpdate
	if wu == nil {
		wu = wireu.NewWireUpdate(msg.RawBytes, 0)
	}
	revision := st.routeRevision
	if err := bp.cacheAdjSections(st, se, wu); err != nil {
		st.replayErr = err
		return err
	}
	if st.routeRevision != revision {
		// A previous body can now describe a real change (A, B, A). An
		// ever-seen memo would suppress that last update and leave B installed.
		bp.mu.Lock()
		delete(bp.dedupState, se.PeerAddress)
		bp.mu.Unlock()
	}
	return nil
}

func (bp *BMPPlugin) cacheAdjSections(st *peerUpState, se *rpc.StructuredEvent, wu *wireu.WireUpdate) error {
	withdrawn, err := wu.Withdrawn()
	if err != nil {
		return err
	}
	nlri, err := wu.NLRI()
	if err != nil {
		return err
	}
	mpReach, err := wu.MPReach()
	if err != nil {
		return err
	}
	if mpReach != nil {
		// RFC 4760 Section 3 places a reserved octet after the declared
		// next hop. NLRIBytes returns nil for truncation as well as empty
		// NLRI, so reject the former before accepting an empty snapshot.
		if len(mpReach) < 5+int(mpReach[3]) {
			return errors.New("bmp: truncated replay MP_REACH next hop")
		}
	}
	mpUnreach, err := wu.MPUnreach()
	if err != nil {
		return err
	}
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())
	peer := peerHeaderFromEvent(se)
	if st.routes == nil {
		st.routes = make(map[adjRouteKey]*adjRoute)
	}
	// RFC 4271 Section 4.3: an NLRI present in both fields is processed
	// "as though the prefix were not contained in the WITHDRAWN ROUTES."
	if err := bp.cacheAdjNLRIs(st, family.IPv4Unicast, se.Direction, withdrawn, ctx, nil); err != nil {
		return err
	}
	if mpUnreach != nil {
		if err := bp.cacheAdjNLRIs(st, mpUnreach.Family(), se.Direction, mpUnreach.WithdrawnBytes(), ctx, nil); err != nil {
			return err
		}
	}
	if len(nlri) == 0 {
		if mpReach == nil || len(mpReach.NLRIBytes()) == 0 {
			return nil
		}
	}
	attrs, err := wu.Attrs()
	if err != nil {
		return err
	}
	var packed []byte
	if attrs != nil {
		packed = attrs.Packed()
	}
	// Cache only non-MP attributes. The per-route MP_REACH is rebuilt with
	// exactly that NLRI; retaining the original would resurrect withdrawn NLRIs.
	it := attribute.NewAttrIterator(packed)
	size := 0
	for it.Remaining() > 0 {
		start := it.Offset()
		code, _, _, ok := it.Next()
		if !ok {
			return errors.New("bmp: malformed replay attributes")
		}
		if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
			continue
		}
		size += it.Offset() - start
	}
	base := make([]byte, size)
	it = attribute.NewAttrIterator(packed)
	written := 0
	for it.Remaining() > 0 {
		start := it.Offset()
		code, _, _, _ := it.Next()
		if code != attribute.AttrMPReachNLRI && code != attribute.AttrMPUnreachNLRI {
			written += copy(base[written:], packed[start:it.Offset()])
		}
	}
	legacy := adjRoute{peer: peer, attrs: base}
	if err := bp.cacheAdjNLRIs(st, family.IPv4Unicast, se.Direction, nlri, ctx, &legacy); err != nil {
		return err
	}
	if mpReach != nil {
		multiprotocol := adjRoute{peer: peer, attrs: base, nextHop: slices.Clone(mpReach.NextHopBytes()), mp: true}
		return bp.cacheAdjNLRIs(st, mpReach.Family(), se.Direction, mpReach.NLRIBytes(), ctx, &multiprotocol)
	}
	return nil
}

func (bp *BMPPlugin) cacheAdjNLRIs(st *peerUpState, fam family.Family, direction rpc.MessageDirection, data []byte, ctx *bgpctx.EncodingContext, announcement *adjRoute) error {
	if len(data) == 0 {
		return nil
	}
	withdraw := announcement == nil
	walk := nlrisplit.Get(fam)
	if withdraw {
		walk = nlrisplit.GetWithdraw(fam)
	}
	if walk == nil {
		return nlrisplit.ErrUnsupported
	}
	addPath := ctx != nil && ctx.AddPath(fam)
	keyOf := nlrisplit.GetPrefixKey(fam)
	var failed error
	_, err := walk(data, addPath, func(raw []byte) {
		if failed != nil {
			return
		}
		prefix := raw
		var pathID uint32
		if addPath {
			pathID = binary.BigEndian.Uint32(raw[:4])
			prefix = raw[4:]
		}
		var scratch [nlrisplit.PrefixKeyScratchSize]byte
		identity, err := keyOf(prefix, scratch[:], withdraw)
		if err != nil {
			failed = err
			return
		}
		key := adjRouteKey{family: fam, pathID: pathID, prefix: string(identity), sent: direction == rpc.DirectionSent}
		old := st.routes[key]
		if old != nil && announcement != nil {
			if old.mp == announcement.mp && bytes.Equal(old.attrs, announcement.attrs) &&
				bytes.Equal(old.nextHop, announcement.nextHop) && bytes.Equal(old.nlri, raw) {
				return
			}
		}
		if old != nil {
			bp.adjReplayBytes -= old.storageBytes() + len(key.prefix)
			bp.adjReplayRoutes--
			delete(st.routes, key)
			st.routeRevision++
		}
		if withdraw {
			return
		}
		need := len(announcement.attrs) + len(announcement.nextHop) + len(raw) + len(key.prefix)
		if bp.adjReplayBytes+need > adjReplayBytesMax || bp.adjReplayRoutes == adjReplayRoutesMax {
			failed = errAdjReplayLimit
			return
		}
		route := *announcement
		route.nlri = slices.Clone(raw)
		st.routes[key] = &route
		bp.adjReplayBytes += need
		bp.adjReplayRoutes++
		st.routeRevision++
	})
	if err != nil {
		return err
	}
	return failed
}

func (bp *BMPPlugin) forgetAdjRoutes(st *peerUpState) {
	if st == nil {
		return
	}
	for key, route := range st.routes {
		bp.adjReplayBytes -= route.storageBytes() + len(key.prefix)
		bp.adjReplayRoutes--
	}
}

// replayPeerLocked writes the snapshot and closes every negotiated family,
// including an empty one, for each selected direction. Caller MUST hold both
// eventMu and ss.writeMu; it MUST NOT send EOR when any replay write failed.
func (bp *BMPPlugin) replayPeerLocked(ss *senderSession, st *peerUpState, policy string) error {
	if st.replayErr != nil {
		return st.replayErr
	}
	for key, route := range st.routes {
		direction := rpc.DirectionReceived
		if key.sent {
			direction = rpc.DirectionSent
		}
		if !policyStreams(policy, direction) {
			continue
		}
		if err := ss.writeAdjRouteLocked(key.family, route); err != nil {
			return err
		}
	}
	var families []family.Family
	sent := openMultiprotocolFamilies(st.sentOpen)
	for _, fam := range openMultiprotocolFamilies(st.recvOpen) {
		if slices.Contains(sent, fam) && !slices.Contains(families, fam) {
			families = append(families, fam)
		}
	}
	if len(families) == 0 {
		// Peers without a negotiated MP family use the base IPv4 UPDATE.
		families = append(families, family.IPv4Unicast)
	}
	// Include observed families too: a native legacy family need not have an
	// MP capability, and the collector must be told when its dump is complete.
	for key := range st.routes {
		if !slices.Contains(families, key.family) {
			families = append(families, key.family)
		}
	}
	for _, direction := range []rpc.MessageDirection{rpc.DirectionReceived, rpc.DirectionSent} {
		if !policyStreams(policy, direction) {
			continue
		}
		peer := st.peer
		peer.Flags &^= PeerFlagL | PeerFlagO
		if direction == rpc.DirectionSent {
			peer.Flags |= PeerFlagL | PeerFlagO
		}
		peer.TimestampSec, peer.TimestampUsec = 0, 0
		for _, fam := range families {
			// RFC 7854 Section 3.3: "Once it has sent all the routes for a
			// given peer, it MUST send an End-of-RIB message for that peer".
			if err := ss.writeRouteMonitoringLocked(peer, msgtype.TypeUPDATE, buildEndOfRIBBody(fam)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeAdjRouteLocked builds one UPDATE directly in the session's bounded
// scratch. The layout is withdrawn-length(2), attr-length(2), attributes, NLRI.
// MP_REACH value: AFI(2), SAFI(1), next-hop-length(1), next-hop, reserved(1), NLRI.
// Caller MUST hold ss.writeMu.
func (ss *senderSession) writeAdjRouteLocked(fam family.Family, route *adjRoute) error {
	attrsLen := len(route.attrs)
	mpLen := 5 + len(route.nextHop) + len(route.nlri)
	mpHeaderLen := 3
	if mpLen > 255 {
		mpHeaderLen = 4
	}
	if route.mp {
		attrsLen += mpHeaderLen + mpLen
	}
	nlriLen := len(route.nlri)
	if route.mp {
		nlriLen = 0
	}
	bodyLen := 4 + attrsLen + nlriLen
	const bodyStart = CommonHeaderSize + PeerHeaderSize + bgpHeaderSize
	buf, err := ss.scratchFor(bodyStart + bodyLen)
	if err != nil {
		return err
	}
	body := buf[bodyStart:]
	clear(body[:4])
	binary.BigEndian.PutUint16(body[2:4], uint16(attrsLen)) //nolint:gosec // scratchFor bounds attrsLen below 65535 after BMP and BGP framing.
	off := 4
	if route.mp {
		body[off], body[off+1] = 0x80, byte(attribute.AttrMPReachNLRI)
		if mpHeaderLen == 4 {
			body[off] |= 0x10
			binary.BigEndian.PutUint16(body[off+2:off+4], uint16(mpLen)) //nolint:gosec // bounded by scratchFor.
		} else {
			body[off+2] = byte(mpLen) //nolint:gosec // at most 255 in this branch.
		}
		off += mpHeaderLen
		binary.BigEndian.PutUint16(body[off:off+2], uint16(fam.AFI))
		body[off+2], body[off+3] = byte(fam.SAFI), byte(len(route.nextHop)) //nolint:gosec // next-hop length is a wire octet.
		off += 4
		off += copy(body[off:], route.nextHop)
		body[off] = 0
		off++
		off += copy(body[off:], route.nlri)
	}
	off += copy(body[off:], route.attrs)
	if !route.mp {
		copy(body[off:], route.nlri)
	}
	return ss.writeRouteMonitoringLocked(route.peer, msgtype.TypeUPDATE, body)
}

// abortAdjReplay ends an incomplete session instead of advertising a false
// completion marker. A storage failure remains visible until the affected BGP
// peer restarts; a TCP reconnect alone cannot recover routes not retained.
func (ss *senderSession) abortAdjReplay(err error) {
	ss.connMu.Lock()
	conn := ss.conn
	ss.conn = nil
	ss.connMu.Unlock()
	if conn != nil {
		logger().Error("bmp: initial Adj-RIB snapshot unavailable", "collector", ss.name, "error", err)
		closeLog(conn, "incomplete-adj-replay")
	}
	ss.resetQueue()
}

func (bp *BMPPlugin) replayFailure(se *rpc.StructuredEvent, err error, senders []*senderSession) {
	for _, ss := range senders {
		ss.writeMu.Lock()
		ss.abortAdjReplay(fmt.Errorf("peer %s: %w", se.PeerAddress, err))
		ss.writeMu.Unlock()
	}
}
