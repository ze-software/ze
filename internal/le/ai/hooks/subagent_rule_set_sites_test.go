// Design: docs/contributing/rule-authoring.md -- the agent rule set an omitting agent starts with
// Related: fixture_catalog.go -- the two rule-set sites and their expected exit and text

package aihooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
	"github.com/ze-software/ze/internal/le/hookruntime"
)

// subagentRuleSetSitePrefix starts the name of every fixture site that runs a
// subagent start payload over a mirrored agent definition.
const subagentRuleSetSitePrefix = "subagent-context-rule-set-"

// subagentRuleSetSites maps each rule-set site to the frontmatter of the
// definition its agent type names: one omits the instruction files, one loads
// them.
var subagentRuleSetSites = map[string]string{
	subagentRuleSetSitePrefix + "omitting-definition-gets-the-rule-set": "name: ze-probe\nomitClaudeMd: true\n",
	subagentRuleSetSitePrefix + "loading-definition-gets-no-rule-set":   "name: ze-probe\n",
}

// TestSubagentRuleSetSitesMatchTheHook runs each rule-set site's start payload
// through the hook's subagent-context action, over a checkout holding the
// definition the site names and the two rule files the rule set reads, and
// holds the exit and the start text to the site.
//
// VALIDATES: the catalog's two rule-set sites state what the hook answers: an
// omitting definition gets the rule set, a loading one gets none. Every
// rule-set site of the catalog has a row here, and both rows are present.
// PREVENTS: a site that names a verdict no run produces, and a start hook
// that stops telling the two definitions apart with every site still green.
func TestSubagentRuleSetSitesMatchTheHook(t *testing.T) {
	sites := 0
	for _, site := range &fixtureSites {
		if !strings.HasPrefix(site.name, subagentRuleSetSitePrefix) {
			continue
		}
		sites++
		frontmatter, mapped := subagentRuleSetSites[site.name]
		if !mapped {
			t.Errorf("site %q runs no definition: add it to subagentRuleSetSites", site.name)
			continue
		}
		if site.category != categorySubagentContext {
			t.Errorf("site %q is in category %q, want %q", site.name, site.category, categorySubagentContext)
		}
		root := t.TempDir()
		files := map[string]string{
			filepath.Join(ruleset.AgentDefinitionsDir, "ze-probe.md"): "---\n" + frontmatter + "---\n\nBody.\n",
			ruleset.InstructionsFile:                                  "# Ze\n\n# " + ruleset.HardBansHeading + "\n\n- Never run the probe's banned verb.\n",
			ruleset.DecisionsFile:                                     "# Decisions\n\n## " + ruleset.DecisionsAgentPartHeading + "\n- A.\n\n## " + ruleset.DecisionsMainPartHeading + "\n- B.\n",
			ruleset.SituationalDecisionsFile:                          "# " + ruleset.DecisionsSituationalPartHeading + "\n\n## Lab work\n" + ruleset.SituationPrefix + "running a lab.\n- C.\n",
		}
		for name, body := range files {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		code, output, found := hookruntime.ProbeLifecycle("subagent-context",
			hookruntime.Payload{SessionID: "parent-17", AgentID: "agent-1", AgentType: "ze-probe"}, root)
		if !found || code != site.expectedExit {
			t.Errorf("site %q: subagent-context = %d, found %v, want %d: %s", site.name, code, found, site.expectedExit, output)
			continue
		}
		var response struct {
			Output struct {
				Context string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(output), &response); err != nil {
			t.Errorf("site %q: the hook answered no JSON: %v: %s", site.name, err, output)
			continue
		}
		if len(site.messages) == 0 {
			t.Errorf("site %q names no message to compare", site.name)
		}
		for _, needle := range site.messages {
			holds := strings.Contains(response.Output.Context, needle.text)
			switch needle.match {
			case matchContains:
				if !holds {
					t.Errorf("site %q expects %q, which the start text lacks:\n%s", site.name, needle.text, response.Output.Context)
				}
			case matchNotContains:
				if holds {
					t.Errorf("site %q expects no %q, which the start text holds:\n%s", site.name, needle.text, response.Output.Context)
				}
			default:
				t.Errorf("site %q message %q is %s, which this test does not judge", site.name, needle.text, needle.match)
			}
		}
	}
	if sites != len(subagentRuleSetSites) {
		t.Errorf("the catalog holds %d rule-set sites, want %d", sites, len(subagentRuleSetSites))
	}
}
