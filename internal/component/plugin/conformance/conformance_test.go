package conformance

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testBudget is long enough for a shell one-liner on a loaded machine. The
// timeout case states its own, shorter one.
const testBudget = 10 * time.Second

// declarationBytes is the line the protocol's own writer produces, so the
// strict form is tested against what ze and the SDK really emit.
func declarationBytes(t *testing.T, declaration *rpc.DeclareRegistrationInput) []byte {
	t.Helper()
	var line bytes.Buffer
	require.NoError(t, rpc.WriteDeclaration(&line, declaration))
	return line.Bytes()
}

// writes builds a run string whose program writes text to stdout verbatim.
// text MUST NOT hold a single quote.
func writes(text string) string {
	return "printf '%s' '" + text + "'"
}

func validDeclaration() *rpc.DeclareRegistrationInput {
	return &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "Show the answer."}},
	}
}

func runTarget(t *testing.T, run string, budget time.Duration) Report {
	t.Helper()
	report, err := Run(context.Background(), Target{Run: run, Name: "conformance-subject"}, budget)
	require.NoError(t, err)
	return report
}

func rowNamed(t *testing.T, report Report, check string) Row {
	t.Helper()
	for _, row := range report.Rows {
		if row.Check == check {
			return row
		}
	}
	t.Fatalf("the report holds no row for the check %q", check)
	return Row{}
}

// VALIDATES: the strict form accepts the line rpc.WriteDeclaration writes.
// PREVENTS: the checker refusing every plugin built on the SDK, and the
// request id written here drifting from the protocol's.
func TestStrictLineAcceptsTheLineTheProtocolWrites(t *testing.T) {
	payload, broken := strictLine(declarationBytes(t, validDeclaration()))
	require.Empty(t, broken)
	assert.True(t, bytes.HasPrefix(payload, []byte("{")), "the payload is the JSON object: %q", payload)
}

// VALIDATES: each rule of the strict form is refused by name.
// PREVENTS: a plugin that writes a banner, a CRLF or a second line reading
// as conforming because the lenient reader skips them.
func TestStrictLineNamesTheRuleTheOutputBroke(t *testing.T) {
	good := string(declarationBytes(t, validDeclaration()))
	body := strings.TrimSuffix(good, "\n")
	brace := strings.Index(body, "{")
	require.GreaterOrEqual(t, brace, 0, "the declaration line holds no JSON object: %q", body)
	json := body[brace:]

	cases := []struct {
		name   string
		stdout string
		want   string
	}{
		{"nothing", "", "wrote nothing"},
		{"crlf", body + "\r\n", "carriage return"},
		{"no line feed", body, "does not end with a line feed"},
		{"banner before", "starting up\n" + good, "holds 2 lines"},
		{"second line after", good + "done\n", "holds 2 lines"},
		{"blank line after", good + "\n", "holds 2 lines"},
		{"wrong id", "#2 " + rpc.MethodDeclareRegistration + " " + json + "\n", "request id is #2"},
		{"wrong method", "#1 ze-plugin-engine:ready " + json + "\n", "the method is"},
		{"not a request line", "hello\n", "not in the form"},
		{"truncated json", "#1 " + rpc.MethodDeclareRegistration + " {\"protocol-version\":1\n", "not valid JSON"},
		{"spaced json", "#1 " + rpc.MethodDeclareRegistration + " {\"protocol-version\": 1}\n", "not compact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, broken := strictLine([]byte(tc.stdout))
			require.NotEmpty(t, broken, "the output was accepted")
			assert.Contains(t, broken, tc.want)
		})
	}
}

// VALIDATES: a conforming program passes every judged row, the report says
// pass and exits 0, and the behaviors nobody judged are rows too.
// PREVENTS: a report with no failure read as full conformance.
func TestRunPassesAConformingProgram(t *testing.T) {
	report := runTarget(t, writes(string(declarationBytes(t, validDeclaration()))), testBudget)

	assert.Equal(t, VerdictPass, report.Verdict)
	assert.Equal(t, ExitConforms, report.ExitCode())
	for _, check := range []string{"program-starts", "exits-clean", "declaration-line", "declaration-decodes", "protocol-version", "help-texts", "command-names"} {
		row := rowNamed(t, report, check)
		assert.Equal(t, VerdictPass, row.Verdict, "%s: %s", check, row.Reason)
	}
	for _, check := range []string{
		"dependencies", "respawn-agreement", "tls-connect-back", "stage-2-config", "stage-3-capabilities",
		"stage-4-registry", "stage-5-ready", "config-callbacks", "execute-command-framing", "bye-and-exit", "schema",
	} {
		row := rowNamed(t, report, check)
		assert.Equal(t, VerdictNotJudged, row.Verdict, check)
		assert.NotEmpty(t, row.Reason, "%s: a row nobody judged says why", check)
	}
}

