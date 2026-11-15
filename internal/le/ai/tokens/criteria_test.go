// Unit coverage for the agent-economy criteria and baseline figures the token
// report carries.
//
// VALIDATES: spec-agent-economy-4-supervision AC-7. Each figure is computed
// over a synthetic store and job record whose expected values are worked out
// by hand in the comments, so a changed definition shows as a changed number.
// The fixture is built so that each plausible wrong definition gives another
// number: contexts grow within an agent (first call against last, mean or
// max), a ze-work agent mixes Edit and Read calls at the budget's edge (the
// budgeted tool list), an out-of-window main call carries a failed Bash (main
// tool-call windowing), and an agent starts inside the window and calls after
// its end (an agent kept whole).
// PREVENTS: a figure computed over calls outside the window, a rate of zero
// printed for a figure whose source holds nothing, an unknown queue wait read
// as zero, and a budget that skips a denial the hook counted.

package aitokens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fixtureTool is one tool_use block and the result recorded for it. input is
// the block's input; resultAt is the result record's timestamp, none when
// empty.
type fixtureTool struct {
	id       string
	name     string
	answered bool
	failed   bool
	denial   string
	input    map[string]any
	resultAt string
}

// fixtureCall is one API call: its time ("" for none), its context and its
// tools.
type fixtureCall struct {
	id      string
	at      string
	context int
	tools   []fixtureTool
}

// fixtureTranscript renders a transcript of the given calls, each followed by the
// user record carrying the results of its answered tools.
func fixtureTranscript(t *testing.T, sid string, calls []fixtureCall) string {
	t.Helper()
	var b strings.Builder
	for _, call := range calls {
		content := []any{}
		for _, tool := range call.tools {
			input := tool.input
			if input == nil {
				input = map[string]any{}
			}
			content = append(content, map[string]any{"type": "tool_use", "id": tool.id, "name": tool.name, "input": input})
		}
		record := map[string]any{
			"type":      "assistant",
			"sessionId": sid,
			"message": map[string]any{
				"id":      call.id,
				"content": content,
				"usage": map[string]any{
					"input_tokens": 0, "cache_creation_input_tokens": 0,
					"cache_read_input_tokens": call.context, "output_tokens": 1,
				},
			},
		}
		if call.at != "" {
			record["timestamp"] = call.at
		}
		b.WriteString(encode(t, record))
		for _, tool := range call.tools {
			if !tool.answered {
				continue
			}
			result := map[string]any{
				"type": "user",
				"message": map[string]any{"role": "user", "content": []any{map[string]any{
					"type": "tool_result", "tool_use_id": tool.id, "content": "out", "is_error": tool.failed,
				}}},
			}
			if tool.denial != "" {
				result["toolDenialKind"] = tool.denial
			}
			if tool.resultAt != "" {
				result["timestamp"] = tool.resultAt
			}
			b.WriteString(encode(t, result))
		}
	}
	return b.String()
}

func encode(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding a fixture record: %v", err)
	}
	return string(body) + "\n"
}

func fixtureMeta(t *testing.T, agentType string, fork bool) string {
	t.Helper()
	return encode(t, map[string]any{"agentType": agentType, "description": "fixture agent", "isFork": fork})
}

// ran is a call of the named tool that ran and succeeded.
func ran(name string) fixtureTool { return fixtureTool{name: name, answered: true} }

// denied is a call of the named tool the harness denied with the given kind.
func denied(name, kind string) fixtureTool {
	return fixtureTool{name: name, answered: true, failed: true, denial: kind}
}

// bash is a Bash call carrying a command.
func bash(id, command string, answered, failed bool) fixtureTool {
	return fixtureTool{id: id, name: BashTool, answered: answered, failed: failed, input: map[string]any{"command": command}}
}

// toolRun answers count calls of one tool each, a second apart from start,
// the context growing by 100 a call so an agent's first call is its smallest,
// ids prefixed so no two agents share one. A Bash call with no input runs
// `ls`, so its command parses and is no read, no test and no commit.
func toolRun(prefix, start string, context, count int, tool fixtureTool) []fixtureCall {
	at, err := time.Parse(time.RFC3339, start)
	if err != nil {
		panic("BUG: fixture time " + start)
	}
	if tool.input == nil && tool.name == BashTool {
		tool.input = map[string]any{"command": "ls"}
	}
	out := make([]fixtureCall, 0, count)
	for i := range count {
		id := prefix + strconv.Itoa(i)
		one := tool
		one.id = "u-" + id
		out = append(out, fixtureCall{
			id: id, at: at.Add(time.Duration(i) * time.Second).Format(time.RFC3339), context: context + 100*i,
			tools: []fixtureTool{one},
		})
	}
	return out
}

func joinRuns(runs ...[]fixtureCall) []fixtureCall {
	out := []fixtureCall{}
	for _, run := range runs {
		out = append(out, run...)
	}
	return out
}

