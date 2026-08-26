// The migration's proof for the token-economy report: scripts/dev/token_economy.py
// and `le token-economy` answer the same page over the same transcript store.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for `token_economy.py`.
// PREVENTS: a port that agrees about a hand-built fixture and disagrees about
// the store a developer actually has.
//
// THIS IS THE FIRST PORTED TOOL WHOSE INPUT IS NOT THE REPOSITORY. Every other
// comparison here points both halves at a fixed real or fixture checkout. This
// tool reads ~/.claude/projects while another program WRITES to it. The content
// differs on each machine. It was 1.4 GB on the machine used for this port.
//
// Comparing both halves over that live store is invalid. The corpus can change
// during the comparison, so a difference does not identify a faulty half.
//
// The comparison is therefore in three parts, and each answers a different
// question:
//
//  1. The test compares both halves byte for byte over a FIXTURE store. The
//     fixture reaches every table and gives each figure an asserted value.
//  2. The test again compares both halves byte for byte over a FROZEN SNAPSHOT
//     of the real store. The snapshot contains symlinks to transcripts unchanged
//     for ten minutes under a store directory owned by this test.
//  3. The test compares the DEFAULT root and DEFAULT project slug by value.
//     Thus, both halves point at one store when the caller names none.
//
// The test also compares the constants by value. Output comparisons cannot
// detect a shared bound, bucket edge, or phase ranking when no fixture uses that
// value.
//
// This file is deliberately HERE instead of beside letools/tokeneconomy. It is
// a migration artifact, so the commit that deletes the script also deletes its
// proof.
//
// It also pins the fail-open defect that the port FIXED but the script still
// has. An unreadable transcript contributes nothing, so the report becomes
// smaller without a diagnosis. This case asserts that the SCRIPT still fails
// open. When somebody repairs the script, the case fails and must be deleted
// with the script.

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/tokeneconomy"
)

const (
	tokenScript  = "token_economy.py"
	tokenCommand = "token-economy"
	tokenSlug    = "-proj"
)

// tokenRecord renders one transcript record. It marshals a map instead of
// writing JSON manually. Otherwise, a missing comma here would make BOTH halves
// skip the record and still pass the comparison.
func tokenRecord(t *testing.T, record map[string]any) string {
	t.Helper()
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("fixture record: %v", err)
	}
	var line strings.Builder
	line.Write(body)
	line.WriteByte('\n')
	return line.String()
}

// tokenUsage is one assistant record's usage block.
func tokenUsage(input, write, read, output int) map[string]any {
	return map[string]any{
		"input_tokens":                input,
		"cache_creation_input_tokens": write,
		"cache_read_input_tokens":     read,
		"output_tokens":               output,
	}
}

// tokenUse is one tool_use block.
func tokenUse(id, name, path string) map[string]any {
	return map[string]any{
		"type": "tool_use", "id": id, "name": name,
		"input": map[string]any{"file_path": path},
	}
}

// tokenAssistant renders one assistant record. maker identifies the session
// that MADE the call, and owner identifies the containing file. These values
// differ for a resumed session.
func tokenAssistant(t *testing.T, maker, owner, id string, usage map[string]any, uses ...map[string]any) string {
	t.Helper()
	message := map[string]any{"id": id, "usage": usage}
	if len(uses) > 0 {
		blocks := make([]any, 0, len(uses))
		for _, use := range uses {
			blocks = append(blocks, use)
		}
		message["content"] = blocks
	}
	return tokenRecord(t, map[string]any{
		"type": "assistant", "session_id": maker, "sessionId": owner, "message": message,
	})
}

// tokenResults renders one user record carrying the results of the tool calls
// named, each padded to the character count given.
func tokenResults(t *testing.T, sizes map[string]int) string {
	t.Helper()
	ids := make([]string, 0, len(sizes))
	for id := range sizes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	blocks := make([]any, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, map[string]any{
			"type": "tool_result", "tool_use_id": id,
			"content": strings.Repeat("x", sizes[id]),
		})
	}
	return tokenRecord(t, map[string]any{"type": "user", "message": map[string]any{"content": blocks}})
}

