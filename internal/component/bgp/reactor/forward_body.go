// Design: docs/architecture/bgp/structural-forwarding.md -- shared body-building for forwarding
// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
// RFC: rfc/short/rfc7606.md — Section 5.1, one NLRI-bearing field per emitted UPDATE
// Related: forward_path_id.go -- the Path Identifier generator both branches read
// Related: reactor_api_forward.go -- ForwardUpdate (caller)
// Related: forward_rs.go -- reactorForwardRS (caller)
package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

// fwdBodyResult holds the output of buildFwdBody.
type fwdBodyResult struct {
	rawBodies [][]byte
	updates   []*message.Update

	// transcodeBuf is the read-pool buffer backing the cross-context RFC 6793
	// transcode, when one ran. The updates above alias it zero-copy, and they are
	// written to TCP by a forward-pool worker AFTER buildFwdBody returns, so the
	// caller MUST adopt it onto the ReceivedUpdate (adoptFwdHandle) rather than
	// release it at end of call. It is then returned exactly once at cache
	// eviction. Zero when no transcode ran, or when the transcode fell back to a
	// collector-owned buffer; adoptFwdHandle ignores a zero handle.
	transcodeBuf BufHandle

	supersedeKey uint64
}

// fwdParseCache caches a parsed UPDATE across peers to avoid redundant parsing.
// Shared across the per-peer loop in both ForwardUpdate and reactorForwardRS.
type fwdParseCache struct {
	update *message.Update
	wire   *wireu.WireUpdate
}