// fortnight is the window every fixture below is read over: 2026-11-01
// inclusive to 2026-11-15 exclusive.
var fortnight = Window{
	From: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	To:   time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
}

// criteriaStore writes the store the full-figure test reads.
//
// Main session s1: m1 (900k, 2026-10-30, outside the window, a failed `cat
// docs/a.md`), m2 (300k, `cat docs/a.md`), m3 (500k, an Edit of docs/a.md
// that failed and a Write of docs/c.md), m4 (1k, a Write of docs/b.md with no
// result), m5 (no timestamp, so undated), m6 (2k, `sed` of docs/b.md: a
// repeat of m4's file), m7 (3k, `./le help`), m8 (4k, a refused `./le commit
// create`), m9 (5k, one that produced a script), m10 (6k, `./le test` from
// 10:00:00 to its result at 10:01:30), m11 (7k, `go test` in the background),
// m12 (8k, a command that does not parse) and m13 (9k, `go build` whose result
// carries no time). In the window: 11 calls, 845k, peak 500k. Its first call
// is outside, so it has no first turn.
// Main session s2: n1 (200k, a failed `ls`). Main session s3: p1 (100k).
//
// Agents of s1, each call's context 100 above the one before:
//   - a, ze-work, first call 20k: 99 Bash and an Edit that ran, then a Bash a
//     hook refused (permission-rule). Counted: 100, so it reached the budget.
//   - b, ze-work, first call 40k: 99 Bash that ran, then two the classifier
//     gave no verdict for (automode-unavailable). The hook counted those two
//     before the harness denied them: 101, reached.
//   - c, Explore: three calls of 30k, 70k and 71k, no tools.
//   - d, ze-work fork, one call of 600k: out of the first-call and budget
//     figures, because a fork carries its parent's context.
//   - e, ze-work, first call on 2026-10-20: the whole agent is outside.
//   - f, ze-work, first call 15k: 98 Bash, an Edit and two Reads that ran,
//     then two Bash a hook refused. Counted: 99, so it did not reach it; with
//     the Reads or the refusals counted it would.
//   - g, general-purpose, first call 25k at 23:59 on the window's last day: a
//     `cat docs/a.md`, then after the window's end a failed `grep` of the same
//     file. Kept whole, so the second call counts and repeats the first's file.
//   - h, no metadata file, first call 12k: one Bash with no result.
//   - i, ze-work, one call with no timestamp: left out and counted apart.
func criteriaStore(t *testing.T) string {
	t.Helper()
	agentA := joinRuns(
		toolRun("a", "2026-11-02T12:00:00Z", 20_000, 99, ran(BashTool)),
		toolRun("a-edit", "2026-11-02T12:30:00Z", 30_000, 1, ran("Edit")),
		toolRun("a-refused", "2026-11-02T13:00:00Z", 30_000, 1, denied(BashTool, DenialHookRefusal)))
	agentB := joinRuns(
		toolRun("b", "2026-11-04T12:00:00Z", 40_000, 99, ran(BashTool)),
		toolRun("b-noverdict", "2026-11-04T13:00:00Z", 40_000, 2, denied(BashTool, DenialNoVerdict)))
	agentE := joinRuns(
		toolRun("e-early", "2026-10-20T12:00:00Z", 10_000, 1, ran(BashTool)),
		toolRun("e-late", "2026-11-02T12:00:00Z", 10_000, 1, ran(BashTool)))
	agentF := joinRuns(
		toolRun("f", "2026-11-07T12:00:00Z", 15_000, 98, ran(BashTool)),
		toolRun("f-edit", "2026-11-07T12:30:00Z", 15_000, 1, ran("Edit")),
		toolRun("f-read", "2026-11-07T12:40:00Z", 15_000, 2, ran(ReadTool)),
		toolRun("f-refused", "2026-11-07T13:00:00Z", 15_000, 2, denied(BashTool, DenialHookRefusal)))

	file := func(id, name, path string, answered, failed bool) fixtureTool {
		return fixtureTool{id: id, name: name, answered: answered, failed: failed, input: map[string]any{"file_path": path}}
	}
	timedTest := bash("u-test", "export ZE_AGENT_ID=x; ./le test unit pkg path ./x/", true, false)
	timedTest.resultAt = "2026-11-04T10:01:30Z"
	background := bash("u-bg", "go test ./x/", true, false)
	background.input["run_in_background"] = true

	return writeStore(t, map[string]string{
		"s1.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "m1", at: "2026-10-30T10:00:00Z", context: 900_000, tools: []fixtureTool{bash("u-out", "cat docs/a.md", true, true)}},
			{id: "m2", at: "2026-11-02T10:00:00Z", context: 300_000, tools: []fixtureTool{bash("u-cat", "cat docs/a.md", true, false)}},
			{id: "m3", at: "2026-11-03T10:00:00Z", context: 500_000, tools: []fixtureTool{
				file("u-edit", "Edit", "docs/a.md", true, true), file("u-write", "Write", "docs/c.md", true, false),
			}},
			{id: "m4", at: "2026-11-03T11:00:00Z", context: 1_000, tools: []fixtureTool{file("u-pending", "Write", "docs/b.md", false, false)}},
			{id: "m5", context: 50},
			{id: "m6", at: "2026-11-04T09:00:00Z", context: 2_000, tools: []fixtureTool{bash("u-sed", "sed -n 1,20p docs/b.md", true, false)}},
			{id: "m7", at: "2026-11-04T09:10:00Z", context: 3_000, tools: []fixtureTool{bash("u-help", "./le help spec", true, false)}},
			{id: "m8", at: "2026-11-04T09:20:00Z", context: 4_000, tools: []fixtureTool{bash("u-commit-refused", "./le commit create x", true, true)}},
			{id: "m9", at: "2026-11-04T09:21:00Z", context: 5_000, tools: []fixtureTool{bash("u-commit", "./le commit create x", true, false)}},
			{id: "m10", at: "2026-11-04T10:00:00Z", context: 6_000, tools: []fixtureTool{timedTest}},
			{id: "m11", at: "2026-11-04T11:00:00Z", context: 7_000, tools: []fixtureTool{background}},
			{id: "m12", at: "2026-11-04T12:00:00Z", context: 8_000, tools: []fixtureTool{bash("u-open", `echo "open`, true, false)}},
			{id: "m13", at: "2026-11-04T12:30:00Z", context: 9_000, tools: []fixtureTool{bash("u-build", "go build -o bin/x ./cmd/x", true, false)}},
		}),
		"s2.jsonl": fixtureTranscript(t, "s2", []fixtureCall{
			{id: "n1", at: "2026-11-05T00:00:00Z", context: 200_000, tools: []fixtureTool{bash("u-ls", "ls", true, true)}},
		}),
		"s3.jsonl": fixtureTranscript(t, "s3", []fixtureCall{
			{id: "p1", at: "2026-11-09T00:00:00Z", context: 100_000},
		}),
		"s1/subagents/agent-a.jsonl":     fixtureTranscript(t, "s1", agentA),
		"s1/subagents/agent-a.meta.json": fixtureMeta(t, BudgetedAgentType, false),
		"s1/subagents/agent-b.jsonl":     fixtureTranscript(t, "s1", agentB),
		"s1/subagents/agent-b.meta.json": fixtureMeta(t, BudgetedAgentType, false),
		"s1/subagents/agent-c.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "c1", at: "2026-11-06T00:00:00Z", context: 30_000},
			{id: "c2", at: "2026-11-06T00:01:00Z", context: 70_000},
			{id: "c3", at: "2026-11-06T00:02:00Z", context: 71_000},
		}),
		"s1/subagents/agent-c.meta.json": fixtureMeta(t, "Explore", false),
		"s1/subagents/agent-d.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "d1", at: "2026-11-06T00:00:00Z", context: 600_000},
		}),
		"s1/subagents/agent-d.meta.json": fixtureMeta(t, BudgetedAgentType, true),
		"s1/subagents/agent-e.jsonl":     fixtureTranscript(t, "s1", agentE),
		"s1/subagents/agent-e.meta.json": fixtureMeta(t, BudgetedAgentType, false),
		"s1/subagents/agent-f.jsonl":     fixtureTranscript(t, "s1", agentF),
		"s1/subagents/agent-f.meta.json": fixtureMeta(t, BudgetedAgentType, false),
		"s1/subagents/agent-g.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "g1", at: "2026-11-14T23:59:00Z", context: 25_000, tools: []fixtureTool{bash("u-g1", "cat docs/a.md", true, false)}},
			{id: "g2", at: "2026-11-16T00:00:00Z", context: 26_000, tools: []fixtureTool{bash("u-g2", "grep -n x docs/a.md", true, true)}},
		}),
		"s1/subagents/agent-g.meta.json": fixtureMeta(t, "general-purpose", false),
		"s1/subagents/agent-h.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "h1", at: "2026-11-08T00:00:00Z", context: 12_000, tools: []fixtureTool{bash("u-h1", "ls", false, false)}},
		}),
		"s1/subagents/agent-i.jsonl": fixtureTranscript(t, "s1", []fixtureCall{
			{id: "i1", context: 5_000},
		}),
		"s1/subagents/agent-i.meta.json": fixtureMeta(t, BudgetedAgentType, false),
	})
}