// tokenPrompt renders the first user record of a subagent transcript, which is
// the prompt its parent wrote. The harness floor subtracts it.
func tokenPrompt(t *testing.T, chars int) string {
	t.Helper()
	return tokenRecord(t, map[string]any{
		"type": "user", "message": map[string]any{"content": strings.Repeat("p", chars)},
	})
}

// tokenMeta renders one spawn metadata file.
func tokenMeta(t *testing.T, agentType, description string, fork bool) string {
	t.Helper()
	meta := map[string]any{"agentType": agentType, "description": description}
	if fork {
		meta["isFork"] = true
	}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("fixture meta: %v", err)
	}
	return string(body)
}

// tokenStore writes a fixture store and answers its ROOT. The caller then passes
// the root and slug in the forms accepted by the command and script.
//
// The store reaches every report table. It has three sessions, split records, a
// resumed copy, and a fork. Twelve tool names make the result table fold a tail.
// The store repeats reads in one thread and across one session. It also has
// calls on three histogram edges and spawn descriptions for four phases. One
// line does not parse.
func tokenStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	store := filepath.Join(root, tokenSlug)

	files := map[string]string{}

	// Session one: split records for one call, a second call on a bucket edge,
	// and twelve tool names whose results are fed back.
	var main strings.Builder
	main.WriteString(tokenAssistant(t, "mmm55555", "mmm55555", "msg_a1", tokenUsage(10, 20, 49_970, 5)))
	main.WriteString(tokenAssistant(t, "mmm55555", "mmm55555", "msg_a1", tokenUsage(10, 20, 49_970, 120)))
	main.WriteString(tokenAssistant(t, "mmm55555", "mmm55555", "msg_a2", tokenUsage(0, 0, 150_000, 60),
		tokenUse("toolu_1", "Read", "internal/a.go"),
		tokenUse("toolu_2", "Read", "internal/a.go"),
		tokenUse("toolu_3", "Bash", "")))
	main.WriteString(tokenResults(t, map[string]int{"toolu_1": 3600, "toolu_2": 7200, "toolu_3": 360}))
	main.WriteString("this line does not parse\n")
	tools := []string{"Grep", "Glob", "Edit", "Write", "Task", "WebFetch", "TodoWrite", "NotebookEdit", "WebSearch", "Skill", "BashOutput"}
	for i, name := range tools {
		id := tokenID("toolu_x", i)
		msg := tokenID("msg_t", i)
		main.WriteString(tokenAssistant(t, "mmm55555", "mmm55555", msg,
			tokenUsage(5, 0, 250_000+i, 3), tokenUse(id, name, "")))
		main.WriteString(tokenResults(t, map[string]int{id: 36 * (i + 1)}))
	}
	files["mmm55555.jsonl"] = main.String()

	// Session two RESUMED session one. It copies the earlier records with its own
	// file ID and the original maker, then adds one call.
	//
	// Its id sorts BEFORE the original's, deliberately. Collection order is
	// the last-resort tie-break, so a store whose original is read first
	// passes with the origin rule deleted from either half.
	var resumed strings.Builder
	resumed.WriteString(tokenAssistant(t, "mmm55555", "bbb22222", "msg_a1", tokenUsage(10, 20, 49_970, 120)))
	resumed.WriteString(tokenAssistant(t, "bbb22222", "bbb22222", "msg_b1", tokenUsage(0, 0, 900_000, 10),
		tokenUse("toolu_b1", "Read", "internal/b.go")))
	resumed.WriteString(tokenResults(t, map[string]int{"toolu_b1": 18_000}))
	files["bbb22222.jsonl"] = resumed.String()

	// Session three carries a non-ASCII id, so the eight-character column and
	// the slug are measured in characters rather than bytes.
	files["ééé33333.jsonl"] = tokenAssistant(t, "ééé33333", "ééé33333", "msg_c1", tokenUsage(0, 0, 0, 0)) +
		tokenAssistant(t, "ééé33333", "ééé33333", "msg_c2", tokenUsage(1, 1, 1, 1))

	// Four subagents under session one: four phases, two agent types, one fork
	// whose calls belong to the agent it forked from.
	agents := []struct {
		name, agentType, description string
		context                      int
		fork                         bool
		prompt                       int
	}{
		// The FORK is agent-1, so it sorts FIRST. Both records are in one
		// session directory. Thus, the origin rule matches both, and only the
		// fork flag separates them. If the original sorted first, the fixture
		// would pass without that rule.
		{"agent-1", "ze-work", "review the implementation", 620_000, true, 7200},
		{"agent-2", "ze-work", "review the implementation", 620_000, false, 7200},
		{"agent-3", "ze-work", "implement the port of a tool", 400_000, false, 3600},
		{"agent-4", "general-purpose", "find the producing function", 90_000, false, 1800},
	}
	for i, agent := range agents {
		var body strings.Builder
		body.WriteString(tokenPrompt(t, agent.prompt))
		msg := tokenID("msg_ag", i)
		if agent.fork {
			// A fork repeats its parent's records, which is the second way one
			// API call reaches two transcripts.
			msg = tokenID("msg_ag", 1)
		}
		body.WriteString(tokenAssistant(t, "mmm55555", "mmm55555", msg,
			tokenUsage(0, 0, agent.context, 40),
			tokenUse(tokenID("toolu_ag", i), "Read", "internal/shared.go")))
		body.WriteString(tokenResults(t, map[string]int{tokenID("toolu_ag", i): 900 * (i + 1)}))
		files[filepath.Join("mmm55555", "subagents", tokenID2(agent.name, ".jsonl"))] = body.String()
		files[filepath.Join("mmm55555", "subagents", tokenID2(agent.name, ".meta.json"))] =
			tokenMeta(t, agent.agentType, agent.description, agent.fork)
	}

	for rel, body := range files {
		path := filepath.Join(store, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

func tokenID(prefix string, index int) string {
	var out strings.Builder
	out.WriteString(prefix)
	out.WriteString(strconv.Itoa(index))
	return out.String()
}

func tokenID2(name, suffix string) string {
	var out strings.Builder
	out.WriteString(name)
	out.WriteString(suffix)
	return out.String()
}

// tokenBothHalves runs the script and the command over one store and answers
// what each said. The keyword grammar is le's own, so the arguments are
// translated here rather than shared.
func tokenBothHalves(t *testing.T, root string, args ...string) (devPyResult, devPyResult) {
	t.Helper()

	scriptArgs := []string{"--root", root, "--project", tokenSlug}
	commandArgs := []string{"root", root, "project", tokenSlug}
	for i := 0; i+1 < len(args); i += 2 {
		var flag strings.Builder
		flag.WriteString("--")
		flag.WriteString(args[i])
		scriptArgs = append(scriptArgs, flag.String(), args[i+1])
		commandArgs = append(commandArgs, args[i], args[i+1])
	}

	script := devPyRunScript(t, tokenScript, scriptArgs, devPyRoot(t))
	command := devPyRunCommand(t, tokenCommand, tokeneconomy.Answer, commandArgs)
	return script, command
}

// tokenAgree compares the two halves. Both write their page to stdout, so the
// streams line up without a translation.
func tokenAgree(t *testing.T, what string, script, command devPyResult) {
	t.Helper()
	devPyAgree(t, what, script, command, script.Stdout, command.Stdout)
}

// tokenFigure answers the whole number that follows a label on the page.
// A case CAN assert an ABSOLUTE count as an independent check.
// Two proofs in this migration passed at a lower count because both halves stopped early.
// A comparison alone cannot see that.
func tokenFigure(t *testing.T, page, label string) int {
	t.Helper()
	index := strings.Index(page, label)
	if index < 0 {
		t.Fatalf("the page carries no %q:\n%s", label, page)
	}
	rest := strings.TrimLeft(page[index+len(label):], " ")
	digits := strings.Builder{}
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if r == ',' {
			continue
		}
		break
	}
	value, err := strconv.Atoi(digits.String())
	if err != nil {
		t.Fatalf("no number after %q in:\n%s", label, page)
	}
	return value
}

// TestTokenEconomyBothHalvesReportTheSameFixtureStore is the whole page over a
// store built to reach every table.
func TestTokenEconomyBothHalvesReportTheSameFixtureStore(t *testing.T) {
	root := tokenStore(t)
	script, command := tokenBothHalves(t, root)
	tokenAgree(t, "a fixture store", script, command)

	// The test asserts the absolute figures.
	// It does not only compare them.
	// This store has 19 API calls after the resumed copy and fork are deduplicated.
	// The main threads have 16 calls, and the subagents have 3.
	// The two records of msg_a1 are one call.
	// Its resumed-session copy is the same call, and the fork repeats its parent's call.
	page := command.Stdout
	if got := tokenFigure(t, page, "API calls:"); got != 19 {
		t.Errorf("the report counted %d API calls, want 19", got)
	}
	if got := tokenFigure(t, page, "Sessions:"); got != 3 {
		t.Errorf("the report counted %d sessions, want 3", got)
	}
	if got := tokenFigure(t, page, "Subagents:"); got != 4 {
		t.Errorf("the report counted %d subagents, want 4", got)
	}
	for _, want := range []string{
		"Context histogram: where the context tokens were fed",
		"Context histogram, main thread against subagents",
		"Capped-context counterfactual",
		"Subagent phases, keyed on the spawn description",
		"Subagent harness floor by agent type",
		"Tool calls per API call",
		"Tool results fed back, by tool",
		"Repeated reads and the main thread's tool mix",
		"other tools",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the fixture did not reach the %q table, so it proves nothing about it", want)
		}
	}
}

