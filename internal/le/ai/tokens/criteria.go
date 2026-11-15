// Design: ai/rules/context-economy.md -- the figures the agent-economy criteria are measured with
// Overview: tokeneconomy.go -- the store read that produces the calls
// Related: baseline.go -- the umbrella's other baseline figures
// Related: report.go -- the page these figures are rendered into
//
// criteria.go holds the window a report is taken over and the figures the
// agent-economy criteria are judged by: the context of a subagent's first
// call, the subagent Bash calls the auto-mode classifier gave no verdict for,
// the failed-call rates, the editing agents that reached their call budget,
// the main sessions' context, and the queue wait admission recorded.
//
// Each figure has one definition, written on its type. A figure whose source
// holds nothing for the window is not measured and says why: a rate of zero
// over no calls would read as a pass.

package aitokens

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// windowLayout is how a window bound is printed, and the time column of the
// job record (appendDuration). dateLayout is how `from` is typed.
const (
	windowLayout = "2006-01-02T15:04:05Z"
	dateLayout   = "2006-01-02"
)

// Window is the time span a report is taken over: From inclusive, To
// exclusive. A zero bound is open on that side, so the zero Window is the
// whole store, which is what a caller who names no window gets.
type Window struct {
	From time.Time
	To   time.Time
}

// Bounded reports whether either side of the window is set.
func (w Window) Bounded() bool { return !w.From.IsZero() || !w.To.IsZero() }

// Holds reports whether a moment falls inside the window. An unbounded window
// holds every call, a dated one or not. A bounded window never holds the zero
// time: a call with no timestamp cannot be placed inside it.
func (w Window) Holds(at time.Time) bool {
	if !w.Bounded() {
		return true
	}
	if at.IsZero() {
		return false
	}
	if !w.From.IsZero() {
		if at.Before(w.From) {
			return false
		}
	}
	if w.To.IsZero() {
		return true
	}
	return at.Before(w.To)
}

// NewWindow answers the window the `from` and `days` keywords select. A date
// alone opens the window there and leaves it open to now. Both together are
// that many days from the date, which is how a measurement over a fixed
// fortnight is typed. A day count alone is the last that many whole UTC days,
// today excluded, so two runs on the same day read the same window. No date
// and no count is the whole store.
func NewWindow(from time.Time, days int, now time.Time) Window {
	if from.IsZero() {
		if days > 0 {
			today := now.UTC().Truncate(24 * time.Hour)
			return Window{From: today.AddDate(0, 0, -days), To: today}
		}
		return Window{}
	}
	if days > 0 {
		return Window{From: from, To: from.AddDate(0, 0, days)}
	}
	return Window{From: from}
}

// WindowSpan is the window as the payload carries it. An empty side is open.
type WindowSpan struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// span renders the window for the payload.
func (w Window) span() WindowSpan {
	out := WindowSpan{}
	if !w.From.IsZero() {
		out.From = w.From.UTC().Format(windowLayout)
	}
	if !w.To.IsZero() {
		out.To = w.To.UTC().Format(windowLayout)
	}
	return out
}

// applyWindow keeps what a bounded window holds. It answers the kept
// sessions, the main-thread calls it left out because they carry no
// timestamp, and the subagents it left out because their first call carries
// none.
//
// The two kinds of thread are selected differently, because they live
// differently. A subagent is one short context window, and its figures (its
// first call, its call budget) belong to the whole agent: an agent is kept
// whole when its first call falls in the window, its calls after the window's
// end included, and dropped whole otherwise. An agent whose first call is
// undated cannot be placed, so it is counted as one agent and none of its
// calls is counted as an undated call. A main session can run for days, so
// each of its calls and tool calls is kept on its own time, and a session
// whose first call fell outside is marked MainTruncated.
func applyWindow(sessions []Session, window Window) ([]Session, int, int) {
	if !window.Bounded() {
		return sessions, 0, 0
	}
	undated := 0
	undatedAgents := 0
	out := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		kept := Session{SID: session.SID}
		if len(session.MainCalls) > 0 {
			kept.MainTruncated = !window.Holds(session.MainCalls[0].At)
		}
		for _, call := range session.MainCalls {
			if window.Holds(call.At) {
				kept.MainCalls = append(kept.MainCalls, call)
				continue
			}
			if call.At.IsZero() {
				undated++
			}
		}
		for i := range session.MainToolCalls {
			if window.Holds(session.MainToolCalls[i].At) {
				kept.MainToolCalls = append(kept.MainToolCalls, session.MainToolCalls[i])
			}
		}
		for _, agent := range session.Agents {
			if len(agent.Calls) == 0 {
				continue
			}
			first := agent.Calls[0].At
			if window.Holds(first) {
				kept.Agents = append(kept.Agents, agent)
				continue
			}
			if first.IsZero() {
				undatedAgents++
			}
		}
		out = append(out, kept)
	}
	return out, undated, undatedAgents
}

