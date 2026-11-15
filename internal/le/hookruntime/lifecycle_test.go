// Design: docs/contributing/committing.md -- the session hook reads the push debt derivation
package hookruntime

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
	"github.com/ze-software/ze/internal/le/derived"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// TestSubagentContextResolvesClaim drives the JSON hook entry over every current
// category and unresolved claims, so a filename cannot become an invented path.
func TestSubagentContextResolvesClaim(t *testing.T) {
	const claim = "spec-context-probe.md"
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{name: "missing", want: "no such spec"},
		{name: "duplicate", paths: []string{
			"plan/release/fixes/" + claim,
			"plan/post-release/engineering/" + claim,
		}, want: "two buckets"},
	}
	for _, category := range specpath.Categories() {
		tests = append(tests, struct {
			name  string
			paths []string
			want  string
		}{name: category.Name, paths: []string{category.Dir + "/" + claim}})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeHookFixture(t, root, "tmp/session/.session-context-parent", claim+"\n")
			for _, path := range test.paths {
				writeHookFixture(t, root, path, "# Spec: context probe\n")
			}
			code, output, diagnostics := runHook(t, root, "subagent-context",
				map[string]any{"session_id": "context-parent"})
			if code != 0 {
				t.Fatalf("code = %d, want advisory context; stderr: %s", code, diagnostics)
			}
			var response struct {
				Output struct {
					Event   string `json:"hookEventName"`
					Context string `json:"additionalContext"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(output), &response); err != nil {
				t.Fatal(err)
			}
			if response.Output.Event != "SubagentStart" {
				t.Fatalf("event = %q", response.Output.Event)
			}
			context := response.Output.Context
			if test.want != "" {
				for _, want := range []string{"Cannot resolve the session's claimed spec " + claim, test.want, "./le spec claim"} {
					if !strings.Contains(context, want) {
						t.Errorf("context missing %q: %s", want, context)
					}
				}
				if strings.Contains(context, "Spec claimed by the session that spawned you:") {
					t.Errorf("unresolved claim emitted a spec path: %s", context)
				}
				return
			}
			if !strings.Contains(context, "Spec claimed by the session that spawned you: "+test.paths[0]+"\n") {
				t.Errorf("context missing category-qualified path: %s", context)
			}
			if strings.Contains(context, "Cannot resolve") {
				t.Errorf("resolved claim emitted a failure: %s", context)
			}
		})
	}
}

// VALIDATES: AC-6, the identity line. The start text of a subagent whose
// payload carries its agent id holds the full line the agent types in front of
// a command; a payload with no agent id holds no identity line and no order
// to type one, because the Bash hook reads a call with no agent id as the
// main thread's and asks it for none, and says the refusal prints the line
// should a call name an agent; and neither text speaks of a first refusal.
// Neither claims that the hook adds the line, because the hook no longer
// changes a command.
// PREVENTS: a subagent learning its identity line only from a refused call.
func TestSubagentStartCarriesTheIdentityLine(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		env     map[string]string
		want    []string
		absent  []string
	}{
		{
			name:    "payload-with-agent-id",
			payload: map[string]any{"session_id": "context-parent", "agent_id": "agent-1"},
			want:    []string{"\nexport ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=context-parent; \n"},
			absent:  []string{"first refusal"},
		},
		{
			name:    "payload-without-agent-id",
			payload: map[string]any{"session_id": "context-parent"},
			// With no agent id there is no line the identity check compares,
			// so the start text prints none and does not order one typed.
			want:   []string{"names no agent id", "refusal then prints the exact line", "Parent session ID: context-parent"},
			absent: []string{"export CLAUDE_CODE_SESSION_ID=", "ZE_AGENT_ID=", "Start every Bash command", "first refusal"},
		},
		{
			// Review N2: an agent id that cannot name a file gets no line to
			// type; every command, readers included, is refused with
			// stop-and-report (review I3), and the start text says so. It
			// does not order a line typed "exactly as printed" when it
			// prints none (round 5, N2).
			name:    "payload-with-an-unusable-agent-id",
			payload: map[string]any{"session_id": "context-parent", "agent_id": "a/b"},
			want:    []string{"stop and report the agent id to the main thread", "refuses every command, readers included"},
			absent:  []string{"ZE_AGENT_ID=", "first refusal", "Start every Bash command", "exactly as printed"},
		},
		{
			// Review round 3, I1: an agent id past the id length bound is one
			// bashAgentIdentity refuses, so no line is built from it, cut or
			// whole; the agent is told to stop and report instead.
			name:    "payload-with-an-over-long-agent-id",
			payload: map[string]any{"session_id": "context-parent", "agent_id": strings.Repeat("a", idLengthMax(t)+1)},
			want:    []string{"stop and report the agent id to the main thread", "refuses every command, readers included", "Parent session ID: context-parent\n", lepath.IDRule()},
			absent:  []string{"ZE_AGENT_ID=", "...", "Start every Bash command", "exactly as printed"},
		},
		{
			// Review round 4: a payload that names an agent and carries no
			// session_id key resolves a parent session for the start text,
			// but bashAgentIdentity reads the payload's own session id and
			// refuses every command, so no line is printed: one built from
			// the resolved id is a line no check accepts.
			name:    "payload-with-an-agent-id-and-no-session-id",
			payload: map[string]any{"agent_id": "agent-1"},
			want:    []string{"carries no session id", "refuses every command, readers included", "stop and report the payload's session_id to the main thread"},
			absent:  []string{"ZE_AGENT_ID=", "CLAUDE_CODE_SESSION_ID=", "Start every Bash command"},
		},
		{
			// Round 5, N1: a payload that names an agent and carries no
			// session_id key, where no parent session resolves either (an
			// unsafe CLAUDE_CODE_SESSION_ID makes lepath refuse to resolve
			// one). The start text still says stop and report, with no
			// parent session and no line.
			name:    "payload-with-an-agent-id-no-session-id-and-no-resolved-session",
			payload: map[string]any{"agent_id": "agent-1"},
			env:     map[string]string{"CLAUDE_CODE_SESSION_ID": "a/b"},
			want:    []string{"carries no session id", "refuses every command, readers included", "stop and report the payload's session_id to the main thread"},
			absent:  []string{"ZE_AGENT_ID=", "CLAUDE_CODE_SESSION_ID=", "Parent session ID", "Start every Bash command"},
		},
		{
			// An over-long session id is neither echoed nor used in a line:
			// the Bash hook refuses every command of such a subagent with
			// stop-and-report, and the start text says the same.
			name:    "payload-with-an-over-long-session-id",
			payload: map[string]any{"session_id": strings.Repeat("s", idLengthMax(t)+1), "agent_id": "agent-1"},
			want:    []string{"session id cannot name a file", "stop and report the payload's session id to the main thread", lepath.IDRule()},
			absent:  []string{"ZE_AGENT_ID=", "CLAUDE_CODE_SESSION_ID=", "Parent session ID", "sss", "..."},
		},
		{
			// Ids at the bound are printed whole, in the exact line the Bash
			// hook compares.
			name:    "payload-with-ids-at-the-length-bound",
			payload: map[string]any{"session_id": strings.Repeat("s", idLengthMax(t)), "agent_id": strings.Repeat("a", idLengthMax(t))},
			want: []string{
				"\nexport ZE_AGENT_ID=" + strings.Repeat("a", idLengthMax(t)) + "; export CLAUDE_CODE_SESSION_ID=" + strings.Repeat("s", idLengthMax(t)) + "; \n",
				"Parent session ID: " + strings.Repeat("s", idLengthMax(t)) + "\n",
			},
			absent: []string{"..."},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			code, output, diagnostics := runHook(t, t.TempDir(), "subagent-context", test.payload)
			if code != 0 {
				t.Fatalf("code = %d, want advisory context; stderr: %s", code, diagnostics)
			}
			var response struct {
				Output struct {
					Context string `json:"additionalContext"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(output), &response); err != nil {
				t.Fatal(err)
			}
			context := response.Output.Context
			for _, want := range test.want {
				if !strings.Contains(context, want) {
					t.Errorf("start text missing %q: %s", want, context)
				}
			}
			for _, absent := range append(test.absent, "adds this prefix") {
				if strings.Contains(context, absent) {
					t.Errorf("start text holds %q: %s", absent, context)
				}
			}
		})
	}
}