// TestTokenEconomyBothHalvesGiveAResumedSessionsCopiesBack is the store-wide
// dedup, seen from the page. Without it the per-session table shows the
// original's calls on the copy's row too.
func TestTokenEconomyBothHalvesGiveAResumedSessionsCopiesBack(t *testing.T) {
	root := tokenStore(t)
	script, command := tokenBothHalves(t, root)
	tokenAgree(t, "a resumed session", script, command)

	// bbb22222 copied one of mmm55555's calls and made one of its own.
	for line := range strings.SplitSeq(command.Stdout, "\n") {
		if !strings.HasPrefix(line, "| bbb22222 |") {
			continue
		}
		cells := strings.Split(line, "|")
		if strings.TrimSpace(cells[2]) != "1" {
			t.Errorf("the resumed session claims %s calls, want its own 1: %s", strings.TrimSpace(cells[2]), line)
		}
		return
	}
	t.Fatalf("the per-session table has no row for the resumed session:\n%s", command.Stdout)
}

// TestTokenEconomyBothHalvesHonourTheSessionFilter. The startup-context
// comparison between two agent types is only valid inside one session, so the
// filter is what makes that table citable.
func TestTokenEconomyBothHalvesHonourTheSessionFilter(t *testing.T) {
	root := tokenStore(t)
	script, command := tokenBothHalves(t, root, "session", "mmm")
	tokenAgree(t, "a session filter", script, command)

	if !strings.Contains(command.Stdout, "Session: mmm (1 matched)") {
		t.Errorf("the filtered page does not name its filter:\n%s", command.Stdout)
	}
	if strings.Contains(command.Stdout, "| bbb22222 |") {
		t.Errorf("the filter let a session through that does not carry the prefix:\n%s", command.Stdout)
	}
	if got := tokenFigure(t, command.Stdout, "Sessions:"); got != 1 {
		t.Errorf("the filtered report counted %d sessions, want 1", got)
	}
}

