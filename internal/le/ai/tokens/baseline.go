// Design: ai/rules/context-economy.md -- the figures the agent-economy criteria are measured with
// Overview: criteria.go -- the window and the criteria these figures sit beside
// Related: shell.go -- what a Bash call's command is read into
//
// baseline.go holds the agent-economy umbrella's baseline figures that no
// criterion bounds but every criterion is compared against: a main session's
// first call, the length of a subagent's run, the subagents' part of the
// cache-read input, and the figures read from Bash commands (shell file
// reads, `le help` lookups, `le commit create` retries, and the wall time of
// foreground tests and builds).

package aitokens

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MainFirstTurn is the context of each main session's first API call. Only a
// session whose first main-thread call falls in the window has one: a session
// that started before it is marked MainTruncated and left out.
type MainFirstTurn struct {
	Measure
	Sessions int   `json:"sessions"`
	Median   int64 `json:"median"`
	Max      int64 `json:"max"`
}

// AgentRuns is the length of every subagent's run in the window, forks
// included: its API calls (turns) and its tool calls of any tool, as medians,
// and the agents with at least CallBudget tool calls of any tool. It is taken
// over all subagents, the way the umbrella's baseline counted them, and is a
// different figure from BudgetWall, which counts what the hook's budget
// counts for the agents it binds.
type AgentRuns struct {
	Measure
	Agents      int     `json:"agents"`
	TurnsMedian int64   `json:"turns-median"`
	CallsMedian int64   `json:"calls-median"`
	Reached     int     `json:"reached"`
	Share       float64 `json:"share"`
	Budget      int     `json:"budget"`
}

// SubagentCacheRead is the cache-read input of every subagent call in the
// window, and its share of the cache-read input of every call in it.
type SubagentCacheRead struct {
	Measure
	Tokens int64   `json:"tokens"`
	Share  float64 `json:"share"`
}

// ShellFigures are the figures read from Bash commands, over the Bash calls
// of every thread in the window. Bash counts them all; Unparsed counts those
// whose command was absent or did not parse, which are in no other count. The
// figures are measured when at least one command parsed.
type ShellFigures struct {
	Measure
	Bash     int           `json:"bash"`
	Unparsed int           `json:"unparsed"`
	Reads    ShellReads    `json:"reads"`
	Help     int           `json:"help"`
	Commit   CommitScripts `json:"commit"`
	Jobs     JobTime       `json:"jobs"`
}

// ShellReads counts the Bash calls whose command runs grep, sed or cat naming
// a file (Shell.Reader). Repeats counts those whose file is the file of the
// thread's previous tool call: a Read, Edit or Write naming it, or a shell
// read of it. The file is compared as typed, so a relative and an absolute
// spelling of one file are two files.
type ShellReads struct {
	Reads   int `json:"reads"`
	Repeats int `json:"repeats"`
}

// CommitScripts counts the Bash calls that run `le commit create`, and of
// them the ones that produced a script: answered with no error, because a
// refused create exits non-zero and writes no script. Unanswered counts the
// calls with no recorded result, which are in neither.
type CommitScripts struct {
	Calls      int `json:"calls"`
	Produced   int `json:"produced"`
	Unanswered int `json:"unanswered"`
}

// JobTime is the wall time of the Bash calls that run a test or a build
// (Shell.Job). A foreground run's time is its result record's timestamp less
// its tool_use record's timestamp. Runs counts the foreground runs, Timed
// those with both timestamps and a result no earlier than the call, Untimed
// the rest; Background counts the runs started with `run_in_background`,
// whose result arrives before the run ends and so carries no wall time.
// Denied counts the calls whose result carries a denial kind (a hook refusal,
// an auto-mode or user denial): the command never ran, so such a call is in
// no other count, rather than a foreground run timed at the moment of its
// refusal.
type JobTime struct {
	Runs       int     `json:"runs"`
	Timed      int     `json:"timed"`
	Untimed    int     `json:"untimed"`
	Seconds    float64 `json:"seconds"`
	Background int     `json:"background"`
	Denied     int     `json:"denied"`
}