// jobRecordFixture writes a job record and answers its path.
//
// In the fortnight: four runs (40s waited 0, 100s waited 12, 200s written
// before the wait was recorded, 50s waited 3). Outside it: one the day
// before and one at the exclusive end. One line is not a row at all.
func jobRecordFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "durations.txt")
	rows := strings.Join([]string{
		"unit-pkg\t30\t2026-10-31T23:59:59Z\t5",
		"unit-pkg\t40\t2026-11-02T10:00:00Z\t0",
		"lint\t100\t2026-11-03T10:00:00Z\t12",
		"verify\t200\t2026-11-04T10:00:00Z",
		"not a row",
		"unit-pkg\t50\t2026-11-10T10:00:00Z\t3",
		"unit-pkg\t60\t2026-11-15T00:00:00Z\t9",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(rows), 0o600); err != nil {
		t.Fatalf("writing the job record: %v", err)
	}
	return path
}

// TestTheTokenReportCarriesEveryUmbrellaFigure reads the fixture store over
// the fortnight and compares every figure with its hand-computed value, then
// the lines the page prints for them.
func TestTheTokenReportCarriesEveryUmbrellaFigure(t *testing.T) {
	store := criteriaStore(t)
	report, code := Run(Options{
		Root: filepath.Dir(store), Project: "-proj", Cap: DefaultCap, Top: DefaultTop,
		Window: fortnight, Jobs: jobRecordFixture(t),
	})
	if code != 0 {
		t.Fatalf("the report exited %d", code)
	}
	if report.State != StateOK {
		t.Fatalf("state reads %q, want %q", report.State, StateOK)
	}
	if report.Undated != 1 || report.UndatedAgents != 1 {
		t.Errorf("undated reads %d calls and %d agents, want 1 (m5) and 1 (i)", report.Undated, report.UndatedAgents)
	}
	c := report.Criteria

	// a 20k, b 40k, c 30k, f 15k, g 25k, h 12k; d is a fork, e started
	// outside, i is undated. Only b is above 35k; h has no type.
	if got := c.FirstTurn; !got.Measured || got.Agents != 6 || got.Median != 22_500 || got.Max != 40_000 ||
		got.Above != 1 || got.Untyped != 1 {
		t.Errorf("first turn reads %+v, want 6 agents, median 22500, max 40000, 1 above, 1 untyped", got)
	}
	wantTypes := []TypeFirstTurn{
		{AgentType: "Explore", Agents: 1, Median: 30_000, Max: 30_000},
		{AgentType: "general-purpose", Agents: 1, Median: 25_000, Max: 25_000},
		{AgentType: BudgetedAgentType, Agents: 3, Median: 20_000, Max: 40_000},
	}
	if got := c.FirstTurn.ByType; len(got) == len(wantTypes) {
		for i := range wantTypes {
			if got[i] != wantTypes[i] {
				t.Errorf("first turn of %s reads %+v, want %+v", wantTypes[i].AgentType, got[i], wantTypes[i])
			}
		}
	} else {
		t.Errorf("first turn by type reads %+v, want %+v", got, wantTypes)
	}
	// s1's first call is outside the window; s2 200k, s3 100k.
	if got := c.MainFirstTurn; !got.Measured || got.Sessions != 2 || got.Median != 150_000 || got.Max != 200_000 {
		t.Errorf("main first turn reads %+v, want 2 sessions, median 150000, max 200000", got)
	}
	// Turns a 101, b 101, c 3, d 1, f 103, g 2, h 1: median 3. Tool calls
	// 101, 101, 0, 0, 103, 2, 1: median 2. a, b and f made 100 or more.
	if got := c.AgentRuns; !got.Measured || got.Agents != 7 || got.TurnsMedian != 3 || got.CallsMedian != 2 || got.Reached != 3 {
		t.Errorf("agent runs read %+v, want 7 agents, medians 3 turns and 2 calls, 3 at 100", got)
	}
	// Answered subagent Bash: a 100, b 101, f 100, g 2; h's has no result.
	if got := c.NoVerdict; !got.Measured || got.Bash != 303 || got.Denied != 2 || got.Unanswered != 1 {
		t.Errorf("no verdict reads %+v, want 2 of 303 and 1 unanswered", got)
	}
	// Main: ten Bash in the window, u-commit-refused and u-ls failed (u-out
	// is outside). Subagents: 303 answered, 6 failed (a 1, b 2, f 2, g 1).
	if got := c.BashFailures; !got.Measured || got.Answered != 313 || got.Failed != 8 || got.Unanswered != 1 {
		t.Errorf("Bash failures read %+v, want 8 of 313 answered and 1 unanswered", got)
	}
	if got := c.EditFailures; !got.Measured || got.Answered != 3 || got.Failed != 1 || got.Unanswered != 0 {
		t.Errorf("Edit failures read %+v, want 1 of 3", got)
	}
	if got := c.WriteFailures; !got.Measured || got.Answered != 1 || got.Failed != 0 || got.Unanswered != 1 {
		t.Errorf("Write failures read %+v, want 0 of 1 and 1 unanswered", got)
	}
	if got := c.EditWriteFailures; !got.Measured || got.Answered != 4 || got.Failed != 1 || got.Unanswered != 1 {
		t.Errorf("Edit and Write failures read %+v, want 1 of 4 and 1 unanswered", got)
	}
	// a counted 100 and b 101: reached. f counted 99. d is a fork, e and i
	// are outside; h has no type.
	if got := c.BudgetWall; !got.Measured || got.Agents != 3 || got.Reached != 2 || got.Budget != CallBudget || got.Untyped != 1 {
		t.Errorf("budget wall reads %+v, want 2 of 3 at %d and 1 untyped", got, CallBudget)
	}
	// Peaks s1 500k (m1's 900k is outside), s2 200k, s3 100k. The 90th
	// percentile sits 0.8 of the way from 200k to 500k. 13 main calls in the
	// window carry 1,145,000.
	if got := c.MainPeak; !got.Measured || got.Sessions != 3 || got.Median != 200_000 || got.P90 != 440_000 ||
		got.Max != 500_000 || got.Calls != 13 || got.Mean != 88_076 {
		t.Errorf("main peak reads %+v, want 3 sessions, median 200000, p90 440000, max 500000, 13 calls, mean 88076", got)
	}
	// Subagents read 9,904,800 of the 11,049,800 cache-read tokens.
	if got := c.SubagentCacheRead; !got.Measured || got.Tokens != 9_904_800 {
		t.Errorf("subagent cache read reads %+v, want 9904800", got)
	}
	wantShell := ShellFigures{
		Measure: measured(), Bash: 314, Unparsed: 1,
		Reads:  ShellReads{Reads: 4, Repeats: 2},
		Help:   1,
		Commit: CommitScripts{Calls: 2, Produced: 1},
		Jobs:   JobTime{Runs: 2, Timed: 1, Untimed: 1, Seconds: 90, Background: 1},
	}
	if c.Shell != wantShell {
		t.Errorf("shell figures read %+v, want %+v", c.Shell, wantShell)
	}
	// 40 + 100 + 200 + 50 run seconds; waits 0, 12 and 3; one row unknown.
	want := QueueWait{
		Measure: measured(), Source: c.QueueWait.Source, Runs: 4, Recorded: 3, Unknown: 1, Unreadable: 1,
		Ran: 390, Wait: WaitFigure{Measure: measured(), Total: 15, Median: 3, Max: 12},
	}
	if c.QueueWait != want {
		t.Errorf("queue wait reads %+v, want %+v", c.QueueWait, want)
	}

	page := report.Text()
	for _, line := range []string{
		"  Window: 2026-11-01T00:00:00Z to 2026-11-15T00:00:00Z (1 main-thread calls with no timestamp left out) (1 subagents whose first call has no timestamp left out)",
		"  First-call context of a subagent: 6 agents, median 22.5k, max 40k, 1 above 35k; 1 with no recorded agent type",
		"    Explore: 1 agents, median 30k, max 30k",
		"    ze-work: 3 agents, median 20k, max 40k",
		"  First-call context of a main session: 2 sessions, median 150k, max 200k",
		"  Subagent turns and tool calls: 7 agents, median 3 turns and 2 tool calls; 3 (42.9%) reached 100 tool calls of any tool",
		"  Subagent Bash calls with no auto-mode verdict: 2 of 303 answered (0.66%), 1 with no result",
		"  Failed Bash calls: 8 of 313 answered (2.6%), 1 with no result",
		"  Failed Edit calls: 1 of 3 answered (33.3%), 0 with no result",
		"  Failed Write calls: 0 of 1 answered (0.0%), 1 with no result",
		"  Failed Edit, Write calls: 1 of 4 answered (25.0%), 1 with no result",
		"  ze-work agents at the 100-call budget: 2 of 3 (66.7%); 1 subagents with no recorded agent type not judged",
		"  Main-session context peak: 3 sessions, median 200k, 90th percentile 440k, max 500k",
		"  Main-thread context per call: 13 calls, mean 88.1k",
		"  Cache-read input in subagents: 9.9M, 89.6% of all cache-read input",
		"  Bash commands read: 313 of 314 parsed; the figures below are over those",
		"  Shell file reads (grep, sed, cat naming a file): 4, of which 2 repeat the previous call's file",
		"  ./le help lookups: 1",
		"  ./le commit create calls: 2 for 1 scripts produced, 0 with no result",
		"  Foreground test and build runs: 2, 1 timed for 90s (0.0 hours), 1 untimed; 1 in the background, 0 denied",
		"  Queue wait of admitted runs: 4 runs, 3 with their wait recorded, 1 without; ran 390s, waited 15s, median 3s, max 12s; 1 unreadable rows in the record",
	} {
		if !strings.Contains(page, line+"\n") {
			t.Errorf("the page does not print %q:\n%s", line, page)
		}
	}
	if strings.Contains(page, "docs/a.md") {
		t.Errorf("the page prints command text:\n%s", page)
	}
	// The payload every pipe operator renders carries no file a command named
	// and no command text either.
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling the report: %v", err)
	}
	for _, typed := range []string{"docs/a.md", "docs/b.md", "./le test", "ZE_AGENT_ID", "cmd/x"} {
		if strings.Contains(string(payload), typed) {
			t.Errorf("the JSON payload carries %q from a command", typed)
		}
	}
}

