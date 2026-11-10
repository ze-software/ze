// Design: docs/architecture/doctor-and-health-checks.md -- the local route asks the running daemon first

package doctor

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// isolateDoctorStore points the store at an empty directory, so the local
// checks a fallback runs read no store the developer's box holds.
func isolateDoctorStore(t *testing.T) {
	t.Helper()
	t.Setenv("ze_config_dir", t.TempDir())
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// localAnswer runs the local route over tokens with ask in place of the SSH
// ask, marked as the operator's own process the way Run marks it, and answers
// the readiness result it drew.
func localAnswer(t *testing.T, ask daemonAsk, tokens ...string) diagnostic.DoctorResult {
	t.Helper()
	handler := func(validated command.ValidatedArgs) command.LocalAnswer {
		return answerLocalWith(validated, ask)
	}
	answer := command.AnswerLocal(handler, commandtest.Args(tokens...))
	readiness, ok := answer.Payload.(readinessAnswer)
	if !ok {
		t.Fatalf("local route answered %T (diagnostic %q), want a readiness answer", answer.Payload, answer.Diagnostic)
	}
	return readiness.DoctorResult
}

// notCheckedRows answers the doctor-runtime-not-checked rows of a result.
func notCheckedRows(result diagnostic.DoctorResult) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == diagnostic.CodeDoctorRuntimeNotChecked {
			rows = append(rows, result.Diagnostics[i])
		}
	}
	return rows
}

// TestDoctorRuntimePhaseNotRunLocally is AC-2: with no daemon answering, the
// local route runs no runtime check, answers its local checks, and adds one
// doctor-runtime-not-checked warning whose message says the runtime checks
// need the running daemon and whose fix starts it.
//
// Method: register a runtime check that counts its runs, ask through a stub
// that answers "no daemon listens" as the SSH ask does, and read the answer.
func TestDoctorRuntimePhaseNotRunLocally(t *testing.T) {
	isolateDoctorStore(t)
	const name = "test-local-route-runtime"
	ran := 0
	check := diagnostic.DoctorCheck{
		Name: name, Phase: diagnostic.DoctorPhaseRuntime, Component: "test",
		Dependencies: []string{"x"}, Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes: []string{"doctor-test-local-route-runtime"},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			ran++
			return []diagnostic.Diagnostic{{Code: "doctor-test-local-route-runtime", Severity: diagnostic.SeverityError, Message: "ran"}}
		},
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(name) })

	asked := 0
	noDaemon := func(string) (diagnostic.DoctorResult, error) {
		asked++
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: "no daemon listens at 127.0.0.1:1 (connection refused)", fix: fixStartDaemon}
	}
	result := localAnswer(t, noDaemon, "--json")

	if asked != 1 {
		t.Fatalf("the local route asked the daemon %d times, want once", asked)
	}
	if ran != 0 {
		t.Fatalf("the local route ran the runtime check %d times in its own process", ran)
	}
	rows := notCheckedRows(result)
	if len(rows) != 1 {
		t.Fatalf("doctor-runtime-not-checked rows = %d, want 1: %+v", len(rows), result.Diagnostics)
	}
	row := rows[0]
	if row.Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %q, want warning: a box whose daemon is down may still be ready to run it", row.Severity)
	}
	if !strings.Contains(row.Message, "need the running daemon") {
		t.Errorf("message %q does not say the runtime checks need the running daemon", row.Message)
	}
	if !strings.Contains(row.Message, "no daemon listens at 127.0.0.1:1") {
		t.Errorf("message %q does not name why no daemon answered", row.Message)
	}
	if row.Fix != "ze start" {
		t.Errorf("fix = %q, want %q", row.Fix, "ze start")
	}
	// The local checks still answered: with an empty store directory, the
	// missing config is one of them.
	if len(result.Diagnostics) < 2 {
		t.Errorf("the fallback answered only the not-checked row, not the local checks: %+v", result.Diagnostics)
	}
}

