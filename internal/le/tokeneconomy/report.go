// Design: ai/rules/context-economy.md -- what `le token-economy` answers
// Overview: aggregate.go -- the figures this renders
//
// report.go contains the command's ANSWERS, separate from their producers.
//
// The payload is an object because the report contains eight tables for one
// corpus. Store path, project slug, and filter identify that corpus. A key
// identifies each table, so `| json` consumers select rows directly.
//
// Text matches the script page line for line. Developers see no migration
// change, and parity checks can compare bytes.

package tokeneconomy

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// State says what the run found, so a reader of the payload can tell an empty
// answer from an answer about nothing.
//
// The zero value is NOT a state. Run sets one on every path it can return
// through, so a Report that reached a caller carries one of the four below.
type State string

const (
	// StateOK means the store was read and the report describes it.
	StateOK State = "ok"
	// StateAbsent means there is no store at the path, which is the ordinary
	// state of a fresh checkout: transcripts are machine-local.
	StateAbsent State = "absent"
	// StateEmpty means the store exists and holds no transcript with a
	// recorded API call.
	StateEmpty State = "empty"
	// StateUnmatched means the session filter named a prefix no session has.
	StateUnmatched State = "unmatched"
)

// SessionRow is one row of the per-session table.
type SessionRow struct {
	SID         string  `json:"sid"`
	Calls       int     `json:"calls"`
	Main        int     `json:"main"`
	Sub         int     `json:"sub"`
	Agents      int     `json:"agents"`
	MeanContext float64 `json:"mean-context"`
	MaxContext  int64   `json:"max-context"`
	CacheRead   int64   `json:"cache-read"`
	CacheWrite  int64   `json:"cache-write"`
	Output      int64   `json:"output"`
}

// PhaseRow is one subagent-phase row. Phase uses a keyword heuristic on the
// spawn description because the store does not record agent phases.
type PhaseRow struct {
	Phase        string  `json:"phase"`
	Agents       int     `json:"agents"`
	Calls        int     `json:"calls"`
	CallsPerAgnt float64 `json:"calls-per-agent"`
	Context      int64   `json:"context"`
	Share        float64 `json:"share"`
	MeanContext  float64 `json:"mean-context"`
}

// Report is the whole answer of one run.
type Report struct {
	State   State  `json:"state"`
	Project string `json:"project"`
	Store   string `json:"store"`
	// Session is the filter prefix. Matched counts sessions with that prefix.
	Session string `json:"session,omitempty"`
	// Unreadable names each transcript that failed to OPEN. The script swallows
	// this error and silently reduces all report figures. A lower result would
	// then appear to pass.
	Unreadable []string `json:"unreadable,omitempty"`

	Cap int `json:"cap"`
	Top int `json:"top"`

	Sessions  int          `json:"sessions"`
	Subagents int          `json:"subagents"`
	Listed    []SessionRow `json:"listed,omitempty"`

	Totals        Totals  `json:"totals"`
	MainCalls     int     `json:"main-calls"`
	SubagentCalls int     `json:"subagent-calls"`
	SubagentShare float64 `json:"subagent-share"`
	// SubagentContext is the context every subagent fed. The phase and
	// agent-type shares are taken against it rather than against the grand
	// total, so a main-thread figure never dilutes a subagent one.
	SubagentContext int64 `json:"subagent-context"`
	// ShowSubagentShare records whether the share line is printed at all. A
	// transcript can record calls whose every usage field is zero, and a share
	// of nothing is not 0%.
	ShowSubagentShare bool `json:"show-subagent-share"`

	Histogram     []Bucket `json:"histogram,omitempty"`
	MainHistogram []Bucket `json:"main-histogram,omitempty"`
	SubHistogram  []Bucket `json:"sub-histogram,omitempty"`

	Capped Capped `json:"capped"`

	Phases     []PhaseRow     `json:"phases,omitempty"`
	AgentTypes []AgentTypeRow `json:"agent-types,omitempty"`

	ToolsPerCall  []ToolsPerCallRow `json:"tools-per-call,omitempty"`
	SingleTool    SingleTool        `json:"single-tool"`
	Results       []ResultRow       `json:"results,omitempty"`
	ThreadReads   Repeats           `json:"thread-reads"`
	SessionReads  Repeats           `json:"session-reads"`
	MainToolCalls int               `json:"main-tool-calls"`
	MainBash      int               `json:"main-bash"`
}

// sidWidth is how much of a session id the per-session table prints. Eight
// characters name a session unambiguously in a store this size, and the id is
// what a caller passes back as the session filter.
const sidWidth = 8