// buildFwdBody builds the rawBodies/updates for a single destination peer.
// Handles wire-level splitting (RFC 8654), zero-copy forwarding, and re-encode.
// Returns ok=false if the peer should be skipped (parse/split error).
func buildFwdBody(
	peerWire *wireu.WireUpdate,
	maxMsgSize int,
	destCtxID bgpctx.ContextID,
	peer *Peer,
	peerAddr netip.Addr,
	cache *fwdParseCache,
) (result fwdBodyResult, ok bool) {
	updateSize := message.HeaderLen + len(peerWire.Payload())
	srcCtxID := peerWire.SourceCtxID()
	sameCtx := srcCtxID != 0 && destCtxID != 0 && srcCtxID == destCtxID

	// SINGLE point that decides a borrowed buffer's fate, whichever branch
	// borrowed it: the RFC 7911 Path Identifier rewrite in the raw branch, or
	// the RFC 6793 transcode in fwdUpdateForDestination. Both hand their bytes
	// to a forward-pool worker that writes them to TCP AFTER this call returns,
	// so on success the handle travels out on result for the caller to adopt
	// onto the ReceivedUpdate (adoptFwdHandle), and on any failure it goes
	// straight back to the pool. Never both -- a caller that sees ok=false never
	// adopts.
	var ownedBuf BufHandle
	defer func() {
		if ok {
			result.transcodeBuf = ownedBuf
			return
		}
		ReturnReadBuffer(ownedBuf)
	}()

	// Preserve the same-context raw-split path before any parse or re-encode.
	// RFC 7911 ADD-PATH and RFC 6793 ASN4 differences are encoded in ContextID,
	// so raw splitting is safe only when the source and destination IDs match.
	if sameCtx {
		// Matching contexts mean matching FRAMING, never a right to relay the
		// identifiers inside that framing. RFC 7911 Section 2 makes a
		// re-advertised route carry ze's own Path Identifier, and this is the
		// branch a route server between like-configured clients takes, so it is
		// the branch where relaying the source's identifier loses a route. The
		// rewrite is length-preserving and lands in a copy, so the split below
		// stays raw. A session that negotiated ADD-PATH for nothing gets a nil
		// patch and keeps its zero-copy forward.
		//
		// The ADD-PATH answer comes from peer.sendContext(), an atomic load, and
		// not from a registry lookup on destCtxID: sameCtx already says the two
		// IDs are one, and a peer sets and clears sendCtx beside sendCtxID
		// (peer.go). A re-negotiation between the forwardFacts snapshot and this
		// call nulls sendCtx, which reads as no ADD-PATH -- and those bytes are
		// bound for a session that is being torn down. The parsed branch below
		// takes the same reading for its split decision.
		bodyOut := peerWire.Payload()
		patched, patchBuf, patchErr := fwdRegenerateRawPathIDs(peerWire, peer.sendContext())
		if patchErr != nil {
			fwdLogger().Warn("forward path identifier rewrite failed", "peer", peerAddr, "err", patchErr)
			return result, false
		}
		if patched != nil {
			bodyOut = patched
			ownedBuf = patchBuf
		}

		// Size is not the only reason to re-chunk. RFC 7606 Section 5.1: "An UPDATE
		// message MUST NOT contain more than one of the following: non-empty Withdrawn
		// Routes field, non-empty Network Layer Reachability Information field,
		// MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute." Ze is the sender of
		// the bytes it relays, so a mixed shape received from a peer must be split before
		// it goes back out. The verdict is cached on the WireUpdate, which is shared
		// across this loop's destinations, so the common single-field UPDATE costs one
		// bool read and keeps the raw append below. The copy has the source's shape
		// byte for byte, so the source's cached verdict is the copy's verdict.
		if updateSize > maxMsgSize || peerWire.MixesNLRIFields() {
			srcCtx := bgpctx.Registry.Get(srcCtxID)
			maxBodySize := maxMsgSize - message.HeaderLen
			// The splitter reads a WireUpdate, so the rewritten payload gets one
			// here rather than on every forward.
			splitWire := peerWire
			if patched != nil {
				var rewritten wireu.WireUpdate
				wireu.InitWireUpdate(&rewritten, patched, srcCtxID)
				splitWire = &rewritten
			}
			splits, err := wireu.SplitWireUpdate(splitWire, maxBodySize, srcCtx)
			if err != nil {
				fwdLogger().Warn("forward split failed", "peer", peerAddr, "err", err)
				return result, false
			}
			for _, split := range splits {
				result.rawBodies = append(result.rawBodies, split.Payload())
			}
		} else {
			result.rawBodies = append(result.rawBodies, bodyOut)
		}
	} else {
		if cache.update == nil || cache.wire != peerWire {
			var parseErr error
			cache.update, parseErr = message.UnpackUpdate(peerWire.Payload())
			if parseErr != nil {
				fwdLogger().Warn("parsing update for forward",
					"peer", peerAddr, "error", parseErr)
				return result, false
			}
			cache.wire = peerWire
		}

		// Context mismatch means source wire bytes may carry RFC 7911 path IDs
		// or RFC 6793 four-octet ASNs the destination did not negotiate. Convert
		// to destination-context UPDATE sections before applying RFC 8654 size
		// splitting so every emitted chunk is valid for the recipient.
		destUpdate, transcodeBuf, encodeErr := fwdUpdateForDestination(cache.update, srcCtxID, destCtxID, peerWire.SourceID())
		if encodeErr != nil {
			fwdLogger().Warn("encoding update for forward",
				"peer", peerAddr, "error", encodeErr)
			return result, false
		}
		// destUpdate aliases transcodeBuf zero-copy (see fwdUpdateForDestination).
		// The handle joins the one ownership point declared at the top of this
		// function, which decides its fate for both branches.
		ownedBuf = transcodeBuf

		// Same RFC 7606 Section 5.1 restriction as the raw branch above. This UPDATE is
		// ze's own composition -- fwdUpdateForDestination rebuilt its sections for the
		// destination context -- so emitting a mixed shape here is the plainer violation
		// of the two.
		if destUpdate.Len(nil) > maxMsgSize || destUpdate.MixesNLRIFields() {
			destSendCtx := peer.sendContext()
			addPath := addPathForUpdate(destSendCtx, destUpdate)

			splitErr := fwdSplitParsedUpdate(destUpdate, maxMsgSize, addPath, &result)
			if splitErr != nil {
				fwdLogger().Warn("forward split failed", "peer", peerAddr, "err", splitErr)
				return result, false
			}
		} else {
			result.updates = append(result.updates, destUpdate)
		}
	}

	result.supersedeKey = fwdSupersedeKey(result.rawBodies)
	return result, true
}

