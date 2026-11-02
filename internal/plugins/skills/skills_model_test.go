// Design: docs/features/ai-first.md — show skill full flag, judged by the model

package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/plugins/skills/yang" // Register ze-skills-cmd, the model the routes judge against.
)

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