// TestTokenEconomyBothHalvesRefuseNothingForAnUnmatchedFilter. A prefix nobody
// carries is a clean skip with the prefix quoted back, never an empty report.
func TestTokenEconomyBothHalvesRefuseNothingForAnUnmatchedFilter(t *testing.T) {
	root := tokenStore(t)
	script, command := tokenBothHalves(t, root, "session", "zzz")
	tokenAgree(t, "an unmatched session filter", script, command)

	if !strings.Contains(command.Stdout, "No session id starts with 'zzz'.") {
		t.Errorf("the page does not quote the prefix back:\n%s", command.Stdout)
	}
	if script.Code != 0 {
		t.Errorf("an unmatched filter exited %d, want 0", script.Code)
	}
}

// TestTokenEconomyBothHalvesSayAnAbsentStoreIsAbsent. A table of zeros here
// would read as "this work is free".
func TestTokenEconomyBothHalvesSayAnAbsentStoreIsAbsent(t *testing.T) {
	script, command := tokenBothHalves(t, t.TempDir())
	tokenAgree(t, "an absent store", script, command)

	if !strings.Contains(command.Stdout, "Found no transcript store there") {
		t.Errorf("the page does not say the store is absent:\n%s", command.Stdout)
	}
	if script.Code != 0 || command.Code != 0 {
		t.Errorf("an absent store exited %d and %d, want 0", script.Code, command.Code)
	}
}