// TestReadShellKeepsOnlyWhatTheFiguresNeed pins the command reading the shell
// figures rest on, one command shape per row.
func TestReadShellKeepsOnlyWhatTheFiguresNeed(t *testing.T) {
	for _, row := range []struct {
		command string
		want    Shell
	}{
		{"export A=1; ZE=2 ./le help spec", Shell{Parsed: true, Help: true}},
		{"bin/le commit create x", Shell{Parsed: true, CommitCreate: true}},
		{"./le commit push x", Shell{Parsed: true}},
		{"cat docs/a.md | grep x", Shell{Parsed: true, Reader: "docs/a.md"}},
		{"grep -rn 'two words' internal/le", Shell{Parsed: true, Reader: "internal/le"}},
		{"ls | grep x", Shell{Parsed: true}},
		{"./le job run label x quiet command go vet ./x/", Shell{Parsed: true, Job: true}},
		{"./le repo changed scope print both", Shell{Parsed: true}},
		{"./le rfc check && go version", Shell{Parsed: true, Job: true}},
		{"time go test ./x/", Shell{Parsed: true, Job: true}},
		{"grep -Ef patterns.txt docs/a.md", Shell{Parsed: true, Reader: "docs/a.md"}},
		{"x | grep -A", Shell{Parsed: true}},
		{"if true; then", Shell{}},
		// An option's value is not an operand: piped, these name no file.
		{"x | grep -A 3 foo", Shell{Parsed: true}},
		{"x | grep -A3 --max-count 2 foo", Shell{Parsed: true}},
		{"x | sed -e s/a/b/ -e s/c/d/", Shell{Parsed: true}},
		// After -e or -f, one operand is the file.
		{"grep -e x docs/a.md", Shell{Parsed: true, Reader: "docs/a.md"}},
		{"grep -nA 3 --regexp=x docs/a.md", Shell{Parsed: true, Reader: "docs/a.md"}},
		{"sed -f script.sed docs/b.md", Shell{Parsed: true, Reader: "docs/b.md"}},
		{"grep -- -x docs/a.md", Shell{Parsed: true, Reader: "docs/a.md"}},
	} {
		if got := readShell(map[string]any{"command": row.command}); got != row.want {
			t.Errorf("readShell(%q) reads %+v, want %+v", row.command, got, row.want)
		}
	}
	if got := readShell(map[string]any{}); got.Parsed {
		t.Errorf("a call with no command reads as parsed: %+v", got)
	}
	// The length bound: a command of shellCommandMax bytes is parsed, one byte
	// more is not handed to the parser and carries nothing from the command.
	read := "cat docs/a.md"
	atBound := read + strings.Repeat(" ", shellCommandMax-len(read))
	if got := readShell(map[string]any{"command": atBound}); !got.Parsed || got.Reader != "docs/a.md" {
		t.Errorf("a command of %d bytes reads %+v, want parsed with its file", len(atBound), got)
	}
	if got := readShell(map[string]any{"command": atBound + " "}); got != (Shell{}) {
		t.Errorf("a command past the bound reads %+v, want unparsed and empty", got)
	}
	if got := readShell(map[string]any{"command": "go test ./x/", "run_in_background": true}); !got.Background || !got.Job {
		t.Errorf("a background test reads %+v", got)
	}
}

