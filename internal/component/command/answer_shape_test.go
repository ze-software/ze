package command

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, payload string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	return v
}

// TestRowsInFindsTheRows covers the three answers, and the third is the one
// that produced a wrong number rather than a refusal: `show bgp | count`
// answered 6, the count of its top-level keys.
func TestRowsInFindsTheRows(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    int
		wantKey string
		wantOK  bool
	}{
		{
			name:    "a bare array is the rows",
			payload: `[{"a":1},{"a":2},{"a":3}]`,
			want:    3, wantKey: "", wantOK: true,
		},
		{
			name:    "one array beside aggregates is the rows",
			payload: `{"peers":[{"address":"192.0.2.1"},{"address":"192.0.2.2"}],"total":2,"up":1}`,
			want:    2, wantKey: "peers", wantOK: true,
		},
		{
			name:    "two arrays are ambiguous, so no rows",
			payload: `{"peers":[{"a":1}],"routes":[{"b":2}],"total":2}`,
			wantOK:  false,
		},
		{
			name:    "a map with no array has no rows",
			payload: `{"version":"ze dev","built":"unknown"}`,
			wantOK:  false,
		},
		{
			name:    "one value has no rows",
			payload: `"ze dev (built unknown)"`,
			wantOK:  false,
		},
		{
			name:    "an empty array is rows, and there are none",
			payload: `{"peers":[]}`,
			want:    0, wantKey: "peers", wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, key, ok := rowsIn(decode(t, tt.payload))
			if ok != tt.wantOK {
				t.Fatalf("rowsIn ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if len(rows) != tt.want {
				t.Errorf("found %d rows, want %d", len(rows), tt.want)
			}
			if key != tt.wantKey {
				t.Errorf("rows under %q, want %q", key, tt.wantKey)
			}
		})
	}
}

// TestShapeOfAnswerSeparatesRowsFromOneValue is what makes a refusal possible
// on a command that declared nothing.
func TestShapeOfAnswerSeparatesRowsFromOneValue(t *testing.T) {
	if got := ShapeOfAnswer(decode(t, `{"peers":[{"a":1}]}`)); got != ShapeMap {
		t.Errorf("an envelope of rows has shape %v, want map", got)
	}
	if got := ShapeOfAnswer(decode(t, `{"version":"ze dev"}`)); got != ShapeDoc {
		t.Errorf("one value has shape %v, want doc", got)
	}
}

// TestRowKeysNamesTheCandidates lets an ambiguous refusal say what it refused
// to choose between, rather than reporting a number nobody asked for.
func TestRowKeysNamesTheCandidates(t *testing.T) {
	keys := rowKeys(decode(t, `{"peers":[],"routes":[],"total":2}`))
	if len(keys) != 2 {
		t.Fatalf("found %d array keys, want 2: %v", len(keys), keys)
	}
}

func TestDeclaredShapeResolvesByLongestPrefix(t *testing.T) {
	ResetShapesForTest()
	t.Cleanup(ResetShapesForTest)

	RegisterShape([]string{"show bgp"}, ShapeDoc)
	RegisterShape([]string{"show bgp peer list"}, ShapeTab)

	if shape, declared := ShapeForCommand("show bgp peer list"); !declared || shape != ShapeTab {
		t.Errorf("show bgp peer list = %v/%v, want tab/declared", shape, declared)
	}
	if shape, declared := ShapeForCommand("show bgp rpki"); !declared || shape != ShapeDoc {
		t.Errorf("show bgp rpki inherits %v/%v, want doc/declared", shape, declared)
	}
	// Undeclared is not doc-by-assertion: it reports that nothing was declared,
	// so the published page can say so and the refusal falls to the answer's
	// own shape instead.
	if shape, declared := ShapeForCommand("show interface"); declared || shape != ShapeDoc {
		t.Errorf("undeclared command = %v/%v, want doc/undeclared", shape, declared)
	}

	// A child whose answer has a different shape from its parent's declares
	// NONE, which stops the inheritance rather than asserting a shape it has
	// not got. RegisterColumns uses the same convention.
	RegisterShape([]string{"show bgp rpki"})
	if shape, declared := ShapeForCommand("show bgp rpki"); declared || shape != ShapeDoc {
		t.Errorf("a child declaring none = %v/%v, want doc/undeclared", shape, declared)
	}
}

func TestDeclaredAddressFields(t *testing.T) {
	ResetAddressFieldsForTest()
	t.Cleanup(ResetAddressFieldsForTest)

	RegisterAddressFields([]string{"show bgp peer list"}, "address", "nexthop")
	if got := AddressFieldsForCommand("show bgp peer list"); len(got) != 2 {
		t.Errorf("declared fields = %v, want two", got)
	}
	// A command that declares none refuses resolve and origin rather than
	// guessing by parsing every value.
	if got := AddressFieldsForCommand("show version"); got != nil {
		t.Errorf("undeclared command answers %v, want nil", got)
	}
}

// TestEmptyAnswerHasZeroRowsRatherThanNone separates "nothing to report" from
// "this answer has a shape that cannot carry rows". A filter that removed every
// row lands here, and refusing it would turn an empty result into an error:
// `show bgp peer list | match nothing` must answer nothing and exit 0.
func TestEmptyAnswerHasZeroRowsRatherThanNone(t *testing.T) {
	for _, payload := range []string{`{}`, `[]`, `null`} {
		rows, _, ok := rowsIn(decode(t, payload))
		if !ok {
			t.Errorf("%s reports no rows; an empty answer has zero rows", payload)
		}
		if len(rows) != 0 {
			t.Errorf("%s reports %d rows, want 0", payload, len(rows))
		}
	}
	// A non-empty answer with no rows is still refused: it HAS content and
	// none of it is rows.
	if _, _, ok := rowsIn(decode(t, `{"version":"ze dev"}`)); ok {
		t.Error("a single-value answer reports rows")
	}
}