// idLengthMax answers the longest id lepath accepts, found by asking the one
// id check rather than copying its bound, and fails the test unless that
// bound sits under the start text's echo bound: the start text prints ids
// uncut only because the check refuses every id that bound would cut.
func idLengthMax(t *testing.T) int {
	t.Helper()
	longest := 0
	for longest <= startTextErrorMax && lepath.ValidAgentID(strings.Repeat("a", longest+1)) {
		longest++
	}
	if longest == 0 || longest >= startTextErrorMax {
		t.Fatalf("the id check admits ids of %d bytes, want a bound in 1..%d so the start text never cuts one", longest, startTextErrorMax-1)
	}
	if lepath.ValidSessionID(strings.Repeat("s", longest+1)) || !lepath.ValidSessionID(strings.Repeat("s", longest)) {
		t.Fatalf("the session id check and the agent id check disagree on the %d-byte bound", longest)
	}
	return longest
}

// TestSubagentStartCarriesTheCallRules drives the subagent-context hook with
// a payload that carries a session and one that carries none (AC-6).
//
// VALIDATES: the start text states the rules a subagent's calls are judged
// by: the whole Read of a door, Edit or Write under plan/ and ai/rules/,
// batching, tests and builds in the main thread, and the budget with refused
// calls uncounted. TestSubagentStartTextNamesOwedCommands holds the owed
// commands, the owed lint and the commit rule.
// PREVENTS: an agent learning each rule from a refusal, a turn apiece.
func TestSubagentStartCarriesTheCallRules(t *testing.T) {
	for _, payload := range []map[string]any{
		{"session_id": "context-parent", "agent_id": "agent-1"},
		{"session_id": ""},
	} {
		code, output, diagnostics := runHook(t, t.TempDir(), "subagent-context", payload)
		if code != 0 {
			t.Fatalf("%v: code = %d, want advisory context; stderr: %s", payload, code, diagnostics)
		}
		var response struct {
			Output struct {
				Context string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(output), &response); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"Read a door or the Go style guide with the Read tool, whole",
			"Edit files under plan/ and ai/rules/ with Edit or Write only",
			"Put independent calls in one message",
			"Hand tests, builds, lint and formatters to the main thread",
			"A ze-work agent's budget is 100 hooked calls",
			"A call a hook refuses is not counted.",
		} {
			if !strings.Contains(response.Output.Context, want) {
				t.Errorf("%v: start text missing %q: %s", payload, want, response.Output.Context)
			}
		}
	}
}

// TestSubagentStartTextNamesOwedCommands drives the subagent-context hook the
// way TestSubagentStartCarriesTheCallRules does (AC-18).
//
// VALIDATES: the start text says what the Bash hook refuses from a subagent,
// names the `Owed commands` list and what each entry carries, names the
// changed-code lint as owed after a Go edit with its command, and says a
// subagent commits only a change that owes no execution.
// PREVENTS: a subagent learning the heavy-work refusal from a refused call,
// reading a silent post-edit hook as a clean lint, or committing code no one
// ran.
func TestSubagentStartTextNamesOwedCommands(t *testing.T) {
	for _, payload := range []map[string]any{
		{"session_id": "context-parent", "agent_id": "agent-1"},
		{"session_id": ""},
	} {
		code, output, diagnostics := runHook(t, t.TempDir(), "subagent-context", payload)
		if code != 0 {
			t.Fatalf("%v: code = %d, want advisory context; stderr: %s", payload, code, diagnostics)
		}
		var response struct {
			Output struct {
				Context string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(output), &response); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"the Bash hook refuses a test, build, vet, lint, formatter, `./le job run` and any heavy `./le` action from you",
			"End your report with an `Owed commands` list (each exact command, what it proves, the result you expect)",
			"the hook formats the file and does not lint it, so the changed-code lint is owed: `./le go lint run scope \"<pkg>\"`",
			"Commit only a change that owes no execution",
			"The main thread commits code.",
		} {
			if !strings.Contains(response.Output.Context, want) {
				t.Errorf("%v: start text missing %q: %s", payload, want, response.Output.Context)
			}
		}
	}
}

// TestSessionStartNamesDueJournalClasses drives the due-class line through the
// hook entry point over committed journals of 9 and 10 rows, and over a tree
// git cannot read. It proves the line appears at the threshold, stays absent
// below it, and that an unreadable journal prints nothing on stdout and says
// why on stderr.
func TestSessionStartNamesDueJournalClasses(t *testing.T) {
	const line = "journal: 1 problem classes are due for a fix pass (./le spec journal report)"
	tests := []struct {
		name string
		rows int
		git  bool
		want bool
	}{
		{name: "nine rows", rows: 9, git: true, want: false},
		{name: "ten rows", rows: 10, git: true, want: true},
		{name: "unreadable journal", rows: 10, git: false, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			// Seed every derived artifact so the hook renders nothing and this
			// test stays about the journal line.
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			journal := "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
			for day := 1; day <= test.rows; day++ {
				journal += fmt.Sprintf("| 2026-09-%02d | - | cli | symptom %d | fixed: fix |\n", day, day)
			}
			writeHookFixture(t, root, "plan/journal/recurring.md", journal)
			if test.git {
				commitHookFixture(t, root)
			}
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(noted, "journal: due classes not counted: ") == test.git {
				t.Fatalf("the hook wrote %q to stderr, want the unreadable-journal note only without git", noted)
			}
			if strings.Contains(printed, line) != test.want {
				t.Fatalf("the hook printed %q, want the due line %v", printed, test.want)
			}
			if !test.want && strings.Contains(printed, "journal:") {
				t.Fatalf("the hook printed %q, want no journal line on stdout", printed)
			}
		})
	}
}

// TestSessionStartCountsOnlyUncapturedOrRecurredClasses drives the due line
// through the hook over a class captured by a check. Twelve rows dated on or
// before the capture print no due line, where the same rows uncaptured would;
// one row dated after it prints the line, because a check capture is due on
// its first recurrence.
func TestSessionStartCountsOnlyUncapturedOrRecurredClasses(t *testing.T) {
	const line = "journal: 1 problem classes are due for a fix pass (./le spec journal report)"
	for _, test := range []struct {
		name  string
		after int
		want  bool
	}{{name: "rows before the capture", after: 0, want: false}, {name: "one row after", after: 1, want: true}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			writeHookFixture(t, root, "internal/guard/guard.go", "package guard\n\nfunc Guard() {}\n")
			writeHookFixture(t, root, "internal/guard/guard_test.go",
				"package guard\n\nimport \"testing\"\n\nfunc TestGuard(t *testing.T) {}\n")
			journal := "Captured: 2026-09-12 check internal/guard.Guard proof internal/guard.TestGuard\n\n" +
				"| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"
			for day := 1; day <= 12+test.after; day++ {
				journal += fmt.Sprintf("| 2026-09-%02d | - | cli | symptom %d | fixed: fix |\n", day, day)
			}
			writeHookFixture(t, root, "plan/journal/captured.md", journal)
			commitHookFixture(t, root)
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(noted, "journal:") {
				t.Fatalf("the hook wrote %q to stderr, want the journal counted", noted)
			}
			if strings.Contains(printed, line) != test.want {
				t.Fatalf("the hook printed %q, want the due line %v", printed, test.want)
			}
		})
	}
}

// commitHookFixture commits the whole fixture tree, because the journal
// report reads git HEAD and never the working tree.
func commitHookFixture(t *testing.T, root string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", "--all"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Ze Test", "-c", "commit.gpgsign=false",
			"commit", "--quiet", "--message=seed"},
	} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s in the fixture: %v\n%s", arguments[0], err, output)
		}
	}
}

// runSessionStart runs the session-start hook over one throwaway root and
// answers what it printed.
func runSessionStart(t *testing.T, root string) string {
	t.Helper()
	printed, _ := runSessionStartStreams(t, root)
	return printed
}

