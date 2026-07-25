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

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

// argCount is the CLI argument keyword that takes a positive integer
// limit on the number of returned entries. Centralized so goconst stops
// flagging the literal repetition across the show package.
const argCount = "count"

// Per-component health checks are registered by their OWNERS (l2tp, iface,
// bgp/plugin, fib/kernel, plugin/process, core/report), not here: deleting a
// component must remove its health row (ai/rules/plugin-self-containment.md).
// This package keeps only the generic show-health RPC below.

func init() {
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
			WireMethod: "ze-show:health",
			Handler:    handleShowHealth,
		},
	)
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
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: show metrics name <name> [label=value ...]"}, nil
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
