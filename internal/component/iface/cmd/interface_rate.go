// Design: docs/features/interfaces.md -- interface rate show + monitor handlers
// Related: show_interface.go -- the `show interface` family; this adds the rate branch
//
// Owned by the iface component: the rate handlers read per-interface rate
// counters through the iface backend (iface.GetRate / ListRates). Relocated
// from the central cmd/show package together with the rest of the interface
// surface. See ai/rules/plugin-self-containment.md.

package cmd

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
			return &plugin.Response{Status: plugin.StatusError, Error: "interface not found: " + name}, nil
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: rate}, nil
	}

	result := sortedRates()
	if result == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "rate tracker not running"}, nil
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Slice[iface.InterfaceRate](result)}, nil
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
		Data: plugin.Map{
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
