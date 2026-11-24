package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// routeSection is a section only the external plugin's module defines.
const routeSection = "acme-monitor {\n\tinterval 5\n}\n"

// askRecorder is a validationAsk that records what it was sent and answers
// what the test set.
type askRecorder struct {
	calls   int
	user    string
	line    string
	verdict daemonVerdict
	err     error
}

func (r *askRecorder) ask(user, line string) (daemonVerdict, error) {
	r.calls++
	r.user = user
	r.line = line
	return r.verdict, r.err
}

// daemonBody is the JSON a daemon answers for a verdict.
func daemonBody(t *testing.T, valid bool, diags ...diagnostic.Diagnostic) []byte {
	t.Helper()
	body, err := json.Marshal(diagnostic.NewValidateResult("ze.conf", valid, diags, nil))
	require.NoError(t, err)
	return body
}

// sentConfig decodes the config a recorded line carries in its content leaf:
// the word after the content keyword.
func sentConfig(t *testing.T, line string) string {
	t.Helper()
	words := strings.Fields(line)
	at := slices.Index(words, suppliedContent)
	require.GreaterOrEqual(t, at, 0, "the line holds no content leaf: %.80q", line)
	require.Greater(t, len(words), at+1, "the content leaf holds no value: %.80q", line)
	decoded, err := base64.StdEncoding.DecodeString(words[at+1])
	require.NoError(t, err)
	return string(decoded)
}

// TestValidateRouteDrawsTheDaemonsAcceptance: a config with a section only an
// external plugin defines is sent to the running ze, and its acceptance is the
// verdict. Method: a stub ask that answers a valid verdict.
//
// VALIDATES: the daemon is asked once, with `show config supplied validation`,
// the config's own bytes in the content leaf and the file's base name in the
// name leaf; the result is valid, is named for the path the operator typed,
// and holds no local refusal.
// PREVENTS: the local command answering "not available" while a running ze
// that holds the plugin's schema could judge the section, and a label with a
// directory, which the daemon refuses.
func TestValidateRouteDrawsTheDaemonsAcceptance(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := markingPlugin(t)
	text := monitorConfig(run, routeSection)
	recorder := &askRecorder{verdict: daemonVerdict{body: daemonBody(t, true), address: "127.0.0.1:2222"}}

	result := judgeConfig(text, "work/ze.conf", "", recorder.ask)

	require.Equal(t, 1, recorder.calls)
	assert.Empty(t, recorder.user)
	assert.True(t, strings.HasPrefix(recorder.line, "show config supplied validation content "), "line: %.80q", recorder.line)
	assert.True(t, strings.HasSuffix(recorder.line, " name ze.conf"), "the line does not end with the file's base name as its label")
	assert.Equal(t, text, sentConfig(t, recorder.line))
	require.True(t, result.Valid, "the daemon's acceptance was not drawn:\n%s", messagesOf(result, diagnostic.SeverityError))
	assert.Equal(t, "work/ze.conf", result.Path)
	assert.Empty(t, result.Diagnostics)
	requireNotStarted(t, marker)
}

// TestValidateRouteLeavesOutALabelTheDaemonRefuses: a config with no file
// name, or with a base name the daemon's name leaf refuses, is sent with no
// label. Method: standard input, a base name that starts with a dash and one
// that holds a control character.
//
// VALIDATES: the line is `show config supplied validation content <base64>`
// and nothing after it, the content is the config's own bytes, and the verdict
// is still named for the path the operator typed.
// PREVENTS: a label that makes the daemon refuse the command (a control
// character, the stdin marker) or that the dispatcher reads as a flag, which
// would turn a config the daemon can judge into a failed ask.
func TestValidateRouteLeavesOutALabelTheDaemonRefuses(t *testing.T) {
	for _, path := range []string{"-", "work/-ze.conf", "work/ze\x07.conf"} {
		t.Run(path, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, _ := markingPlugin(t)
			text := monitorConfig(run, routeSection)
			recorder := &askRecorder{verdict: daemonVerdict{body: daemonBody(t, true), address: "127.0.0.1:2222"}}

			result := judgeConfig(text, path, "", recorder.ask)

			require.Equal(t, 1, recorder.calls)
			words := strings.Fields(recorder.line)
			require.Len(t, words, 6, "want the command, the content keyword and its value alone: %.80q", recorder.line)
			assert.Equal(t, "show config supplied validation content", strings.Join(words[:5], " "))
			assert.Equal(t, text, sentConfig(t, recorder.line))
			require.True(t, result.Valid)
			assert.Equal(t, path, result.Path)
		})
	}
}

