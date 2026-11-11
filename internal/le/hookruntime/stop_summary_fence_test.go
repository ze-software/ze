// Design: docs/contributing/spec-workflow.md -- Stop preserves authored Markdown while rotating recovery snapshots.
// Related: one_off_proof_test.go -- the historical hook categories and their current consumers.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestRenderedRuleRefusalNamesBothGenerators asks the full write-hook entry
// about a rendered rule and each session-payload aggregate. Rendering the
// points first and the session payload second is the documented repair for
// all three subjects, so each command must stand on its own line.
//
// MUTATION: restoring writeRenderedRule's single "run ./le ai rules update
// render." sentence turns every subtest red on the condensed command.
func TestRenderedRuleRefusalNamesBothGenerators(t *testing.T) {
	for _, file := range []string{"ai/rules/commands.md", "ai/rules/CORE.md", "ai/rules/TRIGGERS.md"} {
		t.Run(filepath.Base(file), func(t *testing.T) {
			code, _, message := runHook(t, t.TempDir(), "pretool-writeedit", map[string]any{
				"tool_name":  toolWrite,
				"tool_input": map[string]any{"file_path": file, "content": "# Proposed generated rule\n"},
			})
			if code != 2 {
				t.Fatalf("generated-rule edit answered %d, want refusal: %s", code, message)
			}
			for _, command := range []string{"./le ai rules update render", "./le ai rules update condensed"} {
				if !strings.Contains(message, "\n    "+command+"\n") {
					t.Errorf("generated-rule refusal lacks exact next step %q on its own line: %s", command, message)
				}
			}
		})
	}
}

// TestStopSummaryPreservesQuotedSnapshotHeading runs the Stop summary hook
// against its own dirty fixture checkout. A quoted, otherwise valid snapshot
// within an authored handoff must survive byte for byte while real snapshots
// rotate to the new snapshot and the two newest predecessors.
//
// MUTATION: dropping the fence branches from preserveState turns the fenced
// cases red: the quoted divider splits the handoff and the quoted heading
// takes a retention slot.
func TestStopSummaryPreservesQuotedSnapshotHeading(t *testing.T) {
	const example = "## Session: 2026-08-10T01:02:03Z\n\nBranch: `quoted`\n\nUncommitted:\n- `quoted.go`\n"
	for _, test := range []struct {
		name    string
		handoff string
	}{
		{"plain-example", "## Phase 7 handoff\nThe heading below is an example inside this handoff.\n" + example + "Keep the complete handoff after the example."},
		{"fenced-example", "## Phase 7 handoff\nThe example below is quoted source, not recovery state.\n```text\n" + example + "```\nKeep the complete handoff after the example."},
		{"fenced-divider", "## Phase 7 handoff\nThe divider is quoted source too.\n```text\n---\n" + example + "```\nKeep the complete handoff after the example."},
		{"fenced-title", "## Phase 7 handoff\nThe title is quoted source too.\n~~~text\n# Session State\n\n---\n" + example + "~~~\nKeep the complete handoff after the example."},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertStopSummaryHandoff(t, test.handoff)
		})
	}
}

// TestStopSummaryKeepsInlineBackticksOutsideFenceState ensures an authored
// inline code span cannot swallow the recovery snapshots that follow it.
//
// MUTATION: dropping the backtick test from fenceMarker opens a fence at the
// inline span, which then swallows the middle and oldest snapshots, and the
// oldest is retained.
func TestStopSummaryKeepsInlineBackticksOutsideFenceState(t *testing.T) {
	assertStopSummaryHandoff(t, "## Inline handoff\n```inline example```\nKeep this example.")
}

// TestStopSummaryPreservesIndentedFenceContents runs the actual Stop consumer
// with a closing-looking token indented beyond Markdown's three-space limit.
// The quoted divider and snapshot stay inside the authored fence while the
// fixture's three real predecessors exercise the two-predecessor retention cap.
//
// MUTATION: dropping the indentation limit from fenceMarker closes the fence
// early, so the quoted divider splits the handoff.
func TestStopSummaryPreservesIndentedFenceContents(t *testing.T) {
	const quoted = "---\n## Session: quoted-inside-fence\n\nBranch: `example`\n\nUncommitted:\n- `quoted.go`\n"
	for _, test := range []struct {
		name   string
		marker string
		indent string
	}{
		{"backticks-four-spaces", "```", "    "},
		{"backticks-eight-spaces", "```", "        "},
		{"tildes-four-spaces", "~~~", "    "},
		{"tildes-eight-spaces", "~~~", "        "},
	} {
		t.Run(test.name, func(t *testing.T) {
			handoff := "## Fence handoff\nThe indented token below is quoted content.\n" +
				test.marker + "text\n" + test.indent + test.marker + "\n" + quoted +
				test.marker + "\nKeep the entire authored example."
			assertStopSummaryHandoff(t, handoff)
		})
	}
}

