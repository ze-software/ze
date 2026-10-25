// Related: leaction.go -- the area dispatch these tests drive from its entry point

package leaction

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
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
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "name", Value: "name", Requirement: Required, Kind: KindString, Help: "h"}},
			Answer:     answer, AnswerArgs: answerArgs,
		}}},
		{"zero-argument answer with parameters", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "name", Value: "name", Requirement: Required, Kind: KindString, Help: "h"}},
			Answer:     answer,
		}}},
		{"argument answer with no parameters", []Action{{Verb: "check", Why: "why", AnswerArgs: answerArgs}}},
		{"duplicate parameter", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "name", Kind: KindSwitch, Help: "h"}, {Keyword: "name", Kind: KindSwitch, Help: "h"}}, AnswerArgs: answerArgs,
		}}},
		{"required switch", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "force", Requirement: Required, Kind: KindSwitch, Help: "h"}}, AnswerArgs: answerArgs,
		}}},
		{"repeating switch", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "force", Repeat: true, Kind: KindSwitch, Help: "h"}}, AnswerArgs: answerArgs,
		}}},
		{"value parameter with no requirement", []Action{{
			Verb: "check", Why: "why",
			Parameters: []Parameter{{Keyword: "file", Value: "path", Kind: KindString, Help: "h"}}, AnswerArgs: answerArgs,
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
			{Keyword: "command", Value: "command", Requirement: Required, Kind: KindString, Help: "h"},
			{Keyword: "timeout", Value: "duration", Requirement: Optional, Kind: KindString, Help: "h"},
			{Keyword: "keep-alive", Kind: KindSwitch, Help: "h"},
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
	if payload == nil || got.One("command") != "go test ./..." || got.One("timeout") != "30s" {
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
// data, so `| json` over `le test functional a b` carries both answers.
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
			Parameters: []Parameter{{Keyword: "name", Value: "id", Requirement: Required, Kind: KindString, Help: "h"}},
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
			Parameters: []Parameter{{Keyword: "name", Value: "id", Requirement: Required, Kind: KindString, Help: "h"}},
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

// VALIDATES: AC-9 -- an action that names the whole area runs on its own, so a
// line naming it beside another verb is refused and nothing runs.
// PREVENTS: `le test functional gating encode` running the 24 gating suites and then
// one more, where the same two words refused before the three run-list verbs
// became rows of the table. Two words of typing are not an instruction to start
// hours of work on a shared machine.
func TestSweepRefusesAnActionThatNamesTheWholeArea(t *testing.T) {
	ran := 0
	area := New("probe",
		Action{Verb: "one", Why: "a probe", Answer: func() (any, int) { ran++; return nil, 0 }},
		Action{
			Verb: "every", Why: "a probe naming the whole area", Alone: true,
			Answer: func() (any, int) { ran++; return nil, 0 },
		},
	)

	// Both orders, because a sweep resolves every name before it runs the
	// first: the position of the word on the line cannot decide the answer.
	for _, line := range [][]string{{"every", "one"}, {"one", "every"}} {
		answer, code := area.Sweep(line, RunEveryAction)
		if code != 2 {
			t.Errorf("%v answered %d, want 2", line, code)
		}
		if answer != nil {
			t.Errorf("%v answered a payload: %v", line, answer)
		}
	}
	if ran != 0 {
		t.Errorf("the sweep ran %d action(s) before it refused the selection", ran)
	}

	// The verb still runs when it is the whole line, which is the only shape it
	// was ever for. Both dispatch routes answer it: an area that sends a
	// one-word line to Sweep (internal/le/test/unit) reaches the same refusal
	// site with one name, and that line is not a selection.
	if _, code := area.AnswerOrSweep([]string{"every"}, RunEveryAction); code != 0 || ran != 1 {
		t.Errorf("`every` through AnswerOrSweep answered %d after %d run(s), want 0 after one", code, ran)
	}
	if _, code := area.Sweep([]string{"every"}, RunEveryAction); code != 0 || ran != 2 {
		t.Errorf("`every` through Sweep answered %d after %d run(s), want 0 after two", code, ran)
	}
}

// VALIDATES: a help word answers usage instead of running or refusing.
// PREVENTS: `le <area> <verb> --help` reading the help word as a bad keyword.
func TestAHelpWordAnswersUsageAndRunsNothing(t *testing.T) {
	ran := 0
	area := New("probe",
		Action{
			Verb:       "run",
			Why:        "run the probe over a scope",
			Parameters: []Parameter{{Keyword: "scope", Value: "packages", Requirement: Optional, Kind: KindString, Help: "h"}},
			AnswerArgs: func(Arguments) (any, int) { ran++; return nil, 0 },
		},
	)

	for _, args := range [][]string{{"run", "--help"}, {"run", "-h"}, {"run", "help"},
		{"run", "scope", "./internal", "--help"}} {
		answer, code := area.Answer(args)
		if code != 0 || answer != nil {
			t.Errorf("%v answered %v, %d; want nil, 0", args, answer, code)
		}
	}
	if ran != 0 {
		t.Errorf("a help word ran the action %d time(s)", ran)
	}

	answer, code := area.Answer([]string{"--help"})
	list, ok := answer.(List)
	if code != 0 || !ok || len(list.Actions) != 1 || list.Actions[0].Verb != "run" {
		t.Errorf("the area's own help answered %v, %d", answer, code)
	}
}

// proseAnswer is a native action's answer that renders itself, which is what
// leroot.Prose asks of a payload the dispatcher prints.
type proseAnswer struct{ line string }

// Text renders the whole answer, ending in a newline.
func (p proseAnswer) Text() string { return p.line + "\n" }

// unterminatedAnswer renders itself WITHOUT the closing newline, which the
// dispatcher supplies for a bare Prose answer. Several native reports are
// written that way, and a sweep prints a summary line after them.
type unterminatedAnswer struct{ line string }

// Text renders the whole answer, with no trailing newline.
func (u unterminatedAnswer) Text() string { return u.line }

// TestSweepTextNamesTheCauseAFailingReportHolds drives Sweep over one passing
// and one failing action, both answering a payload that renders itself. The
// method is Sweep.Text, which is the only string the dispatcher prints for an
// area invoked with no pipe operator.
//
// It exists because a native action holds its cause in SweepRow.Answer and
// streams nothing: `./le test functional exabgp-test` answered 127 with "Failed:
// exabgp-test" and no reason, while the report named the child that could not
// start (plan/journal/failing-gate-prints-no-cause.md).
func TestSweepTextNamesTheCauseAFailingReportHolds(t *testing.T) {
	area := New("probe",
		Action{
			Verb: "green", Why: "a probe that passes",
			Answer: func() (any, int) { return proseAnswer{"green did its work"}, 0 },
		},
		Action{
			Verb: "red", Why: "a probe that fails",
			Answer: func() (any, int) { return unterminatedAnswer{"red could not start uv"}, 127 },
		},
	)

	answer, code := area.Sweep([]string{"green", "red"}, RunEveryAction)
	if code != 127 {
		t.Fatalf("sweep code = %d, want 127", code)
	}
	sweep, isSweep := answer.(Sweep)
	if !isSweep {
		t.Fatalf("sweep answered %T, want a Sweep payload", answer)
	}
	text := sweep.Text()
	if !strings.Contains(text, "red could not start uv") {
		t.Errorf("the failing report's cause is absent:\n%s", text)
	}
	if strings.Contains(text, "green did its work") {
		t.Errorf("a passing action's report was printed:\n%s", text)
	}
	// The report is written without a closing newline, so the summary would
	// share its line if the sweep did not close it.
	if !strings.HasSuffix(text, "red could not start uv\nFailed: red\n") {
		t.Errorf("the summary line does not close the text on its own line:\n%s", text)
	}
}

// TestSweepTextStaysAsummaryWhenNothingFailed pins the other half: a sweep that
// passed prints its count and no report, so an area whose actions stream their
// own output does not print it twice.
func TestSweepTextStaysASummaryWhenNothingFailed(t *testing.T) {
	area := New("probe", Action{
		Verb: "green", Why: "a probe that passes",
		Answer: func() (any, int) { return proseAnswer{"green did its work"}, 0 },
	})

	answer, code := area.Sweep([]string{"green"}, RunEveryAction)
	if code != 0 {
		t.Fatalf("sweep code = %d, want 0", code)
	}
	sweep, isSweep := answer.(Sweep)
	if !isSweep {
		t.Fatalf("sweep answered %T, want a Sweep payload", answer)
	}
	if want := "\nprobe: 1 action(s) passed.\n"; sweep.Text() != want {
		t.Errorf("text = %q, want %q", sweep.Text(), want)
	}
}

// ─── The published grammar: requiredness, repetition, and what usage says ────

// captureStderr runs fn with os.Stderr replaced by a pipe and answers what fn
// wrote. Usage and every refusal name os.Stderr directly, which is what keeps
// them inside the one exemption the no-Sprintf check makes, so reading them
// back means moving the file rather than injecting a writer.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("open a pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write
	t.Cleanup(func() { os.Stderr = saved })

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := read.Read(buf)
			sb.Write(buf[:n]) //nolint:errcheck // strings.Builder never fails
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	write.Close() //nolint:errcheck // the reader answers what it has
	captured := <-done
	read.Close() //nolint:errcheck // read to EOF
	return captured
}

// grammarArea is an action whose four parameters cover every published shape:
// a required value, an optional one, a repeatable one, and a boolean switch.
func grammarArea(got *Arguments) Area {
	return New("qemu", Action{
		Verb: "run",
		Why:  "boot a guest",
		Parameters: []Parameter{
			{Keyword: "command", Value: "command", Requirement: Required, Kind: KindString, Help: "h"},
			{Keyword: "timeout", Value: "duration", Requirement: Optional, Kind: KindString, Help: "h"},
			{Keyword: "share", Value: "path", Requirement: Optional, Repeat: true, Kind: KindString, Help: "h"},
			{Keyword: "keep-alive", Kind: KindSwitch, Help: "h"},
		},
		AnswerArgs: func(args Arguments) (any, int) {
			*got = args
			return args, 0
		},
	})
}

// VALIDATES: an area's listing carries each action's whole keyword grammar,
// and publishes required and repeat for every parameter rather than dropping
// the false ones.
// PREVENTS: a reader having to invoke an action to learn what it takes, and a
// manifest whose silence about requiredness reads as "nobody said".
func TestRowCarriesTheParametersItsActionDeclares(t *testing.T) {
	var got Arguments
	rows := grammarArea(&got).Actions().Actions
	if len(rows) != 1 {
		t.Fatalf("the area answers %d actions, want 1", len(rows))
	}

	want := []Parameter{
		{Keyword: "command", Value: "command", Requirement: Required, Kind: KindString, Help: "h"},
		{Keyword: "timeout", Value: "duration", Requirement: Optional, Kind: KindString, Help: "h"},
		{Keyword: "share", Value: "path", Requirement: Optional, Repeat: true, Kind: KindString, Help: "h"},
		{Keyword: "keep-alive", Kind: KindSwitch, Help: "h"},
	}
	if !reflect.DeepEqual(rows[0].Parameters, want) {
		t.Fatalf("the row carries %#v, want %#v", rows[0].Parameters, want)
	}

	raw, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatalf("a row does not encode: %v", err)
	}
	var decoded struct {
		Parameters []map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("a row does not decode: %v", err)
	}
	if len(decoded.Parameters) != len(want) {
		t.Fatalf("the encoded row carries %d parameters, want %d: %s", len(decoded.Parameters), len(want), raw)
	}
	for index, parameter := range decoded.Parameters {
		for _, key := range []string{"keyword", "value", "required", "repeat"} {
			if _, published := parameter[key]; !published {
				t.Errorf("parameter %d publishes no %q: %s", index, key, raw)
			}
		}
	}
}

// VALIDATES: usage renders a required keyword without brackets, an optional one
// inside them, and a repeatable one with the ellipsis that says it may be given
// again.
// PREVENTS: the bracket telling every reader the same thing about every
// keyword, which is what makes the reachable grammar wrong about itself.
func TestUsageDistinguishesARequiredKeywordFromAnOptionalOne(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	page := captureStderr(t, func() {
		if _, code := area.Answer([]string{"run", "--help"}); code != 0 {
			t.Errorf("a help word answered %d, want 0", code)
		}
	})
	if got != nil {
		t.Fatalf("a help word reached the handler with %#v", got)
	}

	want := "usage: le qemu run command <command> [timeout <duration>]" +
		" [share <path>]... [keep-alive] [| json | yaml | table]\n" +
		"  boot a guest\n"
	if page != want {
		t.Errorf("usage reads\n%q\nwant\n%q", page, want)
	}
}

// VALIDATES: a keyword declared Repeat accumulates every value it was given, in
// the order typed, and a keyword without it keeps refusing a second occurrence
// with the message and the code it answers today.
// PREVENTS: loosening the parser for every action rather than the one that
// declared it, and a repeated value silently replacing the one before it.
func TestARepeatableKeywordIsParsedTwiceAndANonRepeatableIsStillRefused(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	if _, code := area.Answer([]string{
		"run", "command", "true", "share", "/one", "share", "/two",
	}); code != 0 {
		t.Fatalf("a repeated keyword answered %d, want 0", code)
	}
	if values := got.Values("share"); !slices.Equal(values, []string{"/one", "/two"}) {
		t.Errorf("share carries %q, want both values in the order typed", values)
	}
	if values := got.Values("command"); !slices.Equal(values, []string{"true"}) {
		t.Errorf("a single-valued keyword answers %q, want one value", values)
	}
	if values := got.Values("timeout"); values != nil {
		t.Errorf("a keyword nobody typed answers %q, want nothing", values)
	}
	if !got.Has("share") {
		t.Error("a repeated keyword is absent from Has")
	}

	got = nil
	refusal := captureStderr(t, func() {
		if _, code := area.Answer([]string{"run", "timeout", "1s", "timeout", "2s"}); code != 2 {
			t.Errorf("a keyword given twice answered %d, want 2", code)
		}
	})
	if got != nil {
		t.Errorf("a refused invocation reached the handler with %#v", got)
	}
	if want := "error: argument keyword \"timeout\" was provided more than once\n"; refusal != want {
		t.Errorf("the refusal reads %q, want %q", refusal, want)
	}
}

// VALIDATES: a keyword given twice has no single-value reading. Values answers
// both, and One refuses rather than hand back one of them.
// PREVENTS: the reversed shape this package shipped first, where indexing the
// map answered "first\x00second" -- a string a caller cannot tell from a value
// an operator typed (ai/rules/principles.md).
func TestARepeatKeywordCannotBeReadAsOneValue(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	if _, code := area.Answer([]string{
		"run", "command", "true", "share", "/one", "share", "/two",
	}); code != 0 {
		t.Fatalf("a repeated keyword answered %d, want 0", code)
	}
	if values := got.Values("share"); !slices.Equal(values, []string{"/one", "/two"}) {
		t.Fatalf("share carries %q, want both values in the order typed", values)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("One answered a keyword that carries two values")
		}
		if message, isText := recovered.(string); !isText || !strings.HasPrefix(message, "BUG:") {
			t.Fatalf("One panicked with %v, want a BUG assertion", recovered)
		}
	}()
	_ = got.One("share")
}

// VALIDATES: Required is PUBLISHED and is not newly enforced: the parser still
// hands a missing keyword to the action, whose own body refuses it exactly
// where and how it refuses it today.
// PREVENTS: marking a parameter required starting to refuse an invocation that
// works today, which is the one way this field could break a caller.
func TestARequiredKeywordIsPublishedRatherThanNewlyEnforced(t *testing.T) {
	reached := 0
	area := New("probe", Action{
		Verb: "run", Why: "run the probe over one file",
		Parameters: []Parameter{{Keyword: "file", Value: "path", Requirement: Required, Kind: KindString, Help: "h"}},
		AnswerArgs: func(args Arguments) (any, int) {
			reached++
			if !args.Has("file") {
				return nil, 4
			}
			return args, 0
		},
	})

	if _, code := area.Answer([]string{"run"}); code != 4 {
		t.Errorf("a missing required keyword answered %d, want the action's own 4", code)
	}
	if reached != 1 {
		t.Errorf("the action's body was reached %d time(s), want 1: requiredness is published, not parsed", reached)
	}
}

// VALIDATES: a help word that a declared keyword introduced is the VALUE the
// operator typed, so the action runs with it, while the same word at a keyword
// position still asks what the action takes.
// PREVENTS: the trailing-help guard swallowing an invocation whose last word is
// data, which answers 0 and runs nothing: a caller cannot tell that no-op from
// the work it asked for (ai/rules/principles.md).
func TestATrailingHelpWordInAValueSlotIsTheKeywordsValue(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	answer, code := area.Answer([]string{"run", "timeout", "5s", "command", "help"})
	if code != 0 {
		t.Errorf("an invocation ending in a value answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the action did not run: the answer was %v", answer)
	}
	if got.One("command") != "help" {
		t.Errorf("command carries %q, want the word the operator typed", got.One("command"))
	}

	got = nil
	page := captureStderr(t, func() {
		if _, code := area.Answer([]string{"run", "command", "ls", "help"}); code != 0 {
			t.Errorf("a help word at a keyword position answered %d, want 0", code)
		}
	})
	if got != nil {
		t.Errorf("a help word at a keyword position ran the action with %#v", got)
	}
	if !strings.HasPrefix(page, "usage: le qemu run") {
		t.Errorf("a help word at a keyword position printed %q", page)
	}
}

// VALIDATES: the value-slot exemption is drawn at the SPELLING. The bare word
// `help` in a keyword's value slot reaches the action as its value. A help
// option in that same TRAILING slot is the question and stops before it.
// PREVENTS: a flag spelling read as data. `ai/rules/cli.md` bans a flag from
// being grammar or a value, so no position turns one into a keyword's value.
// `le verify status check path --help` ran the check over a path named
// `--help`.
func TestAFlagSpellingIsNeverAKeywordsValue(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	if _, code := area.Answer([]string{"run", "timeout", "5s", "command", "help"}); code != 0 {
		t.Errorf("the bare word in a value slot answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the bare word in a value slot did not run the action")
	}
	if got.One("command") != "help" {
		t.Errorf("command carries %q, want the word the operator typed", got.One("command"))
	}

	for _, flag := range []string{"-h", "--help", "-help"} {
		got = nil
		page := captureStderr(t, func() {
			if _, code := area.Answer([]string{"run", "timeout", "5s", "command", flag}); code != 0 {
				t.Errorf("%s in a value slot answered %d, want 0", flag, code)
			}
		})
		if got != nil {
			t.Errorf("%s in a value slot ran the action with %#v", flag, got)
		}
		if !strings.HasPrefix(page, "usage: le qemu run") {
			t.Errorf("%s in a value slot printed %q", flag, page)
		}
	}
}

// VALIDATES: an OPTION in a value slot is refused WHEREVER it stands, not only
// as the last word, and whatever it spells. The parser names the keyword and
// the option, and answers 2, the code every other keyword mistake answers. The
// bare word `help` in the same slot is still the value the operator typed,
// because it carries no dash.
// PREVENTS: `le repo rewrite replace file <path> old beta new --help apply`
// writing the text `--help` into a file. The trailing-position rule read the
// last word only, so an option one slot earlier stayed data. No le action takes
// a value that begins with a dash (ai/rules/cli.md).
func TestAnOptionIsRefusedInAValueSlotAnywhereOnTheLine(t *testing.T) {
	var got Arguments
	area := grammarArea(&got)

	for _, option := range []string{"-h", "--help", "-help", "--list", "-count=1", "-6"} {
		got = nil
		line := []string{"run", "command", option, "keep-alive"}
		page := captureStderr(t, func() {
			if _, code := area.Answer(line); code != 2 {
				t.Errorf("%s in a non-trailing value slot answered %d, want 2", option, code)
			}
		})
		if got != nil {
			t.Errorf("%s in a non-trailing value slot ran the action with %#v", option, got)
		}
		for _, want := range []string{`keyword "command"`, `"` + option + `"`, "never begins with a dash"} {
			if !strings.Contains(page, want) {
				t.Errorf("%s was refused with %q, which does not name %s", option, page, want)
			}
		}
	}

	// The LAST word is a value slot too. A help spelling there asks what the
	// action takes, and every other option is refused like any other.
	got = nil
	page := captureStderr(t, func() {
		if _, code := area.Answer([]string{"run", "command", "ls", "timeout", "-xh"}); code != 2 {
			t.Errorf("a trailing -xh in a value slot answered %d, want 2", code)
		}
	})
	if got != nil {
		t.Errorf("a trailing -xh in a value slot ran the action with %#v", got)
	}
	if !strings.Contains(page, `keyword "timeout" cannot take "-xh"`) {
		t.Errorf("a trailing -xh was refused with %q", page)
	}

	got = nil
	if _, code := area.Answer([]string{"run", "command", helpWord, "keep-alive"}); code != 0 {
		t.Errorf("the bare word in a non-trailing value slot answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the bare word in a non-trailing value slot did not run the action")
	}
	if got.One("command") != helpWord {
		t.Errorf("command carries %q, want the word the operator typed", got.One("command"))
	}
	if !got.Has("keep-alive") {
		t.Error("the keyword after the bare word was not parsed")
	}
}

// TestForwardingActionHandsItsWordsOverVerbatim checks the third answer form:
// the words after the verb reach the action unparsed, options included, the
// row publishes that it forwards them, and a forwarding action that also
// declares parameters is refused at registration.
func TestForwardingActionHandsItsWordsOverVerbatim(t *testing.T) {
	var got []string
	area := New("probe", Action{
		Verb: "send",
		Why:  "forward the rest of the line",
		AnswerWords: func(words []string) (any, int) {
			got = words
			return nil, 7
		},
	})
	words := []string{"--dut-addr", "10.0.0.1", "free", "-x"}
	if _, code := area.Answer(append([]string{"send"}, words...)); code != 7 {
		t.Fatalf("the forwarding action answered %d, want its own 7", code)
	}
	if !slices.Equal(got, words) {
		t.Fatalf("the action received %q, want %q", got, words)
	}
	if !area.TakesArguments("send") {
		t.Error("a sweep would read the forwarded words as verbs")
	}
	rows := area.Actions().Actions
	if len(rows) != 1 || !rows[0].Forwards {
		t.Fatalf("the row does not publish that it forwards: %+v", rows)
	}
	if usage, _ := area.Actions().UsageText("send"); !strings.Contains(usage, "<words>...") {
		t.Errorf("the usage does not say the verb takes words: %q", usage)
	}

	defer func() {
		if recover() == nil {
			t.Error("New accepted a forwarding action that declares parameters")
		}
	}()
	New("probe", Action{
		Verb:        "send",
		Why:         "forward",
		Parameters:  []Parameter{{Keyword: "dut", Value: "name", Requirement: Optional, Kind: KindString, Help: "h"}},
		AnswerWords: func([]string) (any, int) { return nil, 0 },
	})
}

// VALIDATES: a listing names the keyword whose value is another le command line
// only for the verb that declares one, so a workflow reader can credit the
// nested command from the registry rather than from a copy of the verb's name.
// PREVENTS: a wrapper action hiding the lab it runs from the RFC evidence tier,
// and the opposite error of reading an ordinary value as a command.
func TestNestedLeKeywordIsReadFromTheDeclaredParameter(t *testing.T) {
	answer := func(Arguments) (any, int) { return nil, 0 }
	list := New("test vm",
		Action{Verb: "lab", Why: "run a lab in the guest", AnswerArgs: answer, Parameters: []Parameter{
			{Keyword: "timeout", Value: "duration", Requirement: Optional, Kind: KindString, Help: "h"},
			{Keyword: "run", Kind: KindWords, Value: "le-words", Requirement: Optional, Help: "h"},
		}},
		Action{Verb: "boot", Why: "boot the guest", AnswerArgs: answer, Parameters: []Parameter{
			{Keyword: "command", Value: "command", Requirement: Optional, Kind: KindString, Help: "h"},
		}},
	).Actions()

	if keyword, ok := list.NestedLeKeyword("lab"); !ok || keyword != "run" {
		t.Errorf("NestedLeKeyword(lab) = %q, %v; want run, true", keyword, ok)
	}
	if keyword, ok := list.NestedLeKeyword("boot"); ok {
		t.Errorf("NestedLeKeyword(boot) = %q, true; a shell command is not le words", keyword)
	}
	if keyword, ok := list.NestedLeKeyword("absent"); ok {
		t.Errorf("NestedLeKeyword(absent) = %q, true; a verb the listing does not hold nests nothing", keyword)
	}
}

// VALIDATES: D-Q3 positional rule -- at most one positional value, in the
// first slot after the verb, of a kind other than string, and New panics on
// every other shape; the value is read under its Value name.
// PREVENTS: a grammar where a free word could be a value or a keyword, which is
// the ambiguity the keyword-before-value rule exists to remove.
func TestPositionalParameterRules(t *testing.T) {
	answerArgs := func(Arguments) (any, int) { return nil, 0 }
	spec := Parameter{Kind: KindSpec, Value: "spec", Requirement: Required, Help: "the spec to claim"}
	quiet := Parameter{Keyword: "quiet", Kind: KindSwitch, Help: "print nothing"}

	var got Arguments
	area := New("spec", Action{
		Verb: "claim", Why: "claim a spec", Parameters: []Parameter{spec, quiet},
		AnswerArgs: func(args Arguments) (any, int) { got = args; return nil, 0 },
	})
	if _, code := area.Answer([]string{"claim", "plan/x.md", "quiet"}); code != 0 {
		t.Fatalf("claim plan/x.md quiet answered %d", code)
	}
	if got.One("spec") != "plan/x.md" || !got.Has("quiet") {
		t.Errorf("the positional value was read as %q and quiet as %v", got.One("spec"), got.Has("quiet"))
	}
	usage, _ := area.Actions().UsageText("claim")
	if !strings.Contains(usage, "claim <spec> [quiet]") {
		t.Errorf("usage does not render the positional slot first:\n%s", usage)
	}

	cases := []struct {
		name       string
		parameters []Parameter
	}{
		{"two positionals", []Parameter{spec, {Kind: KindPath, Value: "path", Requirement: Optional, Help: "h"}}},
		{"positional not first", []Parameter{quiet, spec}},
		{"positional of kind string", []Parameter{{Kind: KindString, Value: "text", Requirement: Required, Help: "h"}}},
		{"positional switch", []Parameter{{Kind: KindSwitch, Help: "h"}}},
		{"positional repeat", []Parameter{{Kind: KindPath, Value: "path", Requirement: Optional, Repeat: true, Help: "h"}}},
		{"positional named like a keyword", []Parameter{{Kind: KindPath, Value: "quiet", Requirement: Optional, Help: "h"}, quiet}},
		{"no kind", []Parameter{{Keyword: "file", Value: "path", Requirement: Required, Help: "h"}}},
		{"no help", []Parameter{{Keyword: "file", Kind: KindPath, Value: "path", Requirement: Required}}},
		{"switch with a value", []Parameter{{Keyword: "file", Kind: KindSwitch, Value: "path", Requirement: Required, Help: "h"}}},
		{"value kind with no value", []Parameter{{Keyword: "file", Kind: KindPath, Help: "h"}}},
		{"enum with no values", []Parameter{{Keyword: "mode", Kind: KindEnum, Value: "mode", Requirement: Optional, Help: "h"}}},
		{"values on a path", []Parameter{{Keyword: "file", Kind: KindPath, Value: "path", Values: []string{"a"}, Requirement: Optional, Help: "h"}}},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("New accepted a parameter table the positional and kind rules refuse")
				}
			}()
			New("a", Action{Verb: "check", Why: "why", Parameters: one.parameters, AnswerArgs: answerArgs})
		})
	}
}