// TestTokenEconomyBothHalvesSayAnEmptyStoreIsEmpty is the second skip: the
// directory exists and holds no transcript with a recorded API call.
func TestTokenEconomyBothHalvesSayAnEmptyStoreIsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, tokenSlug), 0o750); err != nil {
		t.Fatalf("empty store: %v", err)
	}
	script, command := tokenBothHalves(t, root)
	tokenAgree(t, "an empty store", script, command)

	if !strings.Contains(command.Stdout, "no transcript with a recorded API call") {
		t.Errorf("the page does not say the store is empty:\n%s", command.Stdout)
	}
}

// TestTokenEconomyBothHalvesHonourTheCapAndTheTop. Both are bounds a caller
// passes, and both change the page.
func TestTokenEconomyBothHalvesHonourTheCapAndTheTop(t *testing.T) {
	root := tokenStore(t)

	script, command := tokenBothHalves(t, root, "cap", "150000", "top", "2")
	tokenAgree(t, "a cap and a top", script, command)

	if !strings.Contains(command.Stdout, "cap 150,000 tokens") {
		t.Errorf("the counterfactual did not take the cap:\n%s", command.Stdout)
	}
	if !strings.Contains(command.Stdout, "(top 2 of 3)") {
		t.Errorf("the per-session table did not take the top:\n%s", command.Stdout)
	}
	// The cap has to CHANGE the counterfactual, or the case would pass over a
	// store whose every call is under it.
	wide, _ := tokenBothHalves(t, root, "cap", "1000000")
	if wide.Stdout == script.Stdout {
		t.Error("the same page came back under two different caps, so the cap is not read")
	}
}

// TestTokenEconomyScriptStillDropsAnUnreadableTranscript pins the fail-open the
// port FIXED and the script still carries.
//
// `iter_records` answers an empty generator for any OSError.
// A transcript that cannot be opened contributes no calls.
// Every figure below becomes SMALLER, and the tool prints nothing.
// A lower number is what passing looks like.
//
// The case asserts that the SCRIPT still fails open.
// It becomes red when somebody repairs the script.
// The answer then is to delete the case with the script.
func TestTokenEconomyScriptStillDropsAnUnreadableTranscript(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so nothing is unreadable here")
	}
	root := tokenStore(t)
	locked := filepath.Join(root, tokenSlug, "bbb22222.jsonl")

	full, _ := tokenBothHalves(t, root)
	fullCalls := tokenFigure(t, full.Stdout, "API calls:")

	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("making a transcript unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	if _, err := os.ReadFile(locked); err == nil {
		t.Skip("this filesystem ignores the mode, so nothing is unreadable here")
	}

	var script, command devPyResult
	warnings := tokenWarnings(t, func() {
		script, command = tokenBothHalves(t, root)
	})

	brokenCalls := tokenFigure(t, script.Stdout, "API calls:")
	if brokenCalls >= fullCalls {
		t.Fatalf("the unreadable transcript cost the script nothing (%d against %d), so this case is watching nothing",
			brokenCalls, fullCalls)
	}
	if script.Stderr != "" {
		t.Errorf("the script has been repaired -- it now says something about the transcript it cannot read.\n"+
			"Delete this case with the script and close the journal row:\n%s", script.Stderr)
	}

	// The port names it. The page still describes what it COULD read, because
	// a report that refuses to print is worse than one that states its gap.
	if !strings.Contains(warnings, "unreadable transcript") {
		t.Errorf("the command dropped the transcript in silence too:\n%s", warnings)
	}
	if !strings.Contains(warnings, "bbb22222.jsonl") {
		t.Errorf("the warning does not name the transcript:\n%s", warnings)
	}
	if !strings.Contains(command.Stdout, "Token economy:") {
		t.Errorf("the command printed no report for the transcripts it COULD read:\n%s", command.Stdout)
	}
}

