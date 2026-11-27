// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Related: show.go -- FormatQoS / FormatQoSMap CLI formatting helpers
// Related: show_test.go -- formatting unit tests

package cmd

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/traffic"
)

const (
	keyInterface    = "interface"
	keyQdisc        = "qdisc"
	keyHandle       = "handle"
	keyRateBps      = "rate-bps"
	keyClassCount   = "class-count"
	keyDefaultClass = "default-class"
	keyStats        = "stats"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-traffic:show-traffic",
			Handler:    handleShowTraffic,
		},
	)
}

func handleShowTraffic(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	ifName := ""
	for _, a := range args {
		if a != "" && !strings.HasPrefix(a, "-") {
			ifName = a
			break
		}
	}

	if ifName != "" {
		qos, qErr := traffic.ReadQdiscs(ifName)
		if qErr != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: qErr.Error()}, nil //nolint:nilerr // operational error in Response
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   buildTrafficDetail(qos),
		}, nil
	}

	list, err := traffic.ReadAllQdiscs()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		rows = append(rows, buildTrafficSummaryRow(&list[i]))
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"interfaces": rows, "count": len(rows)},
	}, nil
}

func buildTrafficDetail(qos traffic.InterfaceQoS) plugin.Map {
	classes := make([]map[string]any, len(qos.Qdisc.Classes))
	for i := range qos.Qdisc.Classes {
		cls := &qos.Qdisc.Classes[i]
		entry := map[string]any{
			"name":     cls.Name,
			keyHandle:  cls.Handle,
			keyRateBps: cls.Rate,
			"ceil-bps": cls.Ceil,
			"priority": cls.Priority,
		}
		if cls.Stats != nil {
			entry[keyStats] = qdiscStatsMap(cls.Stats)
		}
		classes[i] = entry
	}

	data := plugin.Map{
		keyInterface:    qos.Interface,
		keyQdisc:        qos.Qdisc.KindName(),
		keyHandle:       qos.Qdisc.Handle,
		keyRateBps:      qos.Qdisc.RateBps,
		keyClassCount:   len(qos.Qdisc.Classes),
		keyDefaultClass: qos.Qdisc.DefaultClass,
		"classes":       classes,
	}
	if qos.Qdisc.Stats != nil {
		data[keyStats] = qdiscStatsMap(qos.Qdisc.Stats)
	}
	return data
}

func buildTrafficSummaryRow(qos *traffic.InterfaceQoS) map[string]any {
	filterCount := 0
	for j := range qos.Qdisc.Classes {
		filterCount += len(qos.Qdisc.Classes[j].Filters)
	}
	row := map[string]any{
		keyInterface:   qos.Interface,
		keyQdisc:       qos.Qdisc.KindName(),
		keyHandle:      qos.Qdisc.Handle,
		keyRateBps:     qos.Qdisc.RateBps,
		keyClassCount:  len(qos.Qdisc.Classes),
		"filter-count": filterCount,
	}
	if qos.Qdisc.Stats != nil {
		addQdiscStatsFields(row, qos.Qdisc.Stats)
	}
	return row
}

// addQdiscStatsFields writes the seven live qdisc/class statistic keys into
// dst. Caller MUST pass a non-nil s; nil stats are omitted so backends without
// Linux egress queue telemetry never report synthetic zeros.
func addQdiscStatsFields(dst map[string]any, s *traffic.QdiscStats) {
	dst["bytes"] = s.Bytes
	dst["packets"] = s.Packets
	dst["qlen"] = s.Qlen
	dst["backlog"] = s.Backlog
	dst["drops"] = s.Drops
	dst["overlimits"] = s.Overlimits
	dst["requeues"] = s.Requeues
}

func qdiscStatsMap(s *traffic.QdiscStats) map[string]any {
	m := make(map[string]any, 7)
	addQdiscStatsFields(m, s)
	return m
}