// Measure says whether a figure was measured, and why not when it was not.
// Its zero value is "not measured" with no reason, which no producer here
// returns: every path that leaves a figure unmeasured names the reason.
type Measure struct {
	Measured bool   `json:"measured"`
	Reason   string `json:"reason,omitempty"`
}

// unmeasured answers a Measure that names why the figure was not taken.
func unmeasured(reason string) Measure { return Measure{Reason: reason} }

// measured answers a Measure for a figure that was taken.
func measured() Measure { return Measure{Measured: true} }

// FirstTurn is the context of each subagent's first API call: everything the
// agent was fed before it did anything, the spawn prompt included. Forks are
// left out, because a fork's first call carries its parent's whole context.
// The umbrella's baseline median (53k) does not say whether it counted forks,
// so a comparison with it compares a figure without forks to one that may
// hold them.
// Above counts the agents whose first call is above PhaseAgentContextMax.
// ByType is the same figure per recorded agent type; Untyped counts the
// agents with no recorded type, which are in the overall figure and in no
// row of ByType.
type FirstTurn struct {
	Measure
	Agents int             `json:"agents"`
	Median int64           `json:"median"`
	Max    int64           `json:"max"`
	Above  int             `json:"above"`
	Target int64           `json:"target"`
	ByType []TypeFirstTurn `json:"by-type,omitempty"`
	// Untyped is the count of non-fork subagents with no recorded agent type.
	Untyped int `json:"untyped"`
}

// TypeFirstTurn is FirstTurn for the agents of one type.
type TypeFirstTurn struct {
	AgentType string `json:"agent-type"`
	Agents    int    `json:"agents"`
	Median    int64  `json:"median"`
	Max       int64  `json:"max"`
}

// NoVerdict counts the subagent Bash calls the harness denied because the
// auto-mode classifier gave no verdict: a result whose record carries the
// denial kind DenialNoVerdict. Taken over the answered subagent Bash calls;
// Unanswered counts the subagent Bash calls with no recorded result, which
// are in neither count.
type NoVerdict struct {
	Measure
	Bash       int     `json:"bash"`
	Denied     int     `json:"denied"`
	Unanswered int     `json:"unanswered"`
	Share      float64 `json:"share"`
}

// Failures counts the answered calls of a set of tools whose result carries
// `is_error`, across every thread. A call with no recorded result is neither
// a success nor a failure, so it is counted apart as unanswered.
type Failures struct {
	Measure
	Tools      string  `json:"tools"`
	Answered   int     `json:"answered"`
	Failed     int     `json:"failed"`
	Unanswered int     `json:"unanswered"`
	Share      float64 `json:"share"`
}

