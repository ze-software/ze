// Unit coverage for the token-economy report.
//
// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-7 for `token_economy.py`.
// Functions run every case without a subprocess.
// PREVENTS: record-counted API calls, per-file store deduplication, and byte-based
// character bounds.

package tokeneconomy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStore writes one transcript store under a temporary root and answers the
// store directory. Keys are paths relative to the store.
func writeStore(t *testing.T, files map[string]string) string {
	t.Helper()
	store := filepath.Join(t.TempDir(), "-proj")
	for rel, body := range files {
		path := filepath.Join(store, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return store
}

// assistant renders one assistant record of a transcript.
func assistant(sid, id string, read, write, output int, extra string) string {
	var b strings.Builder
	b.WriteString(`{"type":"assistant","session_id":"`)
	b.WriteString(sid)
	b.WriteString(`","sessionId":"`)
	b.WriteString(sid)
	b.WriteString(`","message":{"id":"`)
	b.WriteString(id)
	b.WriteString(`","usage":{"input_tokens":10,"cache_read_input_tokens":`)
	b.WriteString(itoa(read))
	b.WriteString(`,"cache_creation_input_tokens":`)
	b.WriteString(itoa(write))
	b.WriteString(`,"output_tokens":`)
	b.WriteString(itoa(output))
	b.WriteString(`}`)
	b.WriteString(extra)
	b.WriteString("}}\n")
	return b.String()
}

func itoa(v int) string { return comma2(v) }

// comma2 renders a whole number with no separators, for a JSON literal.
func comma2(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits //nolint:gocritic // a test's own literal
		v /= 10
	}
	return sign + digits
}

// TestScanCountsOneAPICallPerMessageIdNotPerRecord verifies the central rule.
// One call produces several assistant records with repeated context. Record
// counting would multiply every context figure.
func TestScanCountsOneAPICallPerMessageIdNotPerRecord(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": assistant("s1", "msg_a", 1000, 0, 5, "") +
			assistant("s1", "msg_a", 1000, 0, 40, "") +
			assistant("s1", "msg_a", 1000, 0, 99, ""),
	})

	sessions, _ := FindSessions(store)
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	agg := Aggregate(sessions[0].AllCalls())
	if agg.Calls != 1 {
		t.Errorf("three records of one message id counted as %d API calls, want 1", agg.Calls)
	}
	if agg.Context != 1010 {
		t.Errorf("context fed reads %d, want 1010: the repeated records were summed", agg.Context)
	}
	if agg.Output != 99 {
		t.Errorf("output reads %d, want the finished 99: only the last record carries it", agg.Output)
	}
}

// TestScanIgnoresTheRecordUuidAsADedupKey pins the key. The uuid is per RECORD,
// so keying on it would restore the double count the case above removes.
func TestScanIgnoresTheRecordUuidAsADedupKey(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": strings.ReplaceAll(assistant("s1", "msg_a", 500, 0, 1, ""), `{"type":"assistant"`, `{"uuid":"u1","type":"assistant"`) +
			strings.ReplaceAll(assistant("s1", "msg_a", 500, 0, 2, ""), `{"type":"assistant"`, `{"uuid":"u2","type":"assistant"`),
	})

	sessions, _ := FindSessions(store)
	if got := len(sessions[0].AllCalls()); got != 1 {
		t.Errorf("two uuids of one message id counted as %d calls, want 1", got)
	}
}

// TestScanSkipsALineThatDoesNotParseAndKeepsTheRest is the truncated final line
// of a session still running. The absolute count is asserted, because two
// halves that both stopped early agree on nothing.
func TestScanSkipsALineThatDoesNotParseAndKeepsTheRest(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": assistant("s1", "msg_a", 100, 0, 1, "") +
			"not json at all\n" +
			`{"type":"assistant","message":` + "\n" +
			assistant("s1", "msg_b", 200, 0, 1, "") +
			`[1,2,3]` + "\n" +
			assistant("s1", "msg_c", 300, 0, 1, ""),
	})

	sessions, _ := FindSessions(store)
	if got := len(sessions[0].AllCalls()); got != 3 {
		t.Errorf("three parseable records produced %d calls, want 3", got)
	}
}

