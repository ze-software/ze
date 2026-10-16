// Design: docs/architecture/pool-architecture.md -- pool-stats, the operator view of the attribute pools
// Related: register.go -- the ze-bgp:pool-stats RPC registration

package poolcmd

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// fieldCount is the row count pool-stats answers beside its pool rows. It is
// NOT the `| count` pipe operator.
const fieldCount = "count"

func handlePoolStats(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	pools := pool.AllNamedPools()
	rows := make([]map[string]any, 0, len(pools))
	var totalLive, totalDead int32
	var totalLiveBytes, totalDeadBytes int64
	var totalIntern, totalHits int64
	for _, named := range pools {
		m := named.Pool.Metrics()
		rows = append(rows, map[string]any{
			"name":       named.Name,
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
			fieldCount:         len(rows),
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