// TestDoctorLocalRouteAsksReachableDaemon is AC-10 and R-5. A daemon that
// answers gives its own answer, drawn here, with no warning row; a daemon
// that does not answer gives AC-2; a config file outside the daemon's config
// directories is answered locally, without asking, with the warning row.
//
// Method: three stub asks, one per case, over the local route.
func TestDoctorLocalRouteAsksReachableDaemon(t *testing.T) {
	isolateDoctorStore(t)

	daemonRows := []diagnostic.Diagnostic{
		{Code: "doctor-test-daemon-runtime", Severity: diagnostic.SeverityWarning, Message: "the daemon's live state", Fix: "show test runtime"},
	}
	var sent []string
	daemonUp := func(line string) (diagnostic.DoctorResult, error) {
		sent = append(sent, line)
		return diagnostic.NewDoctorResult(true, daemonRows), nil
	}

	result := localAnswer(t, daemonUp)
	if len(sent) != 1 || sent[0] != "show doctor" {
		t.Fatalf("the local route sent %q, want one show doctor", sent)
	}
	if rows := notCheckedRows(result); len(rows) != 0 {
		t.Errorf("a daemon answered, yet the local route added %+v", rows)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "doctor-test-daemon-runtime" || result.Diagnostics[0].Fix != "show test runtime" {
		t.Errorf("the drawn answer is not the daemon's: %+v", result.Diagnostics)
	}
	if !result.Ready {
		t.Errorf("the daemon answered ready, the drawn answer is not")
	}

	down := func(string) (diagnostic.DoctorResult, error) {
		return diagnostic.DoctorResult{}, &daemonUnanswered{why: "no daemon listens at 127.0.0.1:1", fix: fixStartDaemon}
	}
	if rows := notCheckedRows(localAnswer(t, down)); len(rows) != 1 || rows[0].Fix != fixStartDaemon {
		t.Errorf("no daemon answering: rows %+v, want one with fix %q", rows, fixStartDaemon)
	}

	// R-5: a file outside the daemon's config directories. The daemon would
	// read another file or refuse, so it is not asked.
	sent = nil
	outside := filepath.Join(t.TempDir(), "elsewhere.conf")
	result = localAnswer(t, daemonUp, "config", outside)
	if len(sent) != 0 {
		t.Fatalf("the local route asked the daemon %q about a file it does not read", sent)
	}
	rows := notCheckedRows(result)
	if len(rows) != 1 {
		t.Fatalf("config outside the daemon's directories: rows %+v, want one", rows)
	}
	if !strings.Contains(rows[0].Message, outside) || rows[0].Fix != fixAskActiveConfig {
		t.Errorf("row %+v does not name the file, or its fix is not %q", rows[0], fixAskActiveConfig)
	}
}

// TestDoctorLocalRouteNoCredentialsAnswersLocally is AC-14: a daemon runs,
// but the ask fails (no credentials, refused login). No daemon answered, so
// the local checks answer plus the warning row, whose message says why and
// whose fix is a command that gets the daemon's answer. An error that is not
// the SSH ask's own type still reads as no answer, never as a healthy one.
func TestDoctorLocalRouteNoCredentialsAnswersLocally(t *testing.T) {
	isolateDoctorStore(t)

	noLogin := func(string) (diagnostic.DoctorResult, error) {
		return diagnostic.DoctorResult{}, &daemonUnanswered{
			why: "a daemon listens at 127.0.0.1:2222, but this account has no credentials to log in to it (no stored username)",
			fix: fixAskAsAccount,
		}
	}
	rows := notCheckedRows(localAnswer(t, noLogin))
	if len(rows) != 1 {
		t.Fatalf("rows %+v, want one", rows)
	}
	if !strings.Contains(rows[0].Message, "no credentials to log in") {
		t.Errorf("message %q does not say why the daemon did not answer", rows[0].Message)
	}
	if rows[0].Fix != `ze cli --user <name> -c "show doctor"` {
		t.Errorf("fix = %q, want the command that asks the daemon from an account that can log in", rows[0].Fix)
	}

	refused := func(string) (diagnostic.DoctorResult, error) {
		return diagnostic.DoctorResult{}, errors.New("ssh: unable to authenticate")
	}
	rows = notCheckedRows(localAnswer(t, refused))
	if len(rows) != 1 || !strings.Contains(rows[0].Message, "unable to authenticate") || rows[0].Fix != fixAskAsAccount {
		t.Errorf("a plain ask error: rows %+v, want one naming it with fix %q", rows, fixAskAsAccount)
	}
}

// TestDecodeDaemonDoctorRefusesAForeignAnswer proves an answer that is not a
// doctor result of this schema is no answer: drawing it would show an empty,
// not-ready report the daemon never gave.
func TestDecodeDaemonDoctorRefusesAForeignAnswer(t *testing.T) {
	for _, output := range []string{`table text`, `{"peers":{}}`, `{"schema-version":99,"ready":true,"diagnostics":[]}`} {
		if _, err := decodeDaemonDoctor(output, "127.0.0.1:2222"); err == nil {
			t.Errorf("decodeDaemonDoctor(%q) accepted it", output)
		}
	}
	result, err := decodeDaemonDoctor(`{"schema-version":1,"ready":true,"diagnostics":[{"code":"doctor-x","severity":"warning","message":"m","fix":"ze start"}]}`, "127.0.0.1:2222")
	if err != nil {
		t.Fatalf("a doctor result was refused: %v", err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Fix != "ze start" || !result.Ready {
		t.Errorf("decoded %+v, want the one diagnostic with its fix, ready", result)
	}
}
