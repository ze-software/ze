// Design: docs/architecture/api/process-protocol.md -- stored-route relay (egress-rail)
// RFC: rfc/short/rfc7911.md -- Section 3, the Path Identifier prepended to an ADD-PATH NLRI
// Related: reactor_api_forward.go -- forwardUpdateCore, the single egress transform this reuses
// Related: reactor_api_forward_batch.go -- ForwardUpdatesDirect, the batch shape this mirrors
//
// RelayStoredRoute lets a plugin that holds routes as raw wire bytes (adj-rib-in)
// replay them to a newly-established peer through the SAME rail a live forward
// uses, instead of re-emitting them as "update hex ... add" announce commands.
//
// Why this exists: the announce rail prepends the local AS and THEN runs only the
// session's export filters, while the forward rail runs the full ordered egress
// steps (export policy plus the in-process role/OTC/community filters) and THEN
// prepends. One route, two transforms. A peer establishing while an UPDATE was in
// flight could therefore see a rewritten AS_PATH, a duplicate announce, or an OTC
// route that should have been suppressed. That was
// spec-fixit-bgp-egress-rail-divergence, closed 2026-08-14.

package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Relay reconstruction failures. Each names the specific defect so a malformed
// stored route is greppable rather than an anonymous drop (ai/rules/cli.md).
//
// Reconstruction is not a detail that can be guessed. A stored AttrHex holds the
// WHOLE path-attribute section as received, MP_REACH/MP_UNREACH included, carrying
// every NLRI of the originating UPDATE rather than just this route's (assumption
// A-1 in the spec, verified against wireu.WireUpdate.Attrs). So the build strips
// attribute types 14/15, re-synthesizes a single-NLRI MP_REACH, and preserves the
// order of the surviving attributes -- see relay_payload.go.
//
// Every one of these fails the route CLOSED. A silent success that relayed
// nothing would look exactly like a working replay (ai/rules/evidence.md).
var (
	errRelayFamily     = errors.New("relay-stored-route: unknown address family")
	errRelayHex        = errors.New("relay-stored-route: stored route hex does not decode")
	errRelayTooLarge   = errors.New("relay-stored-route: stored route exceeds the maximum UPDATE size")
	errRelayAttrs      = errors.New("relay-stored-route: stored attribute block is malformed")
	errRelayNoSource   = errors.New("relay-stored-route: source peer is not established")
	errRelayBufferPool = errors.New("relay-stored-route: read buffer pool exhausted")
	errRelayNextHopLen = errors.New("relay-stored-route: ipv4 unicast next-hop is not 4 bytes")
	errRelayIncomplete = errors.New("relay-stored-route: replay incomplete, some routes were not relayed")

	// errRelayNLRIFraming refuses a route whose producer did not record how its
	// stored NLRI bytes are framed, when the source session negotiated ADD-PATH
	// for the family (RFC 7911).
	//
	// The reconstruction is tagged with the source peer's receive context so the
	// forward rail decodes attributes at the right ASN width, and that context
	// also declares the NLRI framing. RFC 7911 Section 3: "the NLRI encoding MUST
	// be extended by prepending the Path Identifier field, which is of four
	// octets". A stored prefix and a stored identifier-plus-prefix are both
	// well-formed byte strings, so the bytes cannot answer which one they are.
	// rpc.StoredRoute.NLRIFraming answers it, and its zero value says the producer
	// did not.
	//
	// Guessing is not available in either direction. Emitting a bare prefix under
	// an add-path context makes a destination that shares the context parse the
	// first four NLRI bytes as an identifier, which is a malformed UPDATE and a
	// session reset -- a peer-up replay would tear down the peer it is replaying
	// to. Inventing an identifier of 0 silently merges every path of the prefix
	// into one at the destination.
	//
	// After 2026-08-19 this reaches only a FORKED bgp-adj-rib-in built against the
	// rpc.StoredRoute that had no framing field: every in-tree producer records
	// it. A logged refusal loses one route; a corrupt frame loses the session.
	errRelayNLRIFraming = errors.New("relay-stored-route: stored NLRI framing is unrecorded under an add-path source")
)