// row renders one table row in the shape the script's `_row` produced.
func row(cells ...string) string {
	var tb textbuf.Buffer
	return tb.Str("| ").Join(cells, " | ").Str(" |").String()
}

// dashes renders the separator under a header of the given width.
func dashes(count int) string {
	cells := make([]string, count)
	for i := range cells {
		cells[i] = "---"
	}
	return row(cells...)
}

// Text renders the whole report for a person, ending in a newline.
//
// This is the Prose rendering leroot uses for the bare command, and every pipe
// operator bypasses it.
func (r *Report) Text() string {
	var tb textbuf.Buffer
	for _, line := range r.Lines() {
		tb.Str(line).Byte('\n')
	}
	return tb.String()
}

// Lines answers report lines so tests do not need to capture stdout.
func (r *Report) Lines() []string {
	var head textbuf.Buffer
	out := []string{head.Str("Token economy: ").Str(r.Project).String()}

	switch r.State {
	case StateOK:
		// Render the report below. This explicit case prevents a later state from
		// falling through as if the tool had measured it.
	case StateAbsent:
		var looked textbuf.Buffer
		return append(out,
			looked.Str("  Looked for:  ").Str(r.Store).String(),
			"  Found no transcript store there, so there is nothing to measure.",
			"  Transcripts are machine-local; a fresh checkout has none.")
	case StateUnmatched:
		var store, none textbuf.Buffer
		return append(out,
			store.Str("  Store: ").Str(r.Store).String(),
			none.Str("  No session id starts with ").Str(pyRepr(r.Session)).Byte('.').String())
	case StateEmpty:
		var looked textbuf.Buffer
		return append(out,
			looked.Str("  Looked for:  ").Str(r.Store).String(),
			"  The directory holds no transcript with a recorded API call, so",
			"  there is nothing to measure.")
	}

	var store textbuf.Buffer
	out = append(out, store.Str("  Store: ").Str(r.Store).String())
	if r.Session != "" {
		var filter textbuf.Buffer
		out = append(out, filter.Str("  Session: ").Str(r.Session).Str(" (").
			Int(int64(r.Sessions)).Str(" matched)").String())
	}
	out = append(out, "")
	return append(out, r.body()...)
}

// body renders the eight tables, which is what the script's `render` produced.
func (r *Report) body() []string {
	out := r.sessionTable()
	out = append(out, "")
	out = append(out, r.totalsBlock()...)
	out = append(out, "")
	out = append(out, r.histogramBlock()...)
	out = append(out, "")
	out = append(out, r.splitHistogramBlock()...)
	out = append(out, "")
	out = append(out, r.cappedBlock()...)
	if r.Subagents > 0 {
		out = append(out, "")
		out = append(out, r.phaseBlock()...)
		out = append(out, "")
		out = append(out, r.agentTypeBlock()...)
	}
	return append(out, r.toolBlock()...)
}

func (r *Report) sessionTable() []string {
	var head textbuf.Buffer
	out := make([]string, 0, len(r.Listed)+3)
	out = append(out,
		head.Str("Per session, by context tokens (top ").Int(int64(len(r.Listed))).
			Str(" of ").Int(int64(r.Sessions)).Byte(')').String(),
		row("session", "calls", "main", "sub", "agents", "mean ctx", "max ctx",
			"cache-read", "cache-write", "output"),
		dashes(10))
	for _, line := range r.Listed {
		out = append(out, row(
			runePrefix(line.SID, sidWidth),
			comma(int64(line.Calls)),
			comma(int64(line.Main)),
			comma(int64(line.Sub)),
			comma(int64(line.Agents)),
			Short(line.MeanContext),
			Short(float64(line.MaxContext)),
			Short(float64(line.CacheRead)),
			Short(float64(line.CacheWrite)),
			Short(float64(line.Output)),
		))
	}
	return out
}

func (r *Report) totalsBlock() []string {
	var calls, counts, fed textbuf.Buffer
	out := []string{
		"Totals",
		calls.Str("  API calls: ").Str(comma(int64(r.Totals.Calls))).
			Str("  (main ").Str(comma(int64(r.MainCalls))).
			Str(", subagent ").Str(comma(int64(r.SubagentCalls))).Byte(')').String(),
		counts.Str("  Sessions: ").Str(comma(int64(r.Sessions))).
			Str("   Subagents: ").Str(comma(int64(r.Subagents))).String(),
		fed.Str("  Context fed: ").Str(Short(float64(r.Totals.Context))).
			Str("   mean ").Str(Short(r.Totals.ContextMean)).Str(" per call").
			Str("   max ").Str(Short(float64(r.Totals.ContextMax))).String(),
	}

	feed := r.Totals.CacheRead + r.Totals.CacheWrite + r.Totals.Input
	named := []struct {
		name  string
		value int64
		share bool
	}{
		{"cache-read", r.Totals.CacheRead, true},
		{"cache-write", r.Totals.CacheWrite, true},
		{"input", r.Totals.Input, true},
		{"output", r.Totals.Output, false},
	}
	for _, item := range named {
		var line textbuf.Buffer
		line.Str("  ").PadRight(item.name, 12).PadLeft(Short(float64(item.value)), 8)
		if item.share && feed != 0 {
			line.Str("  (").Str(fixed(100.0*float64(item.value)/float64(feed), 0)).
				Str("% of context fed)")
		}
		out = append(out, line.String())
	}

	if r.ShowSubagentShare {
		var share textbuf.Buffer
		out = append(out, share.Str("  Subagent share of context fed: ").
			Str(fixed(r.SubagentShare, 0)).Byte('%').String())
	}
	return out
}