func fwdSplitParsedUpdate(update *message.Update, maxMsgSize int, addPath bool, result *fwdBodyResult) error {
	splitter := message.GetSplitter()
	defer message.PutSplitter(splitter)
	// SplitCompliant rather than Split: this path also has to break up an UPDATE that
	// fits but carries more than one NLRI-bearing field (RFC 7606 Section 5.1).
	return splitter.SplitCompliant(update, maxMsgSize, addPath, func(c *message.Update) error {
		result.updates = append(result.updates, &message.Update{
			WithdrawnRoutes: append([]byte(nil), c.WithdrawnRoutes...),
			PathAttributes:  append([]byte(nil), c.PathAttributes...),
			NLRI:            append([]byte(nil), c.NLRI...),
		})
		return nil
	})
}

// fwdUpdateForDestination re-encodes update for the destination encoding context.
//
// Lifetime contract (docs/architecture/memory/lifetime-contracts.md, contract A).
// The returned *message.Update ALIASES the returned BufHandle. message.UnpackUpdate
// is zero-copy: it keeps its input as rawData and slices WithdrawnRoutes, PathAttributes
// and NLRI out of that same array. fwdReencodeNLRIs returns its input unchanged when
// source and destination agree on RFC 7911 ADD-PATH, and fwdReencodeMPAttributes returns
// its input unchanged when nothing needed re-framing -- both the ordinary case here,
// because this branch is entered on an RFC 6793 ASN4 mismatch, not an ADD-PATH one. So
// every section of the result can point into the transcode buffer, and that buffer is
// written to TCP by a forward-pool worker long after this call returns.
//
// The caller therefore MUST NOT release the handle at end of call. It adopts it onto the
// ReceivedUpdate (adoptFwdHandle), which returns it exactly once at cache eviction. A
// zero handle means there is nothing to adopt: either no transcode ran, or the transcode
// wrote into a collector-owned buffer that needs no handle.
func fwdUpdateForDestination(update *message.Update, srcCtxID, destCtxID bgpctx.ContextID, srcID source.SourceID) (destUpdate *message.Update, transcodeBuf BufHandle, err error) {
	if srcCtxID == 0 || destCtxID == 0 || srcCtxID == destCtxID {
		return update, BufHandle{}, nil
	}

	srcCtx := bgpctx.Registry.Get(srcCtxID)
	if srcCtx == nil {
		return nil, BufHandle{}, fmt.Errorf("unknown source context ID: %d", srcCtxID)
	}
	destCtx := bgpctx.Registry.Get(destCtxID)
	if destCtx == nil {
		return nil, BufHandle{}, fmt.Errorf("unknown destination context ID: %d", destCtxID)
	}

	// RFC 6793 ASN width changes live in AS_PATH/AS4_PATH attributes. Transcode
	// the full payload first, then adjust NLRI ADD-PATH framing below.
	baseUpdate := update
	if srcCtx.ASN4() != destCtx.ASN4() {
		payload := update.RawData()
		if payload == nil {
			payload = fwdPackUpdateBody(update)
		}

		// wireu.TranscodeASPath does not bounds-check dst: it copies and writes at
		// computed offsets, so an undersized destination truncates or panics rather
		// than reporting. Keep the size this call site has always asked for -- twice
		// the payload plus slack, which covers the 2->4 ASN widening plus a new
		// AS4_PATH and AS4_AGGREGATOR -- and take the smallest pool class holding it.
		need := len(payload)*2 + 1024
		var handle BufHandle
		switch {
		case need <= message.MaxMsgLen:
			handle = getReadBuf(false)
		case need <= message.ExtMsgLen:
			handle = getReadBuf(true)
		}
		dst := handle.Buf
		if dst == nil {
			// pool-fallback: the requirement is above the largest pool class, or the
			// read pools are at budget. A collector-owned buffer is safe to alias into
			// the async write without a handle, which is exactly how this site worked
			// before it was pooled, so falling back never drops a route.
			dst = make([]byte, need)
		}

		// SINGLE return point for handle. It goes back to the pool here unless
		// ownership leaves with the successful return below, where the caller adopts
		// it. Zeroing the named handle on error first means no future edit can hand a
		// buffer out beside an error and have it returned twice.
		defer func() {
			if err != nil {
				transcodeBuf = BufHandle{}
			}
			if transcodeBuf.Buf == nil {
				ReturnReadBuffer(handle)
			}
		}()

		n, transErr := wireu.TranscodeASPath(dst, payload, srcCtx.ASN4(), destCtx.ASN4())
		if transErr != nil {
			return nil, BufHandle{}, fmt.Errorf("path attributes: %w", transErr)
		}
		if n > 0 {
			transcoded, unpackErr := message.UnpackUpdate(dst[:n])
			if unpackErr != nil {
				return nil, BufHandle{}, fmt.Errorf("transcoded update: %w", unpackErr)
			}
			// The parse is zero-copy, so baseUpdate now aliases dst. Carry the handle
			// out only when dst came from the pool; a collector-owned dst keeps the
			// zero handle and is kept alive by the reference in baseUpdate.
			baseUpdate = transcoded
			transcodeBuf = handle
		}
	}

	// One memo for every section of this UPDATE, so the identifier a withdrawn
	// route leaves under is the one its announcement left under: both sections
	// ask the same table with the same ingress key.
	memo := fwdPathIDMemo{source: srcID}

	withdrawn, err := fwdReencodeNLRIs(baseUpdate.WithdrawnRoutes, family.IPv4Unicast, srcCtx, destCtx, &memo)
	if err != nil {
		return nil, BufHandle{}, fmt.Errorf("withdrawn routes: %w", err)
	}
	announced, err := fwdReencodeNLRIs(baseUpdate.NLRI, family.IPv4Unicast, srcCtx, destCtx, &memo)
	if err != nil {
		return nil, BufHandle{}, fmt.Errorf("nlri: %w", err)
	}

	attrs, err := fwdReencodeMPAttributes(baseUpdate.PathAttributes, srcCtx, destCtx, &memo)
	if err != nil {
		return nil, BufHandle{}, fmt.Errorf("multiprotocol attributes: %w", err)
	}

	return &message.Update{
		WithdrawnRoutes: withdrawn,
		PathAttributes:  attrs,
		NLRI:            announced,
	}, transcodeBuf, nil
}