// VALIDATES: each way a declare-mode run goes wrong fails the row that names
// it, with a reason and a fix, and the report exits 1.
// PREVENTS: a program that exits non-zero, runs past the budget, leaves a
// process behind, writes no declaration or cannot start reading as a plugin.
func TestRunFailsTheRowThatNamesTheViolation(t *testing.T) {
	good := string(declarationBytes(t, validDeclaration()))
	textEra := string(declarationBytes(t, &rpc.DeclareRegistrationInput{}))
	badHelp := string(declarationBytes(t, &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "a tab\tbreaks completion"}},
	}))
	badName := string(declarationBytes(t, &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "Show Upper Case"}},
	}))

	cases := []struct {
		name   string
		run    string
		budget time.Duration
		check  string
		stage  string
		reason string
	}{
		{"cannot start", "/nonexistent/ze-conformance-no-such-program", testBudget, "program-starts", stageDeclare, "started no program"},
		{"non-zero exit", writes(good) + "; exit 3", testBudget, "exits-clean", stageDeclare, "exited 3"},
		{"timeout", writes(good) + "; exec sleep 30", 300 * time.Millisecond, "exits-clean", stageDeclare, "still running"},
		{"descendant outlives it", writes(good) + "; sleep 2 & exit 0", testBudget, "exits-clean", stageDeclare, "still held its standard output"},
		// printf reads the escapes of its format, so this line ends CR LF.
		{"crlf", "printf '%s\\r\\n' '" + strings.TrimSuffix(good, "\n") + "'", testBudget, "declaration-line", stageDeclare, "carriage return"},
		{"second stdout line", "echo banner; " + writes(good), testBudget, "declaration-line", stageDeclare, "holds 2 lines"},
		{"no declaration", "exit 0", testBudget, "declaration-decodes", stageDeclare, "no " + rpc.MethodDeclareRegistration},
		{"wrong method", writes("#1 ze-plugin-engine:ready {}\n"), testBudget, "declaration-decodes", stageDeclare, "no " + rpc.MethodDeclareRegistration},
		{"stage 1 refusal", writes(textEra), testBudget, "protocol-version", stageOne, "declared no protocol version"},
		{"stage 1 wrapped refusal", writes(badHelp), testBudget, "help-texts", stageOne, "invalid help text: conformance-subject: "},
		{"stage 5 command name", writes(badName), testBudget, "command-names", stageFive, "the daemon keeps the plugin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := runTarget(t, tc.run, tc.budget)

			row := rowNamed(t, report, tc.check)
			require.Equal(t, VerdictFail, row.Verdict, "reason %q", row.Reason)
			assert.Equal(t, tc.stage, row.Stage)
			assert.Contains(t, row.Reason, tc.reason)
			assert.NotEmpty(t, row.Fix, "a failed row names the change that fixes it")
			assert.Equal(t, VerdictFail, report.Verdict)
			assert.Equal(t, ExitViolation, report.ExitCode())
		})
	}
}

// VALIDATES: a program that did not start, or declared nothing, leaves the
// later rows not-judged under their own names, never passed.
// PREVENTS: Stage 1 rows passing for a declaration nobody read.
func TestRunNeverPassesARowItCouldNotJudge(t *testing.T) {
	notStarted := runTarget(t, "/nonexistent/ze-conformance-no-such-program", testBudget)
	for _, check := range []string{"exits-clean", "declaration-line", "declaration-decodes", "protocol-version", "command-names"} {
		assert.Equal(t, VerdictNotJudged, rowNamed(t, notStarted, check).Verdict, check)
	}

	silent := runTarget(t, "exit 0", testBudget)
	assert.Equal(t, VerdictPass, rowNamed(t, silent, "exits-clean").Verdict)
	for _, check := range []string{"protocol-version", "drain", "help-texts", "command-names"} {
		assert.Equal(t, VerdictNotJudged, rowNamed(t, silent, check).Verdict, check)
	}
}

// VALIDATES: every registered check answers at least one row, every row
// names its check, stage and verdict, and every row that did not pass says
// why and what to do.
// PREVENTS: a check registered and silent, and a failure with no fix.
func TestEveryRegisteredCheckAnswersCompleteRows(t *testing.T) {
	for _, run := range []string{writes(string(declarationBytes(t, validDeclaration()))), "exit 4", "/nonexistent/ze-conformance-no-such-program"} {
		report := runTarget(t, run, testBudget)
		// Each registered check answers one row or more, so a report holds
		// no fewer rows than there are checks.
		require.GreaterOrEqual(t, len(report.Rows), len(registered()))
		for _, row := range report.Rows {
			assert.NotEmpty(t, row.Check)
			assert.NotEmpty(t, row.Stage, row.Check)
			assert.Contains(t, []Verdict{VerdictPass, VerdictFail, VerdictNotJudged}, row.Verdict, row.Check)
			if row.Verdict != VerdictPass {
				assert.NotEmpty(t, row.Reason, row.Check)
				assert.NotEmpty(t, row.Fix, row.Check)
			}
		}
	}
}