// relaySource holds everything the relay needs about the peer a stored route was
// learned from. Resolved once per distinct source address per call.
//
// ok=false means the source peer is gone or not established. That fails the route
// CLOSED: without the source we cannot reproduce the egress transform a live
// forward would have applied (its AS_PATH prepend decision, RFC 4456 reflection
// rules and RFC 9234 role step all key off the source), and relaying under a
// zero-valued source would send the route with the WRONG transform rather than
// none at all.
type relaySource struct {
	addr   netip.Addr
	info   forwardSourceInfo
	ctxID  bgpctx.ContextID
	srcID  source.SourceID
	strAdr string
	ok     bool
}

// resolveRelaySource resolves the source peer's forwarding facts and receive
// encoding context under a single r.mu read.
//
// The context is load-bearing, and it is the source peer's receive context with
// ASN4 forced true. The stored attribute bytes went through the ingest
// reconciliation (collapseASPathFamily, session_read.go), so their AS_PATH is
// four-octet whatever the source session negotiated, while recvContextID still
// describes the wire that session reads. Labeling the reconstructed wire with
// the unmodified receive context would make every attribute-matching egress
// filter decode a four-octet AS_PATH at two octets. Everything else that
// context declares, the ADD-PATH map included, is preserved by the relabel.
func (a *reactorAPIAdapter) resolveRelaySource(srcAddr netip.Addr) relaySource {
	out := relaySource{addr: srcAddr}
	a.r.mu.RLock()
	srcPeer, found := a.r.findPeerByAddr(srcAddr)
	if found && srcPeer.State() == PeerStateEstablished {
		s := srcPeer.Settings()
		out.info = forwardSourceInfo{
			// Guarded: source may be a dynamic peer still resolving its ASN.
			isIBGP:         srcPeer.IsIBGP(),
			isRRClient:     s.RouteReflectorClient,
			remoteRouterID: srcPeer.RemoteRouterID(),
			globalLocalAS:  s.GlobalLocalAS,
			// Set on the same condition as relaySource.ok, so the facts stay
			// self-describing once they leave this struct for forwardUpdateCore.
			resolved: true,
			peer:     srcPeer,
		}
		if len(a.r.egressFilters) > 0 {
			out.info.filterInfo = filterapi.PeerFilterInfo{
				Address: s.Address,
				PeerAS:  srcPeer.PeerAS(),
				// Effective per-peer local AS, matching the forward path's src
				// filterInfo so no egress filter reads a silent zero.
				LocalAS:   s.LocalAS,
				Name:      s.Name,
				GroupName: s.GroupName,
			}
		}
		out.ctxID = fwdContextIDWithASN4(srcPeer.recvContextID(), true)
		out.srcID = srcPeer.SourceID()
		out.strAdr = srcPeer.addrString
		out.ok = true
	}
	a.r.mu.RUnlock()
	return out
}

