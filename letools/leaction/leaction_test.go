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

	cases := []struct {
		name    string
		actions []Action
	}{
		{"no gate and no verb", []Action{{Why: "why", Answer: answer}}},
		{"gate and verb both", []Action{{Gate: "ze-a-check", Verb: "check", Why: "why", Answer: answer}}},
		{"no answer", []Action{{Verb: "check", Why: "why"}}},
		{"no why", []Action{{Verb: "check", Answer: answer}}},
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
