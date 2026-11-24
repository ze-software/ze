package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// daemonAddress is the listener the stub asks of this file answer from.
const daemonAddress = "127.0.0.1:2222"

// noDaemonListens is the ask of a host where no ze runs: the walk to the
// daemon ends at its listener.
func noDaemonListens(_, _ string) (daemonVerdict, error) {
	return daemonVerdict{}, &sshclient.Unreached{
		Step: sshclient.ReachStepListener, Address: daemonAddress, Cause: errors.New("connection refused"),
	}
}

// No unit test of this package opens a connection: the registered handlers of
// fmt, dump, diff and graph ask through routeAsk, and here that is a host
// where no ze runs. A test that needs an answer sets its own with useRouteAsk.
func init() { routeAsk = noDaemonListens }

// useRouteAsk makes the registered handlers ask through ask for one test.
func useRouteAsk(t *testing.T, ask daemonAsk) {
	t.Helper()
	saved := routeAsk
	routeAsk = ask
	t.Cleanup(func() { routeAsk = saved })
}

// blobBody is the JSON a daemon answers for a text.
func blobBody(t *testing.T, text string) []byte {
	t.Helper()
	body, err := json.Marshal(command.Blob(text))
	require.NoError(t, err)
	return body
}

// routedConfig writes a config that declares external plugin acme (using
// silentPlugin so declare-mode queries fail closed without touching marker) and
// holds a section only its module defines.
func routedConfig(t *testing.T) (text, path, marker string) {
	t.Helper()
	run, marker := silentPlugin(t)
	text = monitorConfig(run, routeSection)
	return text, writeTestConfig(t, text), marker
}

// inspectionRoute is one of the four commands on the operator's own route,
// asked about the config at path through ask.
type inspectionRoute struct {
	name string
	// typed is the command a failed ask names for path.
	typed func(path string) string
	// supplied is the daemon command the route sends.
	supplied string
	// accepted is a body the daemon answers for a config it accepts.
	accepted func(t *testing.T) []byte
	// answer runs the route and answers its code, whether it holds an answer,
	// and the lines it wrote for the operator.
	answer func(t *testing.T, path string, ask daemonAsk) (code int, answered bool, diagnostic string)
}

func inspectionRoutes() []inspectionRoute {
	local := func(answer command.LocalAnswer) (int, bool, string) {
		return answer.Code, answer.Payload != nil, answer.Diagnostic
	}
	return []inspectionRoute{
		{
			name:     "fmt",
			typed:    func(path string) string { return "ze config fmt " + path },
			supplied: pathSuppliedFmt,
			accepted: func(t *testing.T) []byte { return blobBody(t, "formatted\n") },
			answer: func(t *testing.T, path string, ask daemonAsk) (int, bool, string) {
				useRouteAsk(t, ask)
				return local(fmtAnswer(command.RouteLocal, []string{path}))
			},
		},
		{
			name:     "graph",
			typed:    func(path string) string { return "ze config graph " + path },
			supplied: pathSuppliedGraph,
			accepted: func(t *testing.T) []byte { return blobBody(t, "{}\n") },
			answer: func(t *testing.T, path string, ask daemonAsk) (int, bool, string) {
				useRouteAsk(t, ask)
				return local(answerOn(t, pathShowGraph, []string{path}, false))
			},
		},
		{
			name:     "dump",
			typed:    func(path string) string { return "ze config dump " + path },
			supplied: pathSuppliedDump,
			accepted: func(*testing.T) []byte { return []byte(`{"bgp":{}}`) },
			answer: func(t *testing.T, path string, ask daemonAsk) (int, bool, string) {
				useRouteAsk(t, ask)
				return local(answerOn(t, pathShowDump, []string{path}, false))
			},
		},
		{
			name:     "diff",
			typed:    func(path string) string { return "ze config diff " + path + " " + path },
			supplied: pathSuppliedDiff,
			accepted: func(*testing.T) []byte { return []byte(`{"added":{},"removed":{},"changed":{}}`) },
			answer: func(t *testing.T, path string, ask daemonAsk) (int, bool, string) {
				useRouteAsk(t, ask)
				return local(answerOn(t, pathShowDiff, []string{path, path}, false))
			},
		},
	}
}