// TestAnUnreadableTranscriptIsNamedRatherThanDropped verifies a fixed fail-open.
// The script swallows the open error and silently reduces the report. A LOWER
// figure then falsely passes.
func TestAnUnreadableTranscriptIsNamedRatherThanDropped(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": assistant("s1", "msg_a", 100, 0, 1, ""),
		"s2.jsonl": assistant("s2", "msg_b", 200, 0, 1, ""),
	})
	if err := os.Chmod(filepath.Join(store, "s2.jsonl"), 0o000); err != nil {
		t.Fatalf("making a transcript unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(store, "s2.jsonl"), 0o600) })
	if _, err := os.ReadFile(filepath.Join(store, "s2.jsonl")); err == nil {
		t.Skip("this filesystem or user ignores the mode, so nothing is unreadable here")
	}

	sessions, unreadable := FindSessions(store)
	if len(unreadable) != 1 || !strings.HasSuffix(unreadable[0], "s2.jsonl") {
		t.Fatalf("the unreadable transcript was not named: %v", unreadable)
	}
	// The report describes readable data and names the unreadable gap. Silence,
	// not the smaller figure, is the defect.
	if len(sessions) != 2 {
		t.Errorf("want both sessions present, got %d", len(sessions))
	}
	report, code := Run(Options{Root: filepath.Dir(store), Project: "-proj", Cap: DefaultCap, Top: DefaultTop})
	if code != 0 {
		t.Errorf("the report exited %d; it is a report and never blocks", code)
	}
	if len(report.Unreadable) != 1 {
		t.Errorf("the payload named %d unreadable transcripts, want 1", len(report.Unreadable))
	}
}

// TestAResumedSessionDoesNotRecountTheOriginalsCalls verifies store-wide dedup.
// A resumed session COPIES prior records into its own file. Per-file dedup counts
// them twice.
//
// The COPY sorts FIRST. Collection order is the final tie-break, so this fixture
// cannot pass only because the original was read first.
func TestAResumedSessionDoesNotRecountTheOriginalsCalls(t *testing.T) {
	original := assistant("s2", "msg_a", 1000, 0, 1, "")
	store := writeStore(t, map[string]string{
		// The copy changes its file id but keeps the maker id, which identifies
		// the original.
		"s1.jsonl": strings.ReplaceAll(original, `"sessionId":"s2"`, `"sessionId":"s1"`) +
			assistant("s1", "msg_b", 2000, 0, 1, ""),
		"s2.jsonl": original,
	})

	sessions, _ := FindSessions(store)
	total := 0
	byID := map[string]int{}
	for _, session := range sessions {
		total += len(session.AllCalls())
		byID[session.SID] = len(session.AllCalls())
	}
	if total != 2 {
		t.Errorf("one call copied into a resumed session counted %d times, want 2 calls store-wide", total)
	}
	if byID["s1"] != 1 || byID["s2"] != 1 {
		t.Errorf("the copied call did not go back to the session that MADE it: %v", byID)
	}
}

// TestAForkedAgentsCallsStayWithTheParent verifies fork copies. A fork inherits
// the parent conversation, and its metadata identifies that inheritance.
//
// The FORK sorts first. Both transcripts share a session directory, so only the
// fork flag separates them.
func TestAForkedAgentsCallsStayWithTheParent(t *testing.T) {
	shared := assistant("s1", "msg_a", 1000, 0, 1, "")
	store := writeStore(t, map[string]string{
		"s1.jsonl":                       assistant("s1", "msg_root", 10, 0, 1, ""),
		"s1/subagents/agent-1.jsonl":     shared,
		"s1/subagents/agent-1.meta.json": `{"agentType":"ze-work","description":"port a tool","isFork":true}`,
		"s1/subagents/agent-2.jsonl":     shared,
		"s1/subagents/agent-2.meta.json": `{"agentType":"ze-work","description":"port a tool"}`,
		"s1/subagents/agent-3.jsonl":     assistant("s1", "msg_c", 7, 0, 1, ""),
		"s1/subagents/agent-3.meta.json": `{"agentType":"ze-work","description":"port a tool","parentAgentId":"agent-2"}`,
	})

	sessions, _ := FindSessions(store)
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	if got := len(sessions[0].AllCalls()); got != 3 {
		t.Errorf("the fork recounted its parent's call: %d calls store-wide, want 3", got)
	}
	byName := map[string]int{}
	for _, agent := range sessions[0].Agents {
		byName[agent.Name] = len(agent.Calls)
	}
	if byName["agent-2"] != 1 || byName["agent-1"] != 0 {
		t.Errorf("the shared call did not stay with the agent that was not a fork: %v", byName)
	}
	if byName["agent-3"] != 1 {
		t.Errorf("an agent whose meta names a parent lost its OWN call: %v", byName)
	}
}

// TestHarnessFloorSubtractsTheSpawnPrompt makes the floor a property of the
// agent TYPE. The raw first call moves with each spawn's prompt length, so a
// median over it drifts as a live session grows.
func TestHarnessFloorSubtractsTheSpawnPrompt(t *testing.T) {
	agent := Agent{Calls: []Call{{CacheRead: 100_000}}, PromptChars: 3600}
	if got := agent.HarnessFloor(); got != 99_000 {
		t.Errorf("floor reads %d, want 99000: 3600 characters is 1000 tokens", got)
	}

	bigger := Agent{Calls: []Call{{CacheRead: 100}}, PromptChars: 36_000}
	if got := bigger.HarnessFloor(); got != 0 {
		t.Errorf("a prompt larger than the first call gave a floor of %d, want 0", got)
	}
}

// TestPhaseRulesRankReviewAboveTheWordsThatFollowIt pins FIRST MATCH WINS. A
// review agent's description usually holds `check` and `read` too, and the
// order of the table is what stops it landing in research.
func TestPhaseRulesRankReviewAboveTheWordsThatFollowIt(t *testing.T) {
	cases := []struct {
		description, agentType, want string
	}{
		{"review the implementation and check it reads correctly", "", "review"},
		{"implement the port", "", "implement"},
		{"find the producer", "", "research"},
		// The agent TYPE carries a keyword and the description is empty. A
		// phase read from the type alone would label every spawn of this type
		// "review", which is the classification this guard exists to refuse.
		{"", "ze-review", Unclassified},
		{"something nobody named", "", Unclassified},
	}
	for _, item := range cases {
		if got := PhaseOf(item.description, item.agentType); got != item.want {
			t.Errorf("PhaseOf(%q, %q) = %q, want %q", item.description, item.agentType, got, item.want)
		}
	}
}

// TestSlugCountsCharactersNotBytes verifies the rune bound. One accented letter
// becomes one store-name hyphen. A byte walk would produce two and name a
// nonexistent directory.
func TestSlugCountsCharactersNotBytes(t *testing.T) {
	if got := SlugForPath("/home/thomas/Code/ze"); got != "-home-thomas-Code-ze" {
		t.Errorf("SlugForPath answered %q", got)
	}
	if got := SlugForPath("/hé/ze"); got != "-h--ze" {
		t.Errorf("an accented letter produced %q, want %q: it is one character", got, "-h--ze")
	}
}

// TestResultCharsCountsCharactersNotBytes is the same bound over a tool result,
// which is where the token approximation is taken. A four-byte emoji is ONE
// character, and counting it as four inflates the tool's whole row.
func TestResultCharsCountsCharactersNotBytes(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": assistant("s1", "msg_a", 10, 0, 1, `,"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"a.go"}}]`) +
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"😀😀😀"}]}}` + "\n",
	})

	sessions, _ := FindSessions(store)
	tools := sessions[0].AllToolCalls()
	if len(tools) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(tools))
	}
	if tools[0].ResultChars != 3 {
		t.Errorf("three emoji measured %d characters, want 3: the count is runes, not the 12 bytes", tools[0].ResultChars)
	}
}