// runSessionStartStreams runs the session-start hook over one throwaway root
// and answers its stdout and its stderr.
func runSessionStartStreams(t *testing.T, root string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	hookSessionStart(context{root: root, input: map[string]any{}}, &out, &errOut)
	return out.String(), errOut.String()
}

func writeHookFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// artifactsWithPolicy answers the registered artifacts under one session-start
// policy, and fails when there are none: every assertion below is over a
// population, and an empty one agrees with anything.
func artifactsWithPolicy(t *testing.T, policy derived.SessionStartPolicy) []derived.Artifact {
	t.Helper()
	var held []derived.Artifact
	for _, artifact := range derived.All() {
		if artifact.SessionStart == policy {
			held = append(held, artifact)
		}
	}
	if len(held) == 0 {
		t.Fatalf("no registered artifact declares policy %d, so this test asserts nothing", policy)
	}
	return held
}

// TestNamedReadBuildsEveryStartupArtifact proves the on-demand hook builds the
// registry population previously rendered during session startup.
//
// VALIDATES: every formerly eager artifact can be rendered from a capable tree
// through the real pretool-bash entry when a command names it; no private list
// can omit a newly registered member. Startup leaves the same artifacts absent
// (TestSessionStartWithinHarnessBudget).
//
// PREVENTS: removing eager startup work also removing artifact availability.
func TestNamedReadBuildsEveryStartupArtifact(t *testing.T) {
	root := derivedFixture(t)
	artifacts := artifactsWithPolicy(t, derived.SessionStartBuild)
	for _, artifact := range artifacts {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("clear %s: %v", artifact.Path, err)
		}
	}

	for _, artifact := range artifacts {
		code, _, message := runHook(t, root, "pretool-bash", map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "cat " + artifact.Path},
		})
		if code != 0 {
			t.Fatalf("named read of %s refused: %s", artifact.Path, message)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); err != nil {
			t.Errorf("%s is still absent after its named read: %v", artifact.Path, err)
		}
	}
}

// TestSessionStartRendersNoDeferredArtifact is the budget half, over a tree that
// CAN render the deferred family.
//
// VALIDATES: an artifact registered SessionStartDefer is left absent by a
// session start, and no line claims it was built.
// PREVENTS: the 2026-09-11 regression. The five RFC artifacts registered with
// the three documentation indexes, so the hook rendered 194 shards inside its 5
// second timeout: measured at 2.68s and 2.03s with all eight present and 5.32s
// with the five absent. Absent is the common state, because their predicate
// covers every `*.go` write in every session. A hook killed at its timeout
// loses the whole session-start message.
//
// The corpus is what makes this a decision rather than an inability: the last
// step renders one of the deferred artifacts by hand and requires it to land.
func TestSessionStartRendersNoDeferredArtifact(t *testing.T) {
	root := derivedFixture(t)
	rfcCorpusFixture(t, root)
	// The roadmap renders from a committed plan tree (git HEAD), never the
	// working tree, so the fixture holds one spec and is a checkout, or that
	// member cannot render at all.
	writeHookFixture(t, root, "plan/release/fixes/spec-fixture.md",
		"# Spec: fixture\n\n| Field | Value |\n|---|---|\n| Status | ready |\n| Depends | - |\n"+
			"| Phase | 1/1 |\n| Updated | 2026-10-29 |\n| Priority | P1 |\n| Release reason | Session start fixture |\n")
	commitHookFixture(t, root)
	artifacts := artifactsWithPolicy(t, derived.SessionStartDefer)
	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Fatalf("%s is in the fixture before the hook ran, so its absence after proves nothing", artifact.Path)
		}
	}

	printed := runSessionStart(t, root)

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact.Path))); !os.IsNotExist(err) {
			t.Errorf("the session start rendered %s, which is deferred to the command that names it",
				artifact.Path)
		}
		if strings.Contains(printed, "Built "+artifact.Path) {
			t.Errorf("the hook reported building a deferred artifact:\n%s", printed)
		}
	}

	// The tree can render them, every one. Without this, a fixture that simply
	// cannot build a member of the family would pass every assertion above for
	// that member.
	for _, rendered := range artifacts {
		if err := rendered.Rebuild(root); err != nil {
			t.Fatalf("the fixture cannot render %s at all, so nothing above was a decision: %v",
				rendered.Path, err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rendered.Path))); err != nil {
			t.Fatalf("%s does not land in this fixture even when rendered on purpose: %v", rendered.Path, err)
		}
	}
}

// rfcCorpusFixture writes the smallest tree the RFC generator renders from: one
// enrolled summary gating one MUST, that RFC's own text, and the workflow
// directory the carrier walk reads.
//
// The same corpus drives the .ci scenario end to end
// (internal/test/fixture/misc_fixture_runner_rfcledger.go). It is spelled again
// here because this package cannot import that one, and both exist to make an
// absent artifact mean a decision rather than an empty tree.
func rfcCorpusFixture(t *testing.T, root string) {
	t.Helper()
	writeHookFixture(t, root, "docs/features/.keep", "")
	for rel, body := range rfc.FixtureFiles() {
		writeHookFixture(t, root, rel, body)
	}
}

// TestSessionStartLeavesAPresentArtifactAlone bounds what this hook does.
//
// The rebuild is ABSENT-ONLY because the hook has a budget: `.claude/settings.json`
// gives `le ai hooks session-start` 5 seconds, and rendering all three
// artifacts does not fit inside it. A hook killed at its timeout loses the
// whole session-start message with it, the push-gate warning
// included, and leaves every artifact after the kill
// point exactly as it found them. Measure it before changing this:
//
//	dir=$(./le session scratch ensure)
//	echo '{}' | time ./le ai hooks session-start > "$dir/session-start.log" 2>&1
//
// The cost of the bound is a STATED limitation: a write no Write or Edit hook
// sees leaves the artifact present and stale until the next hooked write to one
// of its inputs. `docs/contributing/navigating-the-code.md` names it, and the
// spec's Known Limitations carries it.
func TestSessionStartLeavesAPresentArtifactAlone(t *testing.T) {
	root := derivedFixture(t)
	present := filepath.Join(root, "ai", "DOCS-TO-CODE.md")
	const body = "written by something no hook saw\n"
	if err := os.WriteFile(present, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	printed := runSessionStart(t, root)

	after, err := os.ReadFile(present) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("read the artifact after the session start: %v", err)
	}
	if string(after) != body {
		t.Errorf("a present artifact was rebuilt, which the hook has no budget for:\n%s", after)
	}
	if strings.Contains(printed, "Built ai/DOCS-TO-CODE.md") {
		t.Errorf("the hook reported building an artifact that was already there:\n%s", printed)
	}
}

// TestSessionStartPrintsItsNoticeWhenAStepOverruns drives the hook with a step
// that outlasts the budget, the shape a loaded machine gives the ledger read.
//
// VALIDATES: the fixed notice is the first line whatever a step costs,
// the hook returns at its budget instead of waiting for the step, one stderr
// line names the step that ran out and every step after it, and no report
// after the cut reaches stdout.
// PREVENTS: the harness killing the hook at its 5 s timeout and dropping the
// whole message, the fixed notice included.
func TestSessionStartPrintsItsNoticeWhenAStepOverruns(t *testing.T) {
	root := t.TempDir()
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}
	release := make(chan struct{})
	budget, steps := sessionStartBudget, sessionStartSteps
	t.Cleanup(func() {
		close(release)
		sessionStartBudget, sessionStartSteps = budget, steps
	})
	sessionStartBudget = 100 * time.Millisecond
	slow := sessionStartStep{name: "slow", run: func(stdcontext.Context, *sessionStart, *sessionStartReport) { <-release }}
	sessionStartSteps = append([]sessionStartStep{slow}, steps...)

	started := time.Now()
	printed, noted := runSessionStartStreams(t, root)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("the hook returned after %s, want it cut at its %s budget", elapsed, sessionStartBudget)
	}
	if !strings.HasPrefix(printed, "Warning: RULE: Read spec + source files BEFORE writing any code") {
		t.Fatalf("the hook printed %q, want the fixed notice first", printed)
	}
	names := make([]string, 0, len(sessionStartSteps))
	for _, step := range sessionStartSteps {
		names = append(names, step.name)
	}
	want := "session-start: the 100ms budget ran out, so these reports were skipped: " + strings.Join(names, ", ") + "\n"
	if noted != want {
		t.Fatalf("the hook wrote %q to stderr, want %q", noted, want)
	}
	for _, report := range []string{"Clean tree", "uncommitted", "specs", "Tip:"} {
		if strings.Contains(printed, report) {
			t.Fatalf("the hook printed %q after the cut, want no report containing %q", printed, report)
		}
	}
}

