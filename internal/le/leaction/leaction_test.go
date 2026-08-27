// Related: leaction.go -- the area dispatch these tests drive from its entry point

package leaction

import (
	"strings"
	"testing"
)

// fixture builds a two-action area of the shape every codegen tool declares:
// one action a Make target names, and one that writes and has no target.
func fixture(t *testing.T) (Area, *int) {
	t.Helper()

	ran := 0
	area := New("web-assets",
		Action{
			Gate:   "ze-web-assets-check",
			Why:    "the generated file agrees with the markup",
			Answer: func() (any, int) { ran++; return "checked", 0 },
		},
		Action{
			Verb:   "write",
			Why:    "regenerate it",
			Writes: true,
			Answer: func() (any, int) { ran++; return "written", 3 },
		},
	)

	return area, &ran
}

// VALIDATES: the verb of an action a Make target names is DERIVED from that
// target, by removing the area's own prefix.
// PREVENTS: a verb typed beside its gate name, which is where the two come to
// disagree about what a developer types.
func TestVerbIsDerivedFromTheGateName(t *testing.T) {
	area, _ := fixture(t)

	rows := area.Actions().Actions
	if len(rows) != 2 {
		t.Fatalf("the area answers %d actions, want 2", len(rows))
	}
	if rows[0].Verb != "check" {
		t.Errorf("ze-web-assets-check derives the verb %q, want %q", rows[0].Verb, "check")
	}
	if rows[1].Verb != "write" {
		t.Errorf("the action with no gate answers the verb %q, want %q", rows[1].Verb, "write")
	}
	if rows[1].Gate != "" {
		t.Errorf("the action with no gate reports the gate %q, want none", rows[1].Gate)
	}
}

// VALIDATES: Gates answers the Make target of every action that has one.
// PREVENTS: a parity claim typed by hand beside the action table, which is how a
// gate comes to be counted as ported by a command that does not run it.
func TestGatesAnswersTheTargetsTheTableNames(t *testing.T) {
	area, _ := fixture(t)

	gates := area.Gates()
	if len(gates) != 1 || gates[0] != "ze-web-assets-check" {
		t.Fatalf("the area answers the gates %v, want [ze-web-assets-check]", gates)
	}
}

// VALIDATES: a bare area answers its own listing, and naming an action runs it.
// PREVENTS: a command that does nothing when a developer types it with no
// action, which is the first thing anybody types.
func TestBareAreaListsAndNamedActionRuns(t *testing.T) {
	area, ran := fixture(t)

	payload, code := area.Answer(nil)
	if code != 0 {
		t.Errorf("the bare area answers %d, want 0", code)
	}
	if _, ok := payload.(List); !ok {
		t.Errorf("the bare area answers %T, want a List", payload)
	}
	if *ran != 0 {
		t.Errorf("the bare area ran %d actions, want 0", *ran)
	}

	payload, code = area.Answer([]string{"write"})
	if code != 3 {
		t.Errorf("write answers %d, want its own 3", code)
	}
	if payload != "written" {
		t.Errorf("write answers %v, want the action's own payload", payload)
	}
	if *ran != 1 {
		t.Errorf("write ran %d actions, want 1", *ran)
	}
}

