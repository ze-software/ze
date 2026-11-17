package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// stage1Subject is the plugin name every case below is judged under.
const stage1Subject = "stage1-table-subject"

// validStage1Declaration is a declaration every isolated check accepts.
func validStage1Declaration() *rpc.DeclareRegistrationInput {
	return &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "Show the answer."}},
	}
}

// stage1Case is one isolated check's failing declaration, and the exact text
// the daemon answered for it before the table existed: the validator's own
// error under the prefix the call site wrote.
type stage1Case struct {
	failing *rpc.DeclareRegistrationInput
	// want builds the expected text from the validator, called directly, so
	// the test states the wrap prefix and never a copy of the validator's
	// wording.
	want func(t *testing.T, input *rpc.DeclareRegistrationInput) string
}

func mustFail(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err, "the failing declaration passes the validator, so the case proves nothing")
	return err.Error()
}

// stage1Cases holds one failing declaration for each check JudgeDeclaration
// runs. TestJudgeDeclarationAgreesWithTheValidators refuses a table entry
// with no case here, so a new isolated check cannot arrive untested.
func stage1Cases() map[string]stage1Case {
	return map[string]stage1Case{
		"protocol-version": {
			failing: &rpc.DeclareRegistrationInput{},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				_, err := rpc.NegotiateProtocolVersion(stage1Subject, in.ProtocolVersion)
				return mustFail(t, err)
			},
		},
		"dissemination": {
			failing: &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion, Dissemination: []rpc.DisseminationDecl{{}}},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, registry.CheckDisseminationDeclarations(stage1Subject, in.Dissemination))
			},
		},
		"filter-declarations": {
			failing: &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion, Filters: []rpc.FilterDecl{{}}},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, registry.CheckFilterDeclarations(stage1Subject, in.Filters))
			},
		},
		"filter-insertions": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion:  rpc.ProtocolVersion,
				FilterInsertions: []rpc.FilterInsertion{{Filter: "stage1-table-subject:not-declared"}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, registry.CheckFilterInsertions(stage1Subject, in.Filters,
					registry.OwnedFilterTypes(stage1Subject), in.FilterInsertions))
			},
		},
		"filter-insertion-paths": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				FilterInsertions: []rpc.FilterInsertion{{
					Filter: "stage1-table-subject:not-declared",
					Config: []string{"no-such-node-in-any-compiled-schema"},
				}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, config.CheckFilterInsertionPaths(stage1Subject,
					registry.OwnedYANG(stage1Subject), in.FilterInsertions))
			},
		},
		"drain": {
			failing: &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion, DrainBudget: -1},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, validateDrainDecl(stage1Subject, in.DrainBudget, in.DrainHandler))
			},
		},
		"resolver-caches": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				ResolverCaches:  []string{"no-such-resolver-cache"},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, validateResolverCacheDecls(stage1Subject, in.ResolverCaches))
			},
		},
		"doctor-checks": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				DoctorChecks:    []rpc.DoctorCheckDecl{{Name: ""}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return "invalid doctor check: " + mustFail(t, validateDoctorCheckDecls(in.DoctorChecks))
			},
		},
		"enrichers": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Enrichers:       []rpc.EnricherDecl{{Command: ""}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return "invalid enricher: " + mustFail(t, validateEnricherDecls(in.Enrichers))
			},
		},
		"pipe-aliases": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Pipes:           []rpc.PipeDecl{{Command: "show never declared", Name: ""}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return "invalid pipe alias: " + mustFail(t, validatePipeDecls(in.Pipes, in.Commands))
			},
		},
		"answer-shapes": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Commands:        []rpc.CommandDecl{{Name: "show query answer", Shape: "no-such-shape"}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return "invalid answer shape: " + stage1Subject + ": " + mustFail(t, validateShapeDecls(in.Commands))
			},
		},
		"help-texts": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "a tab\tbreaks completion"}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return "invalid help text: " + stage1Subject + ": " + mustFail(t, validateHelpDecls(in.Commands))
			},
		},
		"command-names": {
			failing: &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Commands:        []rpc.CommandDecl{{Name: "Show Upper Case"}},
			},
			want: func(t *testing.T, in *rpc.DeclareRegistrationInput) string {
				return mustFail(t, validateCommandName(in.Commands[0].Name)) +
					": the daemon keeps the plugin, logs a warning and refuses this command at Stage 5"
			},
		},
	}
}