// BudgetWall counts the editing agents that reached their call budget. An
// editing agent here is an agent of BudgetedAgentType that is not a fork; the
// hook binds by agent type alone, so it also counts a fork of that type, which
// this figure leaves out. Its counted calls are its calls of a BudgetedTools
// tool, except a call whose result carries DenialHookRefusal. That kind is
// written for every hook refusal, and the hook's counter counted some of
// them: it skips a call an earlier check of its own action refused, but it
// counts the budget's own refusal past the budget (bumpAgentCalls runs before
// the comparison) and a call another PreToolUse hook refused. The record
// cannot tell these apart, so the figure skips them all. Skipping the
// budget's own refusals moves no agent, because the budget refuses only once
// the counter has passed CallBudget; skipping another hook's refusal can count
// an agent short of a budget the hook did reach. Every other denial is given
// after the hooks ran, so the counter counted it. An agent reached the budget
// when its count is at least CallBudget. Untyped counts the non-fork subagents with no
// recorded agent type: whether they were editing agents cannot be told, so
// they are not judged.
type BudgetWall struct {
	Measure
	Agents  int     `json:"agents"`
	Reached int     `json:"reached"`
	Share   float64 `json:"share"`
	Budget  int     `json:"budget"`
	Untyped int     `json:"untyped"`
}

// MainPeak is the largest context any one main-thread call of a session
// carried, taken per session over the session's calls in the window: the
// median, the 90th percentile and the largest peak. Calls and Mean are the
// main-thread calls in the window and their mean context.
type MainPeak struct {
	Measure
	Sessions int   `json:"sessions"`
	Median   int64 `json:"median"`
	P90      int64 `json:"p90"`
	Max      int64 `json:"max"`
	Calls    int   `json:"calls"`
	Mean     int64 `json:"mean"`
}

// QueueWait reads the job record admission writes (appendDuration in
// internal/le/job/registry.go): one row per released claim, with the seconds
// the job ran and, on rows written since the wait was recorded, the seconds
// admission made it wait. Runs counts the rows in the window. Recorded counts
// those carrying the wait, Unknown those written without it: an unknown wait
// is never read as zero, so Wait is taken over Recorded alone and is not
// measured when Recorded is zero.
type QueueWait struct {
	Measure
	Source     string     `json:"source"`
	Runs       int        `json:"runs"`
	Recorded   int        `json:"recorded"`
	Unknown    int        `json:"unknown"`
	Unreadable int        `json:"unreadable"`
	Ran        int64      `json:"ran"`
	Wait       WaitFigure `json:"wait"`
}

// WaitFigure is the total, median and largest wait over the rows that carry
// one, in seconds.
type WaitFigure struct {
	Measure
	Total  int64 `json:"total"`
	Median int64 `json:"median"`
	Max    int64 `json:"max"`
}

// Criteria is every figure the agent-economy criteria are judged by, and the
// umbrella's other baseline figures (baseline.go).
type Criteria struct {
	FirstTurn         FirstTurn         `json:"first-turn"`
	MainFirstTurn     MainFirstTurn     `json:"main-first-turn"`
	AgentRuns         AgentRuns         `json:"agent-runs"`
	NoVerdict         NoVerdict         `json:"no-verdict"`
	BashFailures      Failures          `json:"bash-failures"`
	EditFailures      Failures          `json:"edit-failures"`
	WriteFailures     Failures          `json:"write-failures"`
	EditWriteFailures Failures          `json:"edit-write-failures"`
	BudgetWall        BudgetWall        `json:"budget-wall"`
	MainPeak          MainPeak          `json:"main-peak"`
	SubagentCacheRead SubagentCacheRead `json:"subagent-cache-read"`
	Shell             ShellFigures      `json:"shell"`
	QueueWait         QueueWait         `json:"queue-wait"`
}

// These copy the call budget's own declarations, which are unexported in the
// sealed hook package: agentCallBudget, budgetedAgentType and the tools
// bashCallBudget and writeCallBudget are bound to
// (internal/le/hookruntime/budget.go, nativeHookActions). A change there is a
// change here: TestTheTokenReportCopiesTheCallBudget
// (internal/le/hookruntime/budget_copies_test.go) compares the three.
const (
	CallBudget        = 100
	BudgetedAgentType = "ze-work"
)

// BudgetedTools are the tools whose calls the budget counts.
var BudgetedTools = []string{BashTool, WriteTool, EditTool, "MultiEdit", "NotebookEdit"}

