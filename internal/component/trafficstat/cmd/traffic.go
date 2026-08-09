// Design: docs/architecture/traffic/traffic-usage-monitor.md -- show/monitor traffic CLI handlers

package cmd

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/portname"
	"github.com/ze-software/ze/internal/core/stats"
)

func init() {
	pluginserver.RegisterStreamingHandler("monitor traffic stat", streamTraffic)
	pluginserver.RegisterMonitorProvider(pluginserver.MonitorProvider{
		Prefix:   "monitor traffic stat",
		CreateFn: createTrafficMonitorSession,
	})
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:traffic-stat",
			Handler:    handleShowTraffic,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-monitor:traffic-stat",
			Handler:    handleMonitorTraffic,
		},
	)
}

func handleShowTraffic(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	svc := trafficstat.EnsureGlobal()
	if svc == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "trafficstat service not available"}, nil
	}

	id := svc.Attach()
	defer svc.Detach(id)

	snap := svc.Snapshot()

	var filterName string
	if len(args) > 0 {
		filterName = args[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   snapshotToMap(snap, filterName),
	}, nil
}

func handleMonitorTraffic(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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

func streamTraffic(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, args []string) error {
	svc := trafficstat.EnsureGlobal()
	if svc == nil {
		return nil
	}

	id := svc.Attach()
	defer svc.Detach(id)

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
			snap := svc.Snapshot()
			if snap == nil {
				continue
			}
			if err := enc.Encode(snapshotToMap(snap, filterName)); err != nil {
				return err
			}
		}
	}
}

func snapshotToMap(snap *trafficstat.Snapshot, filterName string) plugin.Map {
	m := plugin.Map{
		"at":       snap.At.Format(time.RFC3339),
		"severity": displaySeverity(snap.History),
		"degraded": snap.Degraded,
	}

	ifaces := make([]plugin.Map, 0, len(snap.Interfaces))
	for _, ie := range snap.Interfaces {
		if filterName != "" && ie.Name != filterName {
			continue
		}
		ifaces = append(ifaces, plugin.Map{
			"name":   ie.Name,
			"rx-bps": ie.RxBps,
			"tx-bps": ie.TxBps,
			"rx-pps": ie.RxPps,
			"tx-pps": ie.TxPps,
		})
	}
	m["interfaces"] = ifaces

	talkers := make([]plugin.Map, 0, len(snap.TopSourceIPs))
	for _, te := range snap.TopSourceIPs {
		talkers = append(talkers, plugin.Map{
			"address": te.Addr.String(),
			"bps":     te.Bps,
		})
	}
	m["top-source-ips"] = talkers

	ports := make([]plugin.Map, 0, len(snap.TopPorts))
	for _, pe := range snap.TopPorts {
		info := portname.Lookup(pe.Port, pe.Proto)
		entry := plugin.Map{
			"port":    pe.Port,
			"service": info.Name,
			"proto":   pe.Proto,
			"bps":     pe.Bps,
		}
		if info.Amplification != "" {
			entry["amplification"] = info.Amplification
		}
		ports = append(ports, entry)
	}
	m["top-ports"] = ports

	dests := make([]plugin.Map, 0, len(snap.TopDestIPs))
	for _, te := range snap.TopDestIPs {
		dests = append(dests, plugin.Map{
			"address": te.Addr.String(),
			"bps":     te.Bps,
		})
	}
	m["top-dest-ips"] = dests

	protos := make([]plugin.Map, 0, len(snap.Protocols))
	for _, pm := range snap.Protocols {
		protos = append(protos, plugin.Map{
			"proto":   pm.Proto,
			"name":    pm.Name,
			"bps":     pm.Bps,
			"percent": pm.Pct,
		})
	}
	m["protocol-mix"] = protos

	m["history"] = snap.History

	return m
}

// displaySeverity computes a display-only severity from the neutral Snapshot
// History facts. The trafficstat layer holds no verdict (that moved out of the
// aggregator); the monitor view reproduces the historical ">2x recent-average
// caution / >5x danger" thresholds so operators see no change. History's last
// element is the current total rate; its mean is the rolling average.
func displaySeverity(history []float64) string {
	sev := "normal"
	if len(history) < 5 {
		return sev
	}
	avg := stats.Mean(history)
	if avg <= 0 {
		return sev
	}
	switch ratio := history[len(history)-1] / avg; {
	case ratio > 5:
		sev = "danger"
	case ratio > 2:
		sev = "caution"
	}
	return sev
}
