// Related: fixture_catalog.go -- the subagent heavy-work sites, their expected exit and messages
// Related: reader_row_sites_test.go -- the sibling proof, for the reader rows a subagent types
// Related: heavy_linked_test.go -- why an le line is judged only where its area is linked

package aihooks

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
)

// subagentHeavySitePrefix starts the name of every fixture site that runs a
// line as a subagent to prove the heavy-work check.
const subagentHeavySitePrefix = "pretool-bash-subagent-heavy-"

// subagentHeavyIdentity is the identity line agent-1 of session parent-17
// types in front of a command that is no reader line.
const subagentHeavyIdentity = "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; "

// subagentHeavyHeader is the part of the hand-back refusal no other check
// writes, so its presence names the check that answered.
const subagentHeavyHeader = "a subagent does not run"

// subagentHeavySite is the line one fixture site runs: the command, whether
// the agent types its identity line in front of it, and the exit the main
// thread gets for the same command, which is the site's control.
type subagentHeavySite struct {
	command  string
	identity bool
	mainExit int
}

// subagentHeavySites maps each heavy-work fixture site to its line. The le
// action is this package's own, so every binary that runs these sites links
// it, and it registered as expensive where its verb registers (register.go).
var subagentHeavySites = map[string]subagentHeavySite{
	subagentHeavySitePrefix + "go-test-carrying-its-identity-line-refused":        {command: "go test ./internal/core/env", identity: true, mainExit: 2},
	subagentHeavySitePrefix + "le-action-carrying-its-identity-line-refused":      {command: "./le ai hooks unit", identity: true, mainExit: 0},
	subagentHeavySitePrefix + "go-test-with-a-stated-reason-refused":              {command: "ZE_ADMIT_RAW=\"one case\" go test ./internal/core/env", identity: true, mainExit: 0},
	subagentHeavySitePrefix + "reader-naming-go-test-passes":                      {command: "grep -n 'go test ./internal/core/env' go.mod", identity: false, mainExit: 0},
	subagentHeavySitePrefix + "light-le-action-carrying-its-identity-line-passes": {command: "./le spec current", identity: true, mainExit: 0},
}

// TestSubagentHeavySitesAreJudgedByTheHook runs each heavy-work fixture site
// through the hook's whole pretool-bash action as agent-1 of session
// parent-17, then runs the same command as the main thread
// (hookruntime.ProbeBash), and holds both answers.
//
// VALIDATES: spec-main-thread-schedules-heavy-work. A subagent's `go test`, a
// heavy `./le` action and a `go test` behind a stated reason are refused with
// the hand-back, whose last line is the command the agent typed and which
// names no admitted form; a reader line naming a heavy command and a light
// `./le` action pass with empty standard output. The main thread typing the
// same command gets the verdict it had before the check existed, and never
// the hand-back. Every heavy-work site of the catalog has a row here, and the
// site expects the exit and the messages the hook answers.
// PREVENTS: a site that names a verdict no run produces; the heavy-work check
// judging the main thread; and a refusal that reaches the agent from the
// raw-job guard alone, whose exit code is the same and whose advice a
// subagent cannot follow.
// MUTATION: taking bashSubagentHeavy out of nativeHookActions["pretool-bash"]
// turns the le-action and stated-reason sites to exit 0 and removes the
// header from the go-test site, so all three refused sites go red.
func TestSubagentHeavySitesAreJudgedByTheHook(t *testing.T) {
	tree := bashGoldenProbeTree(t)
	t.Setenv("CLAUDE_PROJECT_DIR", tree)
	// The variables a pretool-bash check reads from its own environment, as
	// ProbeBash pins them, so the verdict does not depend on this shell.
	for _, name := range []string{"GOFLAGS", "CDPATH", "CLAUDE_CODE_FORK_SUBAGENT"} {
		t.Setenv(name, "")
	}
	sites := 0
	for _, site := range &fixtureSites {
		if !strings.HasPrefix(site.name, subagentHeavySitePrefix) {
			continue
		}
		sites++
		row, mapped := subagentHeavySites[site.name]
		if !mapped {
			t.Errorf("site %q runs no line: add it to subagentHeavySites", site.name)
			continue
		}
		if site.category != categorySessionID {
			t.Errorf("site %q is in category %q, want %q", site.name, site.category, categorySessionID)
		}
		typed := row.command
		if row.identity {
			typed = subagentHeavyIdentity + row.command
		}
		code, out, said := runAsSubagent(t, typed)
		if code != site.expectedExit {
			t.Errorf("site %q: the hook answers %d for %q, the site expects %d\n%s%s",
				site.name, code, typed, site.expectedExit, out, said)
		}
		if out != "" {
			t.Errorf("site %q: the hook wrote to standard output for %q: %s", site.name, typed, out)
		}
		holdSiteMessages(t, site, said)
		refused := strings.Contains(said, subagentHeavyHeader)
		if refused != (site.expectedExit == 2) {
			t.Errorf("site %q expects exit %d, and the hand-back header is present: %v\n%s",
				site.name, site.expectedExit, refused, said)
		}
		if strings.Contains(said, "./le job run") {
			t.Errorf("site %q: the refusal names the admitted form, which a subagent is refused too:\n%s", site.name, said)
		}

		mainCode, mainSaid, err := hookruntime.ProbeBash(row.command, tree)
		if err != nil {
			t.Errorf("site %q: the hook could not judge %q for the main thread: %v", site.name, row.command, err)
			continue
		}
		if mainCode != row.mainExit {
			t.Errorf("site %q: the main thread gets %d for %q, want %d\n%s",
				site.name, mainCode, row.command, row.mainExit, mainSaid)
		}
		if strings.Contains(mainSaid, subagentHeavyHeader) {
			t.Errorf("site %q: the main thread is handed its own command back for %q:\n%s", site.name, row.command, mainSaid)
		}
	}
	if sites != len(subagentHeavySites) {
		t.Errorf("the catalog holds %d heavy-work sites, and subagentHeavySites maps %d", sites, len(subagentHeavySites))
	}
}

// runAsSubagent sends one Bash command through the pretool-bash action with
// the payload of agent-1 of session parent-17, and answers the exit code and
// the two streams.
func runAsSubagent(t *testing.T, command string) (code int, out, said string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code = hookruntime.Run("pretool-bash", bytes.NewReader(body), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// holdSiteMessages holds what the hook said to every message the site
// declares. A site here declares only the two kinds a refusal text needs: a
// part it contains, and the text it ends with.
func holdSiteMessages(t *testing.T, site fixtureSite, said string) {
	t.Helper()
	for _, message := range site.messages {
		if message.match == matchContains {
			if !strings.Contains(said, message.text) {
				t.Errorf("site %q expects %q, which the hook does not print:\n%s", site.name, message.text, said)
			}
			continue
		}
		if message.match == matchSuffix {
			if !strings.HasSuffix(strings.TrimSpace(said), message.text) {
				t.Errorf("site %q expects the refusal to end with %q:\n%s", site.name, message.text, said)
			}
			continue
		}
		t.Errorf("site %q declares a %q message, which this test does not read", site.name, message.match)
	}
}