// VALIDATES: an unknown action answers 2, and a value after an action that takes
// none answers 1.
// PREVENTS: a flattened 1 for both. commit_helper.py reads the codes apart, so a
// mistyped verb and a gate that ran and failed must not answer the same thing.
func TestRefusalsAnswerDifferentCodes(t *testing.T) {
	area, ran := fixture(t)

	if _, code := area.Answer([]string{"nosuch"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := area.Answer([]string{"check", "extra"}); code != 1 {
		t.Errorf("a value after an action that takes none answers %d, want 1", code)
	}
	if *ran != 0 {
		t.Errorf("a refused invocation ran %d actions, want 0", *ran)
	}
}

// VALIDATES: the listing and the help line are derived from one table, and both
// say which action writes.
// PREVENTS: a hand-written help line that stops naming an action, or stops
// marking the one that rewrites the tree.
func TestListingAndSubsAgreeAboutWhatWrites(t *testing.T) {
	area, _ := fixture(t)

	text := area.Actions().Text()
	if !strings.Contains(text, "check  checks") {
		t.Errorf("the listing does not mark check as read-only:\n%s", text)
	}
	if !strings.Contains(text, "write  writes") {
		t.Errorf("the listing does not mark write as writing:\n%s", text)
	}
	if !strings.HasPrefix(text, "web-assets:\n") {
		t.Errorf("the listing does not open with the area name:\n%s", text)
	}

	if subs := area.Subs(); subs != "check | write (writes)" {
		t.Errorf("the help line is %q, want %q", subs, "check | write (writes)")
	}
}

// VALIDATES: a table that could not be dispatched is refused at init.
// PREVENTS: an action nobody can type, or two actions sharing one word so that
// the second is unreachable.
func TestNewRefusesATableItCouldNotDispatch(t *testing.T) {
	answer := func() (any, int) { return nil, 0 }
	answerArgs := func(Arguments) (any, int) { return nil, 0 }

	cases := []struct {
		name    string
		actions []Action
	}{
		{"no gate and no verb", []Action{{Why: "why", Answer: answer}}},
		{"gate and verb both", []Action{{Gate: "ze-a-check", Verb: "check", Why: "why", Answer: answer}}},
		{"no answer", []Action{{Verb: "check", Why: "why"}}},
		{"no why", []Action{{Verb: "check", Answer: answer}}},
		{"both answer forms", []Action{{
			Verb: "check", Why: "why", Parameters: []Parameter{{Keyword: "name", Value: "name"}},
			Answer: answer, AnswerArgs: answerArgs,
		}}},
		{"zero-argument answer with parameters", []Action{{
			Verb: "check", Why: "why", Parameters: []Parameter{{Keyword: "name", Value: "name"}},
			Answer: answer,
		}}},
		{"argument answer with no parameters", []Action{{Verb: "check", Why: "why", AnswerArgs: answerArgs}}},
		{"duplicate parameter", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "name"}, {Keyword: "name"}}, AnswerArgs: answerArgs,
		}}},
		{"two actions one verb", []Action{
			{Gate: "ze-a-check", Why: "why", Answer: answer},
			{Verb: "check", Why: "why", Answer: answer},
		}},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("New accepted a table it could not dispatch")
				}
			}()
			New("a", one.actions...)
		})
	}
}
// VALIDATES: argument-aware actions accept only declared keywords, consume a
// value after its keyword, and keep boolean switches as presence.
// PREVENTS: a free-form positional value, a missing keyword value, or a second
// value that the handler could misread as another option.
func TestArgumentAwareActionValidatesItsClosedGrammar(t *testing.T) {
	var got Arguments
	area := New("qemu", Action{
		Verb: "run",
		Why:  "boot a guest",
		Parameters: []Parameter{
			{Keyword: "command", Value: "command"},
			{Keyword: "timeout", Value: "duration"},
			{Keyword: "keep-alive"},
		},
		AnswerArgs: func(args Arguments) (any, int) {
			got = args
			return args, 7
		},
	})

	payload, code := area.Answer([]string{
		"run", "command", "go test ./...", "timeout", "30s", "keep-alive",
	})
	if code != 7 {
		t.Fatalf("argument-aware action code = %d, want 7", code)
	}
	if payload == nil || got["command"] != "go test ./..." || got["timeout"] != "30s" {
		t.Fatalf("argument-aware action got %#v", got)
	}
	if !got.Has("keep-alive") {
		t.Fatalf("argument-aware action lost the boolean keyword: %#v", got)
	}

	refused := [][]string{
		{"run", "unknown"},
		{"run", "command"},
		{"run", "command", "true", "extra"},
		{"run", "keep-alive", "keep-alive"},
	}
	for _, invocation := range refused {
		got = nil
		if _, refusedCode := area.Answer(invocation); refusedCode != 1 {
			t.Errorf("%v answers %d, want 1", invocation, refusedCode)
		}
		if got != nil {
			t.Errorf("%v reached the handler with %#v", invocation, got)
		}
	}
}


// ─── The sweep: several actions on one command line ─────────────────────────

// sweepArea builds an area whose actions return the specified codes. A test
// then drives the exit-code rule. It does not start a gate.
func sweepArea(codes ...int) Area {
	rows := make([]Action, 0, len(codes))
	for i, code := range codes {
		verb := string(rune('a' + i))
		rows = append(rows, Action{
			Verb: verb, Why: "a probe", Answer: probeAnswer(verb, code),
		})
	}
	return New("probe", rows...)
}

