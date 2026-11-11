// Related: parity_catalog.go -- the BASH_GOLDEN rows each site runs
// Related: fixture_catalog.go -- the reader-row sites and their expected exit

package aihooks

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
)

// readerRowSitePrefix starts the name of every fixture site that runs a
// loosened BASH_GOLDEN row as a subagent.
const readerRowSitePrefix = "pretool-bash-subagent-reader-row-"

// readerRowSites maps each reader-row fixture site to the BASH_GOLDEN row it
// runs: the five rows spec-agent-economy-6-shell-parser loosened, each a loop
// of readers whose quoted pattern names a guarded command.
var readerRowSites = map[string]string{
	readerRowSitePrefix + "loop-quoted-go-vet-passes":        "for f in a b; do grep -n \"x\\|go vet ./y\" \"$f\"; done",
	readerRowSitePrefix + "while-quoted-go-test-passes":      "while read -r f; do grep -c 'x;go test ./y' \"$f\"; done < list",
	readerRowSitePrefix + "loop-quoted-piped-go-test-passes": "for f in a b; do grep -c 'x; go test ./y | tail -1' \"$f\"; done",
	readerRowSitePrefix + "loop-quoted-go-build-passes":      "for f in a b; do grep -n 'go build ./cmd/ze' \"$f\"; done",
	readerRowSitePrefix + "loop-quoted-system-tmp-passes":    "for f in a b; do grep -n 'cat /tmp/x' \"$f\"; done",
}

// TestLooserBashRowsPassAsASubagentWithoutTheIdentityLine runs each loosened
// BASH_GOLDEN row through the hook's whole pretool-bash action as agent-1 of
// session parent-17, with no identity line in front of it, and holds the exit
// to its fixture site.
//
// VALIDATES: the reader proof of the parity golden rows table: a subagent's
// reader loop with no identity line passes with empty standard output (AC-5),
// which it does only when bashAgentIdentity's exemption and every guard's lift
// read it as a reader line, the one predicate both ask (judgeReaderLine).
// Every reader-row site of the catalog has a row here, each row is a
// BASH_GOLDEN row expecting 0, and the site expects what the hook answers.
// PREVENTS: a site that names a verdict no run produces, and a looser golden
// row proven only for the main thread, where no identity check applies.
func TestLooserBashRowsPassAsASubagentWithoutTheIdentityLine(t *testing.T) {
	tree := bashGoldenProbeTree(t)
	t.Setenv("CLAUDE_PROJECT_DIR", tree)
	// The variables a pretool-bash check reads from its own environment, as
	// ProbeBash pins them, so the verdict does not depend on this shell.
	for _, name := range []string{"GOFLAGS", "CDPATH", "CLAUDE_CODE_FORK_SUBAGENT"} {
		t.Setenv(name, "")
	}
	golden := make(map[string]int, len(parityCatalog[0].fixtures))
	for _, fixture := range parityCatalog[0].fixtures {
		golden[fixture.name] = fixture.expectedCode
	}
	sites := 0
	for _, site := range &fixtureSites {
		if !strings.HasPrefix(site.name, readerRowSitePrefix) {
			continue
		}
		sites++
		command, mapped := readerRowSites[site.name]
		if !mapped {
			t.Errorf("site %q runs no row: add it to readerRowSites", site.name)
			continue
		}
		if site.category != categorySessionID {
			t.Errorf("site %q is in category %q, want %q", site.name, site.category, categorySessionID)
		}
		code, present := golden[command]
		if !present {
			t.Errorf("site %q runs %q, which is no BASH_GOLDEN row", site.name, command)
			continue
		}
		if code != 0 {
			t.Errorf("site %q runs %q, which the golden expects %d, not a loosened row", site.name, command, code)
		}
		body, err := json.Marshal(map[string]any{
			"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
			"tool_input": map[string]any{"command": command},
		})
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		got := hookruntime.Run("pretool-bash", bytes.NewReader(body), &out, &errOut)
		if got != site.expectedExit {
			t.Errorf("site %q: the hook answers %d for %q, the site expects %d\n%s%s",
				site.name, got, command, site.expectedExit, out.String(), errOut.String())
		}
		if out.Len() != 0 {
			t.Errorf("site %q: the hook wrote to standard output for %q: %s", site.name, command, out.String())
		}
	}
	if sites != len(readerRowSites) {
		t.Errorf("the catalog holds %d reader-row sites, and readerRowSites maps %d", sites, len(readerRowSites))
	}
}
