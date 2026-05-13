// Design: docs/architecture/core-design.md -- BGP redistribution consumer

package redistribute

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

const bgpConsumerName = "bgp"
const originIncomplete = "incomplete"
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
	cmd := formatAnnounce(fam.String(), entry.NextHop, entry.Prefix)
	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	if _, _, err := c.dispatcher.UpdateRoute(cctx, "*", cmd); err != nil {
		slog.Warn("bgp consumer: update-route failed", "error", err, "command", cmd)
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

func formatAnnounce(fam, nextHop, prefix string) string {
	var sb strings.Builder
	sb.Grow(80)
	sb.WriteString("update text origin ")
	sb.WriteString(originIncomplete)
	sb.WriteString(" nhop ")
	if nextHop != "" {
		sb.WriteString(nextHop)
	} else {
		sb.WriteString("self")
	}
	sb.WriteString(" nlri ")
	sb.WriteString(fam)
	sb.WriteString(" add ")
	sb.WriteString(prefix)
	return sb.String()
}

func formatWithdraw(fam, prefix string) string {
	var sb strings.Builder
	sb.Grow(64)
	sb.WriteString("update text nlri ")
	sb.WriteString(fam)
	sb.WriteString(" del ")
	sb.WriteString(prefix)
	return sb.String()
}