// VALIDATES: a run that cannot happen is an error, never a report.
// PREVENTS: an empty run string or a stopped caller answered as a plugin
// that failed every check.
func TestRunRefusesARunThatCannotHappen(t *testing.T) {
	_, err := Run(context.Background(), Target{Run: "  ", Name: "x"}, testBudget)
	require.ErrorIs(t, err, errNoRunString)

	_, err = Run(context.Background(), Target{Run: "exit 0"}, testBudget)
	require.ErrorIs(t, err, errNoPluginName)

	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Run(stopped, Target{Run: "exit 0", Name: "x"}, testBudget)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped before it started")
}

// VALIDATES: the command answers a caller that is not the operator's own
// process with a refusal that names the reason and the local command.
// PREVENTS: the daemon starting a program a remote caller named.
func TestTheCommandRefusesEveryRouteButTheLocalOne(t *testing.T) {
	validated, err := command.ValidateArgs([]string{"exit 0"}, conformanceArgDefs(t), nil)
	require.NoError(t, err)
	run, found := validated.Value(leafCommand)
	require.True(t, found, "the input must carry a run string, so the remote gate is what refuses it")
	require.Equal(t, "exit 0", run)
	for name, answer := range map[string]command.LocalAnswer{
		"remote":   command.AnswerRemote(dataConformance, validated),
		"unmarked": dataConformance(validated),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Nil(t, answer.Payload)
			assert.Equal(t, ExitNoRun, answer.Code)
			assert.Contains(t, answer.Diagnostic, "the daemon starts no program a remote caller names")
			assert.Contains(t, answer.Diagnostic, "ze show plugin conformance run")
		})
	}

	require.NoError(t, refuseRemoteCaller(command.RouteLocal, "exit 0"))
	require.Error(t, refuseRemoteCaller(command.RouteRemote, "exit 0"))
	require.Error(t, refuseRemoteCaller(command.RouteUnspecified, "exit 0"))
}

// VALIDATES: a call whose run string no single token gave (no token, or
// several positional words the validator does not assign) is refused in the
// same words on the local, the remote and the unmarked route, with a fix that
// names the run string and gives the plugin name by keyword.
// PREVENTS: the command carrying on with an empty run string, whose refusals
// named `run ""` as the fix and read differently on the local and the daemon
// route.
func TestTheCommandRefusesAnUnreadRunStringOnEveryRoute(t *testing.T) {
	several, err := command.ValidateArgs([]string{"x", "ze-match-no-such-word"}, conformanceArgDefs(t), nil)
	require.NoError(t, err)
	_, found := several.Value(leafCommand)
	require.False(t, found, "several positional words must leave the run string unassigned")
	for input, validated := range map[string]command.ValidatedArgs{
		"no token":      {},
		"several words": several,
	} {
		t.Run(input, func(t *testing.T) {
			answers := map[string]command.LocalAnswer{
				"local":    command.AnswerLocal(dataConformance, validated),
				"remote":   command.AnswerRemote(dataConformance, validated),
				"unmarked": dataConformance(validated),
			}
			want := answers["local"].Diagnostic
			require.NotEmpty(t, want)
			for route, answer := range answers {
				assert.Nil(t, answer.Payload, route)
				assert.Equal(t, ExitNoRun, answer.Code, route)
				assert.Equal(t, want, answer.Diagnostic, route)
				assert.Contains(t, answer.Diagnostic,
					"\nfix: ze show plugin conformance run \"<run string>\" name <plugin-name>\n", route)
				assert.NotContains(t, answer.Diagnostic, `run ""`, route)
				assert.NotContains(t, answer.Diagnostic, "the daemon starts no program a remote caller names", route)
				assert.NotContains(t, answer.Diagnostic, "it starts the program named on the command line", route)
			}
		})
	}
}

// conformanceArgDefs declares the two leaves of the command's YANG node
// (ze-plugin-show-cmd.yang): command, mandatory, and name, each a string of
// the declared length.
func conformanceArgDefs(t *testing.T) []command.ArgDef {
	t.Helper()
	run, err := command.NewStringArg(leafCommand, []command.UintRange{{Min: 1, Max: 4096}}, nil,
		command.ArgOptions{Mandatory: true})
	require.NoError(t, err)
	name, err := command.NewStringArg(leafName, []command.UintRange{{Min: 1, Max: 128}}, nil,
		command.ArgOptions{})
	require.NoError(t, err)
	return []command.ArgDef{run, name}
}

// VALIDATES: the default plugin name is the file name of the program.
func TestDefaultName(t *testing.T) {
	assert.Equal(t, "my-plugin", DefaultName("./bin/my-plugin --verbose"))
	assert.Equal(t, "ze", DefaultName("ze plugin mrt"))
	assert.Empty(t, DefaultName("   "))
}