// TestTheSessionTableNamesTheFirstEightCharactersOfASid is the third rune
// bound. A byte slice would cut a multi-byte id in half and print a replacement
// character where the store holds a letter.
func TestTheSessionTableNamesTheFirstEightCharactersOfASid(t *testing.T) {
	if got := runePrefix("ééééééééXX", sidWidth); got != "ééééééé é"[:len("ééééééé")]+"é" {
		t.Errorf("runePrefix answered %q", got)
	}
	if got := len([]rune(runePrefix("ééééééééXX", sidWidth))); got != sidWidth {
		t.Errorf("the printed id is %d characters, want %d", got, sidWidth)
	}
}

// TestHistogramAttributesContextToTheCallThatFedIt pins the edges: a call
// EQUAL to an edge lands in that bucket, and the last bucket is open.
func TestHistogramAttributesContextToTheCallThatFedIt(t *testing.T) {
	buckets := Histogram([]Call{
		{CacheRead: 50_000},    // on the first edge
		{CacheRead: 50_001},    // just over it
		{CacheRead: 2_000_000}, // past the last edge
	})
	if buckets[0].Calls != 1 || buckets[0].Context != 50_000 {
		t.Errorf("a call ON the first edge did not land in the first bucket: %+v", buckets[0])
	}
	if buckets[1].Calls != 1 {
		t.Errorf("a call one token over the first edge did not land in the second bucket: %+v", buckets[1])
	}
	last := buckets[len(buckets)-1]
	if last.Calls != 1 || last.Context != 2_000_000 {
		t.Errorf("the open bucket did not take the largest call: %+v", last)
	}
	if len(buckets) != len(BucketEdges)+1 {
		t.Errorf("%d buckets for %d edges, want one more than the edges", len(buckets), len(BucketEdges))
	}
}

