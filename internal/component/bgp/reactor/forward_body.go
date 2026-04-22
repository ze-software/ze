// Design: plan/spec-rs-gap-1-reactor-fastpath.md -- shared body-building for forwarding
// Related: reactor_api_forward.go -- ForwardUpdate (caller)
// Related: forward_rs.go -- reactorForwardRS (caller)
package reactor

import (
	"net/netip"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
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

	if updateSize > maxMsgSize {
		srcCtxID := peerWire.SourceCtxID()
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
		srcCtxID := peerWire.SourceCtxID()
		if srcCtxID != 0 && destCtxID != 0 && srcCtxID == destCtxID {
			result.rawBodies = append(result.rawBodies, peerWire.Payload())
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

			repackedSize := message.HeaderLen + 4 + len(cache.update.WithdrawnRoutes) +
				len(cache.update.PathAttributes) + len(cache.update.NLRI)
			if repackedSize > maxMsgSize {
				destSendCtx := peer.SendContext()
				addPath := addPathForUpdate(destSendCtx, cache.update)

				splitErr := func() error {
					splitter := message.GetSplitter()
					defer message.PutSplitter(splitter)
					return splitter.Split(cache.update, maxMsgSize, addPath, func(c *message.Update) error {
						result.updates = append(result.updates, &message.Update{
							WithdrawnRoutes: append([]byte(nil), c.WithdrawnRoutes...),
							PathAttributes:  append([]byte(nil), c.PathAttributes...),
							NLRI:            append([]byte(nil), c.NLRI...),
						})
						return nil
					})
				}()
				if splitErr != nil {
					fwdLogger().Warn("forward split failed", "peer", peerAddr, "err", splitErr)
					return result, false
				}
			} else {
				result.updates = append(result.updates, cache.update)
			}
		}
	}

	result.supersedeKey = fwdSupersedeKey(result.rawBodies)
	tmp := fwdItem{rawBodies: result.rawBodies, updates: result.updates}
	result.withdrawal = fwdIsWithdrawal(&tmp)
	return result, true
}