// TestInspectRouteAsksTheRunningZe: each of the four commands, given a config
// with a section only an external plugin defines, sends the config to the
// running ze and answers what it sent back. Method: a stub ask that accepts.
//
// VALIDATES: one ask per command, to the command's own supplied form, with the
// file's bytes; the answer is the daemon's, with exit 0 and no local refusal;
// the declared program is never started.
// PREVENTS: "not available" from a local command while a running ze that holds
// the plugin's schema could answer.
func TestInspectRouteAsksTheRunningZe(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			text, path, marker := routedConfig(t)
			recorder := &askRecorder{verdict: daemonVerdict{body: route.accepted(t), address: daemonAddress}}

			code, answered, diagnostic := route.answer(t, path, recorder.ask)

			require.Equal(t, 1, recorder.calls)
			require.True(t, strings.HasPrefix(recorder.line, route.supplied+" content "), "line: %.80q", recorder.line)
			assert.Equal(t, exitOK, code, diagnostic)
			assert.True(t, answered, "the daemon's answer was not drawn: %s", diagnostic)
			assert.NotContains(t, diagnostic, "unknown top-level keyword")
			if route.name != "diff" {
				assert.Equal(t, text, sentConfig(t, recorder.line))
			}
			requireNotStarted(t, marker)
		})
	}
}

// TestFmtRouteMakesTheOperatorsChoicesFromTheDaemonsText: fmt sends the bytes
// with no check or diff leaf, and makes --check, --diff and -w here from the
// text the daemon formatted. Method: a stub ask that answers a formatted text.
//
// VALIDATES: the exact line (content, then the file's base name as its label,
// nothing after); the formatted text as the answer; exit 1 and the local
// "needs formatting" line for --check; the file rewritten by this process for
// -w, with the local "Formatted:" line.
// PREVENTS: a check verdict or a diff sent as a failed status, which reads as
// the daemon's refusal; a -w that writes nothing because the daemon's supplied
// fmt has no write option.
func TestFmtRouteMakesTheOperatorsChoicesFromTheDaemonsText(t *testing.T) {
	const formatted = "bus {\n}\n"
	ask := func(t *testing.T) *askRecorder {
		return &askRecorder{verdict: daemonVerdict{body: blobBody(t, formatted), address: daemonAddress}}
	}

	t.Run("plain", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		text, path, _ := routedConfig(t)
		recorder := ask(t)
		answer := fmtAsking(path, []byte(text), false, false, false, "ops", recorder.ask)
		words := strings.Fields(recorder.line)
		require.Len(t, words, 8, "want the command, the content and the label alone: %.80q", recorder.line)
		assert.Equal(t, "show config supplied fmt content", strings.Join(words[:5], " "))
		assert.Equal(t, "name test.conf", strings.Join(words[6:], " "))
		assert.Equal(t, "ops", recorder.user)
		assert.Equal(t, command.Blob(formatted), answer.Payload)
		assert.Equal(t, exitOK, answer.Code)
	})
	t.Run("check", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		text, path, _ := routedConfig(t)
		recorder := ask(t)
		answer := fmtAsking(path, []byte(text), true, false, false, "", recorder.ask)
		assert.NotContains(t, strings.Fields(recorder.line), "check")
		assert.Equal(t, 1, answer.Code)
		assert.Nil(t, answer.Payload)
		assert.Equal(t, path+" needs formatting\n", answer.Diagnostic)
	})
	t.Run("diff", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		text, path, _ := routedConfig(t)
		recorder := ask(t)
		answer := fmtAsking(path, []byte(text), false, true, false, "", recorder.ask)
		assert.NotContains(t, strings.Fields(recorder.line), "diff")
		assert.Equal(t, exitOK, answer.Code)
		assert.Contains(t, answer.Diagnostic, "+++ "+path+" (formatted)")
	})
	t.Run("write", func(t *testing.T) {
		t.Cleanup(config.ResetDeclaredSchemasForTest())
		text, path, _ := routedConfig(t)
		recorder := ask(t)
		answer := fmtAsking(path, []byte(text), false, false, true, "", recorder.ask)
		assert.Equal(t, exitOK, answer.Code, answer.Diagnostic)
		assert.Equal(t, "Formatted: "+path+"\n", answer.Diagnostic)
		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, formatted, string(written), "the operator's file does not hold the text the daemon formatted")
	})
}