// TestValidateRouteDrawsTheDaemonsRefusal: the daemon's refusal of a value is
// the verdict, with its diagnostics. Method: a stub that answers an invalid
// verdict under a failed status, as the daemon does.
//
// VALIDATES: the result is invalid and holds the daemon's diagnostic, line
// included, and nothing about an unknown keyword or a missing daemon.
// PREVENTS: a refused config read as a failed ask, which would replace the
// range refusal with the local "not available" text.
func TestValidateRouteDrawsTheDaemonsRefusal(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, _ := markingPlugin(t)
	refusal := diagnostic.Diagnostic{
		Code: diagnostic.CodeConfigYANGRange, Severity: diagnostic.SeverityError,
		Message: "acme-monitor interval: 11 is outside 1..10", Line: 9,
	}
	recorder := &askRecorder{verdict: daemonVerdict{body: daemonBody(t, false, refusal), refused: true, address: "127.0.0.1:2222"}}

	result := judgeConfig(monitorConfig(run, "acme-monitor {\n\tinterval 11\n}\n"), "ze.conf", "", recorder.ask)

	require.False(t, result.Valid)
	require.Len(t, result.Diagnostics, 1)
	assert.Equal(t, refusal.Message, result.Diagnostics[0].Message)
	assert.Equal(t, 9, result.Diagnostics[0].Line)
	errs := messagesOf(result, diagnostic.SeverityError)
	assert.NotContains(t, errs, "unknown top-level keyword")
	assert.NotContains(t, errs, "none answered")
}

// TestValidateRouteFallsBackWhenNoDaemonListens verifies S-3 / AC-18 daemon-first
// fallback: when no daemon listens, `ze config validate` asks the daemon once
// and then falls back to offline query mode (ZE_PLUGIN_MODE=declare),
// validating declaring programs without starting their run loop and refusing
// non-declaring programs with the conformance command.
//
// VALIDATES: S-3 / AC-18 — daemon is asked first (1 call), offline query mode
// succeeds on a declaring program without touching the run-loop marker, and
// fails closed on a non-declaring program naming endpoint, run line, cause,
// and `ze show plugin conformance run "<run>"`.
// PREVENTS: skipping the daemon ask, starting an external run loop offline, or
// silently passing a non-declaring external program.
func TestValidateRouteFallsBackWhenNoDaemonListens(t *testing.T) {
	t.Run("declaring_program_succeeds_offline", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		run, marker := declaringPlugin(t, monitorSchemaDecl())
		recorder := &askRecorder{err: &sshclient.Unreached{
			Step: sshclient.ReachStepListener, Address: "127.0.0.1:2222", Cause: errors.New("connection refused"),
		}}

		result := judgeConfig(monitorConfig(run, routeSection), "ze.conf", "", recorder.ask)

		require.Equal(t, 1, recorder.calls)
		require.True(t, result.Valid, "declaring plugin refused offline:\n%s", messagesOf(result, diagnostic.SeverityError))
		assert.Equal(t, []string{"acme"}, result.Queried)
		requireNotStarted(t, marker)
	})

	t.Run("non_declaring_program_refused", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		run, marker := silentPlugin(t)
		recorder := &askRecorder{err: &sshclient.Unreached{
			Step: sshclient.ReachStepListener, Address: "127.0.0.1:2222", Cause: errors.New("connection refused"),
		}}

		result := judgeConfig(monitorConfig(run, routeSection), "ze.conf", "", recorder.ask)

		require.Equal(t, 1, recorder.calls)
		require.False(t, result.Valid, "a non-declaring program was accepted")
		require.NotEmpty(t, result.Diagnostics)
		assert.Contains(t, result.Diagnostics[0].Message, `bus endpoint "acme"`)
		assert.Contains(t, result.Diagnostics[0].Message, run)
		assert.Contains(t, result.Diagnostics[0].Message, "without writing a Stage 1 declaration")
		assert.Equal(t, `ze show plugin conformance run "`+run+`"`, result.Diagnostics[0].Fix)
		requireNotStarted(t, marker)
	})
}

