// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration
// Detail: system.go -- system/* handlers (memory, cpu, date)

package show

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"bufio"
	"context"
	"net/http"
	"net/http/httptest"

	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

// argCount is the CLI argument keyword that takes a positive integer
// limit on the number of returned entries. Centralized so goconst stops
// flagging the literal repetition across the show package.
const (
	argCount               = "count"
	argTimeout             = "timeout"
	msgSubsystemNotRunning = "subsystem not running"
)

func init() {
	health.Register("l2tp", func() (health.Status, string) {
		if l2tp.LookupService() == nil {
			return health.StatusDegraded, msgSubsystemNotRunning
		}
		return health.StatusHealthy, ""
	})
	health.Register("report-bus", func() (health.Status, string) {
		return health.StatusHealthy, ""
	})

	health.Register("bgp", checkBGPHealth)
	health.Register("fib", checkFIBHealth)
	health.Register("iface", checkIfaceHealth)
	health.Register("plugins", checkPluginHealth)

	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:version",
			Handler:    handleShowVersion,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:uptime",
			Handler:    handleShowUptime,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:warnings",
			Handler:    handleShowWarnings,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:aaa-accounting",
			Handler:    handleShowAAAAccounting,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:audit",
			Handler:    handleShowAudit,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:errors",
			Handler:    handleShowErrors,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-memory",
			Handler:    handleShowSystemMemory,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-cpu",
			Handler:    handleShowSystemCPU,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-date",
			Handler:    handleShowSystemDate,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-subsystem-list",
			Handler:    handleShowSystemSubsystemList,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-platform",
			Handler:    handleShowSystemPlatform,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-conntrack",
			Handler:    handleShowSystemConntrack,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-update",
			Handler:    handleShowSystemUpdate,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-update-history",
			Handler:    handleShowSystemUpdateHistory,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:traffic",
			Handler:    handleShowTraffic,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:l2tp-health",
			Handler:    handleShowL2TPHealth,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:bgp-health",
			Handler:    handleShowBGPHealth,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:metrics-query",
			Handler:    handleShowMetricsQuery,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:event-recent",
			Handler:    handleShowEventRecent,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:event-namespaces",
			Handler:    handleShowEventNamespaces,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:capture",
			Handler:    handleShowCapture,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:health",
			Handler:    handleShowHealth,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:doctor",
			Handler:    handleShowDoctor,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:gnmi",
			Handler:    handleShowGNMI,
		},
	)
	// ze-show:host-* RPCs are registered from host.go's own init()
	// via a loop over host.SectionNames(). See rules/derive-not-hardcode.md.
}

// handleShowWarnings returns the snapshot of all active warnings on the report bus.
// Optional args: "source <name>" filters by source.
func handleShowWarnings(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	issues := report.Warnings()
	if source := extractSourceFilter(args); source != "" {
		issues = filterIssuesBySource(issues, source)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"warnings": issues,
			"count":    len(issues),
		},
	}, nil
}

// handleShowErrors returns the most-recent error events on the report bus,
// newest first. Optional args: "source <name>" filters by source,
// "count <N>" limits results.
func handleShowErrors(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	issues := report.Errors(0)
	if source := extractSourceFilter(args); source != "" {
		issues = filterIssuesBySource(issues, source)
	}
	if limit := extractCountFilter(args); limit > 0 && limit < len(issues) {
		issues = issues[:limit]
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"errors": issues,
			"count":  len(issues),
		},
	}, nil
}

