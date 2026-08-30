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

// TestEveryBranchMovingVerbIsRefused covers the family the guard above carried
// one member of. `git merge` was blocked while `git rebase`, `git switch`,
// `git checkout -b` and every branch deletion and rename went through, so the
// rule that a session stays on the branch it started on was enforced against
// the one verb the repository already tells nobody to use.
//
// VALIDATES: creating, switching, renaming, deleting and integrating a branch
// are each refused, and the refusal says the branch is the user's to move.
// PREVENTS: a session landing its work on a branch the user is not looking at,
// or rewriting the history of the one they are.
func TestEveryBranchMovingVerbIsRefused(t *testing.T) {
	commands := []string{
		"git" + " merge other",
		"git" + " rebase main",
		"git" + " rebase -i HEAD~3",
		"git" + " switch other",
		"git" + " switch -c new-branch",
		"git" + " checkout -b new-branch",
		"git" + " checkout -B new-branch",
		"git" + " branch -d old",
		"git" + " branch -D old",
		"git" + " branch --delete old",
		"git" + " branch -m old new",
		"git" + " branch -M old new",
		"git" + " branch --move old new",
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
			if !strings.Contains(message, "the user's to move") {
				t.Errorf("the refusal does not say who owns the branch: %s", message)
			}
		})
	}
}

// TestReadingBranchesIsNotMovingThem is the other polarity. `git branch` with
// no mutating flag lists, and a session asking which branch it is on is the
// first thing the branch rule expects it to do. A guard that blocked the
// question would push sessions into guessing.
func TestReadingBranchesIsNotMovingThem(t *testing.T) {
	commands := []string{
		"git" + " branch --show-current",
		"git" + " branch -a",
		"git" + " branch --list",
		"git" + " branch -vv",
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

// TestAGlobalOptionDoesNotHideTheVerb is the hole under BOTH guards. Each
// compares the command against the literal text `git <verb>`, and git accepts
// options before its verb, so `git -C /path commit` contained no pattern any
// guard held and reached the shared index unopposed. `-c commit.gpgsign=false`
// is the sharper one: CLAUDE.md bans it by name, and it was the flag that
// carried the banned verb past the guard.
//
// VALIDATES: a verb behind -C, -c, --git-dir, --work-tree or --no-pager is
// refused exactly as the bare verb is.
// PREVENTS: every pattern in both lists being one documented flag from useless.
func TestAGlobalOptionDoesNotHideTheVerb(t *testing.T) {
	commands := []string{
		"git" + " -C /other/tree commit -m subject",
		"git" + " -c commit.gpgsign=false commit -m subject",
		"git" + " --git-dir=/other/.git push",
		"git" + " --work-tree /other reset --hard",
		"git" + " --no-pager add .",
		"git" + " -C /other/tree rebase main",
		"cd /x && git" + " -C /other/tree stash",
	}
	for _, command := range commands {
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
	// A separator INSIDE quotes is part of a pattern, not part of the shell.
	// This is where the exemption failed: a grep alternation puts a pipe
	// directly before the verb, so searching the repository for the rule was
	// refused as an attempt to break it. It happened twice in one session, to
	// the session editing this guard.
	quoted := []string{
		`grep -rn "destructive\|git ` + `merge" docs/`,
		`grep -rn 'a;git ` + `commit' ai/`,
		`echo "the verbs are: git ` + `add | git ` + `rm"`,
	}
	for _, command := range append(allowed, quoted...) {
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
	// The last two are the quote exemption withdrawn: handing a string to
	// another shell to RUN is the one place quotes do not mean prose.
	blocked := []string{
		"cd /x && git" + " add .",
		"true; git" + " commit -m subject",
		"(git" + " push)",
		"false || git" + " reset --hard",
		`bash -c "cd /x && git` + ` commit -m subject"`,
		`eval "git` + ` push"`,
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