// TestSessionStartReportsTheTree drives the tree report through the hook over
// a clean repository and over a directory git cannot read.
//
// VALIDATES: a clean repository prints "Clean tree" and no stderr note; a
// status git cannot give is said on stderr and never printed as a clean tree.
// PREVENTS: a failed git status reading as a clean tree, which the hook printed
// until 2026-09-26.
func TestSessionStartReportsTheTree(t *testing.T) {
	tests := []struct {
		name  string
		git   bool
		clean bool
		note  string
	}{
		{name: "clean repository", git: true, clean: true, note: ""},
		{name: "no repository", git: false, clean: false, note: "session-start: git status failed, so the tree was not counted: "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The identity is pinned: with none in the environment and no
			// harness ancestor, the hook mints one and caches it under
			// tmp/session/. Resolving the session also creates its
			// directory and .session-id there, so the fixture ignores tmp
			// as the checkout's own .gitignore does; without that the
			// session's own files read as untracked and the tree as dirty.
			t.Setenv("CLAUDE_CODE_SESSION_ID", "session-start-fixture")
			root := t.TempDir()
			for _, artifact := range derived.All() {
				writeHookFixture(t, root, artifact.Path, "# derived\n")
			}
			writeHookFixture(t, root, ".gitignore", "tmp\n")
			if test.git {
				commitHookFixture(t, root)
			}
			printed, noted := runSessionStartStreams(t, root)
			if strings.Contains(printed, "Clean tree") != test.clean {
				t.Fatalf("the hook printed %q, want the clean-tree line %v", printed, test.clean)
			}
			if test.note == "" && strings.Contains(noted, "session-start:") {
				t.Fatalf("the hook wrote %q to stderr, want no session-start note", noted)
			}
			if test.note != "" && !strings.Contains(noted, test.note) {
				t.Fatalf("the hook wrote %q to stderr, want %q", noted, test.note)
			}
		})
	}
}

// TestSessionStartPrintsAReadyReportPastTheDeadline asks for the next report
// when a report and the deadline are both ready, 100 times.
//
// VALIDATES: the ready report is answered every time.
// PREVENTS: select picking the deadline at random and printing a finished step
// as skipped, which one in two tries would do.
func TestSessionStartPrintsAReadyReportPastTheDeadline(t *testing.T) {
	deadline, cancel := stdcontext.WithCancel(t.Context())
	cancel()
	for try := range 100 {
		reports := make(chan *sessionStartReport, 1)
		reports <- &sessionStartReport{}
		if _, arrived := nextSessionStartReport(deadline, reports); !arrived {
			t.Fatalf("try %d: a ready report was dropped for a passed deadline", try)
		}
		if _, arrived := nextSessionStartReport(deadline, reports); arrived {
			t.Fatalf("try %d: an empty channel answered a report after the deadline", try)
		}
	}
}

// TestSessionStartReportsDerivedDebt drives the session-start hook over real
// git checkouts with a real upstream and asserts the debt line the derivation
// produces: one warning naming the refusal when a pushed commit carries a
// trailer nobody can read, nothing over a plain commit with no verify record,
// a stderr note naming the fix when the main checkout has no upstream, and
// silence in a linked worktree with none, because a worktree agent never
// pushes. Each run stays inside the hook's budget, which is what A-5 claims
// for one rev-list and one log.
// VALIDATES: AC-22, AC-11, AC-12, A-5.
func TestSessionStartReportsDerivedDebt(t *testing.T) {
	t.Run("a commit carrying a malformed trailer", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		derivedDebtCommit(t, root, "Review-owed: malformed")

		started := time.Now()
		printed, noted := runSessionStartStreams(t, root)
		if elapsed := time.Since(started); elapsed >= sessionStartBudget {
			t.Fatalf("the hook took %s, past its %s budget", elapsed, sessionStartBudget)
		}
		if !strings.Contains(printed, "push debt: 1 refusal(s); ./le commit push refuses until they clear") {
			t.Fatalf("the hook printed %q, want the push debt line", printed)
		}
		if !strings.Contains(printed, "malformed Review-owed: trailer") {
			t.Fatalf("the hook printed %q, want the refusal named", printed)
		}
		if strings.Contains(noted, "push debt") {
			t.Fatalf("the hook said on stderr %q over a derivation that answered", noted)
		}
	})
	t.Run("a plain commit with no verify record", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		derivedDebtCommit(t, root)
		printed, noted := runSessionStartStreams(t, root)
		if strings.Contains(printed, "push debt") || strings.Contains(noted, "push debt") {
			t.Fatalf("the hook warned over a HEAD owing nothing: stdout %q stderr %q", printed, noted)
		}
	})
	t.Run("the main checkout has no upstream", func(t *testing.T) {
		root := derivedDebtCheckout(t, false)
		derivedDebtCommit(t, root)
		printed, noted := runSessionStartStreams(t, root)
		if strings.Contains(printed, "push debt") {
			t.Fatalf("the hook printed %q, want nothing on stdout when it cannot derive", printed)
		}
		if !strings.Contains(noted, "git branch --set-upstream-to") {
			t.Fatalf("the hook said %q on stderr, want the missing upstream and its fix", noted)
		}
	})
	t.Run("a linked worktree has no upstream", func(t *testing.T) {
		root := derivedDebtCheckout(t, true)
		linked := filepath.Join(t.TempDir(), "linked")
		derivedDebtGit(t, root, "worktree", "add", "-q", "-b", "agent", linked)
		for _, artifact := range derived.All() {
			writeHookFixture(t, linked, artifact.Path, "# derived\n")
		}
		printed, noted := runSessionStartStreams(t, linked)
		if strings.Contains(printed, "push debt") || strings.Contains(noted, "push debt") {
			t.Fatalf("a linked worktree with no upstream spoke of debt: stdout %q stderr %q", printed, noted)
		}
	})
}

// derivedDebtCheckout is a committed Ze checkout on main with every derived
// artifact seeded, so the hook renders nothing else, and, when upstream is
// set, a bare upstream that holds the baseline.
func derivedDebtCheckout(t *testing.T, upstream bool) string {
	t.Helper()
	root := t.TempDir()
	writeHookFixture(t, root, "go.mod", "module fixture\n")
	writeHookFixture(t, root, "feature-gates.txt", "")
	derivedDebtGit(t, root, "init", "-q", "--initial-branch=main")
	derivedDebtGit(t, root, "add", "go.mod", "feature-gates.txt")
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline")
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}
	if !upstream {
		return root
	}
	remote := filepath.Join(t.TempDir(), "origin.git")
	derivedDebtGit(t, root, "init", "-q", "--bare", "--initial-branch=main", remote)
	derivedDebtGit(t, root, "remote", "add", "origin", remote)
	derivedDebtGit(t, root, "push", "-q", "-u", "origin", "main")
	return root
}

// derivedDebtCommit commits one fixture file with the fixed subject "a change",
// with each trailer as a line of the message's last paragraph, then verifies
// that HEAD resolves to the commit.
func derivedDebtCommit(t *testing.T, root string, trailers ...string) {
	t.Helper()
	const subject = "a change"
	path := "change-" + strings.ReplaceAll(subject, " ", "-") + ".txt"
	writeHookFixture(t, root, path, subject+"\n")
	derivedDebtGit(t, root, "add", "--", path)
	message := subject
	if len(trailers) != 0 {
		message += "\n\n" + strings.Join(trailers, "\n")
	}
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", message)
	derivedDebtGit(t, root, "rev-parse", "HEAD")
}