// tokenWarnings runs one function with os.Stderr redirected, and answers what
// was written there.
//
// The warning belongs to the tool, not the engine.
// leroot gives the tool no error writer.
// Thus, a refusal and a warning use the process stderr, as they do in every other le tool.
// Capturing stderr makes the fix above observable.
// A payload-only assertion would let a silent port pass.
func tokenWarnings(t *testing.T, run func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("capturing stderr: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write

	done := make(chan string, 1)
	go func() {
		body, _ := io.ReadAll(read)
		done <- string(body)
	}()

	run()

	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatalf("closing the capture: %v", err)
	}
	captured := <-done
	if err := read.Close(); err != nil {
		t.Fatalf("closing the capture: %v", err)
	}
	return captured
}

// TestTokenEconomyConstantsAgreeByValue compares constants that no page CAN compare.
// A bound, a bucket edge, a divisor, and a phase ranking are SHARED CONSTANTS.
// A fixture misses a one-sided change when it does not use the changed value.
// No output comparison covers the phase ranking because first-match-wins needs a description that matches twice.
func TestTokenEconomyConstantsAgreeByValue(t *testing.T) {
	const program = `
import json, sys
sys.path.insert(0, "scripts/dev")
import token_economy as t
json.dump({
    "cap-min": t.CAP_MIN, "cap-max": t.CAP_MAX,
    "top-min": t.TOP_MIN, "top-max": t.TOP_MAX,
    "default-cap": t.DEFAULT_CAP, "default-top": t.DEFAULT_TOP,
    "bucket-edges": list(t.BUCKET_EDGES),
    "chars-per-token": t.CHARS_PER_TOKEN,
    "result-table-rows": t.RESULT_TABLE_ROWS,
    "phase-rules": [[p, list(k)] for p, k in t.PHASE_RULES],
}, sys.stdout)
`
	out := tokenRunPython(t, program)

	var declared struct {
		CapMin          int     `json:"cap-min"`
		CapMax          int     `json:"cap-max"`
		TopMin          int     `json:"top-min"`
		TopMax          int     `json:"top-max"`
		DefaultCap      int     `json:"default-cap"`
		DefaultTop      int     `json:"default-top"`
		BucketEdges     []int64 `json:"bucket-edges"`
		CharsPerToken   float64 `json:"chars-per-token"`
		ResultTableRows int     `json:"result-table-rows"`
		PhaseRules      [][]any `json:"phase-rules"`
	}
	if err := json.Unmarshal([]byte(out), &declared); err != nil {
		t.Fatalf("reading the script's constants: %v: %s", err, out)
	}

	numbers := []struct {
		what           string
		script, ported int
	}{
		{"CAP_MIN", declared.CapMin, tokeneconomy.CapMin},
		{"CAP_MAX", declared.CapMax, tokeneconomy.CapMax},
		{"TOP_MIN", declared.TopMin, tokeneconomy.TopMin},
		{"TOP_MAX", declared.TopMax, tokeneconomy.TopMax},
		{"DEFAULT_CAP", declared.DefaultCap, tokeneconomy.DefaultCap},
		{"DEFAULT_TOP", declared.DefaultTop, tokeneconomy.DefaultTop},
		{"RESULT_TABLE_ROWS", declared.ResultTableRows, tokeneconomy.ResultTableRows},
	}
	for _, item := range numbers {
		if item.script != item.ported {
			t.Errorf("%s is %d in the script and %d in the port", item.what, item.script, item.ported)
		}
	}
	if declared.CharsPerToken != tokeneconomy.CharsPerToken {
		t.Errorf("CHARS_PER_TOKEN is %v in the script and %v in the port",
			declared.CharsPerToken, tokeneconomy.CharsPerToken)
	}
	if len(declared.BucketEdges) != len(tokeneconomy.BucketEdges) {
		t.Fatalf("the script has %d bucket edges and the port %d",
			len(declared.BucketEdges), len(tokeneconomy.BucketEdges))
	}
	for i, edge := range declared.BucketEdges {
		if edge != tokeneconomy.BucketEdges[i] {
			t.Errorf("bucket edge %d is %d in the script and %d in the port",
				i, edge, tokeneconomy.BucketEdges[i])
		}
	}

	if len(declared.PhaseRules) != len(tokeneconomy.PhaseRules) {
		t.Fatalf("the script declares %d phases and the port %d",
			len(declared.PhaseRules), len(tokeneconomy.PhaseRules))
	}
	for i, rule := range declared.PhaseRules {
		phase, _ := rule[0].(string)
		if phase != tokeneconomy.PhaseRules[i].Phase {
			t.Errorf("phase %d is %q in the script and %q in the port: the order IS the ranking",
				i, phase, tokeneconomy.PhaseRules[i].Phase)
			continue
		}
		raw, _ := rule[1].([]any)
		if len(raw) != len(tokeneconomy.PhaseRules[i].Keywords) {
			t.Errorf("phase %q has %d keywords in the script and %d in the port",
				phase, len(raw), len(tokeneconomy.PhaseRules[i].Keywords))
			continue
		}
		for j, word := range raw {
			if word != tokeneconomy.PhaseRules[i].Keywords[j] {
				t.Errorf("phase %q keyword %d is %q in the script and %q in the port",
					phase, j, word, tokeneconomy.PhaseRules[i].Keywords[j])
			}
		}
	}
}

