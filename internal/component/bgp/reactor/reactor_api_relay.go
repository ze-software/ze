// Design: docs/architecture/api/process-protocol.md -- stored-route relay (egress-rail)
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
// route that should have been suppressed -- see
// plan/spec-fixit-bgp-egress-rail-divergence.md.

package reactor

import (
	"encoding/hex"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Relay reconstruction failures. Each names the specific defect so a malformed
// stored route is greppable rather than an anonymous drop (ai/rules/error-messages.md).
//
// Reconstruction is not a detail that can be guessed. A stored AttrHex holds the
// WHOLE path-attribute section as received, MP_REACH/MP_UNREACH included, carrying
// every NLRI of the originating UPDATE rather than just this route's (assumption
// A-1 in the spec, verified against wireu.WireUpdate.Attrs). So the build strips
// attribute types 14/15, re-synthesizes a single-NLRI MP_REACH, and preserves the
// order of the surviving attributes -- see relay_payload.go.
//
// Every one of these fails the route CLOSED. A silent success that relayed
// nothing would look exactly like a working replay (ai/rules/fail-closed-guards.md).
var (
	errRelayFamily     = errors.New("relay-stored-route: unknown address family")
	errRelayHex        = errors.New("relay-stored-route: stored route hex does not decode")
	errRelayTooLarge   = errors.New("relay-stored-route: stored route exceeds the maximum UPDATE size")
	errRelayAttrs      = errors.New("relay-stored-route: stored attribute block is malformed")
	errRelayNoSource   = errors.New("relay-stored-route: source peer is not established")
	errRelayBufferPool = errors.New("relay-stored-route: read buffer pool exhausted")
	errRelayNextHopLen = errors.New("relay-stored-route: ipv4 unicast next-hop is not 4 bytes")
	errRelayIncomplete = errors.New("relay-stored-route: replay incomplete, some routes were not relayed")

	// errRelayAddPath refuses a route whose source session negotiated ADD-PATH
	// (RFC 7911). The reconstruction is tagged with the source peer's receive
	// context so the forward rail decodes attributes at the right ASN width, but
	// that context also declares the NLRI framing -- and the stored NLRI does not
	// match it.
	//
	// The structured ingest path drops the 4-byte path-id before storing:
	// nlri.NLRIIterator.Next advances past it and returns only the prefix bytes
	// (internal/core/bgp/nlri/iterator.go), which installStructuredNLRIs hex-encodes
	// verbatim (adj_rib_in/rib.go). The legacy ingest path does the opposite,
	// prepending the path-id in prefixToWireHex -- and only when it is non-zero,
	// though RFC 7911 permits path-id 0. So the stored bytes carry one of two
	// framings and nothing records which.
	//
	// Emitting them under an add-path context is not a cosmetic mismatch: a
	// destination sharing the source's context receives the wire verbatim and
	// parses the first four NLRI bytes as a path-id, producing a malformed UPDATE
	// and a session reset -- a peer-up replay would tear down the peer it is
	// replaying to. A destination WITHOUT add-path takes the re-encode path, which
	// strips four more bytes and announces nothing at all, silently.
	//
	// Refusing is the honest interim: normalizing the stored framing is the
	// remaining work, tracked as assumption A-3 in
	// plan/spec-fixit-bgp-egress-rail-divergence.md and homed in
	// plan/deferrals/fixit-bgp-egress-rail-divergence.md. A logged refusal loses
	// the replay; a corrupt frame loses the session.
	errRelayAddPath = errors.New("relay-stored-route: add-path source not supported by stored-route replay")
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
// The receive context is load-bearing: the stored attribute bytes are still in
// the SOURCE peer's encoding, so AS_PATH is 2- or 4-octet per what that peer
// negotiated. Handing the reconstructed wire a different context would make every
// attribute-matching egress filter read a corrupt AS_PATH.
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
		out.ctxID = srcPeer.RecvContextID()
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
func (a *reactorAPIAdapter) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute) error {
	if len(routes) == 0 {
		return nil
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

	for i := range routes {
		route := &routes[i]

		// Counted BEFORE the parse guard: a route dropped for an unparseable
		// source is still a route the caller asked us to relay, and leaving it
		// out of `eligible` made the completeness check below fail OPEN.
		eligible++

		srcAddr, parseErr := netip.ParseAddr(route.SourcePeer)
		if parseErr != nil {
			var tb textbuf.Buffer
			fwdLogger().Error("relay-stored-route: unparseable source peer",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", parseErr)
			lastErr = errors.New(tb.Str("relay-stored-route: invalid source peer ").Quoted(route.SourcePeer).String())
			continue
		}

		// A route must never be relayed back to the peer that sent it.
		// buildReplayRoutes filters this on the plugin side, but the engine
		// cannot trust a caller-supplied list to have done so.
		if srcAddr == destination {
			// Never eligible: a route is not relayed back to its own source.
			eligible--
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
			continue
		}

		update, updateID, buildErr := a.buildRelayUpdate(route, src, &spans)
		if buildErr != nil {
			fwdLogger().Error("relay-stored-route: reconstruction failed",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", buildErr)
			lastErr = buildErr
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
			relayed++
		case errors.Is(fwdErr, errAllDestinationsSuppressed):
			relayed++ // handled: egress policy decided this peer gets nothing
			fwdLogger().Debug("relay-stored-route: suppressed for destination",
				"source", route.SourcePeer, "destination", destination, "family", route.Family)
		default:
			fwdLogger().Error("relay-stored-route: forward failed",
				"source", route.SourcePeer, "destination", destination,
				"family", route.Family, "err", fwdErr)
			lastErr = fwdErr
		}
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
func (a *reactorAPIAdapter) buildRelayUpdate(route *rpc.StoredRoute, src relaySource, spans *[]relayAttrSpan) (*ReceivedUpdate, uint64, error) {
	fam, known := family.LookupFamily(route.Family)
	if !known {
		return nil, 0, errRelayFamily
	}

	// Refuse before touching a buffer: the stored NLRI framing does not record
	// whether it carries an RFC 7911 path-id, so it cannot be emitted under a
	// context that declares one. See errRelayAddPath.
	if srcCtx := bgpctx.Registry.Get(src.ctxID); srcCtx != nil && srcCtx.AddPath(fam) {
		return nil, 0, errRelayAddPath
	}

	attrLen := hex.DecodedLen(len(route.AttrHex))
	nhLen := hex.DecodedLen(len(route.NextHopHex))
	nlriLen := hex.DecodedLen(len(route.NLRIHex))
	if nlriLen == 0 || nhLen == 0 {
		return nil, 0, errRelayHex
	}

	// Decode the three stored hex fields into ONE pooled scratch buffer rather
	// than three heap slices: a peer-up replay runs this per stored route.
	scratchLen := attrLen + nhLen + nlriLen
	scratch := getReadBuf(scratchLen > message.MaxMsgLen-message.HeaderLen)
	if scratch.Buf == nil {
		return nil, 0, errRelayBufferPool
	}
	defer ReturnReadBuffer(scratch)
	if scratchLen > len(scratch.Buf) {
		return nil, 0, errRelayTooLarge
	}
	attrs := scratch.Buf[:attrLen]
	nextHop := scratch.Buf[attrLen : attrLen+nhLen]
	nlri := scratch.Buf[attrLen+nhLen : scratchLen]
	// hex.Decode does not leak src, so the []byte conversions are elided by the
	// compiler ("zero-copy string->[]byte conversion") and cost nothing per
	// route. A hand-rolled decoder was tried here and reverted: it was premised
	// on an allocation that measurement showed does not happen.
	if _, err := hex.Decode(attrs, []byte(route.AttrHex)); err != nil {
		return nil, 0, errRelayHex
	}
	if _, err := hex.Decode(nextHop, []byte(route.NextHopHex)); err != nil {
		return nil, 0, errRelayHex
	}
	if _, err := hex.Decode(nlri, []byte(route.NLRIHex)); err != nil {
		return nil, 0, errRelayHex
	}

	scanned, ok := scanAttrBlock(*spans, attrs)
	*spans = scanned
	if !ok {
		return nil, 0, errRelayAttrs
	}

	needNextHop := relayNeedsNextHopAttr(scanned, fam)
	// Checked here as well as inside relayPayloadLen so the refusal names the
	// actual defect rather than being folded into "too large".
	if fam == family.IPv4Unicast && needNextHop && len(nextHop) != 4 {
		return nil, 0, errRelayNextHopLen
	}
	size, ok := relayPayloadLen(scanned, nextHop, nlri, fam, needNextHop)
	if !ok {
		return nil, 0, errRelayTooLarge
	}

	// The reconstruction buffer is NOT returned here: it backs the WireUpdate for
	// the whole forward, including asynchronous per-peer writes, and is returned
	// when the cache evicts the entry.
	out := getReadBuf(size > message.MaxMsgLen-message.HeaderLen)
	if out.Buf == nil {
		return nil, 0, errRelayBufferPool
	}
	if size > len(out.Buf) {
		ReturnReadBuffer(out)
		return nil, 0, errRelayTooLarge
	}
	n := writeRelayPayload(out.Buf, 0, scanned, attrs, nextHop, nlri, fam, needNextHop)

	ru := &ReceivedUpdate{
		poolBuf:      out,
		SourcePeerIP: src.addr,
		// The forward path threads this into fwdItem.sourcePeerStr for the sent
		// event callback; the peer's cached string avoids a per-route allocation.
		SourcePeerStr: src.strAdr,
		ReceivedAt:    a.r.clock.Now(),
		// Meta is deliberately nil: the ingress annotations a live UPDATE carries
		// (meta["src-role"], meta["stale"]) are not stored alongside the route.
		//
		// RFC 9494 staleness is genuinely false for a replay, so its absence is
		// correct. RFC 9234 role handling still suppresses correctly here because
		// the ingress filter stamps OTC into the WIRE bytes before the route is
		// stored, so the wire-bytes egress rule (role/otc.go checkOTCEgress) sees
		// it on the reconstruction without needing meta.
		//
		// The meta-based Gao-Rexford safety net in OTCEgressFilter does go
		// unevaluated for a replayed route, which is a real gap -- but a
		// pre-existing and separable one (it fires for ANY caller without ingress
		// meta, not just this path), and closing it means changing an RFC-tagged
		// test. Tracked in plan/deferrals/fixit-bgp-egress-rail-divergence.md.
		Meta: nil,
	}
	// The stored bytes are still in the SOURCE peer's encoding, so the wire must
	// carry that peer's receive context or every attribute-matching egress filter
	// decodes AS_PATH at the wrong ASN width.
	wireu.InitWireUpdate(&ru.wireUpdateInline, out.Buf[:n], src.ctxID)
	updateID := nextMsgID()
	ru.wireUpdateInline.SetMessageID(updateID)
	ru.wireUpdateInline.SetSourceID(src.srcID)
	ru.WireUpdate = &ru.wireUpdateInline

	// Add -> RetainN -> Activate mirrors the received-UPDATE lifecycle with no
	// plugin consumers: Activate(id, 0) clears the pending flag while the
	// build-time retain keeps the entry alive, so a cache-consumer's cumulative
	// ack can pass over this entry without evicting a buffer still in flight
	// (ackEntryLocked only evicts at zero TOTAL consumers).
	a.r.recentUpdates.Add(ru)
	a.r.recentUpdates.RetainN(updateID, 1)
	a.r.recentUpdates.Activate(updateID, 0)

	return ru, updateID, nil
}
