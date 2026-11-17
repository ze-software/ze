package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// This package cannot import internal/component/plugin/server, which holds the
// real Stage 1 judge and imports this package. Its test binary therefore links
// no judge, and every test that forks a plugin and expects a declared row
// would read "not-judged". The init below registers a judge that applies the
// one rule those tests state: the protocol version. The real table is tested
// in server/stage1_checks_test.go and through the ze binary in
// test/plugin/plugin-declarations-stage1-refusal.ci.
func init() {
	SetDeclarationJudge(versionJudge)
}

func versionJudge(name string, declaration *rpc.DeclareRegistrationInput) DeclarationJudgement {
	_, err := rpc.NegotiateProtocolVersion(name, declaration.ProtocolVersion)
	return DeclarationJudgement{Verdicts: []DeclarationVerdict{{Check: "protocol-version", Err: err}}}
}

// withDeclarationJudge swaps the registered judge for one test and restores
// the previous one. A nil judge empties the seam, which SetDeclarationJudge
// refuses to do, so the swap writes the variable under its lock.
//
// The tests that call it MUST NOT run in parallel: the seam is package state.
func withDeclarationJudge(t *testing.T, judge DeclarationJudge) {
	t.Helper()

	declarationJudgeMu.Lock()
	previous := declarationJudge
	declarationJudge = judge
	declarationJudgeMu.Unlock()

	t.Cleanup(func() {
		declarationJudgeMu.Lock()
		declarationJudge = previous
		declarationJudgeMu.Unlock()
	})
}

// VALIDATES: a filled seam answers the registered judge's own judgement.
// PREVENTS: a caller reading a judgement the judge did not give.
func TestJudgeDeclarationAnswersTheRegisteredJudge(t *testing.T) {
	refusal := errors.New("refused by the test judge")
	var judgedName string
	withDeclarationJudge(t, func(name string, _ *rpc.DeclareRegistrationInput) DeclarationJudgement {
		judgedName = name
		return DeclarationJudgement{
			Verdicts: []DeclarationVerdict{{Check: "first"}, {Check: "second", Err: refusal}},
			Unjudged: []UnjudgedCheck{{Check: "third", Reason: "needs the daemon"}},
		}
	})

	judgement, err := JudgeDeclaration("seam-filled", &rpc.DeclareRegistrationInput{})
	if err != nil {
		t.Fatalf("a filled seam answered an error: %v", err)
	}
	if judgedName != "seam-filled" {
		t.Fatalf("the judge was called for %q, want seam-filled", judgedName)
	}
	if len(judgement.Verdicts) != 2 {
		t.Fatalf("got %d verdicts, want the judge's 2", len(judgement.Verdicts))
	}
	if len(judgement.Unjudged) != 1 {
		t.Fatalf("got %d unjudged checks, want the judge's 1", len(judgement.Unjudged))
	}
	if !errors.Is(judgement.Refusal(), refusal) {
		t.Fatalf("Refusal() = %v, want the failed verdict's error", judgement.Refusal())
	}
}

// VALIDATES: an empty seam is a refusal, never an empty judgement that reads
// as a pass.
// PREVENTS: a binary that links no check table reporting every declaration
// as accepted.
func TestJudgeDeclarationRefusesWithNoJudge(t *testing.T) {
	withDeclarationJudge(t, nil)

	_, err := JudgeDeclaration("seam-empty", &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion})
	if !errors.Is(err, ErrNoDeclarationJudge) {
		t.Fatalf("an empty seam answered %v, want ErrNoDeclarationJudge", err)
	}
}

// VALIDATES: SetDeclarationJudge(nil) leaves the registered judge in place.
// PREVENTS: a nil registration emptying the seam for the whole process.
func TestSetDeclarationJudgeIgnoresNil(t *testing.T) {
	withDeclarationJudge(t, versionJudge)

	SetDeclarationJudge(nil)

	if _, err := JudgeDeclaration("still-filled", &rpc.DeclareRegistrationInput{}); err != nil {
		t.Fatalf("a nil registration emptied the seam: %v", err)
	}
}

// VALIDATES: Refusal answers the first failed verdict that refuses the
// plugin, and skips a failed verdict the daemon keeps the plugin for.
// PREVENTS: a Stage 5 command-name warning reported as a refused plugin.
func TestDeclarationJudgementRefusal(t *testing.T) {
	kept := errors.New("a command name the daemon refuses and keeps the plugin for")
	first := errors.New("the first Stage 1 refusal")
	second := errors.New("the second Stage 1 refusal")

	onlyKept := DeclarationJudgement{Verdicts: []DeclarationVerdict{
		{Check: "pass"},
		{Check: "command-names", KeepsPlugin: true, Err: kept},
	}}
	if err := onlyKept.Refusal(); err != nil {
		t.Fatalf("a judgement whose only failure keeps the plugin answered a refusal: %v", err)
	}

	refused := DeclarationJudgement{Verdicts: []DeclarationVerdict{
		{Check: "command-names", KeepsPlugin: true, Err: kept},
		{Check: "one", Err: first},
		{Check: "two", Err: second},
	}}
	if err := refused.Refusal(); !errors.Is(err, first) {
		t.Fatalf("Refusal() = %v, want the first verdict that refuses the plugin", err)
	}
}