// TestValidateRouteNamesOneFixPerReason verifies each unanswered-daemon reason
// produces its exact DaemonUnanswered explanation and fix, and that judgeConfig
// never reads a failed ask as an acceptance when falling back to S-3 offline
// query mode.
//
// VALIDATES: no credentials and a failed ask name the account form of the
// same command with the file; an address that cannot be read names ze status;
// judgeConfig falls back to S-3 offline query mode and refuses non-declaring
// programs with the conformance command without starting their run loop.
// PREVENTS: one fix for every failure, and a failed ask read as a verdict.
func TestValidateRouteNamesOneFixPerReason(t *testing.T) {
	foreign := daemonVerdict{body: []byte(`{"status":"ok"}`), address: "127.0.0.1:2222"}
	disagrees := daemonVerdict{body: daemonBody(t, true), refused: true, address: "127.0.0.1:2222"}
	cases := []struct {
		name    string
		verdict daemonVerdict
		err     error
		why     string
		fix     string
	}{
		{
			name: "address unreadable",
			err:  &sshclient.Unreached{Step: sshclient.ReachStepAddress, Cause: errors.New("store damaged")},
			why:  "the address of ze's SSH listener could not be read (store damaged)",
			fix:  "ze status",
		},
		{
			name: "no credentials",
			err: &sshclient.Unreached{
				Step: sshclient.ReachStepCredentials, Address: "127.0.0.1:2222", Cause: errors.New("no stored username"),
			},
			why: "a ze listens at 127.0.0.1:2222, but this account has no credentials to log in to it (no stored username)",
			fix: "ze config validate --user <name> ze.conf",
		},
		{
			name: "login refused",
			err:  notAnswered("127.0.0.1:2222", pathSuppliedValidation, errors.New("ssh: handshake failed")),
			why:  "the ze at 127.0.0.1:2222 did not answer show config supplied validation (ssh: handshake failed)",
			fix:  "ze config validate --user <name> ze.conf",
		},
		{
			name:    "an answer that is no validation result this ze reads",
			verdict: foreign,
			why:     "the ze at 127.0.0.1:2222 answered show config supplied validation with a validation result of",
			fix:     "ze config validate --user <name> ze.conf",
		},
		{
			name:    "a verdict that disagrees with its status",
			verdict: disagrees,
			why:     "a verdict that disagrees with the status beside it",
			fix:     "ze config validate --user <name> ze.conf",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := silentPlugin(t)
			text := monitorConfig(run, routeSection)
			recorder := &askRecorder{verdict: tc.verdict, err: tc.err}

			q := validationQuestion(text, "ze.conf")
			_, askErr := askDaemonValidation(q, "ze.conf", "", recorder.ask)
			require.Error(t, askErr)
			unanswered := q.unanswered(askErr)
			assert.Contains(t, unanswered.Why, tc.why)
			assert.Equal(t, tc.fix, unanswered.Fix)

			result := judgeConfig(text, "ze.conf", "", recorder.ask)
			require.False(t, result.Valid, "a failed ask was read as an acceptance")
			require.NotEmpty(t, result.Diagnostics)
			assert.Contains(t, result.Diagnostics[0].Message, `bus endpoint "acme"`)
			assert.Equal(t, `ze show plugin conformance run "`+run+`"`, result.Diagnostics[0].Fix)
			requireNotStarted(t, marker)
		})
	}
}

// TestValidateRouteDoesNotAskForCompiledInConfig: a config that declares no
// external plugin is judged here and no daemon is asked. Method: a stub that
// fails the test when called.
//
// VALIDATES: zero asks, and the verdict of runValidation unchanged.
// PREVENTS: a config whose every schema is compiled in depending on a daemon,
// on which binary it runs, or on whether it can be reached.
func TestValidateRouteDoesNotAskForCompiledInConfig(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	recorder := &askRecorder{err: errors.New("asked")}
	text := "environment {\n\tssh {\n\t\tenabled false\n\t}\n}\n"

	result := judgeConfig(text, "ze.conf", "", recorder.ask)
	local := runValidation(text, "ze.conf")

	assert.Zero(t, recorder.calls, "a compiled-in config was sent to a daemon")
	assert.Equal(t, local.Valid, result.Valid)
	assert.Equal(t, local.Diagnostics, result.Diagnostics)
}