func extractSourceFilter(args []string) string {
	for i, a := range args {
		if a == "source" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func extractNamespaceFilter(args []string) string {
	for i, a := range args {
		if a == "namespace" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func extractCountFilter(args []string) int {
	for i, a := range args {
		if a == argCount && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

func filterIssuesBySource(issues []report.Issue, source string) []report.Issue {
	filtered := make([]report.Issue, 0, len(issues))
	for i := range issues {
		if strings.EqualFold(issues[i].Source, source) {
			filtered = append(filtered, issues[i])
		}
	}
	return filtered
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

func handleShowL2TPHealth(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := l2tp.LookupService()
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

func handleShowBGPHealth(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "reactor not available"}, nil
	}
	peers := ctx.Reactor().Peers()
	rows := make([]map[string]any, 0, len(peers))
	notEstablished := 0
	for i := range peers {
		p := &peers[i]
		state := p.State.String()
		if p.State != plugin.PeerStateEstablished {
			notEstablished++
		}
		row := map[string]any{
			"peer":   p.Address.String(),
			"state":  state,
			"as":     p.PeerAS,
			"uptime": p.Uptime.Truncate(time.Second).String(),
		}
		rows = append(rows, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"peers": rows, "count": len(rows), "not-established": notEstablished},
	}, nil
}

func handleShowMetricsQuery(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	reg := registry.GetMetricsRegistry()
	if reg == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "metrics not available"}, nil
	}
	promReg, ok := reg.(*metrics.PrometheusRegistry)
	if !ok {
		return &plugin.Response{Status: plugin.StatusError, Error: "metrics not available"}, nil
	}
	metricName := ""
	labelFilters := make(map[string]string)
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		if metricName == "" {
			metricName = a
			continue
		}
		if parts := strings.SplitN(a, "=", 2); len(parts) == 2 {
			labelFilters[parts[0]] = parts[1]
		}
	}
	if metricName == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: metrics-query <name> [label=value ...]"}, nil
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	rec := httptest.NewRecorder()
	promReg.Handler().ServeHTTP(rec, req)
	text := rec.Body.String()

	matched := filterMetricLines(text, metricName, labelFilters)
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"metric": metricName, "series": matched, "count": len(matched)},
	}, nil
}

func filterMetricLines(text, name string, labelFilters map[string]string) []map[string]any {
	var results []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := line[len(name):]
		if rest != "" && rest[0] != '{' && rest[0] != ' ' {
			continue
		}
		if len(labelFilters) > 0 {
			match := true
			for k, v := range labelFilters {
				want := k + `="` + v + `"`
				if !strings.Contains(line, want) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		results = append(results, map[string]any{"line": line})
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results
}

func handleShowEventRecent(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "event ring not available"}, nil
	}
	ring := ctx.Server.EventRing()
	if ring == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "event ring not available"}, nil
	}
	namespace := extractNamespaceFilter(args)
	limit := extractCountFilter(args)
	records := ring.Snapshot(limit, namespace)
	out := make([]map[string]any, 0, len(records))
	for i := range records {
		out = append(out, map[string]any{
			"timestamp":  records[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			"namespace":  records[i].Namespace,
			"event-type": records[i].EventType,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"events": out, "count": len(out)},
	}, nil
}

func handleShowEventNamespaces(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "event ring not available"}, nil
	}
	ring := ctx.Server.EventRing()
	if ring == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "event ring not available"}, nil
	}
	counts := ring.NamespaceCounts()
	rows := make([]map[string]any, 0, len(counts))
	for ns, count := range counts {
		rows = append(rows, map[string]any{
			"namespace": ns,
			"count":     count,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ni, _ := rows[i]["namespace"].(string)
		nj, _ := rows[j]["namespace"].(string)
		return ni < nj
	})
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"namespaces": rows, "count": len(rows)},
	}, nil
}

const (
	capL2TP = "l2tp"
	capBGP  = "bgp"
	capBFD  = "bfd"
)

func handleShowHealth(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	report := health.Check()
	components := make([]map[string]any, 0, len(report.Components))
	for i := range report.Components {
		c := &report.Components[i]
		m := map[string]any{
			"name":   c.Name,
			"status": string(c.Status),
		}
		if c.Reason != "" {
			m["reason"] = c.Reason
		}
		components = append(components, m)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"status":     string(report.Status),
			"components": components,
			"count":      len(components),
			"checked-at": report.CheckedAt,
		},
	}, nil
}

func handleShowCapture(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	protocol := ""
	tunnelIDFilter := uint16(0)
	peerFilter := ""
	limit := 0
	for i, a := range args {
		switch a {
		case capL2TP, capBGP:
			protocol = a
		case "tunnel-id":
			if i+1 < len(args) {
				if n, err := strconv.ParseUint(args[i+1], 10, 16); err == nil {
					tunnelIDFilter = uint16(n)
				}
			}
		case "peer":
			if i+1 < len(args) {
				peerFilter = args[i+1]
			}
		case argCount:
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					limit = n
				}
			}
		}
	}

	result := map[string]any{}

	if protocol == "" || protocol == capL2TP {
		svc := l2tp.LookupService()
		if svc != nil {
			entries := svc.CaptureSnapshot(limit, tunnelIDFilter, peerFilter)
			if entries != nil {
				result["l2tp"] = entries
				result["l2tp-count"] = len(entries)
			} else {
				result["l2tp"] = "capture not enabled"
			}
		} else if protocol == capL2TP {
			result["l2tp"] = msgSubsystemNotRunning
		}
	}

	if protocol == "" || protocol == capBGP {
		if ctx != nil && ctx.Reactor() != nil {
			if cp, ok := ctx.Reactor().(plugin.BGPCaptureProvider); ok {
				entries := cp.BGPCaptureSnapshot(limit, peerFilter)
				if entries != nil {
					result["bgp"] = entries
					result["bgp-count"] = len(entries)
				} else {
					result["bgp"] = "capture not enabled"
				}
			}
		} else if protocol == capBGP {
			result["bgp"] = "reactor not available"
		}
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}

// handleShowVersion returns the ze version and build date.
func handleShowVersion(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	v, d := pluginserver.GetVersion()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"version": "ze " + v + " (built " + d + ")"},
	}, nil
}

// handleShowUptime returns daemon start time and uptime duration.
func handleShowUptime(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "daemon not running",
		}, nil
	}
	r := ctx.Reactor()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "daemon not running",
		}, nil
	}
	stats := r.Stats()
	data := map[string]any{
		"start-time": stats.StartTime.Format(time.RFC3339),
		"uptime":     stats.Uptime.Truncate(time.Second).String(),
	}
	if hw, err := host.DetectHost(); err == nil && hw != nil {
		data["hardware"] = hw
	} else if err != nil && !errors.Is(err, host.ErrUnsupported) {
		data["hardware-error"] = err.Error()
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
