// Design: docs/architecture/core-design.md -- BGP redistribution consumer

package redistribute

import (
	"context"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const bgpConsumerName = "bgp"
const originIncomplete = "incomplete"
const originIGP = "igp"
const updateRouteTimeout = 10 * time.Second

// RouteDispatcher abstracts the plugin SDK's UpdateRoute method.
type RouteDispatcher interface {
	UpdateRoute(ctx context.Context, peerSelector, command string) (uint32, uint32, error)
}

// BGPConsumer implements redistribute.RedistConsumer for the BGP protocol.
// It translates redistributed routes into text-form update-route commands
// dispatched to the reactor via UpdateRoute.
type BGPConsumer struct {
	dispatcher RouteDispatcher
}

// NewBGPConsumer creates a BGP redistribution consumer wrapping the given dispatcher.
func NewBGPConsumer(d RouteDispatcher) *BGPConsumer {
	return &BGPConsumer{dispatcher: d}
}

func (c *BGPConsumer) Name() string { return bgpConsumerName }

func (c *BGPConsumer) InjectRoute(ctx context.Context, fam family.Family, entry redistribute.RouteEntry) {
	cmd := formatAnnounce(fam.String(), entry.NextHop, entry.Prefix, entry.OriginASN, entry.Community)
	// Peer targets a single peer (replay-on-peer-up); empty is the normal
	// fan-out to all peers.
	sel := entry.Peer
	if sel == "" {
		sel = "*"
	}
	slog.Debug("bgp consumer: injecting route", "command", cmd, "peer", sel)
	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	added, withdrawn, err := c.dispatcher.UpdateRoute(cctx, sel, cmd)
	if err != nil {
		slog.Warn("bgp consumer: update-route failed", "error", err, "command", cmd)
	} else {
		slog.Debug("bgp consumer: update-route ok", "added", added, "withdrawn", withdrawn)
	}
}

func (c *BGPConsumer) WithdrawRoute(ctx context.Context, fam family.Family, prefix string) {
	cmd := formatWithdraw(fam.String(), prefix)
	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	if _, _, err := c.dispatcher.UpdateRoute(cctx, "*", cmd); err != nil {
		slog.Warn("bgp consumer: update-route failed", "error", err, "command", cmd)
	}
}

// formatAnnounce builds the `update text ... add <prefix>` command. When
// originASN is nonzero the route is announced as a locally-originated virtual-
// router route (`origin igp origin-as <originASN>`): the reactor applies the
// normal export rule, so the AS_PATH is [originASN] to iBGP peers and
// [localAS, originASN] to eBGP peers (unlike a verbatim `as-path`). Otherwise it
// keeps the legacy `origin incomplete` with no AS_PATH so existing producers are
// byte-for-byte unchanged. A non-empty community list adds `community [ ... ]`.
func formatAnnounce(fam, nextHop, prefix string, originASN uint32, community []uint32) string {
	// textbuf.Buffer (128B inline, no heap alloc for the common case) per
	// ai/rules/performance.md; grows automatically when a long community
	// list overruns the inline array, so no under-provisioned Grow hint.
	var b textbuf.Buffer
	b.Reset().Str("update text origin ")
	if originASN != 0 {
		b.Str(originIGP).Str(" origin-as ").Uint32(originASN)
	} else {
		b.Str(originIncomplete)
	}
	if len(community) > 0 {
		// Each uint32 renders as <asn>:<value> (high16:low16), which round-trips
		// through attribute.ParseCommunity for every value including well-known
		// ones (e.g. NO_EXPORT 0xFFFFFF01 -> 65535:65281).
		b.Str(" community [")
		for i, c := range community {
			if i > 0 {
				b.Byte(' ')
			}
			b.Uint(uint64(c >> 16)).Byte(':').Uint(uint64(c & 0xFFFF))
		}
		b.Byte(']')
	}
	b.Str(" nhop ")
	if nextHop != "" {
		b.Str(nextHop)
	} else {
		b.Str("self")
	}
	b.Str(" nlri ").Str(fam).Str(" add ").Str(prefix)
	return b.String()
}

func formatWithdraw(fam, prefix string) string {
	var b textbuf.Buffer
	b.Reset().Str("update text nlri ").Str(fam).Str(" del ").Str(prefix)
	return b.String()
}