// ownOverflowBodies makes an item's payload bytes its own before it enters the
// forward worker's overflow queue.
//
// A fast-path item aliases the source cache entry's buffers safely: it reaches
// the channel and drains in microseconds. An overflow item cannot. It sits in
// w.overflow for as long as the destination stays behind, and the recent-update
// cache's safety valve force-evicts a passed-over entry after the valve elapses
// (recent_cache.go runGapScan -> evictLocked: 5 minutes, or 30 seconds while the
// read pool is under pressure). Eviction returns exactly the memory these bodies
// point into -- poolBuf, the EBGP patched slots, and every per-forward handle
// adopted onto the entry -- and the retain the item holds does not protect it,
// because isGapEvictable force-evicts precisely the entries that still have
// consumers. The worker would then write another session's bytes to the peer.
// See plan/spec-fixit-forward-rail-initial-sync-ordering.md D-5.
//
// The copy goes into the tier 2 MixedBufMux handle the item already holds, which
// is sized and byte-budgeted for exactly these bytes and was until now an
// accounting token over borrowed memory. When there is no handle (mux absent,
// congestion denial, or pool exhaustion) or the bodies do not fit one, the copy
// goes on the heap: an allocation on the exhausted-pool path is the correct
// trade against sending wrong bytes, and no route is dropped either way. The
// handle stays with the item, so releaseItem returns it exactly once as before.
func ownOverflowBodies(item *fwdItem) {
	total := 0
	for _, b := range item.rawBodies {
		total += len(b)
	}
	for _, u := range item.updates {
		total += len(u.WithdrawnRoutes) + len(u.PathAttributes) + len(u.NLRI)
	}
	if total == 0 {
		return
	}

	dst := item.overflowBuf.Buf
	if len(dst) < total {
		dst = make([]byte, total)
	}
	off := 0
	own := func(src []byte) []byte {
		if len(src) == 0 {
			return nil
		}
		n := copy(dst[off:], src)
		out := dst[off : off+n]
		off += n
		return out
	}

	if len(item.rawBodies) > 0 {
		// A fresh header slice, never a write into the caller's: one [][]byte is
		// shared across every destination of an UPDATE through the forward body
		// cache (reactor_api_forward.go, forward_rs.go), so re-pointing in place
		// would repoint the other destinations' items too.
		bodies := make([][]byte, len(item.rawBodies))
		for i, b := range item.rawBodies {
			bodies[i] = own(b)
		}
		item.rawBodies = bodies
	}

	if len(item.updates) > 0 {
		// Same sharing, and the sections alias the entry's read buffer or the
		// transcode handle adopted onto it. rawData is not carried: nothing on
		// the send path reads it (RawData has one caller, fwdUpdateForDestination,
		// which ran before this item was built).
		updates := make([]*message.Update, len(item.updates))
		for i, u := range item.updates {
			updates[i] = &message.Update{
				WithdrawnRoutes: own(u.WithdrawnRoutes),
				PathAttributes:  own(u.PathAttributes),
				NLRI:            own(u.NLRI),
			}
		}
		item.updates = updates
	}
}

