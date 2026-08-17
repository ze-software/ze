// Design: docs/architecture/chaos-web-dashboard.md -- Chaos MCP tools for AI queries

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/chaos/watchdog"
	"github.com/ze-software/ze/internal/chaos/web"
)

var validControlActions = map[string]bool{
	"pause": true, "resume": true, "trigger": true, "rate": true, "stop": true,
}

var sortedControlActions = func() string {
	keys := make([]string, 0, len(validControlActions))
	for k := range validControlActions {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return textbuf.Join(keys, ", ")
}()

// ControlDispatcher sends control commands to the chaos scheduler.
type ControlDispatcher func(cmd web.ControlCommand) error

// CommandDispatcher executes a chaos orchestrator command and returns the
// typed response. It is an alias for the unified plugin.CommandDispatcher; the
// chaos MCP provider renders the JSON string at its edge via
// CommandDispatcher.JSON with a zero-value caller identity.
type CommandDispatcher = plugin.CommandDispatcher

// Provider implements mcp.ToolProvider for the chaos MCP server.
type Provider struct {
	State       *web.DashboardState
	Watchdog    *watchdog.Watchdog
	Convergence *validation.Convergence
	Control     ControlDispatcher
	Execute     CommandDispatcher
	Seed        uint64
	StartTime   time.Time
	PeerCount   int
}

func (p *Provider) ServerName() string { return "ze-chaos-mcp" }

func (p *Provider) Tools() []map[string]any {
	return chaosTools
}

func (p *Provider) CallTool(name string, args json.RawMessage) map[string]any {
	result := p.CallToolResult(name, args)
	if result == nil {
		return nil
	}
	return result.Value
}

func (p *Provider) CallToolResult(name string, args json.RawMessage) *zemcp.ToolResult {
	if name == "chaos_execute" {
		return p.toolExecute(args)
	}
	handler, ok := toolHandlers[name]
	if !ok {
		return nil
	}
	return &zemcp.ToolResult{Value: handler(p, args)}
}

var toolHandlers = map[string]func(p *Provider, args json.RawMessage) map[string]any{
	"chaos_status":   (*Provider).toolStatus,
	"chaos_problems": (*Provider).toolProblems,
	"chaos_peers":    (*Provider).toolPeers,
	"chaos_scenario": (*Provider).toolScenario,
	"chaos_control":  (*Provider).toolControl,
}

func (p *Provider) toolStatus(_ json.RawMessage) map[string]any {
	p.State.RLock()
	defer p.State.RUnlock()

	percs := web.ComputeConvergencePercentiles(p.State.ConvergenceTrend)
	hist := p.State.Convergence

	buckets := make([]map[string]any, len(hist.Buckets))
	for i, b := range &hist.Buckets {
		buckets[i] = map[string]any{
			"label": b.Label,
			"count": b.Count,
		}
	}

	convMap := map[string]any{
		"min":       web.FormatDuration(hist.Min),
		"avg":       web.FormatDuration(hist.Avg()),
		"max":       web.FormatDuration(hist.Max),
		"p50":       web.FormatDuration(percs.P50),
		"p90":       web.FormatDuration(percs.P90),
		"p99":       web.FormatDuration(percs.P99),
		"histogram": buckets,
	}

	if p.Convergence != nil {
		byFamily := p.Convergence.StatsByFamily()
		if len(byFamily) > 0 {
			famMap := make(map[string]any, len(byFamily))
			for fam, stats := range byFamily {
				famMap[fam] = map[string]any{
					"min": web.FormatDuration(stats.Min),
					"avg": web.FormatDuration(stats.Avg),
					"max": web.FormatDuration(stats.Max),
					"p99": web.FormatDuration(stats.P99),
				}
			}
			convMap["per-family"] = famMap
		}
	}

	properties := make([]map[string]any, len(p.State.Properties))
	for i, prop := range p.State.Properties {
		properties[i] = map[string]any{
			"name":            prop.Name,
			"status":          statusStr(prop.Pass),
			"violation-count": len(prop.Violations),
		}
	}

	result := map[string]any{
		"seed":             p.Seed,
		"elapsed":          time.Since(p.StartTime).Truncate(time.Millisecond).String(),
		"peers-total":      p.State.PeerCount,
		"peers-up":         p.State.PeersUp,
		"peers-syncing":    p.State.PeersSyncing,
		"routes-announced": p.State.TotalAnnounced,
		"routes-received":  p.State.TotalReceived,
		"routes-missing":   p.State.TotalMissing,
		"routes-withdrawn": p.State.TotalWithdrawn,
		"sync-duration":    p.State.SyncDuration.Truncate(time.Millisecond).String(),
		"convergence":      convMap,
		"chaos-events":     p.State.TotalChaos,
		"chaos-rate":       p.State.ChaosRate(),
		"throughput-in":    web.FormatBitRate(p.State.AggregateThroughput(false)),
		"throughput-out":   web.FormatBitRate(p.State.AggregateThroughput(true)),
		"dropped-events":   p.State.TotalDropped,
		"properties":       properties,
	}

	data, err := json.Marshal(result)
	if err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("encoding status: ").Err(err).String())
	}
	return zemcp.TextResult(string(data))
}