// TestAWindowHoldingNoCallSaysSoAndMeasuresNothing reads the same store over a
// window after every call. Every transcript figure is not measured and says
// why, in the payload as on the page, and the queue wait, which has its own
// source, says its window is empty too rather than printing zeros.
func TestAWindowHoldingNoCallSaysSoAndMeasuresNothing(t *testing.T) {
	store := criteriaStore(t)
	later := Window{From: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2027, 1, 8, 0, 0, 0, 0, time.UTC)}
	report, code := Run(Options{
		Root: filepath.Dir(store), Project: "-proj", Cap: DefaultCap, Top: DefaultTop,
		Window: later, Jobs: jobRecordFixture(t),
	})
	if code != 0 {
		t.Fatalf("the report exited %d; it is a report and never blocks", code)
	}
	if report.State != StateOutside {
		t.Fatalf("state reads %q, want %q", report.State, StateOutside)
	}
	c := report.Criteria
	for name, figure := range map[string]Measure{
		"first turn": c.FirstTurn.Measure, "main first turn": c.MainFirstTurn.Measure,
		"agent runs": c.AgentRuns.Measure, "no verdict": c.NoVerdict.Measure,
		"Bash failures": c.BashFailures.Measure, "Edit failures": c.EditFailures.Measure,
		"Write failures": c.WriteFailures.Measure, "Edit and Write failures": c.EditWriteFailures.Measure,
		"budget wall": c.BudgetWall.Measure, "main peak": c.MainPeak.Measure,
		"subagent cache read": c.SubagentCacheRead.Measure, "shell": c.Shell.Measure,
		"queue wait": c.QueueWait.Measure, "wait": c.QueueWait.Wait.Measure,
	} {
		if figure.Measured || figure.Reason == "" {
			t.Errorf("%s reads %+v over an empty window, want not measured with a reason", name, figure)
		}
	}
	reason := "no admitted run in the window; 1 unreadable rows in the record"
	if q := c.QueueWait; q.Reason != reason {
		t.Errorf("queue wait reads %+v, want the reason %q", q, reason)
	}
	page := report.Text()
	for _, want := range []string{
		"  Window: 2027-01-01T00:00:00Z to 2027-01-08T00:00:00Z",
		"No API call recorded in the store falls inside the window",
		"  Queue wait of admitted runs: not measured, " + reason,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "API calls: 0") {
		t.Errorf("the page printed zero totals for a window it found empty:\n%s", page)
	}
}

