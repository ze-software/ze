// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: session.go -- bashAgentIdentity and commandIdentityPrefix
// Related: bash.go -- bashSystemTmp, bashScratch and bashWorktreeCopy, whose advice these pin

package hookruntime

import (
	"strings"
	"testing"
)

// subagentLine is the identity line runSubagentBash's payload owns.
const subagentLine = "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; "

// TestScratchAdviceCarriesTheSubagentIdentityLine is review N3.
//
// VALIDATES: for a subagent, the /tmp refusal and the tmp/-root scratch
// refusal advise `dir=$(./le session scratch ensure)` behind the agent's own
// identity line, so following the advice is not refused again; for the main
// thread the advice carries no line.
// PREVENTS: advice whose literal command the identity check refuses.
func TestScratchAdviceCarriesTheSubagentIdentityLine(t *testing.T) {
	for _, command := range []string{
		subagentLine + "cat /tmp/x",
		subagentLine + "echo hi > tmp/out.log",
	} {
		code, _, message := runSubagentBash(t, command)
		if code != 2 {
			t.Fatalf("%q: code %d, want 2: %s", command, code, message)
		}
		if !strings.Contains(message, subagentLine+"dir=$(./le session scratch ensure)") {
			t.Errorf("%q: the scratch advice lacks the identity line: %s", command, message)
		}
	}
	code, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"tool_name": "Bash", "tool_input": map[string]any{"command": "cat /tmp/x"},
	})
	if code != 2 {
		t.Fatalf("main thread: code %d, want 2: %s", code, message)
	}
	if strings.Contains(message, "export ZE_AGENT_ID") {
		t.Errorf("main thread: the advice carries an identity line: %s", message)
	}
}

// TestIdentityRefusalEndsWithTheFixingLine is review N4.
//
// VALIDATES: the refusal of a command with no identity line lists the
// readers before the fix and ends with the exact line to type; the agent's
// own exact line after leading whitespace passes, as it does without it.
// PREVENTS: the fix buried above a long list, and a correct line refused as
// another agent's.
func TestIdentityRefusalEndsWithTheFixingLine(t *testing.T) {
	code, _, message := runSubagentBash(t, "./le spec current")
	if code != 2 {
		t.Fatalf("code %d, want 2: %s", code, message)
	}
	// The line ends in "; " and the space is part of it: bashAgentIdentity
	// cuts the exact line, space included, from the command's front. Only
	// newlines after it are trimmed, so the suffix is the whole line.
	trimmed := strings.TrimRight(message, "\n")
	if !strings.HasSuffix(trimmed, subagentLine) {
		t.Errorf("the refusal does not end with the identity line: %s", message)
	}
	if strings.Index(message, "readers") > strings.LastIndex(message, "ZE_AGENT_ID=agent-1") {
		t.Errorf("the reader list follows the fixing line: %s", message)
	}
	for _, command := range []string{subagentLine + "true ./le", "  " + subagentLine + "true ./le", "\n" + subagentLine + "true ./le"} {
		code, _, message := runSubagentBash(t, command)
		if strings.Contains(message, "takes another identity") || strings.Contains(message, "needs the agent's identity line") {
			t.Errorf("%q: the agent's own line was refused (code %d): %s", command, code, message)
		}
	}
}

// TestWorktreeCopyRefusalNamesTheRouteThatWorks asks whether the refusal of a
// copy out of a worktree names a remedy the worktree agent may follow. It
// drives the whole pretool-bash action with one copy line and reads the text.
//
// The refusal said to merge or cherry-pick instead, and a worktree agent may
// do neither: it commits on its own branch, and the user lands that branch on
// main by rebase. The hook refuses the merge verb and lets cherry-pick through,
// so the advice steered the agent onto the banned verb nothing stops.
//
// VALIDATES: AC-1 and AC-2: the copy exits 2, the text names the commit route
// and the user's `git rebase <branch>`, its last line is the commit route, and
// it names neither cherry-pick nor merging as the remedy. It also names the
// file the line writes and the worktree file it writes it from, and where a
// run's output may go instead.
// PREVENTS: a refusal whose remedy is itself a banned verb, and one that does
// not say which write of the line it refused.
func TestWorktreeCopyRefusalNamesTheRouteThatWorks(t *testing.T) {
	const command = "cp .claude/worktrees/a/x.go internal/x.go"
	code, _, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	})
	if code != 2 {
		t.Fatalf("code = %d, want 2 for %q: %s", code, command, message)
	}
	for _, want := range []string{
		"./le commit create", "git" + " rebase <branch>",
		"It writes internal/x.go from .claude/worktrees/a/x.go.",
		"To keep a run's output, write it inside the worktree, under the checkout's tmp/, or outside the checkout.",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal does not name %q: %s", want, message)
		}
	}
	// internal/x.go is placed in the main tree, so the refusal does not say
	// the hook could not place it (TestWorktreeCopyRefusalSaysWhenItCouldNotPlaceTheDestination).
	if strings.Contains(message, worktreeUnplacedText) {
		t.Errorf("the refusal says the hook could not place internal/x.go, which it placed in the main tree: %s", message)
	}
	trimmed := strings.TrimRight(message, "\n")
	last := trimmed[strings.LastIndex(trimmed, "\n")+1:]
	if !strings.HasPrefix(last, "./le commit create") {
		t.Errorf("the refusal's last line is %q, want the ./le commit create line: %s", last, message)
	}
	for _, banned := range []string{"cherry-pick", "Use git" + " merge"} {
		if strings.Contains(message, banned) {
			t.Errorf("the refusal names the banned remedy %q: %s", banned, message)
		}
	}
}

// TestWorktreeCopyRefusalSaysWhenItCouldNotPlaceTheDestination asks
// bashWorktreeCopy alone for its refusal of a copy out of a worktree to a
// destination that starts with `$X`, which the hook cannot place.
//
// VALIDATES: the refusal names the destination as typed and the source, and
// says that a destination the hook cannot place counts as the main tree.
// PREVENTS: a refusal that leaves the reader to guess why a destination that
// may lie outside the checkout was refused.
func TestWorktreeCopyRefusalSaysWhenItCouldNotPlaceTheDestination(t *testing.T) {
	const command = "cp .claude/worktrees/a/x.go $X/internal/x.go"
	result := guardVerdict(bashWorktreeCopy, t.TempDir(), command)
	if result == nil || result.code != 2 {
		t.Fatalf("%q: verdict %+v, want exit 2: its destination may be the main tree", command, result)
	}
	for _, want := range []string{
		"It writes $X/internal/x.go from .claude/worktrees/a/x.go.",
		"A destination the hook cannot place, such as one that starts with a `$VAR` the hook cannot read, counts as the main tree.",
	} {
		if !strings.Contains(result.message, want) {
			t.Errorf("the refusal does not name %q: %s", want, result.message)
		}
	}
}
