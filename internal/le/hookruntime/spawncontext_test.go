// Related: spawncontext.go -- the context-size notice at an agent spawn
//
// VALIDATES: the after-hook of the main thread's Agent call tells it its last
// recorded context size, and to write the state file and hand off, once that
// size is past the 600k threshold, and writes the spawn marker either way.
// PREVENTS: a notice on every spawn, a zero printed for a transcript the hook
// could not read, a spawn refused because of the transcript, a notice to an
// agent for its own spawn, and a whole read of a transcript of any size.
package hookruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/ai/tokens/contextsize"
)

// usageLine is one assistant record whose context is input+cacheWrite+cacheRead.
func usageLine(input, cacheWrite, cacheRead int64) string {
	return `{"type":"assistant","message":{"id":"msg","usage":{"input_tokens":` + strconv.FormatInt(input, 10) +
		`,"cache_creation_input_tokens":` + strconv.FormatInt(cacheWrite, 10) +
		`,"cache_read_input_tokens":` + strconv.FormatInt(cacheRead, 10) + `,"output_tokens":40000}}}` + "\n"
}

func spawnTranscript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func spawnPayload(transcript string) map[string]any {
	return map[string]any{
		"session_id": "parent-17", "tool_name": "Agent", "transcript_path": transcript,
		"tool_input": map[string]any{"subagent_type": "ze-work", "prompt": "package 9"},
	}
}

// runSpawn runs the after-hook and answers its stdout's additional context, or
// "" when it wrote nothing. It fails the test on any exit but 0, on output it
// cannot decode, and when the spawn marker is missing.
func runSpawn(t *testing.T, payload map[string]any) string {
	t.Helper()
	root := t.TempDir()
	code, stdout, stderr := runHook(t, root, "mark-agent-spawned", payload)
	if code != 0 {
		t.Fatalf("mark-agent-spawned exit = %d, want 0 (a spawn is never refused); stderr=%q", code, stderr)
	}
	markers, err := filepath.Glob(filepath.Join(root, "tmp", "session", ".agent-spawned-*"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("spawn markers = %v, %v; want one", markers, err)
	}
	if stdout == "" {
		return ""
	}
	var response struct {
		Output struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("stdout is not a hook answer: %v: %q", err, stdout)
	}
	if response.Output.Event != "PostToolUse" {
		t.Fatalf("hookEventName = %q, want PostToolUse", response.Output.Event)
	}
	if response.Output.Context == "" {
		t.Fatalf("hook answer with no additional context: %q", stdout)
	}
	return response.Output.Context
}

func TestAgentSpawnReportsContextPastTheHandoffThreshold(t *testing.T) {
	transcript := spawnTranscript(t, usageLine(1, 12000, 40000)+usageLine(9, 21000, 692000))
	notice := runSpawn(t, spawnPayload(transcript))
	for _, want := range []string{
		"last recorded context is 713k tokens",
		"past the 600k handoff threshold",
		"ai/rules/planning.md",
		"./le spec state current",
		"One spec per session: the next spec starts in a fresh session.",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice lacks %q:\n%s", want, notice)
		}
	}
}

func TestAgentSpawnIsSilentAtOrBelowTheHandoffThreshold(t *testing.T) {
	for _, tokens := range []int64{45000, handoffContextTokens} {
		transcript := spawnTranscript(t, usageLine(0, 0, tokens))
		if notice := runSpawn(t, spawnPayload(transcript)); notice != "" {
			t.Errorf("context %d: notice %q, want silence", tokens, notice)
		}
	}
	transcript := spawnTranscript(t, usageLine(0, 0, handoffContextTokens+1))
	if notice := runSpawn(t, spawnPayload(transcript)); !strings.Contains(notice, "600k tokens, past") {
		t.Errorf("context %d: notice %q, want the handoff notice", handoffContextTokens+1, notice)
	}
}

func TestAgentSpawnIsSilentWhenTheContextCannotBeRead(t *testing.T) {
	cases := map[string]string{
		"absent transcript":   filepath.Join(t.TempDir(), "absent.jsonl"),
		"no transcript path":  "",
		"no usage record":     spawnTranscript(t, `{"type":"user","message":{"content":"hello"}}`+"\n"),
		"zero usage record":   spawnTranscript(t, usageLine(0, 0, 0)),
		"truncated record":    spawnTranscript(t, `{"type":"assistant","message":{"usage":{"input_tokens":900000`),
		"not a json document": spawnTranscript(t, "\x00\x01 not json\n"),
	}
	for name, transcript := range cases {
		if notice := runSpawn(t, spawnPayload(transcript)); notice != "" {
			t.Errorf("%s: notice %q, want silence and a passing spawn", name, notice)
		}
	}
}

func TestAnAgentsOwnSpawnGetsNoContextNotice(t *testing.T) {
	payload := spawnPayload(spawnTranscript(t, usageLine(0, 0, 900000)))
	payload["agent_id"] = "a1b2c3"
	payload["agent_type"] = "ze-work"
	if notice := runSpawn(t, payload); notice != "" {
		t.Fatalf("agent spawn: notice %q, want none: the threshold is the main thread's", notice)
	}
}

// The head holds a record past the threshold and the tail none, so a notice
// would mean the hook read past contextsize.TailBytes; the same file with a
// record appended proves a large transcript is still read at its tail.
func TestAgentSpawnReadsOnlyTheTranscriptTail(t *testing.T) {
	filler := `{"type":"user","message":{"content":"` + strings.Repeat("r", 8000) + `"}}` + "\n"
	var body strings.Builder
	body.WriteString(usageLine(0, 0, 950000))
	for body.Len() < 4*contextsize.TailBytes {
		body.WriteString(filler)
	}
	if notice := runSpawn(t, spawnPayload(spawnTranscript(t, body.String()))); notice != "" {
		t.Fatalf("notice %q from a record outside the tail, want silence", notice)
	}
	body.WriteString(usageLine(0, 0, 720000))
	notice := runSpawn(t, spawnPayload(spawnTranscript(t, body.String())))
	if !strings.Contains(notice, "720k tokens") {
		t.Fatalf("notice %q, want the 720k record at the tail of a large transcript", notice)
	}
}
