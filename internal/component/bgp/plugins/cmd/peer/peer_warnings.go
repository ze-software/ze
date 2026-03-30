// Design: docs/architecture/api/commands.md -- BGP prefix warnings query
// Overview: peer.go -- BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// prefixStalenessThreshold matches reactor.stalenessThreshold (180 days).
// Duplicated here to avoid an import cycle (plugins/cmd/peer cannot import reactor).
const prefixStalenessThreshold = 180 * 24 * time.Hour

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:warnings", Handler: HandleBgpWarnings},
	)
}

// HandleBgpWarnings returns all active prefix warnings across all peers.
// Two kinds: stale prefix data and prefix count exceeding warning threshold.
func HandleBgpWarnings(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	peers := ctx.Reactor().Peers()
	now := time.Now()

	var warnings []map[string]any
	for i := range peers {
		p := &peers[i]
		label := p.Address.String()
		if p.Name != "" {
			label = p.Name
		}

		if isPrefixStale(p.PrefixUpdated, now) {
			warnings = append(warnings, map[string]any{
				"peer":    label,
				"address": p.Address.String(),
				"as":      p.PeerAS,
				"type":    "stale-data",
				"message": fmt.Sprintf("prefix data updated %s (>6 months old)", p.PrefixUpdated),
			})
		}
		for _, family := range p.PrefixWarnings {
			warnings = append(warnings, map[string]any{
				"peer":    label,
				"address": p.Address.String(),
				"as":      p.PeerAS,
				"type":    "threshold-exceeded",
				"family":  family,
				"message": fmt.Sprintf("%s prefix count exceeds warning threshold", family),
			})
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"warnings": warnings,
			"count":    len(warnings),
		},
	}, nil
}

// isPrefixStale reports whether a prefix updated timestamp is older than 6 months.
// Returns false for empty timestamps (manually configured, no staleness tracking).
func isPrefixStale(updated string, now time.Time) bool {
	if updated == "" {
		return false
	}
	t, err := time.Parse(time.DateOnly, updated)
	if err != nil {
		return false
	}
	return now.Sub(t) > prefixStalenessThreshold
}
