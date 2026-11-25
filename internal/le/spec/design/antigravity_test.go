// VALIDATES: `le spec design judge` works inside Antigravity (agy) by reading
// whole-door and section-read proof from the conversation's transcript,
// refusing unread sections, partial door reads, stale reads, other-checkout
// reads, self-edited specs, malformed transcripts and unsafe conversation ids,
// and allowing `le spec design approve` to consume the resulting judge record.
// PREVENTS: Antigravity sessions being locked out of `le spec design judge`
// and `approve`, or recording an unverified or self-authored judgment.
package specdesign_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

const agyConvID = "66686222-1a32-45e4-852f-4f62f0f457fd"

type agyTestCall struct {
	name    string
	path    string
	content string
}

// useAntigravity selects an isolated Antigravity conversation environment with
// Claude Code identity variables cleared.
func useAntigravity(t *testing.T, appDataDir, convID string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("ZE_AGENT_ID", "")
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", convID)
	t.Setenv("ANTIGRAVITY_APP_DATA_DIR", appDataDir)
	env.ResetCache()
}

// agyFormatShowing formats text lines as Antigravity's read_file response.
func agyFormatShowing(text string, start, end, total int) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var out strings.Builder
	out.WriteString("Created At: 2026-11-22T16:00:00Z\nCompleted At: 2026-11-22T16:00:01Z\n")
	out.WriteString("Showing lines ")
	out.WriteString(strconv.Itoa(start))
	out.WriteByte('-')
	out.WriteString(strconv.Itoa(end))
	out.WriteString(" of ")
	out.WriteString(strconv.Itoa(total))
	out.WriteByte('\n')
	for lineNum := start; lineNum <= end; lineNum++ {
		out.WriteString(strconv.Itoa(lineNum))
		out.WriteString(": ")
		out.WriteString(lines[lineNum-1])
		if lineNum < end {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// agyFormatWhole formats a whole file as Antigravity's read_file response.
func agyFormatWhole(text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	return agyFormatShowing(text, 1, len(lines), len(lines))
}

// writeAgyTranscript writes a transcript file (name is transcript_full.jsonl
// or transcript.jsonl) holding one PLANNER_RESPONSE followed by its GENERIC
// results in exact Antigravity step order.
func writeAgyTranscript(t *testing.T, appDataDir, name string, calls []agyTestCall, quoteArgs bool) {
	t.Helper()
	logsDir := filepath.Join(appDataDir, "brain", agyConvID, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	var lines []string
	toolCalls := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		argPath := call.path
		if quoteArgs {
			argPath = strconv.Quote(argPath)
		}
		toolCalls = append(toolCalls, map[string]any{
			"name": call.name,
			"args": map[string]any{"Path": argPath},
		})
	}
	planner, err := json.Marshal(map[string]any{
		"step_index": 1,
		"type":       "PLANNER_RESPONSE",
		"status":     "DONE",
		"tool_calls": toolCalls,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines = append(lines, string(planner))
	for index, call := range calls {
		generic, err := json.Marshal(map[string]any{
			"step_index": 2 + index,
			"type":       "GENERIC",
			"status":     "DONE",
			"content":    call.content,
		})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(generic))
	}
	target := filepath.Join(logsDir, name)
	if err := os.WriteFile(target, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestAntigravityJudgeAndApproveEndToEnd proves that an Antigravity agent that
// read the whole door and the cited section records a digest-bound verdict
// which `le spec design approve` accepts from Thomas's terminal.
func TestAntigravityJudgeAndApproveEndToEnd(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	appData := t.TempDir()
	useAntigravity(t, appData, agyConvID)

	doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
	if err != nil {
		t.Fatal(err)
	}
	writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
	}, false)

	digest := specdesign.DesignDigest(completeSpecText)
	text, code := runJudge(t, "approve", "every verdict holds against its section")
	if code != 0 {
		t.Fatalf("Antigravity judge answered %d, want 0:\n%s", code, text)
	}
	requireContains(t, "the judge answer", text, digest[:12])
	record := readRecord(t, root)
	requireContains(t, "the judge record", record,
		"Digest: "+digest, "Verdict: approve", "Judge: antigravity+"+agyConvID)

	humantest.Streams(t, "terminal")
	approveOut, approveCode := runApprove(t, root, completeSpecText, "approved in agy flow")
	if approveCode != 0 {
		t.Fatalf("approve after Antigravity judge answered %d, want 0:\n%s", approveCode, approveOut)
	}
	requireContains(t, "the approve answer", approveOut, `"spec":"spec-widget.md"`, `"agrees":1`)
}

// TestAntigravityJudgeCompactTranscriptFallback verifies that when
// transcript_full.jsonl is absent, the judge reads transcript.jsonl and handles
// its quoted Path arguments.
func TestAntigravityJudgeCompactTranscriptFallback(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	appData := t.TempDir()
	useAntigravity(t, appData, agyConvID)

	doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
	if err != nil {
		t.Fatal(err)
	}
	writeAgyTranscript(t, appData, "transcript.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
	}, true)

	text, code := runJudge(t, "approve", "every verdict holds against its section")
	if code != 0 {
		t.Fatalf("Antigravity judge on compact transcript answered %d, want 0:\n%s", code, text)
	}
}

// TestAntigravityJudgeRefusesUnreadPartialAndStaleReads verifies that missing
// reads, partial door reads, truncated section reads and reads made stale by a
// subsequent edit are all refused with Antigravity read_file guidance.
func TestAntigravityJudgeRefusesUnreadPartialAndStaleReads(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
	appData := t.TempDir()
	useAntigravity(t, appData, agyConvID)

	doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
	if err != nil {
		t.Fatal(err)
	}
	doorLines := strings.Split(strings.TrimSuffix(string(doorText), "\n"), "\n")
	pageLines := strings.Split(strings.TrimSuffix(widgetPageText, "\n"), "\n")

	// Partial door read (only line 1 of 3) + cut section read (stops before ## Wire format).
	writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatShowing(string(doorText), 1, 1, len(doorLines))},
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatShowing(widgetPageText, 5, 9, len(pageLines))},
	}, false)

	text, code := runJudge(t, "approve", "every verdict holds")
	if code != 2 {
		t.Fatalf("partial reads answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "partial refusal", text,
		widgetDoor+": Read the whole door",
		widgetPage+", Design decisions: read_file with StartLine 5 and EndLine 11",
		"read_file")
	requireAbsent(t, root, judgedRecord)

	// Byte-truncated Partial read that cuts off before closing heading is refused,
	// while a Partial read that closes Design decisions before the cutoff passes.
	truncatedBeforeClose := "Created At: 2026-11-22T16:00:00Z\nPartial read (lines 1-13 of 13). Showing bytes 0-120 of 300 in this line range, truncated at the 46080-byte limit.\n" +
		"1: # Widget\n2: \n3: How it answers.\n4: \n5: ## Design decisions\n6: \n7: | Decision | It forbids | Source | Date |\n8: |---|---|---|---|\n9: | The widget"
	writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: truncatedBeforeClose},
	}, false)

	text, code = runJudge(t, "approve", "every verdict holds")
	if code != 2 {
		t.Fatalf("byte-truncated unread section answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "byte-truncated refusal", text, widgetPage+", Design decisions")
	requireAbsent(t, root, judgedRecord)

	truncatedAfterClose := "Created At: 2026-11-22T16:00:00Z\nPartial read (lines 1-13 of 13). Showing bytes 0-280 of 300 in this line range, truncated at the 46080-byte limit.\n" +
		"1: # Widget\n2: \n3: How it answers.\n4: \n5: ## Design decisions\n6: \n" +
		"7: | Decision | It forbids | Source | Date |\n8: |---|---|---|---|\n" +
		"9: | " + widgetDecision + " | Rendering text in the widget | " + widgetRecord + " | 2026-08-21 |\n" +
		"10: \n11: ## Wire format\n12: \n13: The answer is J"
	writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
		{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: truncatedAfterClose},
	}, false)

	text, code = runJudge(t, "approve", "every verdict holds")
	if code != 0 {
		t.Fatalf("byte-truncated read after closed section answered %d, want 0:\n%s", code, text)
	}

	// Stale door read after door changes on disk is refused.
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(judgedRecord))); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{widgetDoor: string(doorText) + "\nAdded constraint.\n"})
	text, code = runJudge(t, "approve", "every verdict holds")
	if code != 2 {
		t.Fatalf("stale door read answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "stale door refusal", text, widgetDoor+": Read the whole door")
	requireAbsent(t, root, judgedRecord)
}

// TestAntigravityJudgeRefusesSelfEditedSpec proves independence: an
// Antigravity conversation that called write_file or edit_file on the spec
// cannot judge its own design.
func TestAntigravityJudgeRefusesSelfEditedSpec(t *testing.T) {
	for _, tool := range []string{"write_file", "edit_file"} {
		t.Run(tool, func(t *testing.T) {
			root := widgetRoot(t)
			writeTree(t, root, map[string]string{widgetSpec: completeSpecText})
			appData := t.TempDir()
			useAntigravity(t, appData, agyConvID)

			doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
			if err != nil {
				t.Fatal(err)
			}
			writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
				{name: tool, path: filepath.Join(root, filepath.FromSlash(widgetSpec)), content: "ok"},
				{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
				{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
			}, false)

			text, code := runJudge(t, "approve", "every verdict holds")
			if code != 2 {
				t.Fatalf("self-edited spec via %s answered %d, want 2:\n%s", tool, code, text)
			}
			requireContains(t, "self-edit refusal", text, "edited "+widgetSpec, "invoke_subagent")
			requireAbsent(t, root, judgedRecord)
		})
	}
}

// TestAntigravityJudgeRefusesOtherCheckoutReads pins checkout confinement:
// reading the holding checkout's copies never counts for a worktree's spec.
func TestAntigravityJudgeRefusesOtherCheckoutReads(t *testing.T) {
	holder := widgetWorktree(t)
	appData := t.TempDir()
	useAntigravity(t, appData, agyConvID)

	doorText := "# System door\n\n<!-- door-base: pkg cmd/ze internal -->\n"
	writeTree(t, holder, map[string]string{
		widgetDoor: doorText,
		widgetPage: widgetPageText,
	})
	writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
		{name: "read_file", path: filepath.Join(holder, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(doorText)},
		{name: "read_file", path: filepath.Join(holder, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
	}, false)

	text, code := runJudge(t, "approve", "every verdict holds")
	if code != 2 {
		t.Fatalf("reads of holder copies answered %d, want 2:\n%s", code, text)
	}
	requireContains(t, "other-checkout refusal", text, widgetPage, widgetDoor+": Read the whole door")
	requireAbsent(t, filepath.Join(holder, ".claude", "worktrees", "wt"), judgedRecord)
}

// TestAntigravityJudgeRefusesUnsafeEnvAndCorruptTranscript pins fail-closed
// guards for invalid conversation ids, missing app data dirs, missing
// transcripts and corrupt JSONL.
func TestAntigravityJudgeRefusesUnsafeEnvAndCorruptTranscript(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})

	t.Run("unsafe conversation id", func(t *testing.T) {
		useAntigravity(t, t.TempDir(), "../escape")
		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("unsafe conversation id answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "unsafe id refusal", text, "ANTIGRAVITY_CONVERSATION_ID")
	})

	t.Run("missing app data dir", func(t *testing.T) {
		useAntigravity(t, "", agyConvID)
		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("missing app data dir answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "missing app data refusal", text, "ANTIGRAVITY_APP_DATA_DIR")
	})

	t.Run("missing transcript", func(t *testing.T) {
		useAntigravity(t, t.TempDir(), agyConvID)
		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("missing transcript answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "missing transcript refusal", text, "cannot locate the Antigravity transcript")
	})

	t.Run("corrupt transcript", func(t *testing.T) {
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		logsDir := filepath.Join(appData, "brain", agyConvID, ".system_generated", "logs")
		if err := os.MkdirAll(logsDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(logsDir, "transcript_full.jsonl"), []byte("{not json\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("corrupt transcript answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "corrupt transcript refusal", text, "parse antigravity transcript")
	})
}

// TestAntigravityJudgeStitchesPartialByteOffsetAndContiguousLineReads proves
// that a section split across Partial read ByteOffset continuations (including
// a mid-line split) or across adjacent Showing lines ranges satisfies the read
// check, while a gap between byte chunks or line ranges is still refused.
func TestAntigravityJudgeStitchesPartialByteOffsetAndContiguousLineReads(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})

	doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
	if err != nil {
		t.Fatal(err)
	}
	pageLines := strings.Split(strings.TrimSuffix(widgetPageText, "\n"), "\n")
	var numbered strings.Builder
	for i, line := range pageLines {
		numbered.WriteString(strconv.Itoa(i + 1))
		numbered.WriteString(": ")
		numbered.WriteString(line)
		if i+1 < len(pageLines) {
			numbered.WriteByte('\n')
		}
	}
	fullNumbered := numbered.String()
	// Split right in the middle of line 9 (the Design decisions row).
	splitAt := strings.Index(fullNumbered, widgetDecision) + 12
	byteTotal := len(fullNumbered)

	formatPartial := func(b0, b1 int) string {
		return "Created At: 2026-11-22T16:00:00Z\nCompleted At: 2026-11-22T16:00:01Z\n" +
			"Partial read (lines 1-" + strconv.Itoa(len(pageLines)) + " of " + strconv.Itoa(len(pageLines)) + "). " +
			"Showing bytes " + strconv.Itoa(b0) + "-" + strconv.Itoa(b1) + " of " + strconv.Itoa(byteTotal) + " in this line range.\n" +
			fullNumbered[b0:b1]
	}

	t.Run("contiguous byte-offset chunks stitch cleanly", func(t *testing.T) {
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: formatPartial(0, splitAt)},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: formatPartial(splitAt, byteTotal)},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds against its section")
		if code != 0 {
			t.Fatalf("contiguous Partial read chunks answered %d, want 0:\n%s", code, text)
		}
	})

	t.Run("gapped byte-offset chunks are refused", func(t *testing.T) {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(judgedRecord)))
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: formatPartial(0, splitAt)},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: formatPartial(splitAt+5, byteTotal)},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("gapped Partial read chunks answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "gapped refusal", text, widgetPage+", Design decisions")
		requireAbsent(t, root, judgedRecord)
	})

	t.Run("contiguous Showing lines ranges stitch cleanly", func(t *testing.T) {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(judgedRecord)))
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatShowing(widgetPageText, 5, 8, len(pageLines))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatShowing(widgetPageText, 9, 11, len(pageLines))},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds against its section")
		if code != 0 {
			t.Fatalf("contiguous Showing lines ranges answered %d, want 0:\n%s", code, text)
		}
	})

	t.Run("gapped Showing lines ranges are refused", func(t *testing.T) {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(judgedRecord)))
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatShowing(widgetPageText, 5, 7, len(pageLines))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatShowing(widgetPageText, 9, 11, len(pageLines))},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("gapped Showing lines ranges answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "gapped lines refusal", text, widgetPage+", Design decisions")
		requireAbsent(t, root, judgedRecord)
	})
}