func (r *Report) histogramBlock() []string {
	out := []string{
		"Context histogram: where the context tokens were fed",
		row("context at the call", "calls", "context tokens", "share"),
		dashes(4),
	}
	for _, bucket := range r.Histogram {
		if bucket.Calls == 0 {
			continue
		}
		var share textbuf.Buffer
		out = append(out, row(bucket.Label, comma(int64(bucket.Calls)),
			Short(float64(bucket.Context)), share.Str(fixed(bucket.Share, 1)).Byte('%').String()))
	}
	return out
}

func (r *Report) splitHistogramBlock() []string {
	out := []string{
		"Context histogram, main thread against subagents",
		row("context at the call", "main calls", "main context", "main share",
			"sub calls", "sub context", "sub share"),
		dashes(7),
	}
	for i := range r.MainHistogram {
		main, sub := r.MainHistogram[i], r.SubHistogram[i]
		if main.Calls == 0 && sub.Calls == 0 {
			continue
		}
		var mainShare, subShare textbuf.Buffer
		out = append(out, row(main.Label,
			comma(int64(main.Calls)), Short(float64(main.Context)),
			mainShare.Str(fixed(main.Share, 1)).Byte('%').String(),
			comma(int64(sub.Calls)), Short(float64(sub.Context)),
			subShare.Str(fixed(sub.Share, 1)).Byte('%').String()))
	}
	return append(out,
		"  Each share is of that column's own context total, so a main-thread",
		"  figure is never diluted by the subagent calls beside it.")
}

func (r *Report) cappedBlock() []string {
	var head, real, capped textbuf.Buffer
	return []string{
		head.Str("Capped-context counterfactual (cap ").Str(comma(int64(r.Cap))).Str(" tokens)").String(),
		real.Str("  real     ").Str(Short(float64(r.Capped.Real))).String(),
		capped.Str("  capped   ").Str(Short(float64(r.Capped.Capped))).
			Str("   = ").Str(fixed(r.Capped.Share, 1)).Str("% of real").String(),
		"  Arithmetic over the calls that were made, not a forecast: a run under",
		"  this cap would have made different calls.",
	}
}

func (r *Report) phaseBlock() []string {
	out := []string{
		"Subagent phases, keyed on the spawn description in <agent>.meta.json",
		row("phase", "agents", "calls", "calls/agent", "context tokens", "share", "mean ctx"),
		dashes(7),
	}
	for _, phase := range r.Phases {
		var share textbuf.Buffer
		out = append(out, row(phase.Phase,
			comma(int64(phase.Agents)), comma(int64(phase.Calls)),
			fixed(phase.CallsPerAgnt, 0),
			Short(float64(phase.Context)),
			share.Str(fixed(phase.Share, 1)).Byte('%').String(),
			Short(phase.MeanContext)))
	}
	return append(out,
		"  Phase is a keyword heuristic over the description; nothing in the",
		"  store records the phase an agent ran. calls/agent times mean",
		"  ctx is the context one agent of that phase feeds the model.")
}

func (r *Report) agentTypeBlock() []string {
	out := []string{
		"Subagent harness floor by agent type, from <agent>.meta.json agentType",
		row("agent type", "agents", "calls", "median floor", "context", "share"),
		dashes(6),
	}
	subContext := r.SubagentContext
	for _, agent := range r.AgentTypes {
		share := 0.0
		if subContext != 0 {
			share = 100.0 * float64(agent.Context) / float64(subContext)
		}
		var pct textbuf.Buffer
		out = append(out, row(agent.Name,
			comma(int64(agent.Agents)), comma(int64(agent.Calls)),
			comma(agent.MedianFloor), Short(float64(agent.Context)),
			pct.Str(fixed(share, 1)).Byte('%').String()))
	}
	return append(out,
		"  median floor is the first call with the spawn prompt subtracted:",
		"  what the harness gave the agent before it did anything. It is",
		"  re-fed on every later call, so it multiplies by calls. The",
		"  prompt comes out because otherwise the number tracks how much",
		"  the parent wrote, and drifts as a live session grows. A fork",
		"  reports 0: it inherits its parent's context, so it has no floor.",
		"  ACROSS sessions the rows do not compare, because the always-on",
		"  preamble changes size. Scope with make ze-token-economy-report",
		"  ZE_SESSION=<id>. See ai/agents/, ai/rules/context-economy.md.")
}

