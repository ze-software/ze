// Design: plan/learned/663-rs-gap-0-structural-forwarding.md -- shared body-building for forwarding
// Related: reactor_api_forward.go -- ForwardUpdate (caller)
// Related: forward_rs.go -- reactorForwardRS (caller)
package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// fwdBodyResult holds the output of buildFwdBody.
type fwdBodyResult struct {
	rawBodies    [][]byte
	updates      []*message.Update
	supersedeKey uint64
	withdrawal   bool
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
	// Preserve the same-context zero-copy path before any parse or re-encode.
	// RFC 7911 ADD-PATH and RFC 6793 ASN4 differences are encoded in ContextID,
	// so raw splitting is safe only when the source and destination IDs match.
	if sameCtx {
		// Size is not the only reason to re-chunk. RFC 7606 Section 5.1: "An UPDATE
		// message MUST NOT contain more than one of the following: non-empty Withdrawn
		// Routes field, non-empty Network Layer Reachability Information field,
		// MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute." Ze is the sender of
		// the bytes it relays, so a mixed shape received from a peer must be split before
		// it goes back out. The verdict is cached on the WireUpdate, which is shared
		// across this loop's destinations, so the common single-field UPDATE costs one
		// bool read and keeps the zero-copy append below.
		if updateSize > maxMsgSize || peerWire.MixesNLRIFields() {
			srcCtx := bgpctx.Registry.Get(srcCtxID)
			maxBodySize := maxMsgSize - message.HeaderLen
			splits, err := wireu.SplitWireUpdate(peerWire, maxBodySize, srcCtx)
			if err != nil {
				fwdLogger().Warn("forward split failed", "peer", peerAddr, "err", err)
				return result, false
			}
			for _, split := range splits {
				result.rawBodies = append(result.rawBodies, split.Payload())
			}
		} else {
			result.rawBodies = append(result.rawBodies, peerWire.Payload())
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
		destUpdate, encodeErr := fwdUpdateForDestination(cache.update, srcCtxID, destCtxID)
		if encodeErr != nil {
			fwdLogger().Warn("encoding update for forward",
				"peer", peerAddr, "error", encodeErr)
			return result, false
		}

		// Same RFC 7606 Section 5.1 restriction as the raw branch above. This UPDATE is
		// ze's own composition -- fwdUpdateForDestination rebuilt its sections for the
		// destination context -- so emitting a mixed shape here is the plainer violation
		// of the two.
		if destUpdate.Len(nil) > maxMsgSize || destUpdate.MixesNLRIFields() {
			destSendCtx := peer.SendContext()
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
	tmp := fwdItem{rawBodies: result.rawBodies, updates: result.updates}
	result.withdrawal = fwdIsWithdrawal(&tmp)
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

func fwdUpdateForDestination(update *message.Update, srcCtxID, destCtxID bgpctx.ContextID) (*message.Update, error) {
	if srcCtxID == 0 || destCtxID == 0 || srcCtxID == destCtxID {
		return update, nil
	}

	srcCtx := bgpctx.Registry.Get(srcCtxID)
	if srcCtx == nil {
		return nil, fmt.Errorf("unknown source context ID: %d", srcCtxID)
	}
	destCtx := bgpctx.Registry.Get(destCtxID)
	if destCtx == nil {
		return nil, fmt.Errorf("unknown destination context ID: %d", destCtxID)
	}

	// RFC 6793 ASN width changes live in AS_PATH/AS4_PATH attributes. Transcode
	// the full payload first, then adjust NLRI ADD-PATH framing below.
	baseUpdate := update
	if srcCtx.ASN4() != destCtx.ASN4() {
		payload := update.RawData()
		if payload == nil {
			payload = fwdPackUpdateBody(update)
		}
		buf := make([]byte, len(payload)*2+1024)
		n, err := wireu.TranscodeASPath(buf, payload, srcCtx.ASN4(), destCtx.ASN4())
		if err != nil {
			return nil, fmt.Errorf("path attributes: %w", err)
		}
		if n > 0 {
			baseUpdate, err = message.UnpackUpdate(buf[:n])
			if err != nil {
				return nil, fmt.Errorf("transcoded update: %w", err)
			}
		}
	}

	withdrawn, err := fwdReencodeNLRIs(baseUpdate.WithdrawnRoutes, family.IPv4Unicast, srcCtx, destCtx)
	if err != nil {
		return nil, fmt.Errorf("withdrawn routes: %w", err)
	}
	announced, err := fwdReencodeNLRIs(baseUpdate.NLRI, family.IPv4Unicast, srcCtx, destCtx)
	if err != nil {
		return nil, fmt.Errorf("nlri: %w", err)
	}

	attrs, err := fwdReencodeMPAttributes(baseUpdate.PathAttributes, srcCtx, destCtx)
	if err != nil {
		return nil, fmt.Errorf("multiprotocol attributes: %w", err)
	}

	return &message.Update{
		WithdrawnRoutes: withdrawn,
		PathAttributes:  attrs,
		NLRI:            announced,
	}, nil
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

func fwdReencodeMPAttributes(attrs []byte, srcCtx, destCtx *bgpctx.EncodingContext) ([]byte, error) {
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
			reencoded, err := fwdReencodeNLRIs(mp.NLRI, fam, srcCtx, destCtx)
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
			reencoded, err := fwdReencodeNLRIs(mp.NLRI, fam, srcCtx, destCtx)
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

func fwdReencodeNLRIs(data []byte, fam family.Family, srcCtx, destCtx *bgpctx.EncodingContext) ([]byte, error) {
	srcAddPath := srcCtx.AddPath(fam)
	destAddPath := destCtx.AddPath(fam)
	if srcAddPath == destAddPath || len(data) == 0 {
		return data, nil
	}

	// RFC 7911 path IDs are present per family and direction. Re-frame each NLRI
	// before size splitting so message.Splitter sees destination-context bytes.
	iter := nlri.NewNLRIIterator(data, srcAddPath)
	out := make([]byte, 0, fwdReencodedNLRILen(data, iter, destAddPath))
	iter.Reset()
	for prefix, pathID, ok := iter.Next(); ok; prefix, pathID, ok = iter.Next() {
		if destAddPath {
			var pathBuf [4]byte
			binary.BigEndian.PutUint32(pathBuf[:], pathID)
			out = append(out, pathBuf[:]...)
		}
		out = append(out, prefix...)
	}
	if iter.Remaining() != 0 {
		return nil, fmt.Errorf("trailing malformed NLRI bytes: %d", iter.Remaining())
	}
	return out, nil
}

func fwdReencodedNLRILen(data []byte, iter *nlri.NLRIIterator, destAddPath bool) int {
	if destAddPath {
		return len(data) + iter.Count()*4
	}
	return len(data) - iter.Count()*4
}
