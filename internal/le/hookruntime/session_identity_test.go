// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session.go -- the identity check these tests drive through Run
package hookruntime

import (
	"strings"
	"testing"
)

const (
	// identityLineOfAgent is the line agent-1 of session parent-17 types.
	identityLineOfAgent = "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; "
	// identityLineOfFork is the line a fork subagent of parent-17 types: it
	// has no agent id, so the session part is the whole line.
	identityLineOfFork = "export CLAUDE_CODE_SESSION_ID=parent-17; "
)

// runSubagentBash sends one Bash command through the pretool-bash entry as
// agent-1 of session parent-17.
func runSubagentBash(t *testing.T, command string) (code int, output, message string) {
	t.Helper()
	return runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
}

// runForkBash sends one Bash command through the pretool-bash entry as a fork
// subagent of session parent-17, which carries no agent id.
func runForkBash(t *testing.T, command string) (code int, output, message string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "1")
	return runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
}

// VALIDATES: AC-1. A subagent's command made only of reader commands passes
// with exit 0 and an empty standard output, so the harness receives no changed
// input and the command runs exactly as typed.
// PREVENTS: the hook rewriting a command, which makes auto mode void the call
// and every other call of the same message.
func TestSubagentReaderCommandPassesUnchanged(t *testing.T) {
	for _, command := range []string{
		`grep -n "a\|b" internal/x.go | head -5`,
		`cd /srv/checkout && git log --oneline -3; sed -n 1,5p notes.md 2>&1 | wc -l`,
		`gopls symbols internal/le/hookruntime/session.go`,
		`cat docs/a.md docs/b.md | sort | uniq -c | tail -3`,
		`git diff --stat && git status --short`,
		`ls -la internal/le &> /dev/null`,
		"grep -c x a.md\ngrep -c y b.md",
		`grep -n 'echo $(date)' notes.md`,
		`[ -f notes.md ] && cat notes.md`,
		`test -d docs && ls docs`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 0 {
			t.Errorf("%q: code = %d, want 0; message: %s", command, code, message)
		}
		if output != "" {
			t.Errorf("%q: standard output = %q, want none: the hook returns no changed input", command, output)
		}
		if strings.Contains(message, "identity line") {
			t.Errorf("%q: a reader command was asked for the identity line: %s", command, message)
		}
	}
	code, output, message := runForkBash(t, `grep -n x notes.md | head -5`)
	if code != 0 || output != "" {
		t.Errorf("fork reader command: code=%d output=%q message=%s, want exit 0 and no output", code, output, message)
	}
}

// VALIDATES: AC-2. A subagent's command holding any command outside the reader
// set, and not starting with the agent's identity line, is refused with exit 2
// and a message that prints the full line for this agent.
// PREVENTS: a subagent's ./le call running as the main thread and sharing its
// commit namespace and spec claim.
func TestSubagentCommandOutsideTheReaderSetNeedsTheIdentityLine(t *testing.T) {
	for _, command := range []string{
		`./le spec current`,
		`grep -n x notes.md; ./le commit create`,
		`go version`,
		`echo $(./le spec current)`,
		"echo `./le spec current`",
		`echo "$(./le spec current)"`,
		`LC_ALL=C grep -n x notes.md`,
		`find . -name '*.md' -exec ./le doc check {} \;`,
		`git -C /srv/checkout log --oneline`,
		`git stash list`,
		`cat list.txt | xargs ./le doc check`,
		`for f in a b; do cat "$f"; done`,
		`rg --pre ./le x notes.md`,
		`sort --compress-program=./le notes.md`,
		`cat <<EOF` + "\n./le spec current\nEOF",
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
	code, output, message := runForkBash(t, `./le spec current`)
	if code != 2 || output != "" {
		t.Errorf("fork: code=%d output=%q, want exit 2 and no output", code, output)
	}
	if !strings.Contains(message, identityLineOfFork) || strings.Contains(message, "ZE_AGENT_ID=") {
		t.Errorf("fork: message must print the session part alone, %q: %s", identityLineOfFork, message)
	}
}

// VALIDATES: AC-3. A command that starts with the agent's exact identity line
// passes, whatever follows, with no changed input.
// PREVENTS: an agent that typed the line it was given being refused again.
func TestSubagentCommandCarryingItsIdentityLinePasses(t *testing.T) {
	for _, command := range []string{
		`./le spec current`,
		`grep -n x notes.md; ./le spec state current`,
		`echo "$ZE_AGENT_ID" && ./le spec current`,
		`grep -rn "ZE_AGENT_ID=" docs/ ; ./le spec current`,
		`grep ZE_AGENT_ID= notes.md`,
		`bash tmp/commit-4f95a2b5-a-probe.sh`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
	}
	code, output, message := runForkBash(t, identityLineOfFork+`./le spec current`)
	if code != 0 || output != "" {
		t.Errorf("fork: code=%d output=%q message=%s, want exit 0 and no output", code, output, message)
	}
}

// VALIDATES: AC-4. A command whose identity line names another agent or
// session, or that takes another identity after its own line, is refused with
// the line this agent owns.
// PREVENTS: an agent acting under a sibling's commit namespace and spec claim,
// or under the main thread's.
func TestSubagentIdentityLineOfAnotherAgentIsRefused(t *testing.T) {
	for _, command := range []string{
		`export ZE_AGENT_ID=agent-2; export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		`export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-18; ./le spec current`,
		`export ZE_AGENT_ID=agent-10; export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		`export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		identityLineOfAgent + `export ZE_AGENT_ID=agent-2; ./le spec current`,
		identityLineOfAgent + `ZE_AGENT_ID=agent-2 ./le spec current`,
		identityLineOfAgent + `env CLAUDE_CODE_SESSION_ID=parent-18 ./le spec current`,
		identityLineOfAgent + `unset ZE_AGENT_ID; ./le spec current`,
		identityLineOfAgent + `eval "export ZE_AGENT_ID=agent-2"; ./le spec current`,
		identityLineOfAgent + `bash -c 'CLAUDE_CODE_SESSION_ID=parent-18 ./le spec current'`,
		identityLineOfAgent + `sh -c 'unset ZE_AGENT_ID; ./le spec current'`,
		identityLineOfAgent + `ZE_AGENT_ID=agent-2 ./le spec "unclosed`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", command, message)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line this agent owns %q: %s", command, identityLineOfAgent, message)
		}
	}
}

// VALIDATES: AC-5. A main-thread command, which carries no agent id and is no
// fork, is never asked for the identity line and receives no changed input.
// PREVENTS: the check binding the main thread, which is the identity le
// resolves by default.
func TestMainThreadCommandNeedsNoIdentityLine(t *testing.T) {
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	for _, command := range []string{`./le spec current`, `go version`, `grep "unclosed notes.md`} {
		code, output, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
			"session_id": "parent-17", "tool_name": "Bash",
			"tool_input": map[string]any{"command": command},
		})
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
		if strings.Contains(message, "identity line") {
			t.Errorf("%q: the main thread was asked for the identity line: %s", command, message)
		}
	}
}

// VALIDATES: R-2. A line the word splitter cannot parse (an unclosed quote, a
// trailing backslash) is never judged a reader line, even when the words it
// seems to hold are all readers.
// PREVENTS: a command the hook cannot see whole reaching ./le without the
// agent's identity.
func TestSubagentUnparsableCommandNeedsTheIdentityLine(t *testing.T) {
	for _, command := range []string{
		`grep -n "unclosed notes.md`,
		`grep -n 'unclosed notes.md | head`,
		`cat notes.md \`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
}
