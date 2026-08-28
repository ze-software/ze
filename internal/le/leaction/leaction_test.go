// Related: leaction.go -- the area dispatch these tests drive from its entry point

package leaction

import (
	"strings"
	"testing"
)

// fixture builds a two-action area.
func fixture(t *testing.T) (Area, *int) {
	t.Helper()

	ran := 0
	area := New("web-assets",
		Action{
			Verb:   "check",
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

// VALIDATES: actions expose exactly the verbs declared in their table.
// PREVENTS: implicit identity derivation from a removed build layer.
func TestActionsUseDeclaredVerbs(t *testing.T) {
	area, _ := fixture(t)
	rows := area.Actions().Actions
	if len(rows) != 2 {
		t.Fatalf("the area answers %d actions, want 2", len(rows))
	}
	if rows[0].Verb != "check" || rows[1].Verb != "write" {
		t.Errorf("action verbs = %q, want [check write]", []string{rows[0].Verb, rows[1].Verb})
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

// VALIDATES: unknown actions and malformed action arguments both answer 2.
// PREVENTS: a usage error looking like a check that ran and found a defect.
func TestRefusalsAnswerUsageCode(t *testing.T) {
	area, ran := fixture(t)

	if _, code := area.Answer([]string{"nosuch"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := area.Answer([]string{"check", "extra"}); code != 2 {
		t.Errorf("a value after a zero-argument action answers %d, want 2", code)
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
		{"no verb", []Action{{Why: "why", Answer: answer}}},
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
			{Verb: "check", Why: "first", Answer: answer},
			{Verb: "check", Why: "second", Answer: answer},
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
		if _, refusedCode := area.Answer(invocation); refusedCode != 2 {
			t.Errorf("%v answers %d, want 2", invocation, refusedCode)
		}
		if got != nil {
			t.Errorf("%v reached the handler with %#v", invocation, got)
		}
	}
}

// ─── The sweep: several actions on one command line ─────────────────────────

// sweepArea builds an area whose actions return the specified codes. A test
// then drives the exit-code rule.
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

// TestFirstFailingGateExitCodeWins is AC-8. Native commit preparation reads 3
// apart from 1, so a sweep that answered 1 for every failure would break it.
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

// TestTakesArgumentsSeparatesGrammarFromActionNames pins the answer an area
// asks before it sweeps: the words after an argument-aware action are its
// values, so `render name term` is one action, never three.
func TestTakesArgumentsSeparatesGrammarFromActionNames(t *testing.T) {
	area := New("probe",
		Action{Verb: "plain", Why: "a probe", Answer: func() (any, int) { return nil, 0 }},
		Action{
			Verb: "typed", Why: "a probe that selects one member",
			Parameters: []Parameter{{Keyword: "name", Value: "id"}},
			AnswerArgs: func(Arguments) (any, int) { return nil, 0 },
		},
	)

	if !area.TakesArguments("typed") {
		t.Error("an action declaring a keyword grammar reports that it takes no arguments")
	}
	if area.TakesArguments("plain") {
		t.Error("a zero-argument action reports that it takes arguments")
	}
	if area.TakesArguments("absent") {
		t.Error("an action the area does not hold reports that it takes arguments")
	}
}

// TestSweepRefusesAnArgumentAwareAction fails closed on the one shape a sweep
// cannot run. A sweep calls each action with no arguments, and an
// argument-aware action holds no such entry point, so running it would be a
// call through a nil function rather than a refusal a caller can read.
func TestSweepRefusesAnArgumentAwareAction(t *testing.T) {
	ran := false
	area := New("probe",
		Action{Verb: "plain", Why: "a probe", Answer: func() (any, int) { ran = true; return nil, 0 }},
		Action{
			Verb: "typed", Why: "a probe that selects one member",
			Parameters: []Parameter{{Keyword: "name", Value: "id"}},
			AnswerArgs: func(Arguments) (any, int) { return nil, 0 },
		},
	)

	answer, code := area.Sweep([]string{"plain", "typed"}, RunEveryAction)
	if code != 2 {
		t.Errorf("a swept argument-aware action answered %d, want 2", code)
	}
	if answer != nil {
		t.Errorf("a refused sweep answered a payload: %v", answer)
	}
	if ran {
		t.Error("the sweep ran an action before it refused the selection")
	}
}
