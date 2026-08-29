// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration
// Detail: system.go -- system/* handlers (memory, cpu, date)

package show

import (
	"errors"
	"fmt"
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
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The CLI argument keywords this package reads. argCount takes a positive
// integer limit on the number of returned entries.
const (
	argCount     = "count"
	argNamespace = "namespace"
)

// The response payload keys. They hold the same text as the argument keywords
// above and name something else: one is what an operator types, the other is
// what the JSON carries.
const (
	keyCount      = "count"
	keySubsystems = "subsystems"
	keyNamespace  = "namespace"
	keyType       = "type"
)

// The error messages this package returns more than once.
const (
	msgDaemonNotRunning     = "daemon not running"
	msgEventRingUnavailable = "event ring not available"
)

// Per-component health checks are registered by their OWNERS (l2tp, iface,
// bgp/plugin, fib/kernel, plugin/process, core/report), not here: deleting a
// component must remove its health row (ai/rules/plugins.md).
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
			WireMethod: "ze-show:event-delivery",
			Handler:    handleShowEventDelivery,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:health",
			Handler:    handleShowHealth,
		},
	)
}

// handleShowEventDelivery returns the peer-to-process delivery graph the running
// config produced: for every peer, the processes it attaches, what each one is
// fed, and what each one may send toward it.
//
// It reads the index delivery reads, not the config document, so an operator
// asking "why is my program not fed" sees the answer the daemon acts on. A
// granted token the event registry does not know appears under `unresolved`,
// which is the one way an edge can go missing.
func handleShowEventDelivery(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "delivery graph not available"}, nil
	}
	peers := ctx.Server.DeliveryGraph().Inspect()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"peers": peers, keyCount: len(peers)},
	}, nil
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
			keyCount:   len(issues),
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
			keyCount: len(issues),
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
		if a == argNamespace && i+1 < len(args) {
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

// The label filter grammar of `show metrics name`. One filter is three tokens,
// so the keyword that introduces the value comes from a closed set and the
// label name it selects on comes from the operator (ai/rules/cli.md).
const (
	argLabel = "label"
	// metricsQueryForm is the line an operator types, and the model generates
	// the same one from internal/component/cmd/show/yang/ze-cli-show-cmd.yang.
	// Every error below states it, so the reader sees what to type next.
	metricsQueryForm = "show metrics name <name> [label <key> <value> ...]"
	// labelGroupTokens is the keyword, the key, and the value.
	labelGroupTokens = 3
)

// errLabelIncomplete answers every label group that is not three tokens. The
// three ways to write one are the same mistake, so they read the same message.
var errLabelIncomplete = errors.New(argLabel + " needs a key and a value: " + metricsQueryForm)

// handleShowMetricsQuery answers `show metrics name <name> [label <key> <value> ...]`.
//
// The metric name arrives as a SELECTOR rather than in args: the container and
// its leaf are both called `name`, so matchCommandTokens
// (internal/component/plugin/server/command.go) matches the keyword against the
// leaf of the same name and lifts the value out of the argument list. What
// remains in args is label filters and nothing else.
func handleShowMetricsQuery(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	reg := registry.GetMetricsRegistry()
	if reg == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "metrics not available"}, nil
	}
	promReg, ok := reg.(*metrics.PrometheusRegistry)
	if !ok {
		return &plugin.Response{Status: plugin.StatusError, Error: "metrics not available"}, nil
	}
	metricName := ctx.Selector("name")
	if metricName == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "usage: " + metricsQueryForm}, nil
	}
	labelFilters, err := metricLabelFilters(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operator error in Response
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
		Data:   plugin.Map{"metric": metricName, "series": matched, keyCount: len(matched)},
	}, nil
}

// metricLabelFilters reads the label filters that follow the metric name. Each
// one is `label <key> <value>`, they repeat, and they are ANDed.
//
// Each answer is the `key="value"` text a matching sample line carries, built
// once here rather than once for each line. The answer is a LIST rather than a
// map keyed by label name, so a label named twice states two conditions and
// matches nothing. A map would keep the last one and drop the operator's
// earlier filter in silence, which is the failure this grammar removes.
//
// A group that stops short of its value, and a token that opens no group, are
// both refused. The retired `label=value` packing dropped every token it could
// not split on `=`, so `show metrics name ze_bgp_up peer` answered each series
// of the metric and told the operator nothing about the token it ignored.
//
// The loop is bounded by args: each pass consumes one whole group, so at
// strictly increases.
func metricLabelFilters(args []string) ([]string, error) {
	filters := make([]string, 0, len(args)/labelGroupTokens)
	var tb textbuf.Buffer
	for at := 0; at < len(args); at += labelGroupTokens {
		if !strings.EqualFold(args[at], argLabel) {
			return nil, fmt.Errorf("%q is not the %s keyword: %s", args[at], argLabel, metricsQueryForm)
		}
		if at+labelGroupTokens > len(args) {
			return nil, errLabelIncomplete
		}
		key := args[at+1]
		if key == "" {
			return nil, errLabelIncomplete
		}
		value := args[at+2]
		if value == "" {
			return nil, errLabelIncomplete
		}
		filters = append(filters, tb.Reset().Str(key).Str(`="`).Str(value).Byte('"').String())
	}
	return filters, nil
}

// filterMetricLines returns each sample line of the named metric that carries
// every label filter. labelFilters holds the `key="value"` text of each one.
func filterMetricLines(text, name string, labelFilters []string) []map[string]any {
	// The reader is a strings.Reader over this process's own Prometheus text,
	// so Read returns only io.EOF and every sample line is short. There is no
	// scan error to read back.
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
		if !hasEveryLabel(line, labelFilters) {
			continue
		}
		results = append(results, map[string]any{"line": line})
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results
}

// hasEveryLabel reports whether one sample line carries every label filter.
// An empty filter list selects every line, because the operator asked for none.
//
// The loop is bounded by the filter list, which the operator's own command line
// states.
func hasEveryLabel(line string, labelFilters []string) bool {
	for _, filter := range labelFilters {
		if !strings.Contains(line, filter) {
			return false
		}
	}
	return true
}

func handleShowEventRecent(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: msgEventRingUnavailable}, nil
	}
	ring := ctx.Server.EventRing()
	if ring == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: msgEventRingUnavailable}, nil
	}
	namespace := extractNamespaceFilter(args)
	limit := extractCountFilter(args)
	records := ring.Snapshot(limit, namespace)
	out := make([]map[string]any, 0, len(records))
	for i := range records {
		out = append(out, map[string]any{
			"timestamp":  records[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			keyNamespace: records[i].Namespace,
			"event-type": records[i].EventType,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"events": out, keyCount: len(out)},
	}, nil
}

func handleShowEventNamespaces(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: msgEventRingUnavailable}, nil
	}
	ring := ctx.Server.EventRing()
	if ring == nil {
		return &plugin.Response{Status: plugin.StatusError, Error: msgEventRingUnavailable}, nil
	}
	counts := ring.NamespaceCounts()
	rows := make([]map[string]any, 0, len(counts))
	for ns, count := range counts {
		rows = append(rows, map[string]any{
			keyNamespace: ns,
			keyCount:     count,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ni, _ := rows[i][keyNamespace].(string)
		nj, _ := rows[j][keyNamespace].(string)
		return ni < nj
	})
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"namespaces": rows, keyCount: len(rows)},
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
			keyCount:     len(components),
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
			Error:  msgDaemonNotRunning,
		}, nil
	}
	r := ctx.Reactor()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  msgDaemonNotRunning,
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