// The denial kinds the harness records on a result (`toolDenialKind`).
// DenialNoVerdict is written when the auto-mode classifier gave no verdict
// for a call. DenialHookRefusal is written when a hook refused the call
// before it ran, which is the one denial the call budget does not count.
const (
	DenialNoVerdict   = "automode-unavailable"
	DenialHookRefusal = "permission-rule"
)

// PhaseAgentContextMax is the first-call context a phase agent must stay
// within (the agent-economy umbrella's AC-1).
const PhaseAgentContextMax = 35_000

// The tool sets the failure rates are taken over: Bash, Edit and Write each
// alone, and Edit with Write, which is how AC-3 states its bound.
var (
	bashTools      = []string{BashTool}
	editTools      = []string{EditTool}
	writeTools     = []string{WriteTool}
	editWriteTools = []string{EditTool, WriteTool}
)

// criteriaOf computes every transcript figure over the windowed sessions. The
// queue wait has its own source and is read apart (readQueueWait). Over no
// session, every figure is unmeasured with its reason.
func criteriaOf(sessions []Session) Criteria {
	agents := []Agent{}
	every := []ToolCall{}
	for _, session := range sessions {
		agents = append(agents, session.Agents...)
		every = append(every, session.AllToolCalls()...)
	}
	untyped := untypedOf(agents)
	first := firstTurnOf(agents)
	first.Untyped = untyped
	wall := budgetWallOf(agents)
	wall.Untyped = untyped
	return Criteria{
		FirstTurn:         first,
		MainFirstTurn:     mainFirstTurnOf(sessions),
		AgentRuns:         agentRunsOf(agents),
		NoVerdict:         noVerdictOf(agents),
		BashFailures:      failuresOf(every, bashTools),
		EditFailures:      failuresOf(every, editTools),
		WriteFailures:     failuresOf(every, writeTools),
		EditWriteFailures: failuresOf(every, editWriteTools),
		BudgetWall:        wall,
		MainPeak:          mainPeakOf(sessions),
		SubagentCacheRead: subagentCacheReadOf(sessions),
		Shell:             shellFiguresOf(sessions),
		QueueWait:         QueueWait{Measure: unmeasured("the job record was not read")},
	}
}

// untypedOf counts the non-fork subagents whose metadata records no agent
// type, a store from an older harness or a metadata file that is missing.
func untypedOf(agents []Agent) int {
	count := 0
	for _, agent := range agents {
		if agent.IsFork {
			continue
		}
		if agent.AgentType == "" {
			count++
		}
	}
	return count
}

func firstTurnOf(agents []Agent) FirstTurn {
	firsts := []int64{}
	byType := map[string][]int64{}
	above := 0
	for _, agent := range agents {
		if agent.IsFork {
			continue
		}
		if len(agent.Calls) == 0 {
			continue
		}
		first := agent.Calls[0].Context()
		firsts = append(firsts, first)
		if first > PhaseAgentContextMax {
			above++
		}
		if agent.AgentType != "" {
			byType[agent.AgentType] = append(byType[agent.AgentType], first)
		}
	}
	if len(firsts) == 0 {
		return FirstTurn{Measure: unmeasured("no subagent that is not a fork started in the window"), Target: PhaseAgentContextMax}
	}
	out := FirstTurn{
		Measure: measured(), Agents: len(firsts), Median: median(firsts), Max: largest(firsts),
		Above: above, Target: PhaseAgentContextMax,
	}
	types := make([]string, 0, len(byType))
	for agentType := range byType {
		types = append(types, agentType)
	}
	slices.Sort(types)
	for _, agentType := range types {
		values := byType[agentType]
		out.ByType = append(out.ByType, TypeFirstTurn{
			AgentType: agentType, Agents: len(values), Median: median(values), Max: largest(values),
		})
	}
	return out
}

