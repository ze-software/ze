// Design: docs/architecture/api/commands.md -- show l2tp health handler
// Detail: observer.go -- LoginSummary struct and LoginSummaries method

package l2tp

import (
	"sort"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:l2tp-health", Handler: handleShowL2TPHealth},
	)
}

func handleShowL2TPHealth(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := LookupService()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "l2tp subsystem not running"}, nil
	}
	summaries := svc.LoginSummaries()
	if summaries == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "observer not enabled (CQM disabled)"}, nil
	}
	rows := make([]map[string]any, 0, len(summaries))
	for i := range summaries {
		s := &summaries[i]
		rows = append(rows, map[string]any{
			"login":        s.Login,
			"last-state":   s.LastState,
			"echo-count":   int(s.EchoCount),
			"avg-rtt-ms":   float64(s.AvgRTT.Microseconds()) / 1000.0,
			"bucket-count": s.BucketCount,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, _ := rows[i]["avg-rtt-ms"].(float64)
		rj, _ := rows[j]["avg-rtt-ms"].(float64)
		return ri > rj
	})
	degraded := 0
	for _, r := range rows {
		if st, _ := r["last-state"].(string); st != "established" {
			degraded++
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"logins": rows, "count": len(rows), "degraded": degraded},
	}, nil
}
