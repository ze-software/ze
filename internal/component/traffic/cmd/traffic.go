// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Related: show.go -- FormatQoS / FormatQoSMap CLI formatting helpers
// Related: show_test.go -- formatting unit tests

package cmd

import (
	"strings"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/traffic"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:traffic",
			Handler:    handleShowTraffic,
		},
	)
}

func handleShowTraffic(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	backend := traffic.GetBackend()
	if backend == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "traffic control not available on this platform",
		}, nil
	}
	ifaces, err := iface.ListInterfaces()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	ifName := ""
	for _, a := range args {
		if a != "" && !strings.HasPrefix(a, "-") {
			ifName = a
			break
		}
	}
	if ifName != "" {
		qos, qErr := backend.ListQdiscs(ifName)
		if qErr != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: qErr.Error()}, nil //nolint:nilerr // operational error in Response
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"interface":     qos.Interface,
				"qdisc":         qos.Qdisc.Type.String(),
				"class-count":   len(qos.Qdisc.Classes),
				"default-class": qos.Qdisc.DefaultClass,
			},
		}, nil
	}
	rows := make([]map[string]any, 0, len(ifaces))
	for i := range ifaces {
		qos, qErr := backend.ListQdiscs(ifaces[i].Name)
		if qErr != nil {
			rows = append(rows, map[string]any{
				"interface": ifaces[i].Name,
				"error":     qErr.Error(),
			})
			continue
		}
		filterCount := 0
		for j := range qos.Qdisc.Classes {
			filterCount += len(qos.Qdisc.Classes[j].Filters)
		}
		rows = append(rows, map[string]any{
			"interface":    qos.Interface,
			"qdisc":        qos.Qdisc.Type.String(),
			"class-count":  len(qos.Qdisc.Classes),
			"filter-count": filterCount,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"interfaces": rows, "count": len(rows)},
	}, nil
}