func fwdPackUpdateBody(update *message.Update) []byte {
	body := make([]byte, 2+len(update.WithdrawnRoutes)+2+len(update.PathAttributes)+len(update.NLRI))
	off := 0
	binary.BigEndian.PutUint16(body[off:], uint16(len(update.WithdrawnRoutes))) //nolint:gosec // UPDATE body length fields are BGP-bounded.
	off += 2
	off += copy(body[off:], update.WithdrawnRoutes)
	binary.BigEndian.PutUint16(body[off:], uint16(len(update.PathAttributes))) //nolint:gosec // UPDATE body length fields are BGP-bounded.
	off += 2
	off += copy(body[off:], update.PathAttributes)
	copy(body[off:], update.NLRI)
	return body
}

func fwdReencodeMPAttributes(attrs []byte, srcCtx, destCtx *bgpctx.EncodingContext, memo *fwdPathIDMemo) ([]byte, error) {
	var out []byte
	changed := false
	for off := 0; off < len(attrs); {
		flags, code, length, hdrLen, err := attribute.ParseHeader(attrs[off:])
		if err != nil {
			return nil, err
		}
		end := off + hdrLen + int(length)
		if end > len(attrs) {
			return nil, fmt.Errorf("attribute %s length exceeds buffer", code)
		}

		value := attrs[off+hdrLen : end]
		newValue := value
		switch code {
		case attribute.AttrMPReachNLRI:
			mp, err := attribute.ParseMPReachNLRI(value)
			if err != nil {
				return nil, err
			}
			fam := family.Family{AFI: family.AFI(mp.AFI), SAFI: family.SAFI(mp.SAFI)}
			reencoded, err := fwdReencodeNLRIs(mp.NLRI, fam, srcCtx, destCtx, memo)
			if err != nil {
				return nil, err
			}
			if len(reencoded) != len(mp.NLRI) || (len(reencoded) > 0 && &reencoded[0] != &mp.NLRI[0]) {
				mp.NLRI = reencoded
				newValue = make([]byte, mp.Len())
				mp.WriteTo(newValue, 0)
			}
		case attribute.AttrMPUnreachNLRI:
			mp, err := attribute.ParseMPUnreachNLRI(value)
			if err != nil {
				return nil, err
			}
			fam := family.Family{AFI: family.AFI(mp.AFI), SAFI: family.SAFI(mp.SAFI)}
			reencoded, err := fwdReencodeNLRIs(mp.NLRI, fam, srcCtx, destCtx, memo)
			if err != nil {
				return nil, err
			}
			if len(reencoded) != len(mp.NLRI) || (len(reencoded) > 0 && &reencoded[0] != &mp.NLRI[0]) {
				mp.NLRI = reencoded
				newValue = make([]byte, mp.Len())
				mp.WriteTo(newValue, 0)
			}
		default:
			// Other attributes do not carry NLRI framing and need no context conversion.
		}

		if len(newValue) != len(value) || (len(newValue) > 0 && &newValue[0] != &value[0]) {
			if !changed {
				out = append(out, attrs[:off]...)
			}
			hdr := make([]byte, 4)
			n := attribute.WriteHeaderTo(hdr, 0, flags, code, uint16(len(newValue))) //nolint:gosec // BGP attr value length is bounded by uint16.
			out = append(out, hdr[:n]...)
			out = append(out, newValue...)
			changed = true
		} else if changed {
			out = append(out, attrs[off:end]...)
		}
		off = end
	}
	if !changed {
		return attrs, nil
	}
	return out, nil
}

