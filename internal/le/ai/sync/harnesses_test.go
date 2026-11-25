// VALIDATES: every synced harness directory in skillTrees carries a declared
// HarnessGrants row, every absent capability names its reason, and claude's
// hook enforcement reflects whether `.claude/settings.json` wires hooks.
// PREVENTS: adding a new harness directory to ai sync without declaring which
// rules, hooks, and subagent definitions that harness actually enforces.
package aisync

import (
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestEverySyncedSkillTreeHasADeclaredHarnessRow(t *testing.T) {
	rows := DeclaredHarnesses()
	byDir := make(map[string]HarnessGrants, len(rows))
	for _, row := range rows {
		if _, dup := byDir[row.SkillsDir]; dup {
			t.Fatalf("duplicate harness declaration for %q", row.SkillsDir)
		}
		byDir[row.SkillsDir] = row
	}
	for _, tree := range skillTrees {
		if _, ok := byDir[tree]; !ok {
			t.Errorf("skillTrees contains %q with no HarnessGrants row in declaredHarnesses", tree)
		}
	}
	if len(rows) != len(skillTrees) {
		t.Errorf("declaredHarnesses has %d rows, skillTrees has %d entries", len(rows), len(skillTrees))
	}
}

func TestEveryAbsentHarnessCapabilityCarriesAReason(t *testing.T) {
	for _, row := range DeclaredHarnesses() {
		checks := []struct {
			name string
			cap  Capability
		}{
			{"skill-stubs", row.SkillStubs},
			{"agent-definitions", row.AgentDefinitions},
			{"hook-enforcement", row.HookEnforcement},
			{"include-expansion", row.IncludeExpansion},
		}
		for _, item := range checks {
			if item.cap.State == SupportUnspecified {
				t.Errorf("%s.%s is SupportUnspecified", row.Name, item.name)
			}
			if item.cap.State == SupportNo && strings.TrimSpace(item.cap.Reason) == "" {
				t.Errorf("%s.%s is SupportNo with no reason", row.Name, item.name)
			}
		}
		if row.Name == "claude" && row.AgentDefinitions.State != SupportYes {
			t.Errorf("claude must declare AgentDefinitions=yes because %s is mirrored", claudeAgents)
		}
		if row.Name != "claude" && row.AgentDefinitions.State == SupportYes {
			t.Errorf("%s declares AgentDefinitions=yes, but ai sync mirrors only %s", row.Name, claudeAgents)
		}
	}
}

func TestInspectHarnessesReadsLiveClaudeSettings(t *testing.T) {
	emptyRoot := t.TempDir()
	withoutHooks := InspectHarnesses(emptyRoot)
	var claudeEmpty HarnessGrants
	for _, row := range withoutHooks {
		if row.Name == "claude" {
			claudeEmpty = row
		}
	}
	if claudeEmpty.HookEnforcement.State != SupportNo {
		t.Fatalf("empty checkout reported claude hooks=%v, want SupportNo", claudeEmpty.HookEnforcement.State)
	}
	if !strings.Contains(claudeEmpty.HookEnforcement.Reason, claudeSettingsFile) {
		t.Errorf("missing settings reason = %q, want mention of %s", claudeEmpty.HookEnforcement.Reason, claudeSettingsFile)
	}

	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	live := InspectHarnesses(root)
	var claudeLive HarnessGrants
	for _, row := range live {
		if row.Name == "claude" {
			claudeLive = row
		}
	}
	if claudeLive.HookEnforcement.State != SupportYes {
		t.Fatalf("live checkout %s reported claude hooks=%v (%s), want SupportYes",
			filepath.Join(root, claudeSettingsFile), claudeLive.HookEnforcement.State, claudeLive.HookEnforcement.Reason)
	}
}

func TestReportTextIncludesHarnessCapabilities(t *testing.T) {
	report := Report{
		Mode:      modeCheck,
		Harnesses: DeclaredHarnesses(),
	}
	text := report.Text()
	if !strings.Contains(text, "harnesses (what each synced harness enforces):") {
		t.Fatalf("Report.Text() omitted harness section:\n%s", text)
	}
	if !strings.Contains(text, "claude (.claude/skills): skills=yes, agents=yes, hooks=yes, includes=yes") {
		t.Errorf("Report.Text() missing claude row:\n%s", text)
	}
	if !strings.Contains(text, "agents (.agents/skills): skills=yes, agents=no, hooks=no, includes=no") {
		t.Errorf("Report.Text() missing agents row:\n%s", text)
	}
}