func verdictNamed(t *testing.T, judgement plugin.DeclarationJudgement, check string) plugin.DeclarationVerdict {
	t.Helper()
	for _, verdict := range judgement.Verdicts {
		if verdict.Check == check {
			return verdict
		}
	}
	t.Fatalf("JudgeDeclaration answered no verdict for the check %q", check)
	return plugin.DeclarationVerdict{}
}

// VALIDATES: every table entry is complete: a unique kebab-case name, a
// section, a phase, and exactly one way of being judged.
// PREVENTS: an entry the daemon skips in silence because it names no phase,
// or one a checker reports with no reason for not judging it.
func TestStage1TableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for i := range stage1Checks {
		check := &stage1Checks[i]
		require.NotEmpty(t, check.name, "entry %d has no name", i)
		assert.True(t, isLowerKebab(check.name), "%s: the name is not kebab-case", check.name)
		assert.False(t, seen[check.name], "%s: the name is declared twice", check.name)
		seen[check.name] = true
		assert.NotEmpty(t, check.section, "%s: no protocol.md section", check.name)
		assert.NotEqual(t, stage1PhaseUnspecified, check.phase, "%s: no phase", check.name)

		ways := 0
		if check.isolated != nil {
			ways++
		}
		if check.live != nil {
			ways++
		}
		if check.phase == stage1PhaseInline {
			ways++
		}
		assert.Equal(t, 1, ways, "%s: want exactly one of isolated, live and the inline phase", check.name)
		if check.isolated == nil {
			assert.NotEmpty(t, check.daemonState, "%s: a check nobody judges in isolation says which daemon state it needs", check.name)
		}
		if check.live != nil {
			assert.Equal(t, stage1PhaseSink, check.phase, "%s: only the sink runs a live check", check.name)
		}
	}
}

// VALIDATES: for each check JudgeDeclaration runs, a valid declaration passes
// and a failing one answers the validator's own error under the prefix the
// daemon's call site carried.
// PREVENTS: the table changing a refusal text, and an isolated check with no
// failing case.
func TestJudgeDeclarationAgreesWithTheValidators(t *testing.T) {
	cases := stage1Cases()
	passing := JudgeDeclaration(stage1Subject, validStage1Declaration())

	for i := range stage1Checks {
		check := &stage1Checks[i]
		if check.isolated == nil {
			continue
		}
		t.Run(check.name, func(t *testing.T) {
			tc, covered := cases[check.name]
			require.True(t, covered, "the check %q has no failing case in stage1Cases", check.name)

			pass := verdictNamed(t, passing, check.name)
			require.NoError(t, pass.Err, "the valid declaration fails this check")
			assert.Equal(t, check.section, pass.Section)

			fail := verdictNamed(t, JudgeDeclaration(stage1Subject, tc.failing), check.name)
			require.Error(t, fail.Err)
			assert.Equal(t, tc.want(t, tc.failing), fail.Err.Error())
			assert.Equal(t, check.phase == stage1PhaseReady, fail.KeepsPlugin)
		})
	}
}

// VALIDATES: JudgeDeclaration reports every failed check of one declaration,
// in the daemon's order.
// PREVENTS: a developer fixing one refusal per run where the report could
// name them all.
func TestJudgeDeclarationCollectsEveryFailure(t *testing.T) {
	judgement := JudgeDeclaration(stage1Subject, &rpc.DeclareRegistrationInput{
		DrainBudget:    -1,
		ResolverCaches: []string{"no-such-resolver-cache"},
	})

	var failed []string
	for _, verdict := range judgement.Verdicts {
		if verdict.Err != nil {
			failed = append(failed, verdict.Check)
		}
	}
	assert.Equal(t, []string{"protocol-version", "drain", "resolver-caches"}, failed)

	// The daemon answers the first one, and so does Refusal.
	_, versionErr := rpc.NegotiateProtocolVersion(stage1Subject, 0)
	require.Error(t, versionErr)
	require.Error(t, judgement.Refusal())
	assert.Equal(t, versionErr.Error(), judgement.Refusal().Error())
}