func mainFirstTurnOf(sessions []Session) MainFirstTurn {
	firsts := []int64{}
	for _, session := range sessions {
		if session.MainTruncated {
			continue
		}
		if len(session.MainCalls) == 0 {
			continue
		}
		firsts = append(firsts, session.MainCalls[0].Context())
	}
	if len(firsts) == 0 {
		return MainFirstTurn{Measure: unmeasured("no main session's first call falls in the window")}
	}
	return MainFirstTurn{Measure: measured(), Sessions: len(firsts), Median: median(firsts), Max: largest(firsts)}
}

func agentRunsOf(agents []Agent) AgentRuns {
	out := AgentRuns{Budget: CallBudget}
	turns := []int64{}
	calls := []int64{}
	for _, agent := range agents {
		if len(agent.Calls) == 0 {
			continue
		}
		turns = append(turns, int64(len(agent.Calls)))
		calls = append(calls, int64(len(agent.ToolCalls)))
		if len(agent.ToolCalls) >= CallBudget {
			out.Reached++
		}
	}
	if len(turns) == 0 {
		out.Measure = unmeasured("no subagent started in the window")
		return out
	}
	out.Measure = measured()
	out.Agents = len(turns)
	out.TurnsMedian = median(turns)
	out.CallsMedian = median(calls)
	out.Share = 100.0 * float64(out.Reached) / float64(out.Agents)
	return out
}

func subagentCacheReadOf(sessions []Session) SubagentCacheRead {
	every := int64(0)
	subCalls := 0
	out := SubagentCacheRead{}
	for _, session := range sessions {
		mainTotals := Aggregate(session.MainCalls)
		subTotals := Aggregate(session.SubagentCalls())
		subCalls += subTotals.Calls
		every += mainTotals.CacheRead + subTotals.CacheRead
		out.Tokens += subTotals.CacheRead
	}
	if subCalls == 0 {
		out.Measure = unmeasured("no subagent call in the window")
		return out
	}
	if every == 0 {
		out.Measure = unmeasured("no call in the window read its input from the cache")
		return out
	}
	out.Measure = measured()
	out.Share = 100.0 * float64(out.Tokens) / float64(every)
	return out
}

func shellFiguresOf(sessions []Session) ShellFigures {
	out := ShellFigures{}
	for _, session := range sessions {
		for _, thread := range session.Threads() {
			previous := ""
			for i := range thread {
				tool := &thread[i]
				file := tool.FilePath
				if tool.Name == BashTool {
					out.count(tool, previous)
					file = tool.Shell.Reader
				}
				previous = file
			}
		}
	}
	if out.Bash == out.Unparsed {
		out.Measure = unmeasured("no Bash call in the window carries a command that parses")
		return out
	}
	out.Measure = measured()
	return out
}

// count adds one Bash call; previous is the file of the thread's call before
// it, empty when that call named none.
func (s *ShellFigures) count(tool *ToolCall, previous string) {
	s.Bash++
	shell := tool.Shell
	if !shell.Parsed {
		s.Unparsed++
		return
	}
	if shell.Reader != "" {
		s.Reads.Reads++
		if shell.Reader == previous {
			s.Reads.Repeats++
		}
	}
	if shell.Help {
		s.Help++
	}
	if shell.CommitCreate {
		s.Commit.count(tool)
	}
	if shell.Job {
		s.Jobs.count(tool)
	}
}

func (c *CommitScripts) count(tool *ToolCall) {
	c.Calls++
	if !tool.Answered {
		c.Unanswered++
		return
	}
	if !tool.Failed {
		c.Produced++
	}
}

func (j *JobTime) count(tool *ToolCall) {
	if tool.Denial != "" {
		j.Denied++
		return
	}
	if tool.Shell.Background {
		j.Background++
		return
	}
	j.Runs++
	elapsed, timed := elapsedOf(tool)
	if !timed {
		j.Untimed++
		return
	}
	j.Timed++
	j.Seconds += elapsed.Seconds()
}