// VALIDATES: a declaration the daemon refuses at Stage 1 is a row in state
// unstartable carrying the judge's reason.
// PREVENTS: `show plugin declarations` listing as declared a plugin the daemon
// would refuse to start.
func TestQueriedDeclarationRowReportsAStage1RefusalAsUnstartable(t *testing.T) {
	refusal := errors.New("invalid help text: stage1-refused: a tab is not allowed")
	withDeclarationJudge(t, func(string, *rpc.DeclareRegistrationInput) DeclarationJudgement {
		return DeclarationJudgement{Verdicts: []DeclarationVerdict{{Check: "help-texts", Err: refusal}}}
	})

	declaration := rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show stage1 refused"}},
	}
	row := queriedDeclarationRow(context.Background(), PluginConfig{
		Name: "stage1-refused",
		Run:  queryRunString(t, "", &declaration),
	}, 5*time.Second)

	if row.State != stateUnstartable {
		t.Fatalf("state = %q, want %q; reason %q", row.State, stateUnstartable, row.Reason)
	}
	if row.Reason != refusal.Error() {
		t.Fatalf("reason = %q, want the judge's reason %q", row.Reason, refusal.Error())
	}
	if len(row.Commands) != 0 {
		t.Fatalf("a refused declaration still listed %d commands", len(row.Commands))
	}
}

// VALIDATES: a failed check that keeps the plugin leaves the row declared.
// PREVENTS: a command the daemon refuses at Stage 5 with a warning making the
// whole plugin read as unstartable.
func TestQueriedDeclarationRowKeepsAPluginTheDaemonKeeps(t *testing.T) {
	withDeclarationJudge(t, func(string, *rpc.DeclareRegistrationInput) DeclarationJudgement {
		return DeclarationJudgement{Verdicts: []DeclarationVerdict{
			{Check: "command-names", KeepsPlugin: true, Err: errors.New("bad name")},
		}}
	})

	declaration := rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show stage5 kept"}},
	}
	row := queriedDeclarationRow(context.Background(), PluginConfig{
		Name: "stage5-kept",
		Run:  queryRunString(t, "", &declaration),
	}, 5*time.Second)

	if row.State != stateDeclared {
		t.Fatalf("state = %q, want %q; reason %q", row.State, stateDeclared, row.Reason)
	}
}

// VALIDATES: with no judge linked, a declaration that was read is a row in
// state not-judged that says why.
// PREVENTS: an unjudged declaration listed as declared, and a nil seam
// reaching a call.
func TestQueriedDeclarationRowNeverListsAnUnjudgedDeclarationAsDeclared(t *testing.T) {
	withDeclarationJudge(t, nil)

	declaration := rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show never judged"}},
	}
	row := queriedDeclarationRow(context.Background(), PluginConfig{
		Name: "never-judged",
		Run:  queryRunString(t, "", &declaration),
	}, 5*time.Second)

	if row.State != stateUnjudged {
		t.Fatalf("state = %q, want %q", row.State, stateUnjudged)
	}
	if !strings.Contains(row.Reason, ErrNoDeclarationJudge.Error()) {
		t.Fatalf("reason %q does not say why the declaration was not judged", row.Reason)
	}
}

// VALIDATES: QueryDeclaration names how each run ended, with the status the
// program exited with, and still reads a declaration a failing program wrote.
// PREVENTS: a checker unable to tell a clean exit from a failed one, a
// timeout, a run string that started nothing, or a descendant left running.
func TestQueryDeclarationNamesHowTheRunEnded(t *testing.T) {
	declaration := rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion}
	answers := queryRunString(t, "", &declaration)

	cases := []struct {
		name     string
		run      string
		budget   time.Duration
		exit     QueryExit
		code     int
		declared bool
	}{
		{name: "clean", run: answers, budget: 5 * time.Second, exit: QueryExitClean, declared: true},
		{name: "failed", run: answers + "; exit 3", budget: 5 * time.Second, exit: QueryExitFailed, code: 3, declared: true},
		{name: "silent-failed", run: "exit 4", budget: 5 * time.Second, exit: QueryExitFailed, code: 4},
		{
			name: "not-started", run: "/nonexistent/ze-conformance-no-such-program",
			budget: 5 * time.Second, exit: QueryExitNotStarted, code: exitCommandNotFound,
		},
		// The program answers and then stays, so the budget ends it.
		{name: "timeout", run: answers + "; exec sleep 30", budget: 300 * time.Millisecond, exit: QueryExitTimeout, declared: true},
		// The shell exits 0 while the sleep it started still holds stdout.
		{name: "outlived", run: answers + "; sleep 2 & exit 0", budget: 5 * time.Second, exit: QueryExitOutlived, declared: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query := QueryDeclaration(context.Background(), PluginConfig{Name: "query-" + tc.name, Run: tc.run}, tc.budget)

			if query.Exit != tc.exit {
				t.Fatalf("exit = %d, want %d; cause %q", query.Exit, tc.exit, query.Cause)
			}
			if query.ExitCode != tc.code {
				t.Fatalf("exit code = %d, want %d", query.ExitCode, tc.code)
			}
			if query.Declared != tc.declared {
				t.Fatalf("declared = %v, want %v; stdout %q", query.Declared, tc.declared, query.Stdout)
			}
			if tc.exit != QueryExitClean && query.Cause == "" {
				t.Fatalf("a run that did not end clean carries no cause")
			}
		})
	}
}
