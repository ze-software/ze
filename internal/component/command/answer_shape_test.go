package command

import (
	"encoding/json"
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