func noVerdictOf(agents []Agent) NoVerdict {
	out := NoVerdict{}
	for _, agent := range agents {
		for i := range agent.ToolCalls {
			tool := &agent.ToolCalls[i]
			if tool.Name != BashTool {
				continue
			}
			if !tool.Answered {
				out.Unanswered++
				continue
			}
			out.Bash++
			if tool.Denial == DenialNoVerdict {
				out.Denied++
			}
		}
	}
	if out.Bash == 0 {
		out.Measure = unmeasured("no answered subagent Bash call in the window")
		return out
	}
	out.Measure = measured()
	out.Share = 100.0 * float64(out.Denied) / float64(out.Bash)
	return out
}

func failuresOf(tools []ToolCall, names []string) Failures {
	out := Failures{Tools: strings.Join(names, ", ")}
	for i := range tools {
		tool := &tools[i]
		if !slices.Contains(names, tool.Name) {
			continue
		}
		if !tool.Answered {
			out.Unanswered++
			continue
		}
		out.Answered++
		if tool.Failed {
			out.Failed++
		}
	}
	if out.Answered == 0 {
		var reason textbuf.Buffer
		out.Measure = unmeasured(reason.Str("no answered ").Str(out.Tools).Str(" call in the window").String())
		return out
	}
	out.Measure = measured()
	out.Share = 100.0 * float64(out.Failed) / float64(out.Answered)
	return out
}

func budgetWallOf(agents []Agent) BudgetWall {
	out := BudgetWall{Budget: CallBudget}
	for _, agent := range agents {
		if agent.IsFork {
			continue
		}
		if agent.AgentType != BudgetedAgentType {
			continue
		}
		out.Agents++
		counted := 0
		for i := range agent.ToolCalls {
			tool := &agent.ToolCalls[i]
			if tool.Denial == DenialHookRefusal {
				continue
			}
			if slices.Contains(BudgetedTools, tool.Name) {
				counted++
			}
		}
		if counted >= CallBudget {
			out.Reached++
		}
	}
	if out.Agents == 0 {
		out.Measure = unmeasured("no " + BudgetedAgentType + " agent started in the window")
		return out
	}
	out.Measure = measured()
	out.Share = 100.0 * float64(out.Reached) / float64(out.Agents)
	return out
}

func mainPeakOf(sessions []Session) MainPeak {
	peaks := []int64{}
	calls := 0
	fed := int64(0)
	for _, session := range sessions {
		if len(session.MainCalls) == 0 {
			continue
		}
		totals := Aggregate(session.MainCalls)
		peaks = append(peaks, totals.ContextMax)
		calls += totals.Calls
		fed += totals.Context
	}
	if len(peaks) == 0 {
		return MainPeak{Measure: unmeasured("no main-thread call in the window")}
	}
	return MainPeak{
		Measure: measured(), Sessions: len(peaks), Median: median(peaks), P90: percentile(peaks, 90),
		Max: largest(peaks), Calls: calls, Mean: fed / int64(calls),
	}
}

// readQueueWait reads the job record at path over the window.
//
// A row is label, run seconds, UTC time and waited seconds, tab-separated; a
// row of three columns was written before the wait was recorded. A row whose
// counts or time do not parse is counted as unreadable rather than dropped. A
// bounded window selects a row by its time.
func readQueueWait(path string, window Window) QueueWait {
	out := QueueWait{Source: path}
	if path == "" {
		out.Measure = unmeasured("no job record was named")
		out.Wait.Measure = out.Measure
		return out
	}
	body, err := os.ReadFile(path) //nolint:gosec // a read-only report over the checkout's own job record
	if errors.Is(err, fs.ErrNotExist) {
		out.Measure = unmeasured("no job record at " + path + ": no admitted run released a claim in this checkout")
		out.Wait.Measure = out.Measure
		return out
	}
	if err != nil {
		out.Measure = unmeasured("the job record could not be read: " + err.Error())
		out.Wait.Measure = out.Measure
		return out
	}

	waits := []int64{}
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		row, ok := parseJobRow(line)
		if !ok {
			out.Unreadable++
			continue
		}
		if !window.Holds(row.at) {
			continue
		}
		out.Runs++
		out.Ran += row.ran
		if !row.recorded {
			out.Unknown++
			continue
		}
		out.Recorded++
		out.Wait.Total += row.waited
		waits = append(waits, row.waited)
	}
	if out.Runs == 0 {
		var reason textbuf.Buffer
		reason.Str("no admitted run in the window")
		if out.Unreadable > 0 {
			reason.Str("; ").Str(comma(int64(out.Unreadable))).Str(" unreadable rows in the record")
		}
		out.Measure = unmeasured(reason.String())
		out.Wait.Measure = out.Measure
		return out
	}
	out.Measure = measured()
	if len(waits) == 0 {
		out.Wait.Measure = unmeasured("no run in the window carries its wait: every row was written before the wait was recorded")
		return out
	}
	out.Wait.Measure = measured()
	out.Wait.Median = median(waits)
	out.Wait.Max = largest(waits)
	return out
}