// TestDeclaredShapeRefusesBeforeTheCommandRuns covers the case the ANSWER
// cannot decide.
//
// `show config dump` answers a nested configuration tree. A tree whose one
// top-level key holds a map of maps is indistinguishable from rows keyed by
// identity, so the answer's own shape says "rows" and `| first 1` was accepted
// and answered a fragment of the config. The command knows it holds one
// document; the payload does not say so.
//
// It is also what keeps the published catalog true: `ze help command --json`
// lists a declared command's operators FROM its shape, so without this the
// catalog promised nine operators while the runtime accepted fifteen.
//
// VALIDATES: an operator the declared shape cannot support is refused BEFORE
// the command runs.
// PREVENTS: the published surface and the runtime disagreeing, which is the
// defect this whole spec exists to end.
func TestDeclaredShapeRefusesBeforeTheCommandRuns(t *testing.T) {
	ResetShapesForTest()
	t.Cleanup(ResetShapesForTest)
	RegisterShape([]string{"show doc thing"}, ShapeDoc)
	RegisterShape([]string{"show row thing"}, ShapeTab)

	rowOps := []pipeOp{{kind: pipeFirst, arg: "1"}}

	msg := validateDeclaredShape("show doc thing /some/file.conf", rowOps)
	if msg == "" {
		t.Fatal("a row operator over a declared doc was accepted")
	}
	if !strings.HasPrefix(msg, "first ") {
		t.Errorf("refusal %q does not name the operator", msg)
	}
	if !strings.Contains(msg, "one document") {
		t.Errorf("refusal %q does not say why", msg)
	}
	// The command is resolved by longest prefix, so an argument must not stop
	// the refusal; and it must not be echoed back either, because it is a path
	// the operator just typed.
	if strings.Contains(msg, "/some/file.conf") {
		t.Errorf("refusal echoes the command's argument: %q", msg)
	}

	if msg := validateDeclaredShape("show row thing", rowOps); msg != "" {
		t.Errorf("a row operator over a declared tab was refused: %s", msg)
	}
	// An undeclared command is left to its answer, which is what makes the
	// refusal universal rather than a property of annotated commands.
	if msg := validateDeclaredShape("show undeclared thing", rowOps); msg != "" {
		t.Errorf("an undeclared command was refused before running: %s", msg)
	}
}

// TestFillRefusalSaysWhatItActuallyNeeds covers the one operator whose refusal
// cannot say "acts on rows": `fill` brings back the columns a command declared,
// so it acts on ShapeTab alone and is refused over a `map` answer whose rows
// carry their own keys.
//
// PREVENTS: the message this replaces read "fill cannot apply here: this command
// answers rows, and fill acts on rows", which states the refusal and its own
// contradiction in one sentence and tells the operator nothing to do next
// (ai/rules/cli.md). It became reachable when `show bgp irr prefix` declared
// `map`.
func TestFillRefusalSaysWhatItActuallyNeeds(t *testing.T) {
	ResetShapesForTest()
	t.Cleanup(ResetShapesForTest)
	RegisterShape([]string{"show self describing"}, ShapeMap)
	RegisterShape([]string{"show ordered"}, ShapeTab)

	fillOps := []pipeOp{{kind: pipeFill}}

	msg := validateDeclaredShape("show self describing", fillOps)
	if msg == "" {
		t.Fatal("`fill` over a map answer was accepted; it acts on a declared column order")
	}
	// The two halves must differ. Asserting that the refusal is non-empty would
	// pass on the sentence this test exists to remove.
	if strings.Count(msg, "acts on rows") > 0 && strings.Contains(msg, "answers rows,") {
		t.Errorf("the refusal contradicts itself: %q", msg)
	}
	if !strings.Contains(msg, "declared column order") {
		t.Errorf("the refusal does not say what `fill` needs: %q", msg)
	}

	// A row operator that genuinely acts on any row shape keeps the shorter
	// wording, so this change is scoped to the operator that needed it.
	countMsg := validateDeclaredShape("show self describing", []pipeOp{{kind: pipeCount}})
	if countMsg != "" {
		t.Errorf("`count` over a map answer was refused: %s", countMsg)
	}

	// `fill` over the shape it does act on is not refused at all.
	if msg := validateDeclaredShape("show ordered", fillOps); msg != "" {
		t.Errorf("`fill` over a tab answer was refused: %s", msg)
	}
}

// TestDeclaredShapeRefusesTheAddressOperators keeps `| resolve` and `| origin`
// off a command that declares no address field, which is what stops them
// decorating a value that happens to parse as an address.
func TestDeclaredShapeRefusesTheAddressOperators(t *testing.T) {
	ResetShapesForTest()
	ResetAddressFieldsForTest()
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)
	RegisterShape([]string{"show rows nofields"}, ShapeTab)
	RegisterShape([]string{"show rows withfields"}, ShapeTab)
	RegisterAddressFields([]string{"show rows withfields"}, "address")

	ops := []pipeOp{{kind: pipeOrigin}}
	if msg := validateDeclaredShape("show rows nofields", ops); msg == "" {
		t.Error("origin was accepted on a command declaring no address field")
	}
	if msg := validateDeclaredShape("show rows withfields", ops); msg != "" {
		t.Errorf("origin was refused on a command that declares one: %s", msg)
	}
}