// RelayStoredRoute relays stored routes to one destination peer through the
// forward rail. Implements plugin.ReactorRelayCoordinator.
//
// Each route names the peer it was learned from, so the egress transform applied
// is the one that source implies: the same AS_PATH prepend, role/OTC step and
// export policy a live forward from that source would have run.
//
// An empty routes slice is a success no-op. A destination matching no
// established peer returns errNoPeersMatch without dispatching.
//
// sender is the AUTHORITY, gated on `send [ update ]`: every relayed route
// leaves as an UPDATE on the destination's wire, and both the destination and
// the route bytes come from the CALLER, so this rail could put a plugin's stored
// routes into a peer that never attached it.
func (a *reactorAPIAdapter) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender) error {
	if len(routes) == 0 {
		return nil
	}
	origin := announceOrigin(sender)
	if !origin.sender.IsSet() {
		sendNoSenderDenied(peerTarget(destination), origin)
		return fmt.Errorf("%w: destination %s, send type %s", errSendNoSender, destination, origin.sendType)
	}

	// Resolve the destination once, reusing the batch resolver so this call
	// matches ForwardUpdatesDirect's semantics (Addr-based match, zone stripped,
	// established peers only).
	matchingPeers, err := a.resolveDestinationPeers([]netip.AddrPort{netip.AddrPortFrom(destination, 0)})
	if err != nil {
		fwdLogger().Debug("relay-stored-route: destination did not resolve",
			"destination", destination, "routes", len(routes), "err", err)
		return err
	}

	// One destination, so the permission is all or nothing here.
	a.r.mu.RLock()
	permitted, refused := filterPermittedPeers(matchingPeers, origin)
	a.r.mu.RUnlock()
	if refused > 0 {
		return fmt.Errorf("%w: destination %s, process %s, send type %s",
			errSendNotPermitted, destination, origin.sender, origin.sendType)
	}
	matchingPeers = permitted

	// Cache source facts per distinct source address: a peer-up replay is
	// dominated by a handful of sources, so this avoids re-resolving per route.
	var srcCacheBuf [4]relaySource
	srcCache := srcCacheBuf[:0]

	lookupSource := func(srcAddr netip.Addr) relaySource {
		for i := range srcCache {
			if srcCache[i].addr == srcAddr {
				return srcCache[i]
			}
		}
		src := a.resolveRelaySource(srcAddr)
		srcCache = append(srcCache, src)
		return src
	}

	// Reused across routes: scanAttrBlock replaces the contents each call, so one
	// slice serves the whole replay instead of allocating per route.
	var spansBuf [16]relayAttrSpan
	spans := spansBuf[:0]

	var lastErr error
	relayed := 0
	eligible := 0

	for i := 0; i < len(routes); {
		route := &routes[i]

		// The routes this one reconstruction MAY coalesce: every leading route
		// that shares the source, family, attribute block, next hop and NLRI
		// framing, which is exactly the set the source could have sent in one
		// UPDATE. buildRelayUpdate consumes as many of them as the message
		// ceiling allows and reports how many, so a run longer than one message
		// simply becomes the next iteration's run.
		run := relayRunLen(routes[i:])

		// Counted BEFORE the parse guard: a route dropped for an unparseable
		// source is still a route the caller asked us to relay, and leaving it
		// out of `eligible` made the completeness check below fail OPEN. The
		// whole run shares the source, so a guard below rejects all of it.
		eligible += run

		srcAddr, parseErr := netip.ParseAddr(route.SourcePeer)
		if parseErr != nil {
			var tb textbuf.Buffer
			fwdLogger().Error("relay-stored-route: unparseable source peer",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", parseErr)
			lastErr = errors.New(tb.Str("relay-stored-route: invalid source peer ").Quoted(route.SourcePeer).String())
			i += run
			continue
		}

		// A route must never be relayed back to the peer that sent it.
		// buildReplayRoutes filters this on the plugin side, but the engine
		// cannot trust a caller-supplied list to have done so.
		if srcAddr == destination {
			// Never eligible: a route is not relayed back to its own source.
			eligible -= run
			i += run
			continue
		}

		src := lookupSource(srcAddr)
		if !src.ok {
			// Fail CLOSED. adj-rib-in drops a peer's routes when it goes down, so
			// this is the narrow race where the source left between the replay
			// snapshot and the relay. Sending under a zero-valued source would
			// apply the WRONG egress transform, which is worse than not sending:
			// the route is about to be withdrawn anyway.
			fwdLogger().Warn("relay-stored-route: source peer not established, route not relayed",
				"source", route.SourcePeer, "destination", destination, "family", route.Family)
			lastErr = errRelayNoSource
			i += run
			continue
		}

		update, updateID, consumed, buildErr := a.buildRelayUpdate(routes[i:i+run], src, &spans)
		if buildErr != nil {
			// The whole run is dropped, not just its first route. Every reason
			// buildRelayUpdate refuses is read off the fields the run shares --
			// the family, the attribute block, the next hop, the framing -- so
			// the routes behind this one would each fail the same way. The one
			// size-derived refusal cannot fire for a run, because the fill loop
			// stops at the ceiling rather than overrunning it, and reports too
			// large only when a SINGLE route cannot be encoded.
			fwdLogger().Error("relay-stored-route: reconstruction failed",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "routes", run, "err", buildErr)
			lastErr = buildErr
			i += run
			continue
		}

		// Drop the build-time retain LAST, and unconditionally. forwardUpdateCore
		// took one retain per dispatched peer and releases each from item.done
		// once the write completes, so this release is what evicts the entry --
		// returning the reconstruction buffer and every per-destination wire
		// variant adopted onto it -- exactly once. Deferred into a closure rather
		// than called inline so a panic inside forwardUpdateCore cannot strand the
		// entry (and its pooled buffer) until the 5-minute safety valve.
		fwdErr := func() error {
			defer a.r.recentUpdates.Release(updateID)
			src.info.sender = sender
			src.info.initialUpdate = routes[i].InitialUpdate
			return a.forwardUpdateCore(update, updateID, matchingPeers, src.info)
		}()

		// A route the destination's egress policy suppressed is NOT a failure:
		// RFC 7947 community policy, RFC 4456 reflection rules and the RFC 9234
		// role step all skip the peer, and counting that as a drop made ONE
		// correctly-suppressed route fail the whole replay -- the common case on a
		// route server, leaving bgp-rs to skip its delta convergence loop.
		//
		// It is matched by its OWN sentinel, not by "nothing was dispatched".
		// forwardUpdateCore reaches zero dispatches for failures too (EBGP wire
		// build, read-buffer exhaustion, body build, a stopped pool), and treating
		// those as handled would hide exactly the load-dependent drops this spec
		// exists to surface.
		switch {
		case fwdErr == nil:
			relayed += consumed
		case errors.Is(fwdErr, errAllDestinationsSuppressed):
			relayed += consumed // handled: egress policy decided this peer gets nothing
			fwdLogger().Debug("relay-stored-route: suppressed for destination",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "routes", consumed)
		default:
			fwdLogger().Error("relay-stored-route: forward failed",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "routes", consumed, "err", fwdErr)
			lastErr = fwdErr
		}
		i += consumed
	}

	// A partial relay is NOT a success.
	//
	// Note what this does and does not buy: bgp-rs sends End-of-RIB on BOTH its
	// success and failure paths (rs/server_handlers.go, "Always send EOR when
	// replay terminates"), so an error here does NOT stop the peer being told its
	// table is complete. What it does do is surface the drop in the logs at ERROR
	// and skip bgp-rs's delta-convergence loop. Reporting it remains right --
	// silence would leave a partial replay indistinguishable from a whole one --
	// but the EOR-suppression that would make it fully fail-closed lives in
	// bgp-rs and is not this function's to give.
	if relayed < eligible {
		if lastErr == nil {
			lastErr = errRelayIncomplete
		}
		fwdLogger().Error("relay-stored-route: incomplete replay",
			"destination", destination, "relayed", relayed, "eligible", eligible, "err", lastErr)
		return lastErr
	}
	return nil
}

// buildRelayUpdate reconstructs the received-shape update that forwardUpdateCore
// consumes from a route stored as raw wire bytes, and registers it in the recent
// UPDATE cache so its backing buffers have the SAME single owner a genuinely
// received UPDATE has.
//
// Returns the update and the cache id holding one build-time retain. The caller
// MUST Release that id after forwardUpdateCore returns, whatever the outcome.
//
// Buffer ownership is the reason this goes through the cache rather than owning a
// buffer directly: forwardUpdateCore hands the reconstructed wire to per-peer
// worker goroutines that write asynchronously, and may adopt further pool handles
// onto the entry for per-destination variants (adoptFwdHandle). recent_cache's
// evictLocked is the one place that returns all of them, exactly once. Inventing
// a second refcount here would duplicate that contract and risk returning a
// buffer still aliased by an in-flight write.
// relayRunLen reports how many leading routes share everything a single UPDATE
// must state once: the source, the family, the attribute block, the next hop and
// the NLRI framing. Those are precisely the fields writeRelayPayload emits
// outside the NLRI section, so a run is a set of prefixes one message can carry.
//
// PathID is deliberately NOT part of the key. RFC 7911 Section 3 puts the
// identifier in front of each NLRI, inside the section, so paths with different
// identifiers still share one message.
func relayRunLen(routes []rpc.StoredRoute) int {
	first := &routes[0]
	n := 1
	for n < len(routes) {
		next := &routes[n]
		if next.SourcePeer != first.SourcePeer ||
			next.Family != first.Family ||
			next.NLRIFraming != first.NLRIFraming ||
			next.NextHopHex != first.NextHopHex ||
			next.MsgID != first.MsgID ||
			next.Withdraw != first.Withdraw ||
			next.InitialUpdate != first.InitialUpdate ||
			next.AttrHex != first.AttrHex {
			break
		}
		n++
	}
	return n
}

func (a *reactorAPIAdapter) buildRelayUpdate(routes []rpc.StoredRoute, src relaySource, spans *[]relayAttrSpan) (*ReceivedUpdate, uint64, int, error) {
	// Every field read here outside the NLRI section is shared by the whole run,
	// which is what relayRunLen guarantees, so the first route states them all.
	route := &routes[0]
	fam, known := family.LookupFamily(route.Family)
	if !known {
		return nil, 0, 0, errRelayFamily
	}

	// The reconstruction is emitted under the source's relabeled receive
	// context, whose ADD-PATH map is the source peer's own, so it must carry the
	// framing that context declares. fwdReencodeNLRIs converts
	// it per destination afterwards, in both directions, exactly as it does for a
	// live forward -- so there is one framing decision here and none below.
	srcAddPath := false
	if srcCtx := bgpctx.Registry.Get(src.ctxID); srcCtx != nil {
		srcAddPath = srcCtx.AddPath(fam)
	}
	// RFC 7911 Section 3: an ADD-PATH session prepends a four-octet Path
	// Identifier to every NLRI. Written unconditionally when the context declares
	// ADD-PATH, identifier 0 included -- the RFC reserves no value, and a
	// non-zero-only rule would drop a legal path.
	//
	// A context that declares none needs no decision: the stored bytes carry no
	// identifier under either framing.
	pathIDLen := 0
	if srcAddPath {
		switch route.NLRIFraming {
		case rpc.NLRIFramingPrefixOnly:
			pathIDLen = relayPathIDLen
		case rpc.NLRIFramingSourceWire:
			// The bytes already carry the source's framing, identifiers included.
		default:
			// Refuse before touching a buffer. See errRelayNLRIFraming.
			return nil, 0, 0, errRelayNLRIFraming
		}
	}
	if route.Withdraw {
		return a.buildRelayWithdrawal(route, src, fam, pathIDLen)
	}

	attrLen := hex.DecodedLen(len(route.AttrHex))
	nhLen := hex.DecodedLen(len(route.NextHopHex))
	firstNLRILen := hex.DecodedLen(len(route.NLRIHex))
	if firstNLRILen == 0 || (nhLen == 0 && fam.NeedsNextHop()) {
		return nil, 0, 0, errRelayHex
	}

	// Decode the stored hex fields into ONE pooled scratch buffer rather than
	// separate heap slices. The attribute block and the next hop are stated once
	// for the whole run; the NLRI region that follows them takes one element per
	// route, so the NLRI section stays one contiguous span whatever the run
	// length. The path identifier is written into the four octets reserved ahead
	// of each element.
	runNLRILen := 0
	for i := range routes {
		runNLRILen += pathIDLen + hex.DecodedLen(len(routes[i].NLRIHex))
	}
	scratchLen := attrLen + nhLen + runNLRILen
	scratch := getReadBuf(scratchLen > message.MaxMsgLen-message.HeaderLen)
	if scratch.Buf == nil {
		return nil, 0, 0, errRelayBufferPool
	}
	defer returnReadBuffer(scratch)
	// Only the FIRST element has to fit. A run longer than one message is not an
	// error: the fill loop below stops where the message stops, and the caller
	// starts the next reconstruction at the route that did not fit.
	if attrLen+nhLen+pathIDLen+firstNLRILen > len(scratch.Buf) {
		return nil, 0, 0, errRelayTooLarge
	}
	attrs := scratch.Buf[:attrLen]
	nextHop := scratch.Buf[attrLen : attrLen+nhLen]
	nlriRegion := scratch.Buf[attrLen+nhLen:]
	// hex.Decode does not leak src, so the []byte conversions are elided by the
	// compiler ("zero-copy string->[]byte conversion") and cost nothing per
	// route. A hand-rolled decoder was tried here and reverted: it was premised
	// on an allocation that measurement showed does not happen.
	if _, err := hex.Decode(attrs, []byte(route.AttrHex)); err != nil {
		return nil, 0, 0, errRelayHex
	}
	if _, err := hex.Decode(nextHop, []byte(route.NextHopHex)); err != nil {
		return nil, 0, 0, errRelayHex
	}

	scanned, ok := scanAttrBlock(*spans, attrs)
	*spans = scanned
	if !ok {
		return nil, 0, 0, errRelayAttrs
	}

	needNextHop := relayNeedsNextHopAttr(scanned, fam)
	// The stored next hop belongs to this route. A mixed received UPDATE can
	// also carry a legacy NEXT_HOP belonging to a different IPv4 announcement.
	// Reconstruction emits IPv4 unicast in the legacy field, so replace that
	// attribute in our private decoded scratch before the egress rail reads it.
	if fam == family.IPv4Unicast {
		if len(nextHop) != 4 {
			return nil, 0, 0, errRelayNextHopLen
		}
		if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrNextHop); found {
			if len(value) != 4 {
				return nil, 0, 0, errRelayNextHopLen
			}
			copy(value, nextHop)
		}
	}

	// Fill the NLRI section with as much of the run as this UPDATE can carry.
	// relayPayloadLen is asked before each element is accepted, so the message
	// ceiling is a STOP rather than a refusal: the run resumes in the next
	// reconstruction. That is what puts the replayed stream back into the
	// message shape the source sent, and it is the whole point of coalescing --
	// an egress policy that overflows the rebuilt message must overflow it on
	// this rail exactly as it does on the live forward.
	nlriLen := 0
	size := 0
	consumed := 0
	for i := range routes {
		elemLen := pathIDLen + hex.DecodedLen(len(routes[i].NLRIHex))
		if elemLen == pathIDLen {
			// An element with no NLRI bytes. Refused outright as the first
			// route, and it stops the run otherwise.
			break
		}
		if nlriLen+elemLen > len(nlriRegion) {
			break
		}
		elem := nlriRegion[nlriLen : nlriLen+elemLen]
		if _, err := hex.Decode(elem[pathIDLen:], []byte(routes[i].NLRIHex)); err != nil {
			if consumed == 0 {
				return nil, 0, 0, errRelayHex
			}
			break
		}
		if pathIDLen != 0 {
			binary.BigEndian.PutUint32(elem, routes[i].PathID)
		}
		trialSize, fits := relayPayloadLen(scanned, nextHop, nlriRegion[:nlriLen+elemLen], fam, needNextHop)
		if !fits {
			if consumed == 0 {
				return nil, 0, 0, errRelayTooLarge
			}
			break
		}
		nlriLen += elemLen
		size = trialSize
		consumed++
	}
	if consumed == 0 {
		return nil, 0, 0, errRelayHex
	}
	nlri := nlriRegion[:nlriLen]

	// The reconstruction buffer is NOT returned here: it backs the WireUpdate for
	// the whole forward, including asynchronous per-peer writes, and is returned
	// when the cache evicts the entry.
	out := getReadBuf(size > message.MaxMsgLen-message.HeaderLen)
	if out.Buf == nil {
		return nil, 0, 0, errRelayBufferPool
	}
	if size > len(out.Buf) {
		returnReadBuffer(out)
		return nil, 0, 0, errRelayTooLarge
	}
	n := writeRelayPayload(out.Buf, 0, scanned, attrs, nextHop, nlri, fam, needNextHop)

	ru := &ReceivedUpdate{
		poolBuf:      out,
		SourcePeerIP: src.addr,
		// The forward path threads this into fwdItem.sourcePeerStr for the sent
		// event callback; the peer's cached string avoids a per-route allocation.
		SourcePeerStr:    src.strAdr,
		validationReplay: true,
		validationMsgID:  route.MsgID,
		ReceivedAt:       a.r.clock.Now(),
		// Meta is deliberately nil: the ingress annotations a live UPDATE carries
		// (meta["src-role"], meta["stale"]) are not stored alongside the route.
		//
		// RFC 9494 staleness is genuinely false for a replay, so its absence is
		// correct. RFC 9234 role handling still suppresses correctly here because
		// the ingress filter stamps OTC into the WIRE bytes before the route is
		// stored, so the wire-bytes egress rule (role/otc.go checkOTCEgress) sees
		// it on the reconstruction without needing meta.
		//
		// The meta-based Gao-Rexford safety net in OTCEgressFilter used to go
		// unevaluated for a replayed route, because it read meta["src-role"] and
		// treated the missing key as "no restriction". That is CLOSED: the filter
		// now recovers our role for the source peer from config when meta lacks
		// it (resolveSrcRole, role/otc.go), which is an exact recovery rather than
		// a guess because the ingress filter only ever copied it out of the same
		// config field. So a nil Meta here no longer weakens the leak guard.
		// Proven by TestOTCEgressSuppressProviderLearnedWithoutMeta.
		Meta: nil,
	}
	// The stored bytes carry the four-octet AS path the ingest reconciliation
	// produced, so the wire must carry the relabeled context resolveRelaySource
	// built or every attribute-matching egress filter decodes AS_PATH at the
	// wrong ASN width.
	wireu.InitWireUpdate(&ru.wireUpdateInline, out.Buf[:n], src.ctxID)
	updateID := nextMsgID()
	ru.wireUpdateInline.SetMessageID(updateID)
	ru.wireUpdateInline.SetSourceID(src.srcID)
	ru.WireUpdate = &ru.wireUpdateInline

	// Add -> retainN -> Activate mirrors the received-UPDATE lifecycle with no
	// plugin consumers: Activate(id, 0) clears the pending flag while the
	// build-time retain keeps the entry alive, so a cache-consumer's cumulative
	// ack can pass over this entry without evicting a buffer still in flight
	// (ackEntryLocked only evicts at zero TOTAL consumers).
	a.r.recentUpdates.Add(ru)
	a.r.recentUpdates.retainN(updateID, 1)
	a.r.recentUpdates.Activate(updateID, 0)

	return ru, updateID, consumed, nil
}

