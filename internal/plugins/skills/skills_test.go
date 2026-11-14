// Design: docs/features/ai-first.md — skills command tests

package skills

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// captureOutput runs fn with os.Stdout and os.Stderr redirected to one pipe
// and answers everything written to either.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

// runQuiet runs a handler and fails the test when it writes anything: the
// answer, and any refusal, is a value the route prints.
func runQuiet(t *testing.T, handler command.LocalDataHandler, args ...string) command.LocalAnswer {
	t.Helper()
	var result command.LocalAnswer
	written := captureOutput(t, func() { result = handler(commandtest.Args(args...)) })
	assert.Empty(t, written, "handler wrote to stdout or stderr")
	return result
}

// local answers what the plain local route prints for a successful answer:
// the handler's payload through the registered renderer.
func local(t *testing.T, handler command.LocalDataHandler, path string, args ...string) string {
	t.Helper()
	result := runQuiet(t, handler, args...)
	require.Equal(t, 0, result.Code, "diagnostic: %s", result.Diagnostic)
	require.NotNil(t, result.Payload)
	var code int
	out := captureOutput(t, func() { code = render(path, result.Payload) })
	assert.Equal(t, 0, code)
	return out
}

// TestSkillsListAll checks the list prints one `name — description` line per
// bundled skill, exactly as before the answer became data.
func TestSkillsListAll(t *testing.T) {
	out := local(t, showSkills, "show skills")
	want := ""
	for _, s := range inventory {
		want += s.Name + " — " + s.Description + "\n"
	}
	assert.Equal(t, want, out)
	assert.Contains(t, out, "ze — ")
	assert.Contains(t, out, "ze-diagnostics")
	assert.Contains(t, out, "ze-config")
	assert.Contains(t, out, "ze-commands")
	assert.Contains(t, out, "ze-agent")
}

func TestSkillsListJSON(t *testing.T) {
	out := local(t, showSkills, "show skills", "--json")
	var entries []diagnostic.SkillEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	assert.Len(t, entries, len(inventory))
	assert.Equal(t, "ze", entries[0].Name)
}

// TestSkillsGetCompact checks one skill prints as its bundled Markdown, byte
// for byte.
func TestSkillsGetCompact(t *testing.T) {
	out := local(t, showSkill, "show skill", "ze")
	content, err := skillFS.ReadFile("data/ze.md")
	require.NoError(t, err)
	assert.Equal(t, string(content), out)
	assert.Contains(t, out, "name: ze")
	assert.Contains(t, out, "Agent Entry Points")
}

func TestSkillsGetFull(t *testing.T) {
	compact := local(t, showSkill, "show skill", "ze")
	full := local(t, showSkill, "show skill", "ze", "--full")
	assert.Contains(t, full, "Diagnostic Loop")
	assert.True(t, len(full) > len(compact), "full should be longer than compact")
}

// TestTheZeGuideNamesTheSetupCommand proves the `ze` guide tells its reader how
// to set an agent up.
//
// Goal: an agent or an operator who reads `show skill ze`, compact or
// expanded, learns that `ze create agent skill` sets an agent up, so the guide
// and the setup commands do not live apart. Method: each spelling goes through
// command.ValidateModelArgs, as a route does, then through the real showSkill
// handler, and the guide content it answers must name the command.
func TestTheZeGuideNamesTheSetupCommand(t *testing.T) {
	for _, tokens := range [][]string{{"ze"}, {"ze", "full"}} {
		validated, err := command.ValidateModelArgs("show skill", tokens, nil)
		require.NoError(t, err, "the model refused show skill %q", tokens)
		var result command.LocalAnswer
		written := captureOutput(t, func() { result = showSkill(validated) })
		assert.Empty(t, written, "handler wrote to stdout or stderr for %q", tokens)
		require.Equal(t, 0, result.Code, "show skill %q: %s", tokens, result.Diagnostic)
		answer, ok := result.Payload.(skillAnswer)
		require.True(t, ok, "show skill %q answered %T", tokens, result.Payload)
		assert.Contains(t, answer.Skill.Content, "ze create agent skill", "show skill %q", tokens)
	}
}

func TestSkillsGetInnerSkill(t *testing.T) {
	out := local(t, showSkill, "show skill", "ze-diagnostics")
	assert.Contains(t, out, "ze-diagnostics")
	assert.Contains(t, out, "Diagnostic Shape")
}

// TestSkillsGetJSON checks `--json` stays a local rendering flag: the local
// route prints the indented document, and the data carries no trace of it.
func TestSkillsGetJSON(t *testing.T) {
	out := local(t, showSkill, "show skill", "ze", "--json")
	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "ze", result.Name)
	assert.NotEmpty(t, result.Content)

	encoded, err := json.Marshal(runQuiet(t, showSkill, "ze", "--json").Payload)
	require.NoError(t, err)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	assert.Len(t, envelope, 1)
	assert.Contains(t, envelope, keySkill)
}

// TestSkillsGetUnknown checks an unknown name is refused as a value naming
// the skills that exist.
func TestSkillsGetUnknown(t *testing.T) {
	result := runQuiet(t, showSkill, "nonexistent")
	assert.Equal(t, 1, result.Code)
	assert.Nil(t, result.Payload)
	assert.Equal(t, "error: unknown skill: nonexistent\navailable: "+skillNames()+"\n", result.Diagnostic)
}

// TestSkillsNoArgs proves the skills package refuses an invocation that names
// nothing to fetch: `show skill` with no name exits 1 rather than printing an
// empty skill, and the refusal is returned, not written.
func TestSkillsNoArgs(t *testing.T) {
	result := runQuiet(t, showSkill)
	assert.Equal(t, 1, result.Code)
	assert.Nil(t, result.Payload)
	assert.Equal(t, "error: missing skill name\navailable: "+skillNames()+"\n", result.Diagnostic)
}

// TestSkillsHelpIsReturned checks --help answers the usage page as a value
// with exit code 0.
func TestSkillsHelpIsReturned(t *testing.T) {
	for _, handler := range []command.LocalDataHandler{showSkills, showSkill} {
		result := runQuiet(t, handler, "--help")
		assert.Equal(t, 0, result.Code)
		assert.Nil(t, result.Payload)
		assert.Contains(t, result.Diagnostic, "ze show skill <name> [--full] [--json]")
	}
}
