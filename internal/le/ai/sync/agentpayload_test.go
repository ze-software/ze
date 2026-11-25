package aisync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	airules "github.com/ze-software/ze/internal/le/ai/rules"
	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
)

// fixtureInstructions is an instructions file that holds the hard bans
// section, so the agent rule set read from the fixture is complete.
const fixtureInstructions = "# Instructions\n\nWhat every session loads.\n\n# Hard Bans\n\n- never push on your own\n\n# Other\n\nMore for the main session.\n"

// writeRuleFiles writes the decisions, situational companion, trigger index and core.
func writeRuleFiles(t *testing.T, root string) {
	t.Helper()
	write(t, root, ruleset.DecisionsFile, fixtureDecisions)
	write(t, root, ruleset.SituationalDecisionsFile, fixtureSituationalDecisions)
	write(t, root, ruleset.TriggersFile, fixtureTriggers)
	write(t, root, ruleset.CoreFile, fixtureCore)
}

// payloadFixture is fixture with the hard bans in its instructions and a
// second agent, ze-read, whose definition omits the instruction files beside
// ze-work, which loads them; extra adds definitions by name. It runs a sync,
// so the mirror the predicate reads holds every definition.
func payloadFixture(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := fixture(t)
	write(t, root, instructions, fixtureInstructions)
	write(t, root, "ai/agents/ze-read.md", "---\nname: ze-read\n"+ruleset.OmitInstructionsKey+": true\n---\nagent\n")
	for name, body := range extra {
		write(t, root, "ai/agents/"+name+".md", body)
	}
	syncTree(t, root)
	return root
}

// measuredPayload answers the one payload measure over root, and fails the
// test when either set is incomplete, since a row's figure is then no answer.
func measuredPayload(t *testing.T, root string) airules.PayloadReport {
	t.Helper()
	measured, err := airules.Payload(root)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if len(measured.Missing) > 0 || len(measured.Agent.Missing) > 0 {
		t.Fatalf("the fixture's payload is incomplete: %v %v", measured.Missing, measured.Agent.Missing)
	}
	if measured.Tokens == measured.Agent.Tokens {
		t.Fatalf("both sets measure %d tokens, so a row naming the wrong set would pass", measured.Tokens)
	}
	return measured
}

// rowOf answers the row named name, failing the test when there is none.
func rowOf(t *testing.T, report Report, name string) AgentPayload {
	t.Helper()
	for _, row := range report.AgentPayloads {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("no row for %q in %+v", name, report.AgentPayloads)
	return AgentPayload{}
}

// assertFigure fails the test unless row names payload with exactly tokens.
func assertFigure(t *testing.T, row AgentPayload, payload string, tokens int) {
	t.Helper()
	if row.Payload != payload {
		t.Errorf("%s: payload %q, want %q", row.Name, row.Payload, payload)
	}
	if row.Tokens == nil {
		t.Fatalf("%s: no token figure, want %d", row.Name, tokens)
	}
	if *row.Tokens != tokens {
		t.Errorf("%s: %d tokens, want the measure's %d", row.Name, *row.Tokens, tokens)
	}
}

// TestTheCheckNamesThePayloadEachDefinitionStartsWith drives the check over
// one definition that omits the instruction files and one that loads them.
//
// VALIDATES: the check answers one row per definition in source order; the
// omitting one names the agent rule set with the figure airules.Payload
// measures for it, the loading one names the always-loaded payload with that
// figure, the two figures differ, and the text prints both rows under the
// verdict.
// PREVENTS: a definition that forgets omitClaudeMd starting at the old size
// with nothing to show it, and a figure recomputed apart from the payload
// report.
func TestTheCheckNamesThePayloadEachDefinitionStartsWith(t *testing.T) {
	root := payloadFixture(t, nil)
	measured := measuredPayload(t, root)

	report, err := Mirror{Root: root}.checkWithPayloads()
	if err != nil {
		t.Fatalf("checkWithPayloads: %v", err)
	}
	if !report.Fresh() {
		t.Fatalf("a freshly synced tree is stale: %v", report.Stale)
	}
	if len(report.AgentPayloads) != 2 {
		t.Fatalf("%d rows, want 2: %+v", len(report.AgentPayloads), report.AgentPayloads)
	}
	if report.AgentPayloads[0].Name != "ze-read" || report.AgentPayloads[1].Name != "ze-work" {
		t.Errorf("rows %+v, want ze-read then ze-work", report.AgentPayloads)
	}
	omitting := rowOf(t, report, "ze-read")
	if omitting.Start != startOmits {
		t.Errorf("ze-read: start %q, want %q", omitting.Start, startOmits)
	}
	assertFigure(t, omitting, payloadAgentRuleSet, measured.Agent.Tokens)
	loading := rowOf(t, report, "ze-work")
	if loading.Start != startLoads {
		t.Errorf("ze-work: start %q, want %q", loading.Start, startLoads)
	}
	assertFigure(t, loading, payloadAlwaysLoaded, measured.Tokens)

	text := report.Text()
	for _, want := range []string{
		"generated agent files in sync\n",
		"  ze-read: omits the instruction files, starts with the agent rule set: " + strconv.Itoa(measured.Agent.Tokens) + " tokens\n",
		"  ze-work: loads the instruction files, starts with the always-loaded payload: " + strconv.Itoa(measured.Tokens) + " tokens\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the check text lacks %q:\n%s", want, text)
		}
	}
}