func (p *Provider) toolProblems(_ json.RawMessage) map[string]any {
	var problems []map[string]any

	if p.Watchdog != nil {
		for _, prob := range p.Watchdog.Problems() {
			problems = append(problems, map[string]any{
				"type":    prob.Type,
				"peer":    prob.PeerIndex,
				"message": prob.Message,
				"time":    prob.Time.Format(time.RFC3339),
			})
		}
	}

	p.State.RLock()
	defer p.State.RUnlock()

	for idx, ps := range p.State.Peers {
		if ps == nil {
			continue
		}
		sent := ps.RoutesSent
		recv := ps.RoutesRecv
		missing := ps.Missing
		if missing > 0 {
			problems = append(problems, map[string]any{
				"type":          "missing-routes",
				"peer":          idx,
				"expected":      sent,
				"actual":        recv,
				"missing-count": missing,
			})
		}
	}

	if problems == nil {
		problems = []map[string]any{}
	}

	data, err := json.Marshal(problems)
	if err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("encoding problems: ").Err(err).String())
	}
	return zemcp.TextResult(string(data))
}

func (p *Provider) toolPeers(args json.RawMessage) map[string]any {
	var input struct {
		Peer *int `json:"peer"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &input); err != nil {
			var tb textbuf.Buffer
			return zemcp.ErrResult(tb.Str("invalid arguments: ").Err(err).String())
		}
	}

	p.State.RLock()
	defer p.State.RUnlock()

	if input.Peer != nil {
		idx := *input.Peer
		ps, ok := p.State.Peers[idx]
		if !ok {
			var tb textbuf.Buffer
			return zemcp.ErrResult(tb.Str("peer must be a valid index: ").Int(int64(idx)).String())
		}
		result := peerDetail(ps, true)
		data, err := json.Marshal(result)
		if err != nil {
			var tb textbuf.Buffer
			return zemcp.ErrResult(tb.Str("encoding peer: ").Err(err).String())
		}
		return zemcp.TextResult(string(data))
	}

	peers := make([]map[string]any, 0, len(p.State.Peers))
	for i := range p.State.PeerCount {
		ps, ok := p.State.Peers[i]
		if !ok {
			continue
		}
		peers = append(peers, peerDetail(ps, false))
	}

	data, err := json.Marshal(peers)
	if err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("encoding peers: ").Err(err).String())
	}
	return zemcp.TextResult(string(data))
}

func peerDetail(ps *web.PeerState, detail bool) map[string]any {
	families := make([]map[string]any, 0, len(ps.Families))
	for _, fam := range ps.Families {
		families = append(families, map[string]any{
			"name":     fam,
			"sent":     ps.FamilySent[fam],
			"received": ps.FamilyRecv[fam],
			"target":   ps.FamilySentTarget[fam],
		})
	}

	m := map[string]any{
		"index":           ps.Index,
		"status":          ps.Status.String(),
		"routes-sent":     ps.RoutesSent,
		"routes-received": ps.RoutesRecv,
		"missing":         ps.Missing,
		"families":        families,
		"chaos-count":     ps.ChaosCount,
		"reconnects":      ps.Reconnects,
		"bytes-sent":      ps.BytesSent,
		"bytes-received":  ps.BytesRecv,
		"last-event":      ps.LastEvent.String(),
	}

	if !ps.LastEventAt.IsZero() {
		m["last-event-at"] = ps.LastEventAt.Format(time.RFC3339)
	}

	if detail {
		events := ps.Events.All()
		start := 0
		if len(events) > 5 {
			start = len(events) - 5
		}
		recent := make([]map[string]any, 0, 5)
		for i := start; i < len(events); i++ {
			recent = append(recent, map[string]any{
				"time":   events[i].Time.Format(time.RFC3339),
				"type":   events[i].Type.String(),
				"action": events[i].ChaosAction,
			})
		}
		m["recent-chaos"] = recent
	}

	return m
}

func (p *Provider) toolScenario(_ json.RawMessage) map[string]any {
	result := map[string]any{
		"seed":       p.Seed,
		"peer-count": p.PeerCount,
	}

	data, err := json.Marshal(result)
	if err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("encoding scenario: ").Err(err).String())
	}
	return zemcp.TextResult(string(data))
}

func (p *Provider) toolControl(args json.RawMessage) map[string]any {
	if p.Control == nil {
		return zemcp.ErrResult("control not available")
	}

	var input struct {
		Action      string   `json:"action"`
		Peer        *int     `json:"peer"`
		ChaosAction string   `json:"chaos-action"`
		Value       *float64 `json:"value"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("invalid arguments: ").Err(err).String())
	}

	if !validControlActions[input.Action] {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("action must be one of: ").Str(sortedControlActions).Str("; got ").Str(strconv.Quote(input.Action)).String())
	}

	cmd := web.ControlCommand{Type: input.Action}
	switch input.Action {
	case "rate":
		if input.Value == nil {
			return zemcp.ErrResult("rate action requires value (0.0-1.0)")
		}
		if *input.Value < 0 || *input.Value > 1 {
			return zemcp.ErrResult(fmt.Sprintf("rate must be 0.0-1.0, got %f", *input.Value))
		}
		cmd.Rate = *input.Value
	case "trigger":
		if input.ChaosAction == "" {
			return zemcp.ErrResult("trigger action requires chaos-action")
		}
		trigger := &web.ManualTrigger{ActionType: input.ChaosAction}
		if input.Peer != nil {
			trigger.Peers = []int{*input.Peer}
		}
		cmd.Trigger = trigger
	}

	if err := p.Control(cmd); err != nil {
		var tb textbuf.Buffer
		return zemcp.ErrResult(tb.Str("control: ").Err(err).String())
	}

	var tb textbuf.Buffer
	return zemcp.TextResult(tb.Str("ok: ").Str(input.Action).String())
}