// TestDumpRouteDrawsTheDaemonsDocument: dump sends the strip-private leaf and
// draws the daemon's document for both local spellings. Method: a stub ask
// that answers a document.
//
// VALIDATES: the line ends with the label and the strip-private leaf; the
// document is the answer of `ze show config dump`, section included; the
// plugin list `ze config dump` prints comes from the document.
// PREVENTS: a mask the operator asked for that the daemon is never told of; a
// text dump that prints no plugin because no parsed tree exists here.
func TestDumpRouteDrawsTheDaemonsDocument(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	_, path, marker := routedConfig(t)
	body := `{"acme-monitor":{"interval":"5"},"bgp":{"router-id":"192.0.2.1"},` +
		`"plugin":{"external":{"acme":{"run":"sh plugin.sh","encoder":"json"}}}}`
	recorder := &askRecorder{verdict: daemonVerdict{body: []byte(body), address: daemonAddress}}

	var diag textbuf.Buffer
	result, code := dumpAsking(path, path, true, command.RouteLocal, "", recorder.ask, &diag)

	require.Equal(t, 0, code, diag.String())
	assert.True(t, strings.HasSuffix(recorder.line, " name test.conf strip-private"), "line tail: %.60q", recorder.line[max(0, len(recorder.line)-60):])
	section, held := result.dumpMap["acme-monitor"].(map[string]any)
	require.True(t, held, "the document lost the external section: %#v", result.dumpMap)
	assert.Equal(t, "5", section["interval"])
	assert.Equal(t, "192.0.2.1", result.bgpTree["router-id"])
	assert.Equal(t, config.DisplayStrip, result.mode)
	assert.Equal(t, []dumpPlugin{{name: "acme", run: "sh plugin.sh", hasRun: true, encoder: "json", hasEncoder: true}}, result.declaredPlugins())
	requireNotStarted(t, marker)
}

// TestDiffRouteSendsBothSidesAndDrawsTheDaemonsComparison: diff sends its two
// sides as one JSON object and draws the three sets the daemon answers.
// Method: two files and a stub ask that answers one changed leaf.
//
// VALIDATES: the line is `show config supplied diff content <base64>` alone
// (the supplied diff takes no label); the content decodes to before and after
// holding each file's bytes; the changed leaf is in the diff drawn.
// PREVENTS: a comparison made here, which refuses the external section, while
// a running ze could compare the two.
func TestDiffRouteSendsBothSidesAndDrawsTheDaemonsComparison(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := markingPlugin(t)
	before := monitorConfig(run, routeSection)
	after := monitorConfig(run, "acme-monitor {\n\tinterval 7\n}\n")
	first, second := writeTestConfig(t, before), writeTestConfig(t, after)
	body := `{"added":{},"removed":{},"changed":{"acme-monitor/interval":{"old":"5","new":"7"}}}`
	recorder := &askRecorder{verdict: daemonVerdict{body: []byte(body), address: daemonAddress}}

	var diag textbuf.Buffer
	diff, code := diffAsking(nil, []string{first, second}, first+" "+second, command.RouteLocal, "", recorder.ask, &diag)

	require.Equal(t, exitOK, code, diag.String())
	words := strings.Fields(recorder.line)
	require.Len(t, words, 6, "want the command and its content alone: %.80q", recorder.line)
	assert.Equal(t, "show config supplied diff content", strings.Join(words[:5], " "))
	var sides map[string]string
	require.NoError(t, json.Unmarshal([]byte(sentConfig(t, recorder.line)), &sides))
	assert.Equal(t, map[string]string{"before": before, "after": after}, sides)
	assert.Equal(t, config.DiffPair{Old: "5", New: "7"}, diff.Changed["acme-monitor/interval"])
	requireNotStarted(t, marker)
}