// TestAFigureWhoseSourceIsMissingSaysSoRatherThanZero reads a window holding
// main-thread calls only, with no job record at the named path. The main
// figures are measured; every subagent figure and the queue wait say what is
// missing.
func TestAFigureWhoseSourceIsMissingSaysSoRatherThanZero(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s2.jsonl": fixtureTranscript(t, "s2", []fixtureCall{
			{id: "n1", at: "2026-11-05T00:00:00Z", context: 200_000},
		}),
	})
	absent := filepath.Join(t.TempDir(), "no-such-record.txt")
	report, _ := Run(Options{
		Root: filepath.Dir(store), Project: "-proj", Cap: DefaultCap, Top: DefaultTop,
		Window: fortnight, Jobs: absent,
	})
	c := report.Criteria
	if !c.MainPeak.Measured || c.MainPeak.Max != 200_000 {
		t.Errorf("the main peak reads %+v, want measured at 200000", c.MainPeak)
	}
	if !c.MainFirstTurn.Measured || c.MainFirstTurn.Max != 200_000 {
		t.Errorf("the main first turn reads %+v, want measured at 200000", c.MainFirstTurn)
	}
	for name, figure := range map[string]Measure{
		"first turn": c.FirstTurn.Measure, "no verdict": c.NoVerdict.Measure,
		"agent runs": c.AgentRuns.Measure, "subagent cache read": c.SubagentCacheRead.Measure,
		"Bash failures": c.BashFailures.Measure, "Write failures": c.WriteFailures.Measure,
		"Edit and Write failures": c.EditWriteFailures.Measure, "shell": c.Shell.Measure,
		"budget wall": c.BudgetWall.Measure, "queue wait": c.QueueWait.Measure,
	} {
		if figure.Measured || figure.Reason == "" {
			t.Errorf("%s reads %+v, want not measured with a reason", name, figure)
		}
	}
	if !strings.Contains(c.QueueWait.Reason, absent) {
		t.Errorf("the queue wait reason %q does not name the path it looked at", c.QueueWait.Reason)
	}
	page := report.Text()
	for _, want := range []string{
		"  First-call context of a subagent: not measured, no subagent that is not a fork started in the window",
		"  ze-work agents at the 100-call budget: not measured, no ze-work agent started in the window",
		"  Queue wait of admitted runs: not measured, no job record at " + absent,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q:\n%s", want, page)
		}
	}
}