// buildRelayWithdrawal carries a removed validation path through the same
// source identity and ADD-PATH regeneration as its former advertisement.
func (a *reactorAPIAdapter) buildRelayWithdrawal(route *rpc.StoredRoute, src relaySource, fam family.Family, pathIDLen int) (*ReceivedUpdate, uint64, int, error) {
	nlriLen := pathIDLen + hex.DecodedLen(len(route.NLRIHex))
	if nlriLen == pathIDLen {
		return nil, 0, 0, errRelayHex
	}
	size := 4 + nlriLen
	if fam != family.IPv4Unicast {
		size += attrHeaderLen(3+nlriLen) + 3
	}
	if size > maxUpdateBody {
		return nil, 0, 0, errRelayTooLarge
	}
	out := getReadBuf(size > message.MaxMsgLen-message.HeaderLen)
	if out.Buf == nil {
		return nil, 0, 0, errRelayBufferPool
	}
	buf := out.Buf[:size]
	clear(buf[:4])
	off := 2
	if fam == family.IPv4Unicast {
		binary.BigEndian.PutUint16(buf[:2], uint16(nlriLen))
		binary.BigEndian.PutUint16(buf[2+nlriLen:4+nlriLen], 0)
	} else {
		binary.BigEndian.PutUint16(buf[2:4], uint16(size-4))
		off = 4
		off += writeAttrHeader(buf, off, byte(attribute.FlagOptional), attribute.AttrMPUnreachNLRI, 3+nlriLen)
		binary.BigEndian.PutUint16(buf[off:off+2], uint16(fam.AFI))
		buf[off+2] = byte(fam.SAFI)
		off += 3
	}
	if pathIDLen != 0 {
		binary.BigEndian.PutUint32(buf[off:off+4], route.PathID)
		off += 4
	}
	if _, err := hex.Decode(buf[off:off+nlriLen-pathIDLen], []byte(route.NLRIHex)); err != nil {
		returnReadBuffer(out)
		return nil, 0, 0, errRelayHex
	}
	update := &ReceivedUpdate{
		poolBuf:          out,
		SourcePeerIP:     src.addr,
		SourcePeerStr:    src.strAdr,
		ReceivedAt:       a.r.clock.Now(),
		validationReplay: true,
		validationMsgID:  route.MsgID,
	}
	wireu.InitWireUpdate(&update.wireUpdateInline, buf, src.ctxID)
	updateID := nextMsgID()
	update.wireUpdateInline.SetMessageID(updateID)
	update.wireUpdateInline.SetSourceID(src.srcID)
	update.WireUpdate = &update.wireUpdateInline
	a.r.recentUpdates.Add(update)
	a.r.recentUpdates.retainN(updateID, 1)
	a.r.recentUpdates.Activate(updateID, 0)
	return update, updateID, 1, nil
}
