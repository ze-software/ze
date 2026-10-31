// Design: docs/architecture/core-design.md -- BGP redistribution consumer

package redistribute

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const bgpConsumerName = "bgp"
const originIncomplete = "incomplete"
const originIGP = "igp"
const updateRouteTimeout = 10 * time.Second

// logger is the redistribution subsystem's logger. The bare default slog
// printed nothing under any ze.log key, so the consumer's lines were lost.
var logger = slogutil.LazyLogger("bgp.redistribute")

// BGPConsumer implements redistribute.RedistConsumer for the BGP protocol.
// It translates redistributed routes into text-form update-route commands
// dispatched to the reactor via UpdateRoute, over the redistribute
// orchestrator's engine connection (register.go, the consumer factory).
type BGPConsumer struct {
	dispatcher redistribute.RouteUpdater
}

// NewBGPConsumer creates a BGP redistribution consumer wrapping the given dispatcher.
func NewBGPConsumer(d redistribute.RouteUpdater) *BGPConsumer {
	return &BGPConsumer{dispatcher: d}
}

func (c *BGPConsumer) Name() string { return bgpConsumerName }

func (c *BGPConsumer) InjectRoute(ctx context.Context, fam family.Family, entry redistribute.RouteEntry) {
	cmd := formatAnnounce(fam.String(), &entry)
	// Peer targets a single peer (replay-on-peer-up); empty is the normal
	// fan-out to all peers.
	sel := entry.Peer
	if sel == "" {
		sel = "*"
	}
	logger().Debug("bgp consumer: injecting route", "command", cmd, "peer", sel)
	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	added, withdrawn, err := c.dispatcher.UpdateRoute(cctx, sel, cmd)
	if err != nil {
		logger().Warn("bgp consumer: update-route failed", "error", err, "command", cmd)
		return
	}
	// The reactor answers an announce no peer accepted with a warning that
	// reaches here as added=0 and no error. That is not an announce: say so,
	// or the redistributed route is missing with nothing in the log.
	if added == 0 {
		logger().Warn("bgp consumer: redistributed route reached no BGP peer, so it is not "+
			"announced; check the peer is established with `ze show bgp peer`",
			"peer", sel, "prefix", entry.Prefix, "family", fam.String())
		return
	}
	logger().Debug("bgp consumer: update-route ok", "added", added, "withdrawn", withdrawn)
}

func (c *BGPConsumer) WithdrawRoute(ctx context.Context, fam family.Family, prefix string) {
	cmd := formatWithdraw(fam.String(), prefix)
	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	if _, _, err := c.dispatcher.UpdateRoute(cctx, "*", cmd); err != nil {
		logger().Warn("bgp consumer: update-route failed", "error", err, "command", cmd)
	}
}

// formatAnnounce builds the `update text ... add <prefix>` command. When
// originASN is nonzero the route is announced as a locally-originated virtual-
// router route (`origin igp origin-as <originASN>`): the reactor applies the
// normal export rule, so the AS_PATH is [originASN] to iBGP peers and
// [localAS, originASN] to eBGP peers (unlike a verbatim `as-path`). Otherwise it
// keeps the legacy `origin incomplete` with no AS_PATH so existing producers are
// byte-for-byte unchanged. A non-empty community list adds `community [ ... ]`,
// and a metric the redistribute chain set adds `med <metric>`.
func formatAnnounce(fam string, entry *redistribute.RouteEntry) string {
	nextHop, prefix, originASN, community := entry.NextHop, entry.Prefix, entry.OriginASN, entry.Community
	// textbuf.Buffer (128B inline, no heap alloc for the common case) per
	// ai/rules/performance.md; grows automatically when a long community
	// list overruns the inline array, so no under-provisioned Grow hint.
	var b textbuf.Buffer
	b.Reset().Str("update text origin ")
	if originASN == 0 {
		b.Str(originIncomplete)
	} else {
		b.Str(originIGP).Str(" origin-as ").Uint32(originASN)
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
	// RFC 4271 is silent on how a redistributed route's MULTI_EXIT_DISC is
	// chosen, so Ze follows FRR. bgp_redistribute_add (bgpd/bgp_route.c) sets
	// the route's MED from the redistributed metric and lets the redistribute
	// metric replace it:
	//	bgp_attr_set_med(&attr, metric);
	//	...
	//	if (red->redist_metric_flag)
	//		attr_new.med = red->redist_metric;
	// Ze sends the metric only when a filter set it, the way BIRD sends a MED
	// only when an export filter sets bgp_med: a producer's own metric (an IGP
	// cost, a static route's zero) is not a figure a BGP neighbour can compare.
	if entry.HasMetric {
		b.Str(" med ").Uint32(entry.Metric)
	}
	b.Str(" nhop ")
	if nextHop == "" {
		b.Str("self")
	} else {
		b.Str(nextHop)
	}
	// RFC 4271 Section 4.3, RFC 4760 Section 2: the section keyword names the
	// UPDATE section the route travels in.
	if textparse.LegacyNLRISection(fam, textparse.IPv6NextHopText(nextHop)) {
		b.Str(" nlri")
	} else {
		b.Str(" mp ").Str(fam)
	}
	b.Str(" add ").Str(prefix)
	return b.String()
}

func formatWithdraw(fam, prefix string) string {
	var b textbuf.Buffer
	// RFC 4271 Section 4.3: IPv4 unicast is withdrawn in the Withdrawn Routes field.
	b.Reset().Str("update text")
	if textparse.LegacyNLRISection(fam, false) {
		b.Str(" nlri")
	} else {
		b.Str(" mp ").Str(fam)
	}
	b.Str(" del ").Str(prefix)
	return b.String()
}