// jobRow is one parsed row of the job record.
type jobRow struct {
	ran      int64
	at       time.Time
	recorded bool
	waited   int64
}

// parseJobRow reads one row of three or four columns.
func parseJobRow(line string) (jobRow, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) != 3 && len(fields) != 4 {
		return jobRow{}, false
	}
	ran, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return jobRow{}, false
	}
	at, err := time.Parse(windowLayout, fields[2])
	if err != nil {
		return jobRow{}, false
	}
	row := jobRow{ran: ran, at: at}
	if len(fields) == 3 {
		return row, true
	}
	waited, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return jobRow{}, false
	}
	row.recorded = true
	row.waited = waited
	return row, true
}

// largest answers the largest value of a sample the caller made sure is not
// empty.
func largest(values []int64) int64 {
	top := values[0]
	for _, value := range values[1:] {
		top = max(top, value)
	}
	return top
}

// percentile answers the p-th percentile of a sample the caller made sure is
// not empty, interpolated linearly between the two closest ranks: the same
// rule that makes the median the midpoint of the two middles. The rank is
// p*(n-1)/100, kept as an integer and its remainder in hundredths, so the
// interpolation is exact integer arithmetic truncated once: a float fraction
// such as 0.29 is 0.28999... and would land a token low.
func percentile(values []int64, p int) int64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	rank := p * (len(sorted) - 1)
	low := rank / 100
	if low+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	hundredths := int64(rank % 100)
	return sorted[low] + (sorted[low+1]-sorted[low])*hundredths/100
}

// criteriaBlock renders the figures, one line each, with the reason in place
// of the numbers for a figure that was not measured.
func (r *Report) criteriaBlock() []string {
	c := r.Criteria
	out := []string{"Agent-economy criteria (one definition per figure: docs/contributing/navigating-the-code.md)"}
	out = append(out, c.FirstTurn.lines()...)
	out = append(out, c.MainFirstTurn.line(), c.AgentRuns.line(), c.NoVerdict.line())
	for _, failures := range []Failures{c.BashFailures, c.EditFailures, c.WriteFailures, c.EditWriteFailures} {
		out = append(out, failures.line())
	}
	out = append(out, c.BudgetWall.line())
	out = append(out, c.MainPeak.lines()...)
	out = append(out, c.SubagentCacheRead.line())
	out = append(out, c.Shell.lines()...)
	return append(out, c.QueueWait.line())
}

// lines renders the first-call figure and its per-type rows.
func (f FirstTurn) lines() []string {
	var tb textbuf.Buffer
	tb.Str("  First-call context of a subagent: ")
	if !f.Measured {
		return []string{tb.Str("not measured, ").Str(f.Reason).String()}
	}
	tb.Str(comma(int64(f.Agents))).Str(" agents, median ").Str(Short(float64(f.Median))).
		Str(", max ").Str(Short(float64(f.Max))).Str(", ").Str(comma(int64(f.Above))).
		Str(" above ").Str(Short(float64(f.Target)))
	if f.Untyped > 0 {
		tb.Str("; ").Str(comma(int64(f.Untyped))).Str(" with no recorded agent type")
	}
	out := []string{tb.String()}
	for _, row := range f.ByType {
		var line textbuf.Buffer
		out = append(out, line.Str("    ").Str(row.AgentType).Str(": ").Str(comma(int64(row.Agents))).
			Str(" agents, median ").Str(Short(float64(row.Median))).Str(", max ").Str(Short(float64(row.Max))).String())
	}
	return out
}