// TestInspectRouteDrawsTheDaemonsRefusal: a config the running ze refuses, and
// that this process refuses too, is answered with the daemon's refusal.
// Method: a stub ask that answers a failed status and its text.
//
// VALIDATES: a nonzero exit, no answer, and one operator error in place of the
// local "not available" refusal: that the running ze refused the config, the
// daemon's own lines as what it answered, and the daemon's fix as the one fix.
// PREVENTS: the range a plugin's module declares being hidden behind a local
// refusal that knows nothing of the module; a second fix beside the daemon's.
func TestInspectRouteDrawsTheDaemonsRefusal(t *testing.T) {
	const daemonLine = "test.conf: interval 11 is outside the accepted range"
	const refusal = daemonLine + "\nfix: ze config validate test.conf"
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			_, path, marker := routedConfig(t)
			recorder := &askRecorder{verdict: daemonVerdict{refused: true, refusal: refusal, address: daemonAddress}}

			code, answered, diagnostic := route.answer(t, path, recorder.ask)

			require.Equal(t, 1, recorder.calls)
			assert.NotEqual(t, exitOK, code)
			assert.False(t, answered)
			subject := "config " + path
			if route.name == "diff" {
				subject = "the two configs"
			}
			assert.Equal(t, "error: the running ze refused "+subject+"\n"+
				"It checked the config with the schemas of the plugins it started. What it answered: "+daemonLine+"\n"+
				"fix: ze config validate test.conf\n", diagnostic)
			requireNotStarted(t, marker)
		})
	}
}

// TestDaemonRefusalHoldsOneFix: the refusal of the running ze is drawn as one
// operator error with one fix, the daemon's own when its text ends with one.
// Method: daemonRefusal over the texts a daemon sends.
//
// VALIDATES: every case renders exactly one `fix: `. A text that ends with a
// fix line keeps that fix and no other; every fix line is cut out of what the
// daemon answered; a text with none, or with an empty one, gets the fallback
// command; a text of fix lines alone is drawn as a refusal with no reason.
// PREVENTS: two fix lines, whatever the daemon sent; a refusal with no
// command to run.
func TestDaemonRefusalHoldsOneFix(t *testing.T) {
	const head = "the running ze refused config a.conf\n" +
		"It checked the config with the schemas of the plugins it started. What it answered: "
	rows := []struct {
		name string
		text string
		want string
	}{
		{"the daemon's fix is kept", "interval 11 is out of range\nwhy\nfix: ze show doctor", head + "interval 11 is out of range\nwhy\nfix: ze show doctor"},
		{"a final newline is not part of the fix", "refused\nfix: ze show doctor\n", head + "refused\nfix: ze show doctor"},
		{"no fix line gets the fallback", "not authorized", head + "not authorized\nfix: ze config validate a.conf"},
		{"an empty fix line is cut and the fallback used", "refused\nfix:  ", head + "refused\nfix: ze config validate a.conf"},
		{"a fix line before the last is cut too", "refused\nfix: ze status\nmore\nfix: ze show doctor", head + "refused\nmore\nfix: ze show doctor"},
		{"a text that is only a fix line keeps its fix", "fix: ze start", head + "a refusal with no reason\nfix: ze start"},
		{"an empty text", "", head + "a refusal with no reason\nfix: ze config validate a.conf"},
	}
	for _, row := range rows {
		got := daemonRefusal(configSubject("a.conf"), validateFix("a.conf"), row.text).Error()
		assert.Equal(t, row.want, got, row.name)
		assert.Equal(t, 1, strings.Count(got, "fix: "), "one fix (%s): %s", row.name, got)
	}
	assert.Equal(t, "ze config validate <config-file>", validateFix("-"))
}