// TestBothHalvesResolveTheSameRealStore is the third part of the comparison.
// No fixture CAN tell where both halves look when no caller names a store.
// Every case above names one.
func TestBothHalvesResolveTheSameRealStore(t *testing.T) {
	const program = `
import sys
sys.path.insert(0, "scripts/dev")
import token_economy as t
print(t.DEFAULT_ROOT)
print(t.slug_for_path(t.REPO_ROOT))
`
	lines := strings.Split(strings.TrimRight(tokenRunPython(t, program), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("the script did not answer a root and a slug: %q", lines)
	}

	root, err := tokeneconomy.DefaultRoot()
	if err != nil {
		t.Fatalf("resolving the store root: %v", err)
	}
	if root != lines[0] {
		t.Errorf("the store root is %q in the script and %q in the port", lines[0], root)
	}
	project, err := tokeneconomy.DefaultProject()
	if err != nil {
		t.Fatalf("resolving the project slug: %v", err)
	}
	if project != lines[1] {
		t.Errorf("the project slug is %q in the script and %q in the port", lines[1], project)
	}
}

// TestTokenEconomyBothHalvesReportTheSameSnapshotOfTheRealStore is the second
// part: real transcripts, held still.
//
// The real store cannot be compared in place.
// It was 1.4 GB on the machine used for this port.
// Another program APPENDS during the test, and each developer has a different store.
// Thus, the test builds a bounded snapshot from symlinks to transcripts unchanged for ten minutes.
// Both halves read that snapshot.
//
// A machine with no store skips, with the reason said. Skipping is honest here:
// the corpus is machine-local and the fixture cases above carry the proof.
func TestTokenEconomyBothHalvesReportTheSameSnapshotOfTheRealStore(t *testing.T) {
	root, project, err := tokenRealStore()
	if err != nil {
		t.Skipf("no transcript store on this machine, so there is nothing real to freeze: %v", err)
	}

	snapshot, files := tokenSnapshot(t, filepath.Join(root, project))
	if files < 3 {
		t.Skipf("only %d settled transcripts in the store, which is too few to compare", files)
	}

	script, command := tokenBothHalves(t, snapshot)
	tokenAgree(t, "a frozen snapshot of the real store", script, command)

	// Absolute, not only equal. Two halves that both read nothing agree.
	page := command.Stdout
	if got := tokenFigure(t, page, "API calls:"); got < 50 {
		t.Errorf("the snapshot carried %d API calls, which is too few to have exercised anything", got)
	}
	if got := tokenFigure(t, page, "Sessions:"); got < 3 {
		t.Errorf("the snapshot carried %d sessions, want at least 3", got)
	}
	if !strings.Contains(page, "Tool results fed back, by tool") {
		t.Errorf("the snapshot reached no tool table:\n%s", page)
	}
}

// tokenRealStore answers the root and the project slug of this machine's store,
// or an error when it has none.
func tokenRealStore() (string, string, error) {
	root, err := tokeneconomy.DefaultRoot()
	if err != nil {
		return "", "", err
	}
	project, err := tokeneconomy.DefaultProject()
	if err != nil {
		return "", "", err
	}
	if info, err := os.Stat(filepath.Join(root, project)); err != nil || !info.IsDir() {
		return "", "", os.ErrNotExist
	}
	return root, project, nil
}

// tokenSnapshot freezes part of a real store under a temporary root and answers
// that root and how many transcripts it took.
//
// Symlinks rather than copies: the store runs to gigabytes. The DIRECTORIES are
// real, so both halves glob the same tree, and only the files are links.
//
// Two rules keep the corpus still and bound the snapshot.
// A transcript written in the last ten minutes belongs to a live session.
// That transcript CAN change during the comparison.
// A transcript over the size ceiling would make the test slower than the gate it proves.
func tokenSnapshot(t *testing.T, store string) (string, int) {
	t.Helper()

	const (
		settled  = 10 * time.Minute
		maxBytes = 4 << 20
		maxFiles = 8
	)

	paths, err := filepath.Glob(filepath.Join(store, "*.jsonl"))
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	sort.Strings(paths)

	root := t.TempDir()
	snapshot := filepath.Join(root, tokenSlug)
	if err := os.MkdirAll(snapshot, 0o750); err != nil {
		t.Fatalf("snapshot directory: %v", err)
	}

	taken := 0
	for _, path := range paths {
		if taken == maxFiles {
			break
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() > maxBytes || time.Since(info.ModTime()) < settled {
			continue
		}
		link(t, path, filepath.Join(snapshot, filepath.Base(path)))
		taken++

		// The subagents beside it, when the session has any. They are the only
		// source of the phase and agent-type tables.
		sid := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		subs, _ := filepath.Glob(filepath.Join(store, sid, "subagents", "*"))
		sort.Strings(subs)
		for _, sub := range subs {
			info, err := os.Stat(sub)
			if err != nil || info.IsDir() || info.Size() > maxBytes || time.Since(info.ModTime()) < settled {
				continue
			}
			dir := filepath.Join(snapshot, sid, "subagents")
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatalf("snapshot subagent directory: %v", err)
			}
			link(t, sub, filepath.Join(dir, filepath.Base(sub)))
		}
	}
	return root, taken
}

// tokenRunPython runs a short Python program from the checkout directory.
// The program CAN import the script as a MODULE and answer its constants.
// Only a comparison of the declarations detects a change to a shared bound.
// That comparison needs values instead of a page.
func tokenRunPython(t *testing.T, program string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", program)
	cmd.Dir = devPyRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reading the script's declarations: %v", err)
	}
	return string(out)
}

func link(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Symlink(from, to); err != nil {
		t.Fatalf("freezing %s: %v", from, err)
	}
}
