// Design: docs/architecture/api/commands.md — BGP attribute-pool statistics handler
//
// pool-stats reports per-attribute-pool occupancy and deduplication rates for
// the BGP RIB attribute pools (internal/component/bgp/plugins/rib/pool). Unlike
// the other RIB commands in this package it is an in-process reader, not a proxy
// to the bgp-rib plugin process. It lives with the RIB command cluster because
// the attribute pools are a RIB subsystem; the central "metrics" verb keeps only
// the generic Prometheus-registry commands. The ze-bgp: WireMethod prefix is a
// legacy label, not an ownership claim.

package rib

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:pool-stats", Handler: handlePoolStats},
	)
}

var poolNames = [...]string{
	"origin", "as-path", "local-pref", "med", "next-hop",
	"communities", "large-communities", "ext-communities",
	"cluster-list", "originator-id", "atomic-aggregate",
	"aggregator", "other-attrs",
}

func handlePoolStats(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	pools := pool.AllPools()
	rows := make([]map[string]any, 0, len(pools))
	var totalLive, totalDead int32
	var totalLiveBytes, totalDeadBytes int64
	var totalIntern, totalHits int64
	for i, p := range pools {
		m := p.Metrics()
		name := "unknown"
		if i < len(poolNames) {
			name = poolNames[i]
		}
		rows = append(rows, map[string]any{
			"name":       name,
			"live-slots": m.LiveSlots,
			"dead-slots": m.DeadSlots,
			"live-bytes": m.LiveBytes,
			"dead-bytes": m.DeadBytes,
			"intern":     m.InternTotal,
			"hits":       m.InternHits,
			"dedup-rate": formatDedup(m),
		})
		totalLive += m.LiveSlots
		totalDead += m.DeadSlots
		totalLiveBytes += m.LiveBytes
		totalDeadBytes += m.DeadBytes
		totalIntern += m.InternTotal
		totalHits += m.InternHits
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"pools":            rows,
			"count":            len(rows),
			"total-live-slots": totalLive,
			"total-dead-slots": totalDead,
			"total-live-bytes": totalLiveBytes,
			"total-dead-bytes": totalDeadBytes,
			"total-intern":     totalIntern,
			"total-hits":       totalHits,
		},
	}, nil
}

func formatDedup(m attrpool.Metrics) string {
	rate := m.DeduplicationRate()
	return strconv.FormatFloat(rate*100, 'f', 1, 64) + "%"
}
