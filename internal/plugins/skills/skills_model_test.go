// Design: docs/features/ai-first.md — show skill full flag, judged by the model

package skills

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/plugins/skills/yang" // Register ze-skills-cmd, the model the routes judge against.
)

// agentDefOf answers the agent leaf the model declares for path.
func agentDefOf(t *testing.T, path string) command.ArgDef {
	t.Helper()
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err, "load the embedded YANG modules")
	defs := configyang.PathToArgDefs(loader)[path]
	for i := range defs {
		if defs[i].Name() == leafAgent {
			return defs[i]
		}
	}
	require.Failf(t, "no agent leaf", "the model declares no agent leaf for %s", path)
	return command.ArgDef{}
}

// TestTheModelsAgentValuesAreTheHarnessTable proves R-11 and the selector
// rule of AC-26, as the owner changed it: agent is optional on all three.
//
// Goal: the agent enumeration of create and show is exactly the harness
// table, and delete's is the table plus all, and no path makes agent
// mandatory, so the model and the table cannot drift and a bare delete
// reaches the handler. Method: read each path's agent leaf from the loaded
// model and compare its values and its mandatory flag with the table.
func TestTheModelsAgentValuesAreTheHarnessTable(t *testing.T) {
	names := make([]string, len(harnesses))
	for i := range harnesses {
		names[i] = harnesses[i].name
	}
	cases := []struct {
		path      string
		values    []string
		mandatory bool
	}{
		{pathCreateAgentSkill, names, false},
		{pathShowAgentSkill, names, false},
		{pathDeleteAgentSkill, append(slices.Clone(names), agentAll), false},
	}
	for _, tc := range cases {
		def := agentDefOf(t, tc.path)
		assert.ElementsMatch(t, tc.values, slices.Collect(def.EnumValues()), tc.path)
		assert.Equal(t, tc.mandatory, def.Mandatory(), "%s: agent mandatory", tc.path)
	}
}

// TestTheValidatorRefusesAnUnknownOrRepeatedAgent proves AC-26.
//
// Goal: an agent value the plugin does not know is refused before any
// handler runs, naming the accepted values, and agent given twice is refused.
// Method: the model's validator, as every route runs it, on each skill path.
func TestTheValidatorRefusesAnUnknownOrRepeatedAgent(t *testing.T) {
	for _, path := range []string{pathCreateAgentSkill, pathShowAgentSkill, pathDeleteAgentSkill} {
		_, err := command.ValidateModelArgs(path, []string{"agent", "cursor"}, nil)
		require.Error(t, err, "%s accepted an unknown agent", path)
		for i := range harnesses {
			assert.Contains(t, err.Error(), harnesses[i].name, "%s: the refusal names no %s", path, harnesses[i].name)
		}
		if path == pathDeleteAgentSkill {
			assert.Contains(t, err.Error(), agentAll, "%s: the refusal does not name all", path)
		}
		_, err = command.ValidateModelArgs(path, []string{"agent", "claude", "agent", "codex"}, nil)
		assert.Error(t, err, "%s accepted agent twice", path)
	}
}

// TestBareDeleteAgentSkillActsAsAgentAll proves the owner's default for
// delete: with no agent, `delete agent skill` acts on every harness found,
// exactly as `delete agent skill agent all` does.
//
// Goal: a bare delete passes the model's validator, and it leaves a home
// exactly as agent all leaves an identical one, stub and hook alike, for a
// harness with its own stub directory and a hook (Claude Code), one that
// reads the shared stub and has a hook (Codex), and one with no hook (Pi).
// Method: two homes prepared the same way by create, beside an operator's own
// skill; a bare delete in one and agent all in the other; then the rows and
// every entry left compared with the home replaced by ~. The bare delete's
// own rows and files are checked removed too, so the comparison cannot pass
// on two identical refusals.
func TestBareDeleteAgentSkillActsAsAgentAll(t *testing.T) {
	_, err := command.ValidateModelArgs(pathDeleteAgentSkill, nil, nil)
	require.NoError(t, err, "the model refused a bare delete agent skill")

	bare, bareFiles := deleteInPreparedHome(t)
	all, allFiles := deleteInPreparedHome(t, "agent", agentAll)

	assert.Equal(t, all, bare, "a bare delete answered unlike agent all")
	assert.Equal(t, allFiles, bareFiles, "a bare delete left the home unlike agent all")
	for _, agent := range []string{"claude", "codex", "pi"} {
		assert.Equal(t, stateRemoved, rowOf(t, bare, agent, itemSkill).State, "%s skill", agent)
	}
	for _, agent := range []string{"claude", "codex"} {
		assert.Equal(t, stateRemoved, rowOf(t, bare, agent, itemHook).State, "%s hook", agent)
	}
	assert.NotContains(t, bareFiles, filepath.Join("~", ".claude", "skills", "ze", "SKILL.md"))
	assert.NotContains(t, bareFiles, filepath.Join("~", ".agents", "skills", "ze", "SKILL.md"))
	assert.Contains(t, bareFiles, filepath.Join("~", ".claude", "skills", "other", "SKILL.md"))
}