// TestACheckRowNamesTheSafeSideOfAnUnreadFrontmatter drives the check over a
// definition whose frontmatter names another agent.
//
// VALIDATES: the row says the frontmatter could not be read and is read as
// omitting, and names the agent rule set with the measured figure, as the
// start hook sends that agent the rule set.
// PREVENTS: a definition the predicate cannot read reported as one that loads
// the instruction files, or with no word that its start is a guess.
func TestACheckRowNamesTheSafeSideOfAnUnreadFrontmatter(t *testing.T) {
	root := payloadFixture(t, map[string]string{"ze-odd": "---\nname: other\n" + ruleset.OmitInstructionsKey + ": false\n---\nagent\n"})
	measured := measuredPayload(t, root)

	report, err := Mirror{Root: root}.checkWithPayloads()
	if err != nil {
		t.Fatalf("checkWithPayloads: %v", err)
	}
	row := rowOf(t, report, "ze-odd")
	if row.Start != startUnread {
		t.Errorf("ze-odd: start %q, want %q", row.Start, startUnread)
	}
	assertFigure(t, row, payloadAgentRuleSet, measured.Agent.Tokens)
	if text := report.Text(); !strings.Contains(text, "so it is read as omitting the instruction files, starts with the agent rule set: ") {
		t.Errorf("the check text does not state the safe side:\n%s", text)
	}
}

// TestACheckRowForAnUnmirroredSourceCarriesNoFigure adds a definition after
// the sync.
//
// VALIDATES: the check is stale, and the new definition's row says it is not
// in the mirror, names the sync, and carries no payload and no figure.
// PREVENTS: a figure printed for an agent no harness can start.
func TestACheckRowForAnUnmirroredSourceCarriesNoFigure(t *testing.T) {
	root := payloadFixture(t, nil)
	write(t, root, "ai/agents/ze-new.md", "---\nname: ze-new\n---\nagent\n")

	report, err := Mirror{Root: root}.checkWithPayloads()
	if err != nil {
		t.Fatalf("checkWithPayloads: %v", err)
	}
	if report.Fresh() {
		t.Errorf("a source with no mirror reads as fresh")
	}
	row := rowOf(t, report, "ze-new")
	if row.Start != startNotMirrored {
		t.Errorf("ze-new: start %q, want %q", row.Start, startNotMirrored)
	}
	if row.Payload != "" || row.Tokens != nil {
		t.Errorf("ze-new: payload %q and tokens %v, want neither", row.Payload, row.Tokens)
	}
	if text := report.Text(); !strings.Contains(text, "  ze-new: not in "+claudeAgents+", so no agent of this type starts until: ./le ai sync update\n") {
		t.Errorf("the check text does not name the missing mirror:\n%s", text)
	}
}

// TestACheckFailsRatherThanPrintAFigureWhenARulePartIsMissing removes a part
// each row's payload needs.
//
// VALIDATES: with the core gone, the check answers ErrPayloadUnmeasured and no
// rows, and the check verb exits 1 with no report; with the hard bans heading
// gone, an omitting definition fails the same way.
// PREVENTS: a payload with an absent part printed as a smaller figure.
func TestACheckFailsRatherThanPrintAFigureWhenARulePartIsMissing(t *testing.T) {
	t.Run("core", func(t *testing.T) {
		root := payloadFixture(t, nil)
		if err := os.Remove(filepath.Join(root, ruleset.CoreFile)); err != nil {
			t.Fatal(err)
		}
		report, err := Mirror{Root: root}.checkWithPayloads()
		if !errors.Is(err, ErrPayloadUnmeasured) {
			t.Fatalf("checkWithPayloads error %v, want ErrPayloadUnmeasured", err)
		}
		if len(report.AgentPayloads) != 0 {
			t.Errorf("a failed check answered rows: %+v", report.AgentPayloads)
		}
		useCheckout(t, root)
		payload, code := Answer([]string{"check"})
		if code != 1 {
			t.Errorf("the check verb exited %d with the core missing, want 1", code)
		}
		if payload != nil {
			t.Errorf("the check verb answered %v with the core missing, want nothing", payload)
		}
	})
	t.Run("hard-bans", func(t *testing.T) {
		root := payloadFixture(t, nil)
		write(t, root, instructions, "# Instructions\n\nNo bans heading.\n")
		syncTree(t, root)
		if _, err := (Mirror{Root: root}).checkWithPayloads(); !errors.Is(err, ErrPayloadUnmeasured) {
			t.Fatalf("checkWithPayloads error %v, want ErrPayloadUnmeasured for ze-read's agent rule set", err)
		}
	})
}

// TestTheCheckRowsAreStructuredDataWithKebabCaseKeys encodes a check that
// carries rows.
//
// VALIDATES: the report carries the rows under "agent-payloads", each row
// carries name, start, payload and tokens, and every key is kebab-case.
// PREVENTS: `| json` carrying the rows under a key style no other le answer
// uses, or dropping a field the text prints.
func TestTheCheckRowsAreStructuredDataWithKebabCaseKeys(t *testing.T) {
	root := payloadFixture(t, nil)
	report, err := Mirror{Root: root}.checkWithPayloads()
	if err != nil {
		t.Fatalf("checkWithPayloads: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	var got struct {
		AgentPayloads []map[string]any `json:"agent-payloads"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the report is not an object: %v", err)
	}
	if len(got.AgentPayloads) != 2 {
		t.Fatalf("%d encoded rows, want 2: %s", len(got.AgentPayloads), raw)
	}
	for _, row := range got.AgentPayloads {
		for _, key := range []string{"name", "start", "payload", "tokens"} {
			if _, ok := row[key]; !ok {
				t.Errorf("row %v has no %q key", row, key)
			}
		}
		for key := range row {
			if strings.ToLower(key) != key || strings.Contains(key, "_") {
				t.Errorf("key %q is not kebab-case", key)
			}
		}
	}
}