// TestSessionStartDebtLineNeverNamesVerify drives the session-start hook over
// a checkout whose push is refused and asserts the debt line names no verify.
// VALIDATES: the owner rule that no agent runs any `./le verify` command.
// PREVENTS: an agent told at session start to run a verify it may not run.
func TestSessionStartDebtLineNeverNamesVerify(t *testing.T) {
	root := derivedDebtCheckout(t, true)
	derivedDebtCommit(t, root, "Review-owed: malformed")
	printed, _ := runSessionStartStreams(t, root)
	at := strings.Index(printed, "push debt: ")
	if at < 0 {
		t.Fatalf("the hook printed %q, want a debt line", printed)
	}
	if strings.Contains(printed[at:], "le verify") {
		t.Fatalf("the debt warning %q names a verify", printed[at:])
	}
}

// The fixture texts of the agent rule-set tests. Each marker word stands for
// a rule of one part, so a test can say which part reached the start text.
const (
	fixtureBan             = "- Never run the fixture's banned verb."
	fixtureMainInstruction = "MAIN-INSTRUCTIONS-MARK"
	fixtureAgentDecision   = "AGENT-DECISION-MARK"
	fixtureMainDecision    = "MAIN-DECISION-MARK"
	// fixtureSituationalDecision marks the bullet of the fixture's one section
	// for one kind of work: named by range, never sent.
	fixtureSituationalDecision = "SITUATIONAL-DECISION-MARK"
)

// writeRuleSetTree writes a checkout holding a mirrored agent definition with
// the given frontmatter, and the instruction and decisions files when their
// text is not empty.
func writeRuleSetTree(t *testing.T, frontmatter, instructions, decisions string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		filepath.Join(ruleset.AgentDefinitionsDir, "ze-read.md"): "---\n" + frontmatter + "---\n\nBody.\n",
		ruleset.InstructionsFile:                                 instructions,
		ruleset.DecisionsFile:                                    decisions,
	}
	for name, body := range files {
		if body == "" {
			continue
		}
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// ruleSetInstructions is an INSTRUCTIONS.md whose hard bans section is
// followed by a main-session section the start text must not carry.
const ruleSetInstructions = "# Ze\n\nIntro.\n\n# " + ruleset.HardBansHeading + "\n\n## A ban\n" + fixtureBan + "\n\n# Owner Meta Decisions\n" + fixtureMainInstruction + "\n"

// ruleSetDecisions is a META-DECISIONS.md in three parts. Its every-agent part
// runs from line 4 (its heading) to line 6 (the blank line before the
// main-session heading), and the part for one kind of work holds one section,
// lines 12-14.
const ruleSetDecisions = "# Owner Meta Decisions\nOpening.\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- " + fixtureAgentDecision + "\n\n## " + ruleset.DecisionsMainPartHeading + "\n- " + fixtureMainDecision + "\n" +
	"\n## " + ruleset.DecisionsSituationalPartHeading + "\n\n### Lab work\n" + ruleset.SituationPrefix + "running a lab.\n- " + fixtureSituationalDecision + "\n"

// subagentStartText runs the subagent-context hook on a payload and returns
// the additionalContext it answered, failing the test on any other answer.
func subagentStartText(t *testing.T, root string, payload map[string]any) string {
	t.Helper()
	code, output, diagnostics := runHook(t, root, "subagent-context", payload)
	if code != 0 {
		t.Fatalf("code = %d, want advisory context; stderr: %s", code, diagnostics)
	}
	var response struct {
		Output struct {
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatal(err)
	}
	return response.Output.Context
}

// TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions
// drives the hook for an agent whose mirrored definition sets omitClaudeMd
// (spec agent-economy-3, AC-1).
//
// VALIDATES: the start text carries the rule-set heading, the hard bans read
// from ai/INSTRUCTIONS.md, the core and the trigger index by path, and the
// exact line range of the decisions file's every-agent part; it carries no
// text of the main-session sections of either file.
// PREVENTS: an agent that omits the instruction files starting with no ban,
// or reading the main-session decisions it was meant to skip. The same holds
// for a definition at the type's path whose name the reader cannot match
// (another name, uniformly indented frontmatter): it cannot be read for sure,
// so it gets the rule set rather than none.
func TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions(t *testing.T) {
	for _, test := range []struct {
		name        string
		frontmatter string
	}{
		{"omits", "name: ze-read\n" + omitInstructionsKey + ": true\n"},
		{"name-differs-from-type", "name: other\n" + omitInstructionsKey + ": false\n"},
		{"uniformly-indented-frontmatter", "  name: ze-read\n  " + omitInstructionsKey + ": true\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writeRuleSetTree(t, test.frontmatter, ruleSetInstructions, ruleSetDecisions)
			text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
			for _, want := range []string{
				"\n# " + agentRuleSetHeading + "\n",
				"# " + ruleset.HardBansHeading + "\n\n## A ban\n" + fixtureBan,
				ruleset.CoreFile,
				ruleset.TriggersFile,
				ruleset.DecisionsFile + " lines 4-6 (offset 4, limit 3), the part headed \"" + ruleset.DecisionsAgentPartHeading + "\"",
				"\".\nBefore work one of these situations names, read that section of " + ruleset.DecisionsFile + " too:\n" +
					"- Lab work (lines 12-14, offset 12, limit 3): running a lab.\n" +
					"The rest of that file binds the main session only.\n",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("start text missing %q:\n%s", want, text)
				}
			}
			for _, absent := range []string{fixtureMainInstruction, fixtureMainDecision, fixtureAgentDecision, fixtureSituationalDecision, "# Owner Meta Decisions"} {
				if strings.Contains(text, absent) {
					t.Errorf("start text holds %q, which the agent reads or skips by file:\n%s", absent, text)
				}
			}
		})
	}
}

// TestSubagentStartAddsNoRuleSetForAnAgentThatLoadsTheInstructions drives
// the hook for agents the harness starts with the instruction files (AC-3).
//
// VALIDATES: no rule set for a definition whose name matches the type and
// that sets no omitClaudeMd or a bare false, a type with no mirrored
// definition, a path and a plugin-scoped type. A definition whose name does
// not match is in the test above: it gets the rule set.
// PREVENTS: an agent that already loads the files paying for the set twice,
// and an agent type reaching a path outside the mirror.
func TestSubagentStartAddsNoRuleSetForAnAgentThatLoadsTheInstructions(t *testing.T) {
	tests := []struct {
		name        string
		frontmatter string
		agentType   string
	}{
		{"no-omit-line", "name: ze-read\n", "ze-read"},
		{"omit-false", "name: ze-read\n" + omitInstructionsKey + ": false\n", "ze-read"},
		{"no-definition", "name: ze-read\n" + omitInstructionsKey + ": true\n", "general-purpose"},
		{"path-type", "name: ze-read\n" + omitInstructionsKey + ": true\n", "../" + ruleset.AgentDefinitionsDir + "/ze-read"},
		{"plugin-type", "name: ze-read\n" + omitInstructionsKey + ": true\n", "plugin:ze-read"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeRuleSetTree(t, test.frontmatter, ruleSetInstructions, ruleSetDecisions)
			text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": test.agentType})
			for _, absent := range []string{agentRuleSetHeading, fixtureBan, "Cannot read your agent definition"} {
				if strings.Contains(text, absent) {
					t.Errorf("start text holds %q:\n%s", absent, text)
				}
			}
		})
	}
}