func probeAnswer(verb string, code int) func() (any, int) {
	return func() (any, int) { return map[string]any{"verb": verb}, code }
}

// TestFirstFailingGateExitCodeWins is AC-8. `commit_helper.py` reads 3 apart
// from 1, so a sweep that answered 1 for every failure would break it.
func TestFirstFailingGateExitCodeWins(t *testing.T) {
	area := sweepArea(0, 3, 1)
	for _, policy := range []SweepPolicy{StopAtFirstFailure, RunEveryAction} {
		_, code := area.Sweep([]string{"a", "b", "c"}, policy)
		if code != 3 {
			t.Errorf("policy %v answered %d, want the first failing action's own 3", policy, code)
		}
	}
}

// TestStopAtFirstFailureRunsNoActionBehindTheRed tests the functional-area rule.
// If a scan runs after its selftest fails, it reports findings from a checker
// that the selftest has shown to be broken.
func TestStopAtFirstFailureRunsNoActionBehindTheRed(t *testing.T) {
	area := sweepArea(2, 0)
	answer, code := area.Sweep([]string{"a", "b"}, StopAtFirstFailure)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	sweep, ok := answer.(Sweep)
	if !ok {
		t.Fatalf("Sweep answered %T, want a Sweep payload", answer)
	}
	if len(sweep.Ran) != 1 {
		t.Errorf("the sweep ran %d actions, want 1: %v", len(sweep.Ran), sweep.Ran)
	}
}

// TestRunEveryActionReportsEveryFailureByName is the integration area's rule:
// a sweep exists to hand back the whole list.
func TestRunEveryActionReportsEveryFailureByName(t *testing.T) {
	area := sweepArea(4, 0, 5)
	answer, code := area.Sweep([]string{"a", "b", "c"}, RunEveryAction)
	if code != 4 {
		t.Fatalf("code = %d, want the first failing action's own 4", code)
	}
	sweep, ok := answer.(Sweep)
	if !ok {
		t.Fatalf("Sweep answered %T, want a Sweep payload", answer)
	}
	if len(sweep.Ran) != 3 {
		t.Errorf("the sweep ran %d actions, want 3", len(sweep.Ran))
	}
	if len(sweep.Failed) != 2 {
		t.Errorf("the sweep named %v as failed, want both a and c", sweep.Failed)
	}
}

// TestSweepRefusesAnActionTheAreaDoesNotHold keeps a mistyped name apart from a
// gate that ran and failed, which is what code 2 says.
func TestSweepRefusesAnActionTheAreaDoesNotHold(t *testing.T) {
	area := sweepArea(0)
	answer, code := area.Sweep([]string{"a", "nope"}, RunEveryAction)
	if code != 2 {
		t.Errorf("a mistyped action answered %d, want 2", code)
	}
	if answer != nil {
		t.Errorf("a refused sweep answered a payload: %v", answer)
	}
}

// TestSweepRefusesBeforeItRunsAnything pins that the whole selection is
// resolved first. Running the good half of a mistyped command line leaves the
// tree half-swept and the caller reading a refusal.
func TestSweepRefusesBeforeItRunsAnything(t *testing.T) {
	ran := false
	area := New("probe",
		Action{Verb: "a", Why: "a probe", Answer: func() (any, int) { ran = true; return nil, 0 }},
	)
	if _, code := area.Sweep([]string{"a", "nope"}, RunEveryAction); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if ran {
		t.Error("the sweep ran an action before it had resolved every name on the line")
	}
}

// TestSweepCarriesEachActionsOwnAnswer is AC-7 for a sweep: the payload is the
// data, so `| json` over `le functional a b` carries both answers.
func TestSweepCarriesEachActionsOwnAnswer(t *testing.T) {
	area := sweepArea(0, 0)
	answer, _ := area.Sweep([]string{"a", "b"}, RunEveryAction)
	sweep, ok := answer.(Sweep)
	if !ok {
		t.Fatalf("Sweep answered %T, want a Sweep payload", answer)
	}
	for i, row := range sweep.Ran {
		got, ok := row.Answer.(map[string]any)
		if !ok {
			t.Fatalf("row %d carried %T, want the action's own payload", i, row.Answer)
		}
		if got["verb"] != row.Verb {
			t.Errorf("row %d carries %v, want the answer of %q", i, got, row.Verb)
		}
	}
}
