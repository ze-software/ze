// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session.go -- fromSubagent, the caller predicate these tests hold to the payload
// Related: session_identity_test.go -- runSubagentBash and the agent's identity line
package hookruntime

import (
	"os"
	"strings"
	"testing"
)

// forkVariable is a setting of the harness (fork mode on or off), present in
// the environment of every hook process of a session, the main thread's
// included. The hook does not read it: the tests set it to prove that.
const forkVariable = "CLAUDE_CODE_FORK_SUBAGENT"

// runAgentlessBash sends one Bash command through the pretool-bash entry with
// a payload that names no agent, and leaves the environment as the caller set
// it.
func runAgentlessBash(t *testing.T, command string) (code int, output, message string) {
	t.Helper()
	return runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
}

// TestForkVariableDoesNotMakeTheMainThreadASubagent sets the harness's fork
// setting to each value an owner can give it and sends the main thread's
// payload, which names no agent, through the predicate and through each of
// the four surfaces that read it: the identity check, the heavy-work check,
// the post-edit lint and the post-Bash gofmt advice.
//
// VALIDATES: with CLAUDE_CODE_FORK_SUBAGENT set to 1 or to 0, a payload with
// no agent id is the main thread's: a heavy line gets the main thread's
// refusal with the admitted form and no hand-back, a formatter and an
// admitted le action pass, a command outside the reader set is asked for no
// identity line and receives no changed input, advice carries no identity
// prefix, its Go edit is linted, and an unformatted file's advice tells it to
// run gofmt.
// PREVENTS: the setting read as a mark of a fork's hook process, which made
// the main thread a subagent: an identity line owed on every command, and its
// own tests refused by bashSubagentHeavy
// (plan/journal/gate-verdict-depends-on-the-machine.md, 2026-11-13).
func TestForkVariableDoesNotMakeTheMainThreadASubagent(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, value := range []string{"1", "0"} {
		t.Run("set to "+value, func(t *testing.T) {
			t.Setenv(forkVariable, value)
			main := context{payload: Payload{SessionID: "parent-17"}}
			if fromSubagent(main) {
				t.Errorf("a payload with no agent id is read as a subagent")
			}
			if got := commandIdentityPrefix(main); got != "" {
				t.Errorf("the main thread's advice prefix = %q, want none", got)
			}

			for _, command := range []string{`go test ./x`, `golangci-lint run`} {
				code, output, message := runAgentlessBash(t, command)
				if code != 2 || output != "" || !strings.Contains(message, "run raw, outside job admission") || !strings.Contains(message, "./le job run label ") {
					t.Errorf("%q: code=%d output=%q, want the main thread's raw-job refusal with the admitted form and no changed input: %s", command, code, output, message)
				}
				for _, added := range []string{handBackHeader, "Owed commands", "identity line"} {
					if strings.Contains(message, added) {
						t.Errorf("%q: the main thread's message holds %q, a subagent's text: %s", command, added, message)
					}
				}
			}
			for _, command := range []string{`./le test bgp plugin 42`, `./le heavy-probe run`, `gofmt -w x.go`, `./le spec current`, `go version`, `echo ok`} {
				code, output, message := runAgentlessBash(t, command)
				if code != 0 || output != "" {
					t.Errorf("%q: code=%d output=%q, want exit 0 and no changed input for the main thread: %s", command, code, output, message)
				}
				if strings.Contains(message, "identity line") {
					t.Errorf("%q: the main thread was asked for the identity line: %s", command, message)
				}
			}

			// The gofmt-advice case runs before the lint case: gitFixture
			// needs git from the real PATH, and lintProbeFixture narrows
			// PATH to its stand-in linter for the rest of this subtest.
			unformatted := gitFixture(t)
			writeHookFixture(t, unformatted, "pkg/bad.go", unformattedGo)
			input := map[string]any{"command": "sed -i s/a/b/ pkg/bad.go"}
			code, message, _ := Probe("postBashGofmt", Payload{ToolName: "Bash", ToolInput: input}, unformatted)
			if code != 2 || !strings.Contains(message, "If this command changed the file: gofmt -w <path>") {
				t.Errorf("the main thread is not told to run gofmt (code %d):\n%s", code, message)
			}
			if strings.Contains(message, "Owed commands") {
				t.Errorf("the main thread got the subagent's gofmt advice:\n%s", message)
			}

			root, edited, marker := lintProbeFixture(t)
			code, message, found := Probe("postFormatGo", Payload{ToolName: "Edit", ToolInput: map[string]any{"file_path": edited}}, root)
			if !found {
				t.Fatal("postFormatGo is not registered in nativeHookActions")
			}
			if _, err := os.Stat(marker); err != nil {
				t.Errorf("the main thread's edit did not start the linter: %v", err)
			}
			if code != 2 || !strings.Contains(message, "1 issue(s) in pkg/x.go") {
				t.Errorf("the linter's issue in the edited file did not block (code %d): %q", code, message)
			}
		})
	}
}

// TestAgentIDAloneMakesACallerASubagent removes the fork setting from the
// environment and sends a payload that names an agent, which is how the
// harness sends a fork's calls and every other subagent's.
//
// VALIDATES: the agent id in the payload is what makes a caller a subagent:
// with the variable unset, its command outside the reader set is refused with
// the agent's identity line, its heavy line is handed back, its advice
// carries its identity line, and its Go edit starts no linter.
// PREVENTS: a fork judged as the main thread once the variable is no longer
// read, and starting heavy work or sharing the main thread's identity.
func TestAgentIDAloneMakesACallerASubagent(t *testing.T) {
	// Setenv registers the restore, and Unsetenv then removes the variable
	// for this test.
	t.Setenv(forkVariable, "")
	if err := os.Unsetenv(forkVariable); err != nil {
		t.Fatal(err)
	}
	agent := context{payload: Payload{SessionID: "parent-17", AgentID: "agent-1"}}
	if !fromSubagent(agent) {
		t.Errorf("a payload naming an agent is not read as a subagent")
	}
	if got := commandIdentityPrefix(agent); got != identityLineOfAgent {
		t.Errorf("the agent's advice prefix = %q, want its identity line %q", got, identityLineOfAgent)
	}
	code, output, message := runSubagentBash(t, `./le spec current`)
	if code != 2 || output != "" || !strings.Contains(message, identityLineOfAgent) {
		t.Errorf("agent without its line: code=%d output=%q, want exit 2 and the line %q: %s", code, output, identityLineOfAgent, message)
	}
	for _, command := range []string{`go test ./x`, `golangci-lint run`, `gofmt -w x.go`} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 2 || output != "" || !handedBack(message, command) {
			t.Errorf("agent %q: code=%d output=%q, want exit 2 with the hand-back: %s", command, code, output, message)
		}
	}

	root, edited, marker := lintProbeFixture(t)
	payload := Payload{ToolName: "Edit", ToolInput: map[string]any{"file_path": edited}, AgentID: "agent-1"}
	code, message, found := Probe("postFormatGo", payload, root)
	if !found {
		t.Fatal("postFormatGo is not registered in nativeHookActions")
	}
	if code != 0 || message != "" {
		t.Errorf("a subagent's edit got a lint verdict (code %d): %q", code, message)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("a subagent's edit started the linter (marker stat: %v)", err)
	}
}