func (p *Provider) toolExecute(args json.RawMessage) *zemcp.ToolResult {
	if p.Execute == nil {
		return &zemcp.ToolResult{Value: zemcp.ErrResult("execute not available")}
	}

	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		var tb textbuf.Buffer
		return &zemcp.ToolResult{Value: zemcp.ErrResult(tb.Str("invalid arguments: ").Err(err).String())}
	}
	if input.Command == "" {
		return &zemcp.ToolResult{Value: zemcp.ErrResult("missing required argument: command")}
	}

	result, err := p.Execute.JSON(context.Background(), plugin.CallerIdentity{}, input.Command)
	if err != nil {
		return &zemcp.ToolResult{Value: zemcp.ErrResult(err.Error()), Completion: result}
	}
	return &zemcp.ToolResult{Value: zemcp.TextResult(result.Output), Completion: result}
}

func statusStr(pass bool) string {
	if pass {
		return "pass"
	}
	return "fail"
}

var chaosTools = []map[string]any{
	{
		"name":        "chaos_status",
		"description": "Full chaos test status snapshot: peers, routes, convergence stats, throughput, properties.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "chaos_problems",
		"description": "Filtered list of actionable issues. Empty array means healthy.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "chaos_peers",
		"description": "Per-peer detail. Omit 'peer' for all peers summary, provide index for single peer with recent chaos history.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"peer": map[string]any{
					"type":        "integer",
					"description": "Peer index for detail view. Omit for all peers.",
				},
			},
		},
	},
	{
		"name":        "chaos_scenario",
		"description": "Static scenario metadata: seed, peer count.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "chaos_control",
		"description": "Control chaos scheduling. Use chaos_problems or chaos_status first to understand the situation before changing anything.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"pause", "resume", "trigger", "rate", "stop"},
					"description": "Control action to perform.",
				},
				"peer":         map[string]any{"type": "integer", "description": "Peer index for trigger action."},
				"chaos-action": map[string]any{"type": "string", "description": "Chaos action name for trigger (e.g. tcp-disconnect)."},
				"value":        map[string]any{"type": "number", "description": "New rate for rate action (0.0-1.0)."},
			},
			"required": []string{"action"},
		},
	},
	{
		"name":        "chaos_execute",
		"description": "Execute a chaos orchestrator command. Prefer the specific tools when possible.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command to execute.",
				},
			},
			"required": []string{"command"},
		},
	},
}