// TestCappedCounterfactualIsArithmeticOverTheCallsThatHappened pins that the
// ceiling is applied per CALL, and that an empty corpus divides by nothing.
func TestCappedCounterfactualIsArithmeticOverTheCallsThatHappened(t *testing.T) {
	got := CappedCounterfactual([]Call{{CacheRead: 300}, {CacheRead: 50}}, 100)
	if got.Real != 350 || got.Capped != 150 {
		t.Errorf("want real 350 and capped 150, got %+v", got)
	}
	if empty := CappedCounterfactual(nil, 100); empty.Share != 0 {
		t.Errorf("an empty call list divided by zero: %+v", empty)
	}
}

// TestMedianOfAnEvenCountIsTheMidpointNotTheUpper. Taking the upper of the two
// middles reports a floor nobody paid.
func TestMedianOfAnEvenCountIsTheMidpointNotTheUpper(t *testing.T) {
	if got := median([]int64{10, 20, 30, 41}); got != 25 {
		t.Errorf("median reads %d, want 25", got)
	}
	if got := median([]int64{10, 20, 30}); got != 20 {
		t.Errorf("median of an odd count reads %d, want 20", got)
	}
	if got := median(nil); got != 0 {
		t.Errorf("an empty sample answered %d", got)
	}
}

// TestResultSizesOrderTiesByFirstAppearance. Two tools that fed back nothing
// have the same total, and a random order there makes the whole table
// irreproducible.
func TestResultSizesOrderTiesByFirstAppearance(t *testing.T) {
	rows := ResultSizes([]ToolCall{
		{Name: "Alpha"}, {Name: "Beta"}, {Name: "Gamma", ResultChars: 36},
	})
	if len(rows) != 3 || rows[0].Name != "Gamma" {
		t.Fatalf("the biggest total is not first: %+v", rows)
	}
	if rows[1].Name != "Alpha" || rows[2].Name != "Beta" {
		t.Errorf("a tie did not keep first-seen order: %+v", rows)
	}
	if rows[0].Tokens != 10 || rows[0].MeanTokens != 10 {
		t.Errorf("36 characters is 10 tokens at %v per token: %+v", CharsPerToken, rows[0])
	}
}

// TestRepeatReadsCountsOnlyAFileTheWindowAlreadyHeld pins the population: the
// read tool, and only a call that named a file.
func TestRepeatReadsCountsOnlyAFileTheWindowAlreadyHeld(t *testing.T) {
	got := RepeatReads([]ToolCall{
		{Name: "Read", FilePath: "a.go"},
		{Name: "Read", FilePath: "a.go"},
		{Name: "Read", FilePath: "b.go"},
		{Name: "Read"},
		{Name: "Bash", FilePath: "a.go"},
	})
	if got.Reads != 3 || got.Repeats != 1 {
		t.Errorf("want 3 reads and 1 repeat, got %+v", got)
	}
}