// line renders the no-verdict figure.
func (n NoVerdict) line() string {
	var tb textbuf.Buffer
	tb.Str("  Subagent Bash calls with no auto-mode verdict: ")
	if !n.Measured {
		return tb.Str("not measured, ").Str(n.Reason).String()
	}
	return tb.Str(comma(int64(n.Denied))).Str(" of ").Str(comma(int64(n.Bash))).
		Str(" answered (").Str(fixed(n.Share, 2)).Str("%), ").
		Str(comma(int64(n.Unanswered))).Str(" with no result").String()
}

// line renders one failure rate.
func (f Failures) line() string {
	var tb textbuf.Buffer
	tb.Str("  Failed ").Str(f.Tools).Str(" calls: ")
	if !f.Measured {
		return tb.Str("not measured, ").Str(f.Reason).Str("; ").
			Str(comma(int64(f.Unanswered))).Str(" with no result").String()
	}
	return tb.Str(comma(int64(f.Failed))).Str(" of ").Str(comma(int64(f.Answered))).
		Str(" answered (").Str(fixed(f.Share, 1)).Str("%), ").
		Str(comma(int64(f.Unanswered))).Str(" with no result").String()
}

// line renders the budget figure.
func (b BudgetWall) line() string {
	var tb textbuf.Buffer
	tb.Str("  ").Str(BudgetedAgentType).Str(" agents at the ").Int(int64(b.Budget)).Str("-call budget: ")
	if b.Measured {
		tb.Str(comma(int64(b.Reached))).Str(" of ").Str(comma(int64(b.Agents))).
			Str(" (").Str(fixed(b.Share, 1)).Str("%)")
	} else {
		tb.Str("not measured, ").Str(b.Reason)
	}
	if b.Untyped > 0 {
		tb.Str("; ").Str(comma(int64(b.Untyped))).Str(" subagents with no recorded agent type not judged")
	}
	return tb.String()
}

// lines renders the main-thread context figures.
func (m MainPeak) lines() []string {
	var peak textbuf.Buffer
	peak.Str("  Main-session context peak: ")
	if !m.Measured {
		return []string{peak.Str("not measured, ").Str(m.Reason).String()}
	}
	peak.Str(comma(int64(m.Sessions))).Str(" sessions, median ").Str(Short(float64(m.Median))).
		Str(", 90th percentile ").Str(Short(float64(m.P90))).Str(", max ").Str(Short(float64(m.Max)))
	var mean textbuf.Buffer
	mean.Str("  Main-thread context per call: ").Str(comma(int64(m.Calls))).Str(" calls, mean ").
		Str(Short(float64(m.Mean)))
	return []string{peak.String(), mean.String()}
}

// line renders the queue wait figure.
func (q QueueWait) line() string {
	var tb textbuf.Buffer
	tb.Str("  Queue wait of admitted runs: ")
	if !q.Measured {
		return tb.Str("not measured, ").Str(q.Reason).String()
	}
	tb.Str(comma(int64(q.Runs))).Str(" runs, ").Str(comma(int64(q.Recorded))).Str(" with their wait recorded, ").
		Str(comma(int64(q.Unknown))).Str(" without; ran ").Str(comma(q.Ran)).Str("s")
	if q.Wait.Measured {
		tb.Str(", waited ").Str(comma(q.Wait.Total)).Str("s, median ").Str(comma(q.Wait.Median)).
			Str("s, max ").Str(comma(q.Wait.Max)).Byte('s')
	} else {
		tb.Str(", wait not measured, ").Str(q.Wait.Reason)
	}
	if q.Unreadable > 0 {
		tb.Str("; ").Str(comma(int64(q.Unreadable))).Str(" unreadable rows in the record")
	}
	return tb.String()
}