// elapsedOf answers the time from a call's tool_use record to its result
// record, and false when either is missing or the result predates the call:
// such a run is untimed, never timed at zero.
func elapsedOf(tool *ToolCall) (time.Duration, bool) {
	if !tool.Answered {
		return 0, false
	}
	if tool.UsedAt.IsZero() {
		return 0, false
	}
	if tool.ResultAt.IsZero() {
		return 0, false
	}
	if tool.ResultAt.Before(tool.UsedAt) {
		return 0, false
	}
	return tool.ResultAt.Sub(tool.UsedAt), true
}

// line renders the main session's first-call figure.
func (m MainFirstTurn) line() string {
	var tb textbuf.Buffer
	tb.Str("  First-call context of a main session: ")
	if !m.Measured {
		return tb.Str("not measured, ").Str(m.Reason).String()
	}
	return tb.Str(comma(int64(m.Sessions))).Str(" sessions, median ").Str(Short(float64(m.Median))).
		Str(", max ").Str(Short(float64(m.Max))).String()
}

// line renders the subagent run figure.
func (a AgentRuns) line() string {
	var tb textbuf.Buffer
	tb.Str("  Subagent turns and tool calls: ")
	if !a.Measured {
		return tb.Str("not measured, ").Str(a.Reason).String()
	}
	return tb.Str(comma(int64(a.Agents))).Str(" agents, median ").Int(a.TurnsMedian).Str(" turns and ").
		Int(a.CallsMedian).Str(" tool calls; ").Str(comma(int64(a.Reached))).Str(" (").Str(fixed(a.Share, 1)).
		Str("%) reached ").Int(int64(a.Budget)).Str(" tool calls of any tool").String()
}

// line renders the subagents' part of the cache-read input.
func (c SubagentCacheRead) line() string {
	var tb textbuf.Buffer
	tb.Str("  Cache-read input in subagents: ")
	if !c.Measured {
		return tb.Str("not measured, ").Str(c.Reason).String()
	}
	return tb.Str(Short(float64(c.Tokens))).Str(", ").Str(fixed(c.Share, 1)).
		Str("% of all cache-read input").String()
}

// lines renders the figures read from Bash commands.
func (s ShellFigures) lines() []string {
	var parsed textbuf.Buffer
	parsed.Str("  Bash commands read: ")
	if !s.Measured {
		return []string{parsed.Str("not measured, ").Str(s.Reason).String()}
	}
	parsed.Str(comma(int64(s.Bash - s.Unparsed))).Str(" of ").Str(comma(int64(s.Bash))).
		Str(" parsed; the figures below are over those")

	var reads textbuf.Buffer
	reads.Str("  Shell file reads (grep, sed, cat naming a file): ").Str(comma(int64(s.Reads.Reads))).
		Str(", of which ").Str(comma(int64(s.Reads.Repeats))).Str(" repeat the previous call's file")

	var help textbuf.Buffer
	help.Str("  ./le help lookups: ").Str(comma(int64(s.Help)))

	var commit textbuf.Buffer
	commit.Str("  ./le commit create calls: ").Str(comma(int64(s.Commit.Calls))).Str(" for ").
		Str(comma(int64(s.Commit.Produced))).Str(" scripts produced, ").
		Str(comma(int64(s.Commit.Unanswered))).Str(" with no result")

	var jobs textbuf.Buffer
	jobs.Str("  Foreground test and build runs: ").Str(comma(int64(s.Jobs.Runs))).Str(", ").
		Str(comma(int64(s.Jobs.Timed))).Str(" timed for ").Str(comma(int64(s.Jobs.Seconds))).Str("s (").
		Str(fixed(s.Jobs.Seconds/3600, 1)).Str(" hours), ").Str(comma(int64(s.Jobs.Untimed))).
		Str(" untimed; ").Str(comma(int64(s.Jobs.Background))).Str(" in the background, ").
		Str(comma(int64(s.Jobs.Denied))).Str(" denied")

	return []string{parsed.String(), reads.String(), help.String(), commit.String(), jobs.String()}
}