// TestInspectRouteFallsBackWhenNoZeAnswers verifies S-3 / AC-18 fallback across
// all four inspection routes when no daemon listens: a non-declaring external
// program is refused naming endpoint, cause, and one conformance fix line, and
// a declare-mode external program succeeds offline without starting its run loop.
//
// VALIDATES: S-3 / AC-18 — local fallback queries external programs in declare
// mode without starting their run loop, succeeding on declaring programs and
// failing closed with `fix: ze show plugin conformance run ...` otherwise.
// PREVENTS: a failed ask read as an answer, starting an external run loop
// offline, or omitting the conformance fix.
func TestInspectRouteFallsBackWhenNoZeAnswers(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name+"/non_declaring_refused", func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			_, path, marker := routedConfig(t)

			code, answered, diagnostic := route.answer(t, path, noDaemonListens)

			assert.NotEqual(t, exitOK, code)
			assert.False(t, answered)
			assert.Contains(t, diagnostic, `bus endpoint "acme"`)
			assert.Contains(t, diagnostic, "without writing a Stage 1 declaration")
			assert.Equal(t, 1, strings.Count(diagnostic, "fix: "), "one fix: %s", diagnostic)
			assert.Contains(t, diagnostic, "fix: ze show plugin conformance run ")
			requireNotStarted(t, marker)
		})

		t.Run(route.name+"/declaring_succeeds", func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := declaringPlugin(t, monitorSchemaDecl())
			path := writeTestConfig(t, monitorConfig(run, routeSection)+testConfigBase)

			code, answered, diagnostic := route.answer(t, path, noDaemonListens)

			assert.Equal(t, exitOK, code, diagnostic)
			assert.True(t, answered)
			requireNotStarted(t, marker)
		})
	}
}

// TestInspectRouteNamesTheAccountFixOfTheCommandItself verifies that a failed
// login ask builds the `--user <name>` fix for each inspection command, and
// that local fallback fails closed on a non-declaring program without starting
// its run loop.
//
// VALIDATES: suppliedQuestion.unanswered produces the `--user <name>` fix for
// each route, and local fallback fails closed under S-3.
// PREVENTS: a fix that names `ze config validate` instead of the command the
// operator typed.
func TestInspectRouteNamesTheAccountFixOfTheCommandItself(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			_, path, marker := routedConfig(t)
			askErr := notAnswered(daemonAddress, route.supplied, errors.New("ssh: handshake failed"))
			q := suppliedQuestion{
				local:    "ze config " + route.name,
				operands: path,
				files:    []string{path},
			}
			if route.name == "diff" {
				q.operands = path + " " + path
				q.files = []string{path, path}
			}
			unanswered := q.unanswered(askErr)
			fix := strings.Replace(route.typed(path), " "+path, " --user <name> "+path, 1)
			assert.Contains(t, unanswered.Why, "did not answer "+route.supplied+" (ssh: handshake failed)")
			assert.Equal(t, fix, unanswered.Fix)

			recorder := &askRecorder{err: askErr}
			code, answered, diagnostic := route.answer(t, path, recorder.ask)
			assert.NotEqual(t, exitOK, code)
			assert.False(t, answered)
			assert.Contains(t, diagnostic, "fix: ze show plugin conformance run ")
			requireNotStarted(t, marker)
		})
	}
}

// TestInspectRouteAsksNobodyForACompiledInConfig: a config that declares no
// external plugin is answered here and no daemon is asked. Method: a config
// with an empty plugin block and a recording ask. The config holds a bgp
// block, which the dump resolves and refuses a config without.
//
// VALIDATES: zero asks and a local answer.
// PREVENTS: every local command opening a connection to the daemon.
func TestInspectRouteAsksNobodyForACompiledInConfig(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			path := writeTestConfig(t, "bus {\n}\n"+testConfigBase)
			recorder := &askRecorder{err: errors.New("asked")}

			code, _, diagnostic := route.answer(t, path, recorder.ask)

			assert.Equal(t, 0, recorder.calls)
			assert.Equal(t, exitOK, code, diagnostic)
		})
	}
}