// TestValidateRouteDoesNotAskFromInsideADaemon: a process that holds a
// daemon's module set never asks a daemon. Method: publish the plugin's
// module, as boot does, then judge.
//
// VALIDATES: zero asks, and the section is judged with the published module.
// PREVENTS: a daemon-served validation asking the daemon again, in a loop.
func TestValidateRouteDoesNotAskFromInsideADaemon(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, _ := markingPlugin(t)
	require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: monitorModule,
	}}))
	recorder := &askRecorder{err: errors.New("asked")}

	result := judgeConfig(monitorConfig(run, routeSection), "ze.conf", "", recorder.ask)

	assert.Zero(t, recorder.calls, "a process that holds the module set asked a daemon")
	assert.True(t, result.Valid, "the section was not judged with the published module:\n%s", messagesOf(result, diagnostic.SeverityError))
}

// TestValidateRouteDoesNotSendOversizeContent: a config over the bound one
// command carries is not sent over SSH; the notSent error names the bound and
// the config directory, and judgeConfig falls back locally to S-3 query mode.
//
// VALIDATES: zero asks; suppliedQuestion.line names 195,840 input bytes and
// the daemon config directory fix; judgeConfig validates the oversize config
// locally in declare mode without starting the run loop.
// PREVENTS: a line the SSH packet cannot carry being sent over SSH.
func TestValidateRouteDoesNotSendOversizeContent(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	run, marker := declaringPlugin(t, monitorSchemaDecl())
	text := monitorConfig(run, routeSection) + "# " + strings.Repeat("x", 200000) + "\n"
	recorder := &askRecorder{err: errors.New("asked")}

	q := validationQuestion(text, "big.conf")
	_, lineErr := q.line()
	require.Error(t, lineErr)
	unanswered := q.unanswered(lineErr)
	assert.Contains(t, unanswered.Why, "config big.conf does not travel in one command")
	assert.Contains(t, unanswered.Why, "195,840 input bytes")
	assert.Contains(t, unanswered.Why, "reads a config file itself only from its config directories")
	assert.True(t, strings.HasPrefix(unanswered.Fix, "ze config validate /"), "fix: %q", unanswered.Fix)
	assert.True(t, strings.HasSuffix(unanswered.Fix, "/big.conf"), "fix: %q", unanswered.Fix)

	result := judgeConfig(text, "big.conf", "", recorder.ask)

	assert.Zero(t, recorder.calls, "a config over the bound was sent")
	require.True(t, result.Valid, "oversize config with declaring plugin refused offline:\n%s", messagesOf(result, diagnostic.SeverityError))
	assert.Equal(t, []string{"acme"}, result.Queried)
	requireNotStarted(t, marker)
}

// TestValidateRouteSendsOversizeConfigByPath: a config over the bound that
// lies in a directory the daemon reads from is named to `show config
// validation` by path. Method: the same padded config, in the XDG config
// directory the daemon route admits.
//
// VALIDATES: one ask, whose line is `show config validation file <path>` with
// the path the daemon resolves, and carries no content.
// PREVENTS: a large config in the daemon's own directory refused though the
// daemon can read it, and its bytes sent in a line one SSH packet cannot hold.
func TestValidateRouteSendsOversizeConfigByPath(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	run, _ := markingPlugin(t)
	text := monitorConfig(run, routeSection) + "# " + strings.Repeat("x", 200000) + "\n"
	recorder := &askRecorder{verdict: daemonVerdict{body: daemonBody(t, true), address: "127.0.0.1:2222"}}
	// The daemon route admits a regular file that exists inside a config
	// directory that exists (command.CallerPath), so the file is written.
	dir := xdgConfigDir()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "big.conf")
	require.NoError(t, os.WriteFile(path, []byte(text), 0o600))

	result := judgeConfig(text, path, "", recorder.ask)

	require.Equal(t, 1, recorder.calls, "errors:\n%s", messagesOf(result, diagnostic.SeverityError))
	assert.True(t, strings.HasPrefix(recorder.line, "show config validation file "), "line: %.120q", recorder.line)
	assert.True(t, strings.HasSuffix(recorder.line, "big.conf"), "line: %.120q", recorder.line)
	assert.NotContains(t, recorder.line, " "+suppliedContent+" ")
	assert.True(t, result.Valid)
}