// TestStopSummaryClosesFencesIndentedAtMostThreeSpaces keeps the legal closing
// indentation cases paired with the refusal above. A missed real close would
// swallow the following real snapshot boundaries and retain the oldest one.
func TestStopSummaryClosesFencesIndentedAtMostThreeSpaces(t *testing.T) {
	const quoted = "---\n## Session: quoted-before-legal-close\n\nBranch: `example`\n"
	for _, test := range []struct {
		name   string
		spaces int
	}{
		{"zero", 0},
		{"one", 1},
		{"two", 2},
		{"three", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			handoff := "## Legal fence close\n```text\n" + quoted +
				strings.Repeat(" ", test.spaces) + "```\nKeep the authored block after its legal close."
			assertStopSummaryHandoff(t, handoff)
		})
	}
}

// TestStopSummaryWritesOneSeparatorBetweenBlocks rotates a state file whose
// handoff sits between real snapshots and asserts the rewrite holds exactly
// one separator between each pair of blocks.
//
// MUTATION: keeping the separator line inside the block it ends (no flush on
// "---" in preserveState) writes it again through the join, so the handoff is
// followed by two separators and the count is one too many.
func TestStopSummaryWritesOneSeparatorBetweenBlocks(t *testing.T) {
	text := rotateStopSummary(t, "# Session State\n\n"+
		"## Session: newest\n\nBranch: `main`\n\n---\n"+
		"## Phase handoff\nkept between\n\n---\n"+
		"## Session: middle\n\nBranch: `main`\n\n---\n"+
		"## Last compaction\nkept after\n")
	// The new snapshot, newest, middle and the two authored blocks: five
	// blocks, so four separators.
	separators := 0
	for line := range strings.SplitSeq(text, "\n") {
		if line == "---" {
			separators++
		}
	}
	if separators != 4 {
		t.Fatalf("rotated state holds %d separators, want 4:\n%s", separators, text)
	}
	for _, kept := range []string{"## Phase handoff\nkept between", "## Last compaction\nkept after", "## Session: newest", "## Session: middle"} {
		if !strings.Contains(text, kept) {
			t.Errorf("rotated state lost %q:\n%s", kept, text)
		}
	}
}

// assertStopSummaryHandoff places handoff between the newest and the middle of
// three real snapshots, rotates, and asserts the handoff survives byte for
// byte, the two newest predecessors stay and the oldest goes.
func assertStopSummaryHandoff(t *testing.T, handoff string) {
	t.Helper()
	text := rotateStopSummary(t, "# Session State\n\n"+
		"## Session: newest\n\nBranch: `main`\n\n---\n"+handoff+"\n\n---\n"+
		"## Session: middle\n\nBranch: `main`\n\n---\n"+
		"## Session: oldest\n\nBranch: `main`\n")
	if !strings.Contains(text, handoff) {
		t.Errorf("Stop summary split or deleted authored handoff bytes:\n%s", text)
	}
	for _, snapshot := range []string{"## Session: newest", "## Session: middle"} {
		if !strings.Contains(text, snapshot) {
			t.Errorf("Stop summary lost retained recovery snapshot %q:\n%s", snapshot, text)
		}
	}
	if strings.Contains(text, "## Session: oldest") {
		t.Errorf("Stop summary retained a fourth real snapshot:\n%s", text)
	}
}

// rotateStopSummary runs the session-end-summary hook once to create the
// session's state file in a dirty fixture checkout, replaces its content with
// prior, runs the hook again, and returns the rotated content.
func rotateStopSummary(t *testing.T, prior string) string {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "history-summary")
	t.Setenv("ZE_AGENT_ID", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	root := gitFixture(t)
	writeHookFixture(t, root, "dirty.txt", "owned fixture change\n")
	payload := map[string]any{"session_id": "history-summary"}
	if code, _, message := runHook(t, root, "session-end-summary", payload); code != 0 || message != "" {
		t.Fatalf("initial Stop summary: %d: %s", code, message)
	}
	states, err := filepath.Glob(filepath.Join(root, "tmp", "session", "*", "state", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("Stop summary wrote %d state files, want one: %v", len(states), states)
	}
	if err := os.WriteFile(states[0], []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, message := runHook(t, root, "session-end-summary", payload); code != 0 || message != "" {
		t.Fatalf("rotating Stop summary: %d: %s", code, message)
	}
	body, err := os.ReadFile(states[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