// TestInspectRouteKeepsTheLocalAnswerWhenTheDaemonRefusesWhatThisProcessAccepts:
// a daemon refusal of a config this process answers about is not about the
// config, and the local answer stands. Method: a config that declares an
// external plugin and holds no section of it, and a stub ask that refuses. The
// config holds a bgp block, which the dump resolves and refuses a config
// without, so every one of the four commands answers about it here.
//
// VALIDATES: the local answer, exit 0, and none of the daemon's refusal text.
// PREVENTS: a denied account, or a daemon that does not serve the command,
// turning a config this binary can answer about into a failure.
func TestInspectRouteKeepsTheLocalAnswerWhenTheDaemonRefusesWhatThisProcessAccepts(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := declaringPlugin(t)
			path := writeTestConfig(t, monitorConfig(run, testConfigBase))
			recorder := &askRecorder{verdict: daemonVerdict{refused: true, refusal: "not authorized", address: daemonAddress}}

			code, _, diagnostic := route.answer(t, path, recorder.ask)

			require.Equal(t, 1, recorder.calls)
			assert.Equal(t, exitOK, code, diagnostic)
			assert.NotContains(t, diagnostic, "not authorized")
			requireNotStarted(t, marker)
		})
	}
}

// TestInspectRouteDoesNotSendContentOverTheBound: a config too large for one
// command is not sent over SSH; local fallback runs S-3 query mode without
// sending the oversize payload.
//
// VALIDATES: zero asks; local fallback fails closed on a non-declaring program
// with one conformance fix and never starts its run loop.
// PREVENTS: a line the daemon's validator refuses, or one that does not fit an
// SSH packet, being sent over SSH.
func TestInspectRouteDoesNotSendContentOverTheBound(t *testing.T) {
	for _, route := range inspectionRoutes() {
		t.Run(route.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := silentPlugin(t)
			padding := strings.Repeat("# "+strings.Repeat("x", 78)+"\n", 2600)
			path := filepath.Join(t.TempDir(), "large.conf")
			require.NoError(t, os.WriteFile(path, []byte(monitorConfig(run, routeSection)+padding), 0o600))
			recorder := &askRecorder{err: errors.New("asked")}

			code, answered, diagnostic := route.answer(t, path, recorder.ask)

			assert.Equal(t, 0, recorder.calls, "content over the bound was sent")
			assert.NotEqual(t, exitOK, code)
			assert.False(t, answered)
			assert.Contains(t, diagnostic, `bus endpoint "acme"`)
			assert.Contains(t, diagnostic, "fix: ze show plugin conformance run ")
			assert.Equal(t, 1, strings.Count(diagnostic, "fix: "), "one fix: %s", diagnostic)
			requireNotStarted(t, marker)
		})
	}
}

// TestInspectRouteAsksNobodyOnTheDaemonRoute: the daemon is the process that
// answers, so a command served on its route asks no other ze and starts no
// external program. Method: fix the module set as a running daemon does, then
// run dump and diff on RouteRemote with a recording ask.
//
// VALIDATES: zero asks on the remote route, no external program started, and
// daemon refusal naming `restart ze`.
// PREVENTS: a daemon that asks a daemon or forks external programs on inspect.
func TestInspectRouteAsksNobodyOnTheDaemonRoute(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	configyang.KeepsModuleSet("test daemon")
	_, path, marker := routedConfig(t)
	recorder := &askRecorder{err: errors.New("asked")}

	var diag textbuf.Buffer
	_, dumpCode := dumpAsking(path, path, false, command.RouteRemote, "", recorder.ask, &diag)
	_, diffCode := diffAsking(nil, []string{path, path}, path+" "+path, command.RouteRemote, "", recorder.ask, &diag)

	assert.Equal(t, 0, recorder.calls)
	assert.NotEqual(t, 0, dumpCode)
	assert.NotEqual(t, exitOK, diffCode)
	got := diag.String()
	assert.Contains(t, got, "this ze holds no config schema")
	assert.Contains(t, got, "fix: restart ze")
	requireNotStarted(t, marker)
}