// VALIDATES: the checks that need daemon state come back unjudged, each with
// the state it needs, and never as a passed verdict.
// PREVENTS: a report with no failure read as full Stage 1 conformance.
func TestJudgeDeclarationNamesTheChecksItCannotJudge(t *testing.T) {
	judgement := JudgeDeclaration(stage1Subject, validStage1Declaration())

	var unjudged []string
	for _, check := range judgement.Unjudged {
		unjudged = append(unjudged, check.Check)
		assert.NotEmpty(t, check.Reason, "%s: no reason", check.Check)
		assert.NotEmpty(t, check.Section, "%s: no section", check.Check)
	}
	assert.Equal(t, []string{
		"ambiguous-registration", "doctor-check-declared-twice", "respawn-agreement",
		"schema-unchanged", "dependencies", "registration-conflicts",
	}, unjudged)
	assert.Len(t, judgement.Verdicts, len(stage1Checks)-len(unjudged))
}

// VALIDATES: the server fills the seam of package plugin from init(), with
// this package's judge.
// PREVENTS: `show plugin declarations` reporting "not-judged" in the ze binary.
func TestTheServerFillsTheDeclarationJudgeSeam(t *testing.T) {
	judgement, err := plugin.JudgeDeclaration(stage1Subject, &rpc.DeclareRegistrationInput{})
	require.NoError(t, err)
	require.Error(t, judgement.Refusal())
	assert.Len(t, judgement.Verdicts, len(JudgeDeclaration(stage1Subject, &rpc.DeclareRegistrationInput{}).Verdicts))
}

// VALIDATES: onRegistration refuses each sink-phase declaration with the text
// it answered before the table existed, and answers the first failed check
// when two fail.
// PREVENTS: the table changing what a plugin reads when the daemon refuses it.
func TestOnRegistrationRefusesWithTheSameText(t *testing.T) {
	cases := stage1Cases()
	sinkChecks := []string{"drain", "resolver-caches", "doctor-checks", "enrichers", "pipe-aliases", "answer-shapes", "help-texts"}

	s, _ := newLifecycleStartupServer(t)
	newSink := func() *engineStartupSink {
		proc := process.NewProcess(plugin.PluginConfig{Name: stage1Subject, Internal: true})
		return &engineStartupSink{s: s, proc: proc}
	}

	for _, name := range sinkChecks {
		t.Run(name, func(t *testing.T) {
			tc, covered := cases[name]
			require.True(t, covered)

			err := newSink().onRegistration(tc.failing)
			require.Error(t, err)
			assert.Equal(t, tc.want(t, tc.failing), err.Error())
		})
	}

	t.Run("the first failed check answers", func(t *testing.T) {
		both := &rpc.DeclareRegistrationInput{
			ProtocolVersion: rpc.ProtocolVersion,
			DrainBudget:     -1,
			Commands:        []rpc.CommandDecl{{Name: "show query answer", ShortHelp: "a tab\tbreaks completion"}},
		}
		err := newSink().onRegistration(both)
		require.Error(t, err)
		assert.Equal(t, mustFail(t, validateDrainDecl(stage1Subject, both.DrainBudget, both.DrainHandler)), err.Error())
	})
}

// VALIDATES: the driver-phase checks name the plugin the way the handshake
// did: the protocol version by the operator's block, the filters by the
// registration the process runs.
// PREVENTS: a filter refusal naming the operator's block, which no filter
// reference carries.
func TestRefuseDriverChecksNamesEachCheckAsTheHandshakeDid(t *testing.T) {
	const block, registration = "operator-block", "running-registration"

	versionErr := refuseDriverChecks(block, registration, &rpc.DeclareRegistrationInput{})
	_, want := rpc.NegotiateProtocolVersion(block, 0)
	require.Error(t, want)
	require.Error(t, versionErr)
	assert.Equal(t, want.Error(), versionErr.Error())

	insertions := []rpc.FilterInsertion{{Filter: "running-registration:not-declared"}}
	insertionErr := refuseDriverChecks(block, registration, &rpc.DeclareRegistrationInput{
		ProtocolVersion:  rpc.ProtocolVersion,
		FilterInsertions: insertions,
	})
	wantInsertion := registry.CheckFilterInsertions(registration, nil,
		registry.OwnedFilterTypes(registration), insertions)
	require.Error(t, wantInsertion)
	require.Error(t, insertionErr)
	assert.Equal(t, wantInsertion.Error(), insertionErr.Error())

	require.NoError(t, refuseDriverChecks(block, registration, validStage1Declaration()))
}