// TestAWaitWrittenByNoRowIsNotMeasured reads a job record whose rows in the
// window all predate the wait: the runs are measured, the wait is not, and
// it says why rather than reporting a wait of zero.
func TestAWaitWrittenByNoRowIsNotMeasured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "durations.txt")
	if err := os.WriteFile(path, []byte("verify\t200\t2026-11-04T10:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("writing the job record: %v", err)
	}
	q := readQueueWait(path, fortnight)
	if !q.Measured || q.Runs != 1 || q.Unknown != 1 {
		t.Errorf("the runs read %+v, want one run of unknown wait", q)
	}
	if q.Wait.Measured || q.Wait.Reason == "" {
		t.Errorf("the wait reads %+v, want not measured with a reason", q.Wait)
	}
}

// TestAProjectThatIsNotACheckoutSaysSo names a directory with no .git as the
// project: its queue wait says it is not a checkout, not that no run released
// a claim.
func TestAProjectThatIsNotACheckoutSaysSo(t *testing.T) {
	directory := t.TempDir()
	opts, code := ParseOptions([]string{"project", directory})
	if code != 0 {
		t.Fatalf("a project directory was refused with %d", code)
	}
	if q := queueWaitOf(opts); q.Measured || !strings.Contains(q.Reason, "is not a git checkout") {
		t.Errorf("the queue wait reads %+v, want not measured because the project is not a checkout", q)
	}
}

