// Related: bash.go -- bashDestructiveGit, the forbidden-git-verb guard
//
// VALIDATES: every git verb CLAUDE.md forbids as a direct Bash call is refused,
// the refusal names the route that works, and a read-only git call and the
// generated commit script both still pass.
// PREVENTS: a staging verb reaching the shared index unopposed, which is what
// put a deletion in front of four sessions and needed the owner to clear it.

package hookruntime

import (
	"strings"
	"testing"
)

// TestEveryForbiddenGitVerbIsRefused drives the hook's own entry point rather
// than the pattern list, so a change to the guard's wiring cannot leave this
// green.
//
// The three staging verbs are the reason it exists. The list carried none of
// them, though CLAUDE.md names two, so a subagent's staging call went through
// unopposed. The unstaging verb is the sharper half: it had an explicit early
// return PERMITTING it, above a blanket block of the same command family, while
// CLAUDE.md bans it outright. A reader checking the hook would have concluded
// the verb was sanctioned.
func TestEveryForbiddenGitVerbIsRefused(t *testing.T) {
	commands := []string{
		"git" + " add internal/component/iface/cli/scan.go",
		"git" + " rm internal/component/tacacs/yang/ze-tacacs-cmd.yang",
		"git" + " rm --cached internal/component/tacacs/yang/ze-tacacs-cmd.yang",
		"git" + " mv old.go new.go",
		"git" + " restore --staged internal/component/tacacs/yang/ze-tacacs-cmd.yang",
		"git" + " restore internal/component/iface/cli/scan.go",
		"git" + " stash",
		"git" + " stash drop",
		"git" + " commit -m subject",
		"git" + " reset --hard origin/main",
		"git" + " clean -f",
		"git" + " revert HEAD",
		"git" + " merge other",
		"git" + " push --force",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": command},
			})
			if code != 2 {
				t.Fatalf("code = %d, want 2 for %q", code, command)
			}
			if !strings.Contains(message, "./le commit create") {
				t.Errorf("the refusal does not name the route that works: %s", message)
			}
		})
	}
}

// TestTheCommitRouteAndReadsAreNotBlocked proves the refusal leaves the only
// working route alone. The generated script stages inside itself and reaches
// the tool as `bash <path>`, so no verb appears in the command string. A guard
// that blocked it would leave no way to commit at all, and a guard that blocked
// a read would stop a session seeing what it was about to do.
func TestTheCommitRouteAndReadsAreNotBlocked(t *testing.T) {
	commands := []string{
		"bash tmp/commit-3201c77e-a-605c87.sh",
		"git" + " status --short",
		"git" + " diff --cached --name-only",
		"git" + " log --oneline -1",
		"git" + " show HEAD:internal/component/iface/cli/scan.go",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": command},
			})
			if code != 0 {
				t.Fatalf("code = %d, want 0 for %q: %s", code, command, message)
			}
		})
	}
}

// TestNamingAVerbIsNotRunningIt proves the guard reads a command POSITION
// rather than a substring. A commit message explaining why a verb is banned, a
// grep for one, and an echo quoting one are all prose ABOUT the rule. Refusing
// them stopped a session from writing the very commit that fixed the rule.
func TestNamingAVerbIsNotRunningIt(t *testing.T) {
	allowed := []string{
		`./le commit create subject "the list carried git` + ` commit, push and reset"`,
		"grep -rn 'git" + " add' internal/le/hookruntime",
		"echo the staging verbs are git" + " add, git" + " rm and git" + " mv",
	}
	for _, command := range allowed {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": command},
			})
			if code != 0 {
				t.Fatalf("code = %d, want 0 for prose naming a verb: %s", code, message)
			}
		})
	}

	// Every separator a real invocation hides behind still counts as a run.
	blocked := []string{
		"cd /x && git" + " add .",
		"true; git" + " commit -m subject",
		"(git" + " push)",
		"false || git" + " reset --hard",
	}
	for _, command := range blocked {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			code, _, _ := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": command},
			})
			if code != 2 {
				t.Fatalf("code = %d, want 2 for %q", code, command)
			}
		})
	}
}