// VALIDATES: D-Q3 Retired -- a retired keyword is refused naming its
// successor, and no listing, usage line or spelling renders it.
// PREVENTS: an old spelling answered "takes no argument keyword", which leaves
// the reader to guess the new one, and a retired keyword offered by help.
func TestRetiredKeywordIsRefusedNamingItsSuccessor(t *testing.T) {
	ran := false
	area := New("commit", Action{
		Verb: "create", Why: "write a commit script",
		Parameters: []Parameter{
			{Keyword: "subject", Kind: KindString, Value: "text", Requirement: Required, Help: "the subject line"},
			{Keyword: "push", Retired: "le commit push"},
		},
		AnswerArgs: func(Arguments) (any, int) { ran = true; return nil, 0 },
	})

	var code int
	stderr := captureStderr(t, func() { _, code = area.Answer([]string{"create", "subject", "x", "push"}) })
	if code != 2 {
		t.Errorf("a retired keyword answered %d, want the usage code 2", code)
	}
	if ran {
		t.Error("the action ran with a retired keyword on its line")
	}
	if !strings.Contains(stderr, "push") || !strings.Contains(stderr, "le commit push") {
		t.Errorf("the refusal does not name the keyword and its successor:\n%s", stderr)
	}

	list := area.Actions()
	usage, _ := list.UsageText("create")
	if strings.Contains(usage, "push") {
		t.Errorf("usage renders the retired keyword:\n%s", usage)
	}
	for _, parameter := range list.Actions[0].Parameters {
		if parameter.Keyword == "push" {
			t.Error("the listing publishes the retired keyword")
		}
	}
	if spelled := spellings("le commit create", area.actions[0].Parameters); strings.Contains(spelled, "push") {
		t.Errorf("the spellings offer the retired keyword: %s", spelled)
	}
}