// storedDiffForm is one form of `ze config diff` whose sides are read from a
// store: the store `--backup` opens, or the one a storage-aware caller holds.
type storedDiffForm struct {
	name string
	// args are the two words after the options.
	args []string
	// typed is the operator's words after `ze config diff`, as cmdDiffImpl
	// writes them for diffAsking.
	typed string
	// storedBefore is true when the first side is the stored before.conf, and
	// false when it is revision 1 of after.conf.
	storedBefore bool
}

func storedDiffForms() []storedDiffForm {
	return []storedDiffForm{
		{name: "two stored files", args: []string{"before.conf", "after.conf"}, typed: "before.conf after.conf", storedBefore: true},
		{name: "revision and stored file", args: []string{"1", "after.conf"}, typed: "1 after.conf"},
		{
			name: "backup artifact", args: []string{"before.conf", "after.conf"},
			typed: "--backup site.backup before.conf after.conf", storedBefore: true,
		},
	}
}

// storedDiffSides writes a store that holds before.conf, after.conf and one
// revision of after.conf, each a config with a section only external plugin
// acme defines and each with its own text, and answers the store, the three
// texts and the marker of the plugin's program.
func storedDiffSides(t *testing.T) (store storage.Storage, before, after, revision, marker string) {
	t.Helper()
	run, marker := silentPlugin(t)
	base := monitorConfig(run, routeSection)
	before, after, revision = base+"# before\n", base+"# after\n", base+"# revision\n"
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("before.conf", []byte(before), 0o600))
	require.NoError(t, store.WriteFile("after.conf", []byte(after), 0o600))
	require.NoError(t, store.WriteVersion("after.conf", []byte(revision), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)))
	return store, before, after, revision, marker
}

// TestDiffRouteAsksTheRunningZeAboutStoredSides: `ze config diff` over a
// store, in the two-file form and the revision form, sends both sides to the
// running ze and draws its comparison. Method: diffAsking with a store, as
// cmdDiffImpl calls it, and a stub ask that accepts.
//
// VALIDATES: one ask to `show config supplied diff`; the content holds the
// bytes read from the store, the revision's for the revision form; exit 0 and
// the daemon's comparison; no local refusal of the external section.
// PREVENTS: the stored and revision forms judged in this process alone while
// a running ze that holds the plugin's schema could compare them.
func TestDiffRouteAsksTheRunningZeAboutStoredSides(t *testing.T) {
	for _, form := range storedDiffForms() {
		t.Run(form.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			store, before, after, revision, marker := storedDiffSides(t)
			recorder := &askRecorder{verdict: daemonVerdict{
				body: []byte(`{"added":{"marker":"daemon"},"removed":{},"changed":{}}`), address: daemonAddress,
			}}

			var diag textbuf.Buffer
			diff, code := diffAsking(store, form.args, form.typed, command.RouteLocal, "ops", recorder.ask, &diag)

			require.Equal(t, 1, recorder.calls)
			require.True(t, strings.HasPrefix(recorder.line, pathSuppliedDiff+" content "), "line: %.80q", recorder.line)
			assert.Equal(t, "ops", recorder.user)
			var sent map[string]string
			require.NoError(t, json.Unmarshal([]byte(sentConfig(t, recorder.line)), &sent))
			first := revision
			if form.storedBefore {
				first = before
			}
			assert.Equal(t, map[string]string{suppliedBefore: first, suppliedAfter: after}, sent)
			require.Equal(t, exitOK, code, diag.String())
			require.NotNil(t, diff)
			assert.Equal(t, map[string]any{"marker": "daemon"}, diff.Added, "the daemon's comparison was not drawn")
			assert.NotContains(t, diag.String(), "unknown top-level keyword")
			requireNotStarted(t, marker)
		})
	}
}

