// Design: docs/features/ai-first.md — skills command tests

package skills

import (
	"encoding/json"
	"io"
	"os"
	"strings"
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

// TestSkillsDiagnosticsIncludesLiveTriagePlaybook proves the bundled
// ze-diagnostics, ze, and ze full skills teach live runtime bug triage via
// show system triage while keeping existing config-validation and setup text.
//
// VALIDATES: AC-6 -- show skill ze-diagnostics, show skill ze, and show skill
// ze full contain show system triage and live runtime triage guidance while
// preserving show config validation, Diagnostic Shape, and ze create agent skill.
// PREVENTS: bundled agent skills omitting live runtime triage entry points or
// dropping existing config validation and agent setup instructions.
func TestSkillsDiagnosticsIncludesLiveTriagePlaybook(t *testing.T) {
	const (
		cmdShowSystemTriage     = "show system triage"
		cmdShowConfigValidation = "show config validation"
		cmdCreateAgentSkill     = "ze create agent skill"
	)

	cases := []struct {
		name     string
		tokens   []string
		required []string
	}{
		{
			name:   "ze-diagnostics",
			tokens: []string{"ze-diagnostics"},
			required: []string{
				cmdShowSystemTriage,
				"## Live Runtime Triage",
				"doctor-internal-defect",
				"show crashes name",
				"show errors",
				"show warnings",
				"show log recent",
				"show event recent",
				"show event delivery",
				"show system goroutines",
				"show system profile",
				"show doctor",
				"show reload-status",
				"show plugin list",
				"Bug Report Essentials",
				cmdShowConfigValidation,
				"show diagnostic",
				"ze config fix",
				"Diagnostic Shape",
			},
		},
		{
			name:   "ze",
			tokens: []string{"ze"},
			required: []string{
				"## Agent Entry Points",
				cmdShowSystemTriage,
				"ze show skill ze-diagnostics",
				cmdShowConfigValidation,
				"show diagnostic",
				"ze config fix",
				cmdCreateAgentSkill,
			},
		},
		{
			name:   "ze full",
			tokens: []string{"ze", "full"},
			required: []string{
				"## Diagnostic Loop",
				"## Live Runtime Triage Loop",
				cmdShowSystemTriage,
				"show crashes name",
				"show log recent",
				"show event recent",
				"show system goroutines",
				cmdShowConfigValidation,
				cmdCreateAgentSkill,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validated, err := command.ValidateModelArgs("show skill", tc.tokens, nil)
			require.NoError(t, err, "model refused show skill %q", tc.tokens)

			var result command.LocalAnswer
			written := captureOutput(t, func() { result = showSkill(validated) })
			assert.Empty(t, written, "handler wrote to stdout or stderr")
			require.Equal(t, 0, result.Code, "show skill %q: %s", tc.tokens, result.Diagnostic)

			answer, ok := result.Payload.(skillAnswer)
			require.True(t, ok, "show skill %q answered %T", tc.tokens, result.Payload)
			for _, want := range tc.required {
				assert.Contains(t, answer.Skill.Content, want)
			}
			lower := strings.ToLower(answer.Skill.Content)
			assert.Contains(t, lower, "triage")
			assert.Contains(t, lower, "runtime")
		})
	}
}

// TestSkillsDiagnosticsIncludesPart2Commands verifies the bundled
// ze-diagnostics skill teaches profile summary pipes, mutex and block
// contention profiling, flight-recorder trace capture commands, the dedicated
// fault log ring, and enriched event history fields.
//
// VALIDATES: AC-7 -- show skill ze-diagnostics documents | summary, mutex,
// block, show system trace, and request system trace snapshot.
// PREVENTS: bundled runtime triage skill omitting Part 2 diagnostic commands.
func TestSkillsDiagnosticsIncludesPart2Commands(t *testing.T) {
	validated, err := command.ValidateModelArgs("show skill", []string{"ze-diagnostics"}, nil)
	require.NoError(t, err)

	var result command.LocalAnswer
	written := captureOutput(t, func() { result = showSkill(validated) })
	assert.Empty(t, written, "handler wrote to stdout or stderr")
	require.Equal(t, 0, result.Code, "diagnostic: %s", result.Diagnostic)

	answer, ok := result.Payload.(skillAnswer)
	require.True(t, ok, "show skill answered %T", result.Payload)
	for _, want := range []string{
		"| summary",
		"show system profile type heap | summary",
		"type cpu duration 10s | summary",
		"type mutex | summary",
		"type block | summary",
		`pipe: [{"name": "summary"}]`,
		"pprof-base64",
		"show system trace",
		"show system trace id <id>",
		"request system trace snapshot",
		"fault ring",
		"`source`",
		"`subject`",
		"`direction`",
	} {
		assert.Contains(t, answer.Skill.Content, want)
	}
}
