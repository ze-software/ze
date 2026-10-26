// Related: leaction.go -- New's Default refusals and List.AtVerb, which these tests drive

package leaction

import (
	"slices"
	"strings"
	"testing"
)

// defaultAnswer is the answer every action in these tables gives.
func defaultAnswer() (any, int) { return nil, 0 }

// VALIDATES: New refuses a table where two actions declare Default, and one
// where the sole, verbless action declares it, each with the panic that names
// that defect, not another one the table also happens to trip.
// PREVENTS: a line opening with no verb having two owners, and a Default flag
// on an action whose area word already names it, reaching dispatch.
func TestNewRefusesAnAmbiguousDefault(t *testing.T) {
	cases := []struct {
		name    string
		actions []Action
		want    string
	}{
		{"two defaults", []Action{
			{Verb: "report", Default: true, Why: "first", Answer: defaultAnswer},
			{Verb: "write", Default: true, Why: "second", Answer: defaultAnswer},
		}, "two actions of one area declare Default"},
		{"a verbless default", []Action{
			{Default: true, Does: "report", Why: "sole", Answer: defaultAnswer},
		}, "a verbless action declares Default"},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("New accepted the table")
				}
				message, isString := recovered.(string)
				if !isString {
					t.Fatalf("New panicked with %T %v, want a string", recovered, recovered)
				}
				if !strings.Contains(message, one.want) {
					t.Errorf("New panicked with %q, want it to name %q", message, one.want)
				}
			}()
			New("a", one.actions...)
		})
	}
}

// VALIDATES: AtVerb begins a line that opens with none of the listing's verbs,
// the empty line included, at the default action's verb, and leaves a line
// that opens with a verb, the default's own included, as typed. A listing
// with no default leaves every line as typed.
// PREVENTS: `le repo working-tree max-areas 3` read as an unknown verb, or
// `le repo working-tree write` read as the default's keyword.
func TestAtVerbResolvesALineWithNoVerbToTheDefaultAction(t *testing.T) {
	defaulted := New("probe-area",
		Action{Verb: "report", Default: true, Summary: "report", Why: "the default", Answer: defaultAnswer},
		Action{Verb: "write", Summary: "write", Why: "the other", Answer: defaultAnswer},
	).Actions()
	plain, _ := fixture(t)

	cases := []struct {
		name  string
		list  List
		words []string
		want  []string
	}{
		{"the bare line", defaulted, nil, []string{"report"}},
		{"a keyword and its value", defaulted, []string{"max-areas", "3"}, []string{"report", "max-areas", "3"}},
		{"the other verb", defaulted, []string{"write"}, []string{"write"}},
		{"the default's own verb", defaulted, []string{"report", "max-areas", "3"}, []string{"report", "max-areas", "3"}},
		{"no default declared", plain.Actions(), []string{"max-areas", "3"}, []string{"max-areas", "3"}},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := one.list.AtVerb(one.words); !slices.Equal(got, one.want) {
				t.Errorf("AtVerb(%q) = %q, want %q", one.words, got, one.want)
			}
		})
	}
}

// VALIDATES: the listing's heading names the default action, derived from the
// table, so the text page says what a line naming no action runs; a listing
// with no default keeps a bare heading.
// PREVENTS: `le repo working-tree help` listing `report` without saying that
// `le repo working-tree` alone runs it, a word le accepts that its help hid.
func TestTheListingNamesTheDefaultAction(t *testing.T) {
	defaulted := New("probe-area",
		Action{Verb: "report", Default: true, Summary: "report", Why: "the default", Answer: defaultAnswer},
		Action{Verb: "write", Summary: "write", Why: "the other", Answer: defaultAnswer},
	).Actions()
	if got, want := defaulted.Section().Summary, "a line that names no action runs report"; got != want {
		t.Errorf("Section().Summary = %q, want %q", got, want)
	}
	if text := defaulted.Text(); !strings.Contains(text, "Actions: a line that names no action runs report") {
		t.Errorf("the area page does not name the default action:\n%s", text)
	}

	plain, _ := fixture(t)
	if got := plain.Actions().Section().Summary; got != "" {
		t.Errorf("a listing with no default has the heading summary %q", got)
	}
}