// deleteInPreparedHome prepares a fresh home with Claude Code, Codex and Pi
// found, an operator's own skill beside Ze's, and Ze's stubs and hooks written
// by create; runs delete agent skill with tokens through the model, as the
// shell route does; and answers its rows and every entry left under the home,
// each with the home replaced by ~ so two homes compare.
func deleteInPreparedHome(t *testing.T, tokens ...string) (agentSkillAnswer, map[string]string) {
	t.Helper()
	home := isolatedHome(t, ".claude", ".codex", ".pi")
	writeFile(t, filepath.Join(home, ".claude", "skills", "other", "SKILL.md"), "another skill\n")
	runSetup(t, verbCreate, pathCreateAgentSkill)
	require.FileExists(t, claudeStub(home))
	require.FileExists(t, sharedStub(home))
	require.FileExists(t, claudeSettings(home))
	require.FileExists(t, codexHooks(home))

	answer := runSetup(t, verbDelete, pathDeleteAgentSkill, tokens...)

	answer.Setup.Note = strings.ReplaceAll(answer.Setup.Note, home, "~")
	for i := range answer.Setup.Rows {
		answer.Setup.Rows[i].Path = strings.ReplaceAll(answer.Setup.Rows[i].Path, home, "~")
		answer.Setup.Rows[i].Note = strings.ReplaceAll(answer.Setup.Rows[i].Note, home, "~")
	}
	files := make(map[string]string)
	for path, entry := range snapshotOf(t, home) {
		files[strings.Replace(path, home, "~", 1)] = entry
	}
	return answer, files
}

// TestCreateAndShowRefuseAgentAll proves the second half of AC-26's selector
// rule: only delete takes all.
//
// Goal: `agent all` is refused on create and show, whose no-agent forms are
// their every-harness forms. Method: the model's validator on both paths.
func TestCreateAndShowRefuseAgentAll(t *testing.T) {
	for _, path := range []string{pathCreateAgentSkill, pathShowAgentSkill} {
		_, err := command.ValidateModelArgs(path, []string{"agent", agentAll}, nil)
		assert.Error(t, err, "%s accepted agent all", path)
	}
}

// TestShowSkillFullSpellingsThroughTheModel proves the full flag leaf.
//
// Goal: every spelling of `show skill` the local grammar accepted still passes
// the validator every route runs, and the spellings the model's leaves add (the
// flag `full`, the keyed `name`) answer the same document as the local ones.
// Method: each token list goes through command.ValidateModelArgs, exactly as a
// route does before it calls the handler, and the handler's answer is compared
// with the compact and the expanded document of the `ze` skill.
func TestShowSkillFullSpellingsThroughTheModel(t *testing.T) {
	compact, err := skillFS.ReadFile("data/ze.md")
	require.NoError(t, err)
	expanded, err := skillFS.ReadFile("data/ze-full.md")
	require.NoError(t, err)
	require.NotEqual(t, string(compact), string(expanded))

	cases := []struct {
		tokens []string
		want   []byte
	}{
		{[]string{"ze"}, compact},
		{[]string{"name", "ze"}, compact},
		{[]string{"ze", "--full"}, expanded},
		{[]string{"--full", "ze"}, expanded},
		{[]string{"ze", "full"}, expanded},
		{[]string{"full", "ze"}, expanded},
		{[]string{"name", "ze", "full"}, expanded},
		{[]string{"full", "name", "ze"}, expanded},
	}
	for _, tc := range cases {
		validated, err := command.ValidateModelArgs("show skill", tc.tokens, nil)
		require.NoError(t, err, "the model refused show skill %q", tc.tokens)
		var result command.LocalAnswer
		written := captureOutput(t, func() { result = showSkill(validated) })
		assert.Empty(t, written, "handler wrote to stdout or stderr for %q", tc.tokens)
		require.Equal(t, 0, result.Code, "show skill %q: %s", tc.tokens, result.Diagnostic)
		answer, ok := result.Payload.(skillAnswer)
		require.True(t, ok, "show skill %q answered %T", tc.tokens, result.Payload)
		assert.Equal(t, "ze", answer.Skill.Name, "show skill %q", tc.tokens)
		assert.Equal(t, string(tc.want), answer.Skill.Content, "show skill %q", tc.tokens)
	}
}

// TestShowSkillFullFlagTakesNoValue proves the model judges `full` as a flag:
// a second skill name after it is not swallowed as the flag's value, and a
// missing name is still refused.
func TestShowSkillFullFlagTakesNoValue(t *testing.T) {
	_, err := command.ValidateModelArgs("show skill", []string{"full"}, nil)
	require.Error(t, err, "full alone leaves the mandatory name unfilled")
	_, err = command.ValidateModelArgs("show skill", []string{"full", "full"}, nil)
	require.Error(t, err, "a flag is typed once")
}