// TestSubagentStartSurvivesAMissingRuleFile drives the hook for an agent that
// omits the instruction files in a checkout holding neither ai/INSTRUCTIONS.md
// nor ai/META-DECISIONS.md (R-1).
//
// VALIDATES: the hook still answers, names the file it could not read, tells
// the agent to read the hard bans section itself, and sends the decisions file
// whole rather than a range.
// PREVENTS: a missing rule file leaving an agent with no bans and no pointer.
func TestSubagentStartSurvivesAMissingRuleFile(t *testing.T) {
	root := writeRuleSetTree(t, "name: ze-read\n"+omitInstructionsKey+": true\n", "", "")
	text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
	for _, want := range []string{
		agentRuleSetHeading,
		"Cannot read the hard bans: ",
		ruleset.InstructionsFile,
		"Read the \"# " + ruleset.HardBansHeading + "\" section of " + ruleset.InstructionsFile + " before acting.",
		ruleset.CoreFile,
		ruleset.TriggersFile,
		ruleset.DecisionsFile + " whole: its every-agent part cannot be found",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("start text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, " lines ") {
		t.Errorf("start text names a line range of a file that does not exist:\n%s", text)
	}
}

// TestSubagentStartSendsTheWholeDecisionsFileWhenAPartHeadingIsMissing drives
// the hook over decisions files whose two parts cannot be found: no part
// heading (the file before it is split), one heading only, and the two in the
// wrong order.
//
// VALIDATES: the start text names the heading it could not find or order, and
// sends the whole file, never a range.
// PREVENTS: an empty or guessed range that hides decisions an agent must keep.
func TestSubagentStartSendsTheWholeDecisionsFileWhenAPartHeadingIsMissing(t *testing.T) {
	tests := []struct {
		name      string
		decisions string
		want      string
	}{
		{"no-part-heading", "# Owner Meta Decisions\n\n## Goal and scope\n- A rule.\n", "0 headings \"" + ruleset.DecisionsAgentPartHeading + "\""},
		{"no-main-heading", "# Owner Meta Decisions\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- A rule.\n", "0 headings \"" + ruleset.DecisionsMainPartHeading + "\""},
		{"main-first", "# Owner Meta Decisions\n\n## " + ruleset.DecisionsMainPartHeading + "\n- A.\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- B.\n", "puts the heading \"" + ruleset.DecisionsMainPartHeading + "\" before"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeRuleSetTree(t, "name: ze-read\n"+omitInstructionsKey+": true\n", ruleSetInstructions, test.decisions)
			text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
			for _, want := range []string{ruleset.DecisionsFile + " whole: its every-agent part cannot be found", test.want} {
				if !strings.Contains(text, want) {
					t.Errorf("start text missing %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, " lines ") {
				t.Errorf("start text names a line range:\n%s", text)
			}
		})
	}
}

// TestSubagentStartSendsTheWholeDecisionsFileWhenItsSectionsCannotBeListed
// drives the hook over decisions files whose every-agent part is found but
// whose sections for one kind of work cannot be listed: no third part, a
// section with no situation, and a list longer than startTextSectionsMax.
//
// VALIDATES: the start text sends the whole file with the reason, and names no
// range, so the agent reads every section rather than none (AC-15).
// PREVENTS: an agent told to read the every-agent range alone, as if no
// section bound its work, when the sections cannot be listed.
func TestSubagentStartSendsTheWholeDecisionsFileWhenItsSectionsCannotBeListed(t *testing.T) {
	twoParts := "# Owner Meta Decisions\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- A.\n\n## " + ruleset.DecisionsMainPartHeading + "\n- B.\n"
	third := "\n## " + ruleset.DecisionsSituationalPartHeading + "\n"
	long := third
	for index := range 12 {
		long += "### Section " + strconv.Itoa(index) + "\n" + ruleset.SituationPrefix + strings.Repeat("running a lab, ", 10) + "\n- " + fixtureSituationalDecision + "\n"
	}
	tests := []struct {
		name      string
		decisions string
		want      string
	}{
		{"no-third-part", twoParts, "0 headings \"" + ruleset.DecisionsSituationalPartHeading + "\""},
		{"no-situation", twoParts + third + "### Lab work\n- " + fixtureSituationalDecision + "\n", "section \"Lab work\" names no work"},
		{"list-past-the-bound", twoParts + long, "more than the " + strconv.Itoa(startTextSectionsMax) + " this start text carries"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeRuleSetTree(t, "name: ze-read\n"+omitInstructionsKey+": true\n", ruleSetInstructions, test.decisions)
			text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
			for _, want := range []string{ruleset.DecisionsFile + " whole: ", test.want} {
				if !strings.Contains(text, want) {
					t.Errorf("start text missing %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, " lines ") {
				t.Errorf("start text names a line range:\n%s", text)
			}
			if strings.Contains(text, fixtureSituationalDecision) {
				t.Errorf("start text carries a section's bullet:\n%s", text)
			}
		})
	}
}

// TestSubagentStartCutsALongSectionHeading drives the hook over a decisions
// file whose one section for one kind of work has a 300-byte heading.
//
// VALIDATES: the heading, file content echoed into the start text, is cut to
// startTextErrorMax bytes and marked, while the situation and the range stay
// whole.
// PREVENTS: a heading the checkout chooses spending the start text's room.
func TestSubagentStartCutsALongSectionHeading(t *testing.T) {
	heading := strings.Repeat("h", 300)
	decisions := "# Owner Meta Decisions\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- A.\n\n## " + ruleset.DecisionsMainPartHeading + "\n- B.\n" +
		"\n## " + ruleset.DecisionsSituationalPartHeading + "\n\n### " + heading + "\n" + ruleset.SituationPrefix + "running a lab.\n- " + fixtureSituationalDecision + "\n"
	root := writeRuleSetTree(t, "name: ze-read\n"+omitInstructionsKey+": true\n", ruleSetInstructions, decisions)
	text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
	want := "\n- " + heading[:startTextErrorMax] + "... (lines 11-13, offset 11, limit 3): running a lab.\n"
	if !strings.Contains(text, want) {
		t.Errorf("start text missing %q:\n%s", want, text)
	}
	if strings.Contains(text, heading) {
		t.Errorf("start text carries the whole 300-byte heading:\n%s", text)
	}
}

// TestSubagentStartListsEachSituationalSectionOfTheRepository drives the hook
// over the repository's own decisions file.
//
// VALIDATES: the start text lists every section ruleset.SituationalDecisions
// answers, each with its heading, its exact range and its whole situation,
// uncut (AC-15).
// PREVENTS: a section an agent never learns to read, or a situation cut short
// of the words that name the agent's work.
func TestSubagentStartListsEachSituationalSectionOfTheRepository(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sections, err := ruleset.SituationalDecisions(repository)
	if err != nil {
		t.Fatalf("SituationalDecisions: %v", err)
	}
	text := repositoryStartText(t, "feature/agent-economy-3-preamble-subagent-start-text")
	for _, section := range sections {
		want := "\n- " + section.Heading + " (lines " + strconv.FormatInt(section.First, 10) + "-" + strconv.FormatInt(section.Last, 10) +
			", offset " + strconv.FormatInt(section.First, 10) + ", limit " + strconv.FormatInt(section.Last-section.First+1, 10) +
			"): " + section.Situation + "\n"
		if !strings.Contains(text, want) {
			t.Errorf("start text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, ruleset.DecisionsFile+" whole: ") {
		t.Errorf("start text sends the whole decisions file over the repository's own:\n%s", text)
	}
}

// subagentContextCap is the harness's limit on one hook's additionalContext,
// in characters. Over it the harness swaps the text for a file path and a
// preview, and never asks the agent to read the file (Claude Code hooks
// reference, "Add context for Claude").
const subagentContextCap = 10000

// subagentContextReserve is the part of the cap the measured start text must
// leave free, in characters. It reserves room for what a real start carries
// beyond this fixture: a spec path or branch name longer than the realistic
// ones driven here, the bounded error line (startTextErrorMax twice) that
// replaces the spec line when a claim cannot be resolved, and a few lines of
// growth in the hard bans section before the next change re-reads this test.
const subagentContextReserve = 1500

// TestSubagentStartStaysUnderTheHarnessContextCap drives the hook for an
// agent that omits the instruction files, over the repository's own
// ai/INSTRUCTIONS.md, ai/META-DECISIONS.md and ze-work definition, from a
// session that has claimed a spec, on a named branch, under a real session id.
//
// VALIDATES: the whole start text, spec line, identity line, scratch path and
// rule set included, stays under the cap less subagentContextReserve, counted
// in UTF-16 code units as a JavaScript string length counts them.
// PREVENTS: a longer hard bans section or start text turning the agent's
// rules into a path it is never told to open.
func TestSubagentStartStaysUnderTheHarnessContextCap(t *testing.T) {
	text := repositoryStartText(t, "feature/agent-economy-3-preamble-subagent-start-text")
	if !strings.Contains(text, "\n# "+agentRuleSetHeading+"\n") {
		t.Fatalf("the ze-work definition does not omit the instruction files, so the cap was not measured:\n%s", text)
	}
	for _, want := range []string{"Spec claimed by the session that spawned you: " + capSpecPath, "Branch: feature/agent-economy-3-preamble-subagent-start-text", "Parent session scratch: "} {
		if !strings.Contains(text, want) {
			t.Fatalf("start text missing %q, so the measure is of a shorter start than a real one:\n%s", want, text)
		}
	}
	if length := len(utf16.Encode([]rune(text))); length >= subagentContextCap-subagentContextReserve {
		t.Fatalf("start text is %d characters, at or over the harness cap of %d less the %d reserved", length, subagentContextCap, subagentContextReserve)
	}
}

// TestSubagentStartBoundsALongBranchName drives the same start as
// TestSubagentStartStaysUnderTheHarnessContextCap on a branch whose name is
// longer than startTextErrorMax. Git keeps a ref as a file, so its last
// component stays under the 255-byte file name limit: 250 bytes here.
//
// VALIDATES: the start text echoes the branch cut to at most
// startTextErrorMax bytes, marked "...", and stays under the harness cap less
// subagentContextReserve.
// PREVENTS: a value the checkout chooses spending the reserve, so the rule
// set becomes a path the agent is never told to open.
func TestSubagentStartBoundsALongBranchName(t *testing.T) {
	const xs = 250
	branch := "feature/" + strings.Repeat("x", xs)
	text := repositoryStartText(t, branch)
	// The fixture guard: git answers no branch it did not create, and the
	// start text then says "unknown", which no bound acted on.
	if strings.Contains(text, "- Branch: unknown\n") {
		t.Fatalf("the fixture's %d-byte branch was not created (the start text says unknown), so nothing was measured", len(branch))
	}
	if want := "- Branch: " + branch[:startTextErrorMax] + "...\n"; !strings.Contains(text, want) {
		t.Fatalf("the long branch was not echoed cut to %d bytes and marked, want %q in:\n%s", startTextErrorMax, want, text)
	}
	if strings.Contains(text, strings.Repeat("x", xs)) {
		t.Errorf("start text echoes the whole %d-byte branch", len(branch))
	}
	if length := len(utf16.Encode([]rune(text))); length >= subagentContextCap-subagentContextReserve {
		t.Errorf("start text is %d characters, at or over the harness cap of %d less the %d reserved", length, subagentContextCap, subagentContextReserve)
	}
}

// capSpecPath is the spec repositoryStartText's session has claimed.
const capSpecPath = "plan/post-release/engineering/spec-agent-economy-3-preamble-subagent-start-text.md"

// repositoryStartText answers the start text of a ze-work agent over the
// repository's own ai/INSTRUCTIONS.md, ai/META-DECISIONS.md and ze-work
// definition, from a session that has claimed capSpecPath, on branch, under a
// real session id.
func repositoryStartText(t *testing.T, branch string) string {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	const (
		sessionID = "972de0b2-472a-4cf3-9820-6385b6ff38e0"
		claim     = "spec-agent-economy-3-preamble-subagent-start-text.md"
	)
	derivedDebtGit(t, root, "init", "-q", "-b", branch)
	writeHookFixture(t, root, "tmp/session/.session-"+sessionID, claim+"\n")
	writeHookFixture(t, root, capSpecPath, "# Spec: agent economy 3\n")
	for _, name := range []string{ruleset.InstructionsFile, ruleset.DecisionsFile, filepath.Join("ai", "agents", "ze-work.md")} {
		body, err := os.ReadFile(filepath.Join(repository, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, name)
		if name == filepath.Join("ai", "agents", "ze-work.md") {
			target = filepath.Join(root, ruleset.AgentDefinitionsDir, "ze-work.md")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return subagentStartText(t, root, map[string]any{"session_id": sessionID, "agent_id": "acc391cc13cdcd73d", "agent_type": "ze-work"})
}

// TestSubagentStartBoundsAnUnresolvableClaim drives the hook from a session
// whose claim marker holds a claim far longer than any spec name.
//
// VALIDATES: the "Cannot resolve the session's claimed spec" line echoes at
// most startTextErrorMax bytes of the claim and of the error, marked "...".
// PREVENTS: a marker or an error text of any size pushing the agent rule set
// past the harness cap.
func TestSubagentStartBoundsAnUnresolvableClaim(t *testing.T) {
	root := t.TempDir()
	claim := "spec-" + strings.Repeat("x", 4*startTextErrorMax) + ".md"
	writeHookFixture(t, root, "tmp/session/.session-context-parent", claim+"\n")
	text := subagentStartText(t, root, map[string]any{"session_id": "context-parent"})
	if !strings.Contains(text, "Cannot resolve the session's claimed spec ") {
		t.Fatalf("the claim was not reported unresolved, so the bound was not driven:\n%s", text)
	}
	if strings.Contains(text, strings.Repeat("x", startTextErrorMax)) {
		t.Errorf("start text echoes more than %d bytes of the claim:\n%s", startTextErrorMax, text)
	}
	if !strings.Contains(text, "...") {
		t.Errorf("start text does not mark the cut claim:\n%s", text)
	}
}

// TestStartTextBoundedCutsOnARuneBoundary holds the bound to its contract.
//
// VALIDATES: a value at the bound is answered whole; a longer one is cut to at
// most startTextErrorMax bytes, never inside a rune, and marked "...".
// PREVENTS: an echo that splits a UTF-8 sequence into an invalid string.
func TestStartTextBoundedCutsOnARuneBoundary(t *testing.T) {
	whole := strings.Repeat("a", startTextErrorMax)
	if got := startTextBounded(whole); got != whole {
		t.Errorf("a value at the bound was changed: %q", got)
	}
	long := strings.Repeat("a", startTextErrorMax-1) + "é" + "tail"
	got := startTextBounded(long)
	if got != strings.Repeat("a", startTextErrorMax-1)+"..." {
		t.Errorf("startTextBounded = %q, want the value cut before the two-byte rune", got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("startTextBounded answered invalid UTF-8: %q", got)
	}
	if got := startTextBounded("one\ntwo\r\nthree"); got != "one two  three" {
		t.Errorf("startTextBounded = %q, want every line break read as a space", got)
	}
}

// writeAgentDefinition writes one mirrored definition named agentType.md
// with the given frontmatter, and answers the checkout root.
func writeAgentDefinition(t *testing.T, agentType, frontmatter string) string {
	t.Helper()
	root := t.TempDir()
	writeHookFixture(t, root, filepath.Join(ruleset.AgentDefinitionsDir, agentType+".md"), "---\n"+frontmatter+"---\n\nBody.\n")
	return root
}

// TestAgentOmitsInstructionsReadsTheFrontmatterTheHarnessReads holds the
// frontmatter reader to the values YAML gives the harness.
//
// VALIDATES: a trailing comment, a capitalised or quoted true, a quoted name,
// and an indented key inside a multi-line description (for name and for
// omitClaudeMd) are each read as an agent that omits the instruction files;
// so are an indented "---" inside a description, which does not close the
// block, a quoted key, and a repeated key any occurrence of which is not a
// bare false; so is a definition at the type's path whose name the reader
// cannot match (another name, a name only indented, uniformly indented
// frontmatter, no name), whatever its omitClaudeMd; absent and false under a
// matching name, with or without a comment or repeated, are read as loading
// them. Method: each row writes one definition and asks
// agentOmitsInstructions, so a reader that took one occurrence of a repeated
// key, read an indented line as a key or as the closing line, kept a key's
// quotes or compared the raw text would turn a row red.
// PREVENTS: an agent that starts without the instruction files given no ban
// because its definition spells the value in another valid way.
func TestAgentOmitsInstructionsReadsTheFrontmatterTheHarnessReads(t *testing.T) {
	const key = omitInstructionsKey
	tests := []struct {
		name        string
		frontmatter string
		omits       bool
	}{
		{"comment-after-true", "name: ze-read\n" + key + ": true # see the spec\n", true},
		{"capitalised-true", "name: ze-read\n" + key + ": True\n", true},
		{"quoted-true", "name: ze-read\n" + key + ": \"true\"\n", true},
		{"quoted-name", "name: \"ze-read\"\n" + key + ": true\n", true},
		{"single-quoted-name", "name: 'ze-read'\n" + key + ": true\n", true},
		{"indented-name-in-description", "name: ze-read\ndescription: |\n  Reads.\n  name: x\n" + key + ": true\n", true},
		{"indented-false-in-description", key + ": true\nname: ze-read\ndescription: >\n  Reads.\n  " + key + ": false\n", true},
		{"tab-indented-false", "name: ze-read\n" + key + ": true\n\t" + key + ": false\n", true},
		{"repeated-key-true-then-false", "name: ze-read\n" + key + ": true\n" + key + ": false\n", true},
		{"repeated-key-false-then-true", "name: ze-read\n" + key + ": false\n" + key + ": true\n", true},
		{"indented-closing-line-in-description", "name: ze-read\ndescription: |\n  a\n  ---\n" + key + ": true\n", true},
		{"quoted-key", "name: ze-read\n\"" + key + "\": true\n", true},
		{"single-quoted-key", "name: ze-read\n'" + key + "': true\n", true},
		{"repeated-false", "name: ze-read\n" + key + ": false\n" + key + ": false\n", false},
		{"crlf-lines", "name: ze-read\r\n" + key + ": true\r\n", true},
		{"absent", "name: ze-read\n", false},
		{"false", "name: ze-read\n" + key + ": false\n", false},
		{"false-with-comment", "name: ze-read\n" + key + ": false # loads them\n", false},
		// A definition at the type's path whose name the reader cannot match
		// cannot be read for sure, so it takes the side that sends the rule set.
		{"name-differs", "name: other\n" + key + ": false\n", true},
		{"indented-name-only", "description: |\n  name: ze-read\n" + key + ": false\n", true},
		{"uniformly-indented-frontmatter", "  name: ze-read\n  " + key + ": true\n", true},
		{"no-name", key + ": false\n", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeAgentDefinition(t, "ze-read", test.frontmatter)
			omits, err := agentOmitsInstructions(root, "ze-read")
			if err != nil {
				t.Fatalf("agentOmitsInstructions: %v", err)
			}
			if omits != test.omits {
				t.Errorf("agentOmitsInstructions = %v, want %v for:\n%s", omits, test.omits, test.frontmatter)
			}
		})
	}
}

// TestValidAgentTypeAdmitsOnlyAFileName holds the guard that keeps a payload's
// agent type from naming a path outside the mirror.
//
// VALIDATES: letters, digits, '-' and '_' up to agentTypeLengthMax are
// admitted; a dot segment, a separator, a plugin scope, non-ASCII, one
// character over the bound and the empty type are refused.
// PREVENTS: a hostile agent type reading a file outside .claude/agents, or a
// type the harness could not report reaching the reader.
func TestValidAgentTypeAdmitsOnlyAFileName(t *testing.T) {
	tests := []struct {
		agentType string
		valid     bool
	}{
		{"ze-read", true},
		{"ze_work2", true},
		{"ZE-Read", true},
		{strings.Repeat("a", agentTypeLengthMax), true},
		{"", false},
		{strings.Repeat("a", agentTypeLengthMax+1), false},
		{"..", false},
		{"ze.read", false},
		{"../ze-read", false},
		{"agents/ze-read", false},
		{"ze\\read", false},
		{"plugin:ze-read", false},
		{"zé-read", false},
		{"ze read", false},
	}
	for _, test := range tests {
		if got := validAgentType(test.agentType); got != test.valid {
			t.Errorf("validAgentType(%q) = %v, want %v", test.agentType, got, test.valid)
		}
	}
}

// TestSubagentStartNamesTheFilesWhenTheDefinitionCannotBeRead drives the hook
// for an agent type whose definition path is a directory, which exists and
// cannot be read as a file.
//
// VALIDATES: the start text says it cannot read the definition and names the
// hard bans section and the three files to read, and carries no rule set.
// PREVENTS: an unreadable definition read as an agent that loads the files,
// so the agent starts with no ban and no pointer.
func TestSubagentStartNamesTheFilesWhenTheDefinitionCannotBeRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ruleset.AgentDefinitionsDir, "ze-read.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	text := subagentStartText(t, root, map[string]any{"session_id": "context-parent", "agent_id": "agent-1", "agent_type": "ze-read"})
	for _, want := range []string{
		"Cannot read your agent definition (",
		"read its \"# " + ruleset.HardBansHeading + "\" section",
		ruleset.InstructionsFile, ruleset.CoreFile, ruleset.TriggersFile, ruleset.DecisionsFile,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("start text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "# "+agentRuleSetHeading) {
		t.Errorf("start text carries a rule set read from no definition:\n%s", text)
	}
}

// Begin and end lines of the hard bans block in an agent definition body.
const (
	agentBansBegin = "<!-- hard-bans: begin"
	agentBansEnd   = "<!-- hard-bans: end -->"
)

// TestAgentDefinitionsCarryTheHardBansOfTheInstructions reads every
// definition under ai/agents that omits the instruction files (R-1).
//
// VALIDATES: every definition's frontmatter name, as the hook's reader reads
// it, equals its file stem, so a definition the reader cannot read fails
// rather than being skipped as one that loads the files; and each definition
// that omits them holds one block between the begin and end
// lines, and the block equals the "# Hard Bans" section of ai/INSTRUCTIONS.md;
// its body names the start text's rule-set heading (agentRuleSetHeading) and
// the three files the rule set has the agent read, which is the fallback a
// start with no hook text follows; and its frontmatter name equals its file
// stem, because the harness reports the name and the hook reads the file the
// name predicts. At least one definition omits the files, so the test cannot
// pass empty.
// PREVENTS: the copy a hookless start relies on drifting from the bans, a
// fallback that names a heading the hook no longer writes, and a definition the
// hook never finds because its name and its file disagree.
func TestAgentDefinitionsCarryTheHardBansOfTheInstructions(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	section, err := ruleset.HardBans(repository)
	if err != nil {
		t.Fatal(err)
	}
	bans := strings.TrimSpace(section.Text)
	definitions, err := filepath.Glob(filepath.Join(repository, "ai", "agents", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	omitting := 0
	for _, definition := range definitions {
		body, err := os.ReadFile(definition)
		if err != nil {
			t.Fatal(err)
		}
		fields := frontmatterFields(string(body))
		// Every definition is judged here, before the omit filter: one whose
		// name the reader cannot read (indented frontmatter, an unclosed
		// block) also yields no omitClaudeMd, so the filter would skip it.
		if stem := strings.TrimSuffix(filepath.Base(definition), ".md"); len(fields["name"]) != 1 || frontmatterUnquoted(fields["name"][0]) != stem {
			t.Errorf("%s: the hook's frontmatter reader reads name %q, not the file stem %q, so the hook cannot tell whether this agent loads the instruction files", definition, fields["name"], stem)
			continue
		}
		if !frontmatterOmits(fields[omitInstructionsKey]) {
			continue
		}
		omitting++
		// The body wraps at any space, so the fallback is judged with its line
		// breaks read as spaces.
		prose := strings.Join(strings.Fields(string(body)), " ")
		for _, want := range []string{agentRuleSetHeading, ruleset.CoreFile, ruleset.TriggersFile, ruleset.DecisionsFile} {
			if !strings.Contains(prose, want) {
				t.Errorf("%s omits the instruction files and its fallback does not name %q", definition, want)
			}
		}
		_, rest, found := strings.Cut(string(body), "\n"+agentBansBegin)
		if !found {
			t.Errorf("%s omits the instruction files and has no %q line", definition, agentBansBegin)
			continue
		}
		_, rest, _ = strings.Cut(rest, "\n")
		block, _, found := strings.Cut(rest, "\n"+agentBansEnd)
		if !found {
			t.Errorf("%s has no %q line after its begin line", definition, agentBansEnd)
			continue
		}
		if strings.TrimSpace(block) != bans {
			t.Errorf("%s: hard bans block differs from the %q section of %s; copy the section between the two lines", definition, ruleset.HardBansHeading, ruleset.InstructionsFile)
		}
	}
	if omitting == 0 {
		t.Fatalf("no definition under ai/agents sets %s: true", omitInstructionsKey)
	}
}

func derivedDebtGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