func (r *Report) toolBlock() []string {
	if len(r.ToolsPerCall) == 0 {
		return nil
	}

	out := []string{
		"",
		"Tool calls per API call",
		row("tool calls on the call", "API calls", "share"),
		dashes(3),
	}
	for _, item := range r.ToolsPerCall {
		var share textbuf.Buffer
		out = append(out, row(item.Label, comma(int64(item.Calls)),
			share.Str(fixed(item.Share, 1)).Byte('%').String()))
	}
	var single textbuf.Buffer
	out = append(out,
		single.Str("  Of the ").Str(comma(int64(r.SingleTool.Using))).
			Str(" API calls that used a tool, ").Str(comma(int64(r.SingleTool.Alone))).
			Str(" used exactly one (").Str(fixed(r.SingleTool.Share, 1)).Str("%).").String(),
		"  Each of those paid its whole context for a single result.",
		"")

	var head textbuf.Buffer
	out = append(out,
		head.Str("Tool results fed back, by tool (tokens = characters / ").
			Str(strconv.FormatFloat(CharsPerToken, 'g', -1, 64)).
			Str(", an approximation)").String(),
		row("tool", "results", "mean tokens", "total tokens"),
		dashes(4))

	named := r.Results
	if len(named) > ResultTableRows {
		named = named[:ResultTableRows]
	}
	for _, result := range named {
		out = append(out, row(result.Name, comma(int64(result.Results)),
			Short(result.MeanTokens), Short(result.Tokens)))
	}
	if len(r.Results) > ResultTableRows {
		rest := r.Results[ResultTableRows:]
		count, total := 0, 0.0
		for _, result := range rest {
			count += result.Results
			total += result.Tokens
		}
		mean := "0"
		if count != 0 {
			mean = Short(total / float64(count))
		}
		var label textbuf.Buffer
		out = append(out, row(label.Int(int64(len(rest))).Str(" other tools").String(),
			comma(int64(count)), mean, Short(total)))
	}

	out = append(out, "", "Repeated reads and the main thread's tool mix")
	for _, group := range []struct {
		label string
		reads Repeats
	}{{"one thread", r.ThreadReads}, {"one session", r.SessionReads}} {
		share := 0.0
		if group.reads.Reads != 0 {
			share = 100.0 * float64(group.reads.Repeats) / float64(group.reads.Reads)
		}
		var line textbuf.Buffer
		out = append(out, line.Str("  Read calls naming a file: ").
			Str(comma(int64(group.reads.Reads))).Str("   already read within ").
			Str(group.label).Str(": ").Str(comma(int64(group.reads.Repeats))).
			Str(" (").Str(fixed(share, 1)).Str("%)").String())
	}
	var mix textbuf.Buffer
	return append(out,
		"  A thread is one context window; a session is its main thread and",
		"  every agent under it, which do not share a context window.",
		mix.Str("  Main-thread tool calls: ").Str(comma(int64(r.MainToolCalls))).
			Str("   of them Bash: ").Str(comma(int64(r.MainBash))).String())
}

// runePrefix answers the first n CHARACTERS of a value, which is what the
// script's slice does. A byte slice would cut a multi-byte session id in half
// and print a replacement character where the store holds a letter.
func runePrefix(value string, n int) string {
	count := 0
	for index := range value {
		if count == n {
			return value[:index]
		}
		count++
	}
	return value
}

// pyRepr quotes a value the way the script's `!r` conversion does, so the
// refusal a caller reads is the one the gate printed.
func pyRepr(value string) string {
	quote := byte('\'')
	if strings.Contains(value, "'") && !strings.Contains(value, "\"") {
		quote = '"'
	}
	var tb textbuf.Buffer
	tb.Byte(quote)
	for _, r := range value {
		switch {
		case r == '\\':
			tb.Str("\\\\")
		case r == rune(quote):
			tb.Byte('\\').Byte(quote)
		case r == '\n':
			tb.Str("\\n")
		case r == '\t':
			tb.Str("\\t")
		case r == '\r':
			tb.Str("\\r")
		default:
			tb.Str(string(r))
		}
	}
	return tb.Byte(quote).String()
}