// TestTheWindowKeywordsSelectAFixedSpan pins the grammar the measurement is
// typed with: a date and a day count make a fixed span, a day count alone is
// whole UTC days that do not move within a day, and a date that does not
// parse or a count outside its bound is refused rather than read as open.
func TestTheWindowKeywordsSelectAFixedSpan(t *testing.T) {
	opts, code := ParseOptions([]string{"from", "2026-11-01", "days", "14"})
	if code != 0 {
		t.Fatalf("a date and a day count were refused with %d", code)
	}
	if !opts.Window.From.Equal(fortnight.From) || !opts.Window.To.Equal(fortnight.To) {
		t.Errorf("the window reads %+v, want %+v", opts.Window, fortnight)
	}

	now := time.Date(2026, 11, 20, 15, 30, 0, 0, time.UTC)
	week := Window{From: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	for _, at := range []time.Time{now, now.Add(8 * time.Hour)} {
		if got := NewWindow(time.Time{}, 7, at); !got.From.Equal(week.From) || !got.To.Equal(week.To) {
			t.Errorf("the last seven days at %s read %+v, want %+v", at, got, week)
		}
	}
	if got := NewWindow(fortnight.From, 0, now); !got.From.Equal(fortnight.From) || !got.To.IsZero() {
		t.Errorf("a date alone reads %+v, want open to now", got)
	}
	if NewWindow(time.Time{}, 0, now).Bounded() {
		t.Error("no keyword bounded the window")
	}

	for _, args := range [][]string{{"from", "01/11/2026"}, {"days", "0"}, {"days", "3651"}} {
		if _, code := ParseOptions(args); code == 0 {
			t.Errorf("le ai tokens %v was accepted", args)
		}
	}
}

// TestABoundedWindowNeverHoldsAnUndatedCall pins the guard the undated count
// rests on: an unbounded window holds a call with no time, a bounded one does
// not, and the window's end is exclusive.
func TestABoundedWindowNeverHoldsAnUndatedCall(t *testing.T) {
	if !(Window{}).Holds(time.Time{}) {
		t.Error("the whole store dropped an undated call")
	}
	if fortnight.Holds(time.Time{}) {
		t.Error("a bounded window held an undated call")
	}
	if !fortnight.Holds(fortnight.From) {
		t.Error("the window does not hold its own start")
	}
	if fortnight.Holds(fortnight.To) {
		t.Error("the window holds its exclusive end")
	}
}

// TestThePercentileInterpolatesInWholeTokens pins the percentile on the
// samples the main-peak fixture does not reach: one element, two elements in
// either order, and a rank whose fraction a float cannot hold exactly (0.29
// of the way is 28.999... in binary, which truncated lands one token low).
func TestThePercentileInterpolatesInWholeTokens(t *testing.T) {
	for _, row := range []struct {
		values []int64
		p      int
		want   int64
	}{
		{[]int64{7}, 90, 7},
		{[]int64{100, 200}, 90, 190},
		{[]int64{200, 100}, 90, 190},
		{[]int64{0, 100}, 29, 29},
		{[]int64{0, 100}, 100, 100},
	} {
		if got := percentile(row.values, row.p); got != row.want {
			t.Errorf("percentile(%v, %d) reads %d, want %d", row.values, row.p, got, row.want)
		}
	}
}

// TestACommitCreateWithNoResultIsCountedApart reads three `le commit create`
// calls through the shell figures: one that produced a script, one refused,
// and one with no recorded result, which is in neither count.
func TestACommitCreateWithNoResultIsCountedApart(t *testing.T) {
	create := readShell(map[string]any{"command": "./le commit create x"})
	sessions := []Session{{SID: "s", MainToolCalls: []ToolCall{
		{Name: BashTool, Shell: create, Answered: true},
		{Name: BashTool, Shell: create, Answered: true, Failed: true},
		{Name: BashTool, Shell: create},
	}}}
	want := CommitScripts{Calls: 3, Produced: 1, Unanswered: 1}
	if got := shellFiguresOf(sessions).Commit; got != want {
		t.Errorf("the commit figure reads %+v, want %+v", got, want)
	}
}

// TestAJobCallIsTimedOnlyWhenItRan reads three foreground test calls through
// the shell figures: one a hook refused, which never ran and is counted as
// denied rather than as a run timed at its refusal; one that ran a minute;
// and one whose result record predates its call, which is untimed rather than
// timed at zero or below.
func TestAJobCallIsTimedOnlyWhenItRan(t *testing.T) {
	job := readShell(map[string]any{"command": "./le test unit pkg path ./x/"})
	used := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	backwards := ToolCall{Name: BashTool, Shell: job, Answered: true, UsedAt: used, ResultAt: used.Add(-time.Second)}
	if elapsed, timed := elapsedOf(&backwards); timed {
		t.Errorf("a result before its call was timed at %s", elapsed)
	}
	sessions := []Session{{SID: "s", MainToolCalls: []ToolCall{
		{
			Name: BashTool, Shell: job, Answered: true, Failed: true, Denial: DenialHookRefusal,
			UsedAt: used, ResultAt: used.Add(time.Millisecond),
		},
		{Name: BashTool, Shell: job, Answered: true, UsedAt: used, ResultAt: used.Add(time.Minute)},
		backwards,
	}}}
	want := JobTime{Runs: 2, Timed: 1, Untimed: 1, Seconds: 60, Denied: 1}
	if got := shellFiguresOf(sessions).Jobs; got != want {
		t.Errorf("the job figure reads %+v, want %+v", got, want)
	}
}