// TestAntigravityJudgeResolvesWithInheritedClaudeSession proves that an
// Antigravity agent whose shell inherited CLAUDE_CODE_SESSION_ID (without
// ZE_AGENT_ID) still resolves its Antigravity conversation transcript and keeps
// the self-edit refusal intact.
func TestAntigravityJudgeResolvesWithInheritedClaudeSession(t *testing.T) {
	root := widgetRoot(t)
	writeTree(t, root, map[string]string{widgetSpec: completeSpecText})

	doorText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(widgetDoor)))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("approves when independent conversation read every row", func(t *testing.T) {
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		t.Setenv("CLAUDE_CODE_SESSION_ID", "inherited-claude-session")
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds against its section")
		if code != 0 {
			t.Fatalf("judge with inherited CLAUDE_CODE_SESSION_ID answered %d, want 0:\n%s", code, text)
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(judgedRecord)))
		if err != nil {
			t.Fatalf("read judge record: %v", err)
		}
		requireContains(t, "judge fingerprint", string(body), "Judge: antigravity+"+agyConvID)
	})

	t.Run("still refuses self-edited spec with inherited CLAUDE_CODE_SESSION_ID", func(t *testing.T) {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(judgedRecord)))
		appData := t.TempDir()
		useAntigravity(t, appData, agyConvID)
		t.Setenv("CLAUDE_CODE_SESSION_ID", "inherited-claude-session")
		writeAgyTranscript(t, appData, "transcript_full.jsonl", []agyTestCall{
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetDoor)), content: agyFormatWhole(string(doorText))},
			{name: "read_file", path: filepath.Join(root, filepath.FromSlash(widgetPage)), content: agyFormatWhole(widgetPageText)},
			{name: "edit_file", path: filepath.Join(root, filepath.FromSlash(widgetSpec)), content: "edited"},
		}, false)

		text, code := runJudge(t, "approve", "every verdict holds")
		if code != 2 {
			t.Fatalf("self-edited spec answered %d, want 2:\n%s", code, text)
		}
		requireContains(t, "self-edit refusal", text, "edited "+widgetSpec)
		requireAbsent(t, root, judgedRecord)
	})
}
