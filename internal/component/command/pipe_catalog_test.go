package command

import (
	"sort"
	"testing"
)

// TestCatalogIsTheParser proves the parser reads the catalog rather than a
// second literal. Before the catalog, knownPipeOps was a hand-written map and
// five other surfaces held their own copies.
func TestCatalogIsTheParser(t *testing.T) {
	if len(knownPipeOps) != len(pipeCatalog) {
		t.Fatalf("parser knows %d operators, catalog holds %d", len(knownPipeOps), len(pipeCatalog))
	}
	for _, op := range pipeCatalog {
		kind, known := knownPipeOps[op.Name]
		if !known {
			t.Errorf("catalog operator %q is not known to the parser", op.Name)
			continue
		}
		if kind != op.Kind {
			t.Errorf("operator %q: parser kind %v, catalog kind %v", op.Name, kind, op.Kind)
		}
	}
}

// TestCompletionDerivesFromCatalog replaces the self-comparison in
// completer_test.go, which asserted CompletePipe returned len(PipeOperators)
// suggestions and so could never see the two lists drift apart.
func TestCompletionDerivesFromCatalog(t *testing.T) {
	got := make([]string, 0, len(PipeOperators))
	byName := make(map[string]string, len(PipeOperators))
	for _, s := range PipeOperators {
		got = append(got, s.Text)
		byName[s.Text] = s.Description
		if s.Type != "pipe" {
			t.Errorf("completion %q has type %q, want pipe", s.Text, s.Type)
		}
	}
	want := PipeOperatorNames()
	sortedEqual(t, "completion", got, want)

	for _, op := range pipeCatalog {
		if desc := byName[op.Name]; desc != op.Description {
			t.Errorf("operator %q: completion describes it %q, catalog says %q", op.Name, desc, op.Description)
		}
	}
}

// TestEveryOperatorStatesItsContract holds each entry to the properties the
// derivation reads. An entry missing one is an operator no surface can publish
// and no shape can refuse.
func TestEveryOperatorStatesItsContract(t *testing.T) {
	seen := make(map[string]bool, len(pipeCatalog))
	for _, op := range pipeCatalog {
		if seen[op.Name] {
			t.Errorf("operator %q appears twice in the catalog", op.Name)
		}
		seen[op.Name] = true

		if op.Description == "" {
			t.Errorf("operator %q has no description; five surfaces publish one", op.Name)
		}
		if len(op.Shapes()) == 0 {
			t.Errorf("operator %q acts on no shape, so nothing could ever run it", op.Name)
		}
		// The owner's first class: a global operation acts on the answer
		// whatever it holds, so every command owes it on every shape.
		if op.Class == ClassGlobal && len(op.Shapes()) != 3 {
			t.Errorf("operator %q is global but acts on %v; a global operator owes every shape",
				op.Name, op.Shapes())
		}
		// The second class is owed only where the data supports it, so a
		// data-dependent operator that claimed every shape would be global.
		if op.Class == ClassData && op.Applies(ShapeDoc) {
			t.Errorf("operator %q is data-dependent but applies to doc, which has no rows", op.Name)
		}
		// The third acts on a SEQUENCE of answers, so it says nothing about
		// what one answer holds and must apply to every shape.
		if op.Class == ClassStream && len(op.Shapes()) != 3 {
			t.Errorf("operator %q acts on a stream but applies to %v; the shape of one answer does not decide it",
				op.Name, op.Shapes())
		}
	}
}

// TestShapeSpellingMatchesTheWire keeps the declared shape and the answer
// head's item type in one vocabulary, so a refusal names what the head names.
func TestShapeSpellingMatchesTheWire(t *testing.T) {
	for shape, want := range map[AnswerShape]string{ShapeDoc: "doc", ShapeMap: "map", ShapeTab: "tab"} {
		if got := shape.String(); got != want {
			t.Errorf("shape %d spells %q, the wire spells it %q", shape, got, want)
		}
	}
}

// TestParseAnswerShape reads the round trip from the shape population itself,
// so a shape added to allShapes is covered here without an edit. It also proves
// a spelling no shape writes is refused.
//
// VALIDATES: ParseAnswerShape answers every shape String() writes, and only those.
// PREVENTS: A plugin's typo parsing as doc, which would publish the wrong
// operator set and refuse operators the command supports.
func TestParseAnswerShape(t *testing.T) {
	for _, shape := range allShapes {
		spelling := shape.String()
		got, ok := ParseAnswerShape(spelling)
		if !ok {
			t.Errorf("shape %d spells %q, and %q does not parse", shape, spelling, spelling)
			continue
		}
		if got != shape {
			t.Errorf("%q parses to shape %d, and shape %d spells it", spelling, got, shape)
		}
	}

	// "Doc" is here because the wire spelling is lowercase: a permissive parse
	// is what would let a plugin's typo become ShapeDoc in silence.
	for _, spelling := range []string{"", "Doc", "row", "table", "tabs"} {
		if got, ok := ParseAnswerShape(spelling); ok {
			t.Errorf("%q parses to shape %d, and no shape spells it", spelling, got)
		}
	}
}

func sortedEqual(t *testing.T, what string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if len(g) != len(w) {
		t.Fatalf("%s names %d operators, catalog holds %d\n  %s: %v\n  catalog: %v", what, len(g), len(w), what, g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("%s and catalog disagree\n  %s: %v\n  catalog: %v", what, what, g, w)
		}
	}
}