// VALIDATES: D-Q3 pair arity -- a pair keyword takes two values, usage renders
// both value names from the declaration, and every pair given is kept.
// PREVENTS: a hand-written usage string for a two-value keyword, and a second
// value read as the next keyword.
func TestPairKeywordUsageIsDerived(t *testing.T) {
	var got Arguments
	area := New("repo", Action{
		Verb: "move", Why: "move files",
		Parameters: []Parameter{{
			Keyword: "rename", Kind: KindPath, Value: "old", Pair: "new",
			Requirement: Optional, Repeat: true, Help: "one path and the path it becomes",
		}},
		AnswerArgs: func(args Arguments) (any, int) { got = args; return nil, 0 },
	})

	usage, _ := area.Actions().UsageText("move")
	if !strings.Contains(usage, "move [rename <old> <new>]...") {
		t.Errorf("usage does not derive the pair form:\n%s", usage)
	}
	if _, code := area.Answer([]string{"move", "rename", "a", "b", "rename", "c", "d"}); code != 0 {
		t.Fatalf("two pairs answered %d", code)
	}
	want := [][2]string{{"a", "b"}, {"c", "d"}}
	if pairs := got.Pairs("rename"); !slices.Equal(pairs, want) {
		t.Errorf("Pairs(rename) = %v, want %v", pairs, want)
	}

	var code int
	stderr := captureStderr(t, func() { _, code = area.Answer([]string{"move", "rename", "a"}) })
	if code != 2 || !strings.Contains(stderr, "<new>") {
		t.Errorf("a pair missing its second value answered %d with:\n%s", code, stderr)
	}
	if !trailingIsValue(area.actions[0].Parameters, []string{"rename", "a", "help"}) {
		t.Error("the word in a pair's second slot is not read as its value")
	}
}