// TestAnAbsentStoreIsSaidRatherThanReportedAsZero. A table of zeros here would
// read as "this work is free".
func TestAnAbsentStoreIsSaidRatherThanReportedAsZero(t *testing.T) {
	report, code := Run(Options{Root: t.TempDir(), Project: "-nothing", Cap: DefaultCap, Top: DefaultTop})
	if code != 0 {
		t.Errorf("an absent store exited %d, want 0", code)
	}
	if report.State != StateAbsent {
		t.Errorf("state reads %q, want %q", report.State, StateAbsent)
	}
	page := report.Text()
	if !strings.Contains(page, "Found no transcript store there") {
		t.Errorf("the page does not say the store is absent:\n%s", page)
	}
	if strings.Contains(page, "API calls: 0") {
		t.Errorf("the page reported zeros for a store it never read:\n%s", page)
	}
}

// TestReportIsStructuredDataWithKebabCaseKeys verifies AC-7 in the payload.
// Pipe operators render this object, so keys cannot use Go spelling.
func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	store := writeStore(t, map[string]string{
		"s1.jsonl": assistant("s1", "msg_a", 1000, 5, 7, ""),
	})
	report, _ := Run(Options{Root: filepath.Dir(store), Project: "-proj", Cap: DefaultCap, Top: DefaultTop})

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("the payload does not decode: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("the payload is empty, so this case checks nothing")
	}
	for key := range decoded {
		if strings.ToLower(key) != key || strings.Contains(key, "_") {
			t.Errorf("payload key %q is not kebab-case", key)
		}
	}
	for _, want := range []string{"totals", "histogram", "capped", "store", "project"} {
		if _, ok := decoded[want]; !ok {
			t.Errorf("the payload carries no %q key: %v", want, keysOf(decoded))
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

// TestParseOptionsHoldsANumberToTheScriptsBounds. The bound is invisible to any
// output comparison whose fixture does not sit on it, so it is asserted here.
func TestParseOptionsHoldsANumberToTheScriptsBounds(t *testing.T) {
	accepted := [][]string{
		{"cap", "1"}, {"cap", "1000000"}, {"top", "1"}, {"top", "1000"},
	}
	for _, args := range accepted {
		if _, code := ParseOptions(args); code != 0 {
			t.Errorf("le token-economy %v was refused with %d, and it is inside the bound", args, code)
		}
	}
	refused := [][]string{
		{"cap", "0"}, {"cap", "1000001"}, {"top", "0"}, {"top", "1001"},
		{"cap", "many"}, {"nosuch", "1"}, {"cap"},
	}
	for _, args := range refused {
		if _, code := ParseOptions(args); code == 0 {
			t.Errorf("le token-economy %v was accepted, and it is outside the bound", args)
		}
	}
}

// TestAProjectSlugBeginningWithAHyphenIsAValue verifies the Go replacement for
// the script workaround. Absolute paths produce slugs that ALWAYS start with a
// hyphen. Keyword grammar makes the value unambiguous.
func TestAProjectSlugBeginningWithAHyphenIsAValue(t *testing.T) {
	opts, code := ParseOptions([]string{"project", "-home-thomas-Code-ze"})
	if code != 0 {
		t.Fatalf("a slug beginning with a hyphen was refused with %d", code)
	}
	if opts.Project != "-home-thomas-Code-ze" {
		t.Errorf("the slug reads %q", opts.Project)
	}
}

// TestDefaultsAreTheScriptsDefaults pins what a caller who names no keyword
// gets, which is what the gate runs with.
func TestDefaultsAreTheScriptsDefaults(t *testing.T) {
	opts := Defaults()
	if opts.Cap != DefaultCap || opts.Top != DefaultTop {
		t.Errorf("defaults read %+v, want cap %d and top %d", opts, DefaultCap, DefaultTop)
	}
}
