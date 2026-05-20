// Design: docs/features/interfaces.md — Interface rate CLI commands
// Related: show.go — handleShowInterface dispatch (rate branch)
// Related: register_netlink_monitor.go — streaming handler registration pattern

package show

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterStreamingHandler("monitor interface rate", streamInterfaceRate)
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-monitor:interface-rate",
			Handler:    handleMonitorInterfaceRate,
		},
	)
}

func handleShowInterfaceRate(args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		name := args[0]
		rate, ok := iface.GetRate(name)
		if !ok {
			return &plugin.Response{Status: plugin.StatusError, Data: "interface not found: " + name}, nil
		}
		data, err := json.Marshal(rate)
		if err != nil {
			return nil, err
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
	}

	result := sortedRates()
	if result == nil {
		return &plugin.Response{Status: plugin.StatusError, Data: "rate tracker not running"}, nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: string(data)}, nil
}

func sortedRates() []iface.InterfaceRate {
	rates := iface.ListRates()
	if rates == nil {
		return nil
	}
	result := make([]iface.InterfaceRate, 0, len(rates))
	for _, r := range rates {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func handleMonitorInterfaceRate(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	var filterName string
	if len(args) > 0 {
		filterName = args[0]
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"status": "monitor-configured",
			"filter": filterName,
		},
	}, nil
}

func streamInterfaceRate(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, args []string) error {
	var filterName string
	if len(args) > 0 {
		filterName = args[0]
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if filterName != "" {
				rate, ok := iface.GetRate(filterName)
				if !ok {
					continue
				}
				if err := enc.Encode(rate); err != nil {
					return err
				}
			} else {
				result := sortedRates()
				if result == nil {
					continue
				}
				if err := enc.Encode(result); err != nil {
					return err
				}
			}
		}
	}
}