func fwdReencodeNLRIs(data []byte, fam family.Family, srcCtx, destCtx *bgpctx.EncodingContext, memo *fwdPathIDMemo) ([]byte, error) {
	srcAddPath := srcCtx.AddPath(fam)
	destAddPath := destCtx.AddPath(fam)
	if len(data) == 0 || (!srcAddPath && !destAddPath) {
		return data, nil
	}

	// RFC 7911 path IDs are present per family and direction. Re-frame each NLRI
	// before size splitting so message.Splitter sees destination-context bytes.
	//
	// A destination that reads path IDs gets ze's own, never the source's (RFC
	// 7911 Section 2), so this runs even when both contexts frame alike and only
	// the value changes. The identifier the iterator reports is 0 for every
	// prefix when the SOURCE negotiated no ADD-PATH, and 0 is then the ingress
	// key of every path that source sends: one key, one identifier per source,
	// which is what separates two such sources at the destination.
	iter := nlri.NewNLRIIterator(data, srcAddPath)
	out := make([]byte, 0, fwdReencodedNLRILen(data, iter, srcAddPath, destAddPath))
	iter.Reset()
	for prefix, pathID, ok := iter.Next(); ok; prefix, pathID, ok = iter.Next() {
		if destAddPath {
			var pathBuf [4]byte
			binary.BigEndian.PutUint32(pathBuf[:], memo.generate(pathID))
			out = append(out, pathBuf[:]...)
		}
		out = append(out, prefix...)
	}
	if iter.Remaining() != 0 {
		return nil, fmt.Errorf("trailing malformed NLRI bytes: %d", iter.Remaining())
	}
	return out, nil
}

// fwdReencodedNLRILen sizes the re-framed section exactly: four octets per NLRI
// appear when only the destination reads path IDs, disappear when only the
// source wrote them, and neither when both do -- ze rewrites the value in place
// of the source's and the length does not move.
func fwdReencodedNLRILen(data []byte, iter *nlri.NLRIIterator, srcAddPath, destAddPath bool) int {
	switch {
	case srcAddPath == destAddPath:
		return len(data)
	case destAddPath:
		return len(data) + iter.Count()*4
	default:
		return len(data) - iter.Count()*4
	}
}