// TestDiffCommandAsksAboutABackupArtifactAndNamesItWhenNoZeAnswers:
// `ze config diff --backup <artifact> <name1> <name2>` opens the artifact,
// asks the running ze about the two configs it holds, and, when none answers,
// names the command to run again with the --backup option as the operator
// typed it. Method: cmdDiffImpl over an artifact written with
// storage.CreateBlob and closed, as an operator's backup is, first with a
// recording ask that accepts, then with the ask of a host where no ze listens.
//
// VALIDATES: one ask holding the artifact's two configs and exit 0; with no
// ze, a nonzero exit, the one fix `ze start`, and "then `ze config diff
// --backup <artifact> before.conf after.conf` again", the words cmdDiffImpl
// builds.
// PREVENTS: a --backup comparison judged here alone; a rerun command that
// drops the option and so names two files that do not exist.
func TestDiffCommandAsksAboutABackupArtifactAndNamesItWhenNoZeAnswers(t *testing.T) {
	t.Cleanup(config.ResetDeclaredSchemasForTest())
	run, marker := silentPlugin(t)
	base := monitorConfig(run, routeSection)
	before, after := base+"# before\n", base+"# after\n"
	artifact := filepath.Join(t.TempDir(), "site.zefs")
	created, err := storage.CreateBlob(artifact)
	require.NoError(t, err)
	require.NoError(t, created.WriteFile("before.conf", []byte(before), 0))
	require.NoError(t, created.WriteFile("after.conf", []byte(after), 0))
	require.NoError(t, created.Close())
	args := []string{"--backup", artifact, "before.conf", "after.conf"}

	recorder := &askRecorder{verdict: daemonVerdict{
		body: []byte(`{"added":{},"removed":{},"changed":{}}`), address: daemonAddress,
	}}
	useRouteAsk(t, recorder.ask)
	var code int
	captureDiffStdout(t, func() int {
		code, _ = captureStderr(t, func() int { return cmdDiffImpl(nil, args) })
		return code
	})
	require.Equal(t, 1, recorder.calls)
	var sent map[string]string
	require.NoError(t, json.Unmarshal([]byte(sentConfig(t, recorder.line)), &sent))
	assert.Equal(t, map[string]string{suppliedBefore: before, suppliedAfter: after}, sent)
	assert.Equal(t, exitOK, code)

	useRouteAsk(t, noDaemonListens)
	code, diagnostic := captureStderr(t, func() int { return cmdDiffImpl(nil, args) })
	assert.NotEqual(t, exitOK, code)
	assert.Contains(t, diagnostic, `bus endpoint "acme"`)
	assert.Contains(t, diagnostic, "without writing a Stage 1 declaration")
	assert.Equal(t, 1, strings.Count(diagnostic, "fix: "), "one fix: %s", diagnostic)
	assert.True(t, strings.HasSuffix(diagnostic, `fix: ze show plugin conformance run "`+run+"\"\n"), "conformance fix lost: %s", diagnostic)
	requireNotStarted(t, marker)
}

// TestDiffRouteFallsBackOverStoredSidesWhenNoZeAnswers verifies S-3 / AC-18
// fallback on stored diff sides when no running ze answers: offline query mode
// refuses a non-declaring external program naming endpoint, cause, and one
// conformance fix line, without starting its run loop.
//
// VALIDATES: S-3 / AC-18 on stored and revision diff forms.
// PREVENTS: silent acceptance or run-loop execution on offline stored diffs.
func TestDiffRouteFallsBackOverStoredSidesWhenNoZeAnswers(t *testing.T) {
	for _, form := range storedDiffForms() {
		t.Run(form.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			store, _, _, _, marker := storedDiffSides(t)

			var diag textbuf.Buffer
			diff, code := diffAsking(store, form.args, form.typed, command.RouteLocal, "", noDaemonListens, &diag)

			diagnostic := diag.String()
			assert.NotEqual(t, exitOK, code)
			assert.Nil(t, diff)
			assert.Contains(t, diagnostic, `bus endpoint "acme"`)
			assert.Contains(t, diagnostic, "without writing a Stage 1 declaration")
			assert.Equal(t, 1, strings.Count(diagnostic, "fix: "), "one fix: %s", diagnostic)
			assert.Contains(t, diagnostic, "fix: ze show plugin conformance run ")
			requireNotStarted(t, marker)
		})
	}
}
