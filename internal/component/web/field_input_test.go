package web

import (
	"bytes"
	"context"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// fieldEditorMarkers is the markup each field type must reach. Two types have
// an editor of their own. Every other type the schema produces falls back to
// the text editor, which reads neither Min nor Max.
//
// The keys are checked against the producer below, in both directions, so this
// table cannot drift from what a leaf can actually carry.
var fieldEditorMarkers = map[string]string{
	"bool":     `class="ze-tristate"`,
	"enum":     `class="ze-field-select"`,
	"string":   `type="text"`,
	"uint16":   `type="text"`,
	"uint32":   `type="text"`,
	"int":      `type="text"`,
	"ip":       `type="text"`,
	"prefix":   `type="text"`,
	"duration": `type="text"`,
}

// producedFieldTypes returns every string a FieldMeta.Type can hold, derived
// from the producer rather than restated by hand.
//
// buildFieldMeta (fragment.go) is the only non-test producer of a FieldMeta.
// It reads valueTypeToFieldType for the leaf's type, and it overwrites the
// answer with "enum" when the leaf declares enums.
//
// The walk is over config.ValueType, an iota run from TypeString to TypeEmpty.
// A constant added past the end would fall outside that walk. The guard below
// therefore reads the value after TypeEmpty, and fails when the String method
// recognizes it.
func producedFieldTypes(t *testing.T) []string {
	t.Helper()

	if next := config.ValueType(int(config.TypeEmpty) + 1); next.String() != "unknown" {
		t.Fatalf("config.ValueType gained %q past TypeEmpty; extend this walk", next.String())
	}

	seen := map[string]bool{
		// buildFieldMeta sets this one directly for a leaf with enums.
		"enum": true,
	}

	for vt := config.TypeString; vt <= config.TypeEmpty; vt++ {
		seen[valueTypeToFieldType(vt)] = true
	}

	types := make([]string, 0, len(seen))
	for name := range seen {
		types = append(types, name)
	}

	sort.Strings(types)

	return types
}

// TestFieldInputRegistryAnswersEachType verifies every field type a leaf can
// carry reaches the editor the old template lookup reached.
// VALIDATES: the registry replaces the runtime "input_<type>" template lookup
// with the same dispatch, fallback included, over the set the schema produces.
// PREVENTS: a field type silently rendering nothing, which is what the old
// lookup did when it swallowed its execution error.
func TestFieldInputRegistryAnswersEachType(t *testing.T) {
	produced := producedFieldTypes(t)

	// The expectation table and the producer must name the same set. A type the
	// schema gained with no row here would go untested. A row for a type
	// nothing produces would pin dead behavior. Both happened. This test
	// covered "number", "text" and "", none of which a leaf can carry, and it
	// omitted "int", which every YANG int8 through int64 becomes.
	for _, fieldType := range produced {
		if _, ok := fieldEditorMarkers[fieldType]; !ok {
			t.Errorf("valueTypeToFieldType produces %q and no editor is asserted for it", fieldType)
		}
	}

	for fieldType := range fieldEditorMarkers {
		if !slices.Contains(produced, fieldType) {
			t.Errorf("%q is asserted here and no leaf can carry it", fieldType)
		}
	}

	for _, fieldType := range produced {
		want, ok := fieldEditorMarkers[fieldType]
		if !ok {
			continue
		}

		assertFieldEditor(t, fieldType, want)
	}
}

// TestFieldInputForFallsBackToText pins the rule fieldInputFor applies to a
// type no editor claims.
// VALIDATES: an unregistered type reaches the text editor.
// PREVENTS: a future field type rendering an empty editor. No leaf carries the
// value below, so the fallback has no other cover.
func TestFieldInputForFallsBackToText(t *testing.T) {
	assertFieldEditor(t, "a-type-no-editor-claims", `type="text"`)
}

// assertFieldEditor renders one field type and reports the editor it reached.
func assertFieldEditor(t *testing.T, fieldType, want string) {
	t.Helper()

	field := FieldMeta{Leaf: "hold-time", Path: "bgp", Type: fieldType}

	var buf bytes.Buffer
	if err := fieldInputFor(field).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render %q: %v", fieldType, err)
	}

	if !strings.Contains(buf.String(), want) {
		t.Errorf("type %q rendered %q, want it to carry %s", fieldType, buf.String(), want)
	}
}

// TestFieldInputRegistryCoversTheDeclaredSet verifies the registry holds every
// type that has an editor of its own, and no more.
// VALIDATES: a new editor is reachable only once it is registered.
// PREVENTS: an editor added as a component and never wired, which renders as
// the text fallback and looks like a styling bug.
//
// "number" is in this set and no leaf reaches it: valueTypeToFieldType answers
// uint16, uint32 or int for a numeric leaf. The pre-port lookup missed
// input_number for the same reason, so the port preserved the behavior rather
// than the intent. Recorded in plan/journal/unwired-feature.md.
func TestFieldInputRegistryCoversTheDeclaredSet(t *testing.T) {
	want := map[string]bool{"bool": true, "enum": true, "number": true, "text": true}

	for name := range fieldInputs {
		if !want[name] {
			t.Errorf("registry holds %q, which no editor declares", name)
		}

		delete(want, name)
	}

	for name := range want {
		t.Errorf("registry is missing the %q editor", name)
	}
}
