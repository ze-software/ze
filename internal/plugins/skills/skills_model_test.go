// Design: docs/features/ai-first.md — show skill full flag, judged by the model

package skills

import (
	"errors"
	"slices"
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
// rule of AC-26.
//
// Goal: the agent enumeration of create and show is exactly the harness
// table, and delete's is the table plus all, mandatory, so the model and the
// table cannot drift and a bare delete cannot act on every harness. Method:
// read each path's agent leaf from the loaded model and compare its values
// and its mandatory flag with the table.
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
		{pathDeleteAgentSkill, append(slices.Clone(names), agentAll), true},
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
		_, err = command.ValidateModelArgs(path, []string{"agent", "claude", "agent", "codex"}, nil)
		assert.Error(t, err, "%s accepted agent twice", path)
	}
}

// TestBareDeleteAgentSkillIsRefusedNamingTheValues proves AC-19's refusal
// and AC-26's selector rule.
//
// Goal: `delete agent skill` with no agent is refused by the validator, and
// the refusal's usage names the harnesses and all. Method: the model's
// validator with no token, then the missing-argument fault's usage.
func TestBareDeleteAgentSkillIsRefusedNamingTheValues(t *testing.T) {
	_, err := command.ValidateModelArgs(pathDeleteAgentSkill, nil, nil)
	require.Error(t, err, "a bare delete agent skill was accepted")
	var missing *command.MissingArgumentError
	require.True(t, errors.As(err, &missing), "the refusal is not a missing argument: %v", err)
	assert.Equal(t, leafAgent, missing.Leaf)
	for _, value := range append([]string{agentAll}, harnesses[0].name) {
		assert.Contains(t, missing.Usage(), value, "the refusal's usage names no %s", value)
	}
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
