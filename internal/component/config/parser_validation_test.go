// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: parser_validation.go -- diagnostic collection never yields an invalid tree

package config

import (
	"errors"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestParseDiagnosticsCollectsScalarStatements checks all four scalar node
// kinds, the actual enclosing list key and normal Parse's first-error behavior.
func TestParseDiagnosticsCollectsScalarStatements(t *testing.T) {
	input := "top { item a { color purple; words purple; tags [ purple ]; modes purple; } }"
	p := NewParser(refusalSchema())
	tree, failures := p.ParseDiagnostics(input)
	if tree != nil {
		t.Fatal("diagnostic parse returned a usable invalid tree")
	}
	if len(failures) != 4 {
		t.Fatalf("got %d failures, want 4: %v", len(failures), failures)
	}
	for i, leaf := range []string{"color", "words", "tags", "modes"} {
		var refusal *configyang.ValidationError
		if !errors.As(failures[i], &refusal) {
			t.Fatalf("failure %d lost its typed refusal: %v", i, failures[i])
		}
		want := "top item a " + leaf
		if got := strings.Join(refusal.Segments(), " "); got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
	tree, err := p.Parse(input)
	if err == nil {
		t.Fatal("normal Parse accepted an invalid value after diagnostic collection")
	}
	if tree != nil {
		t.Fatal("normal Parse returned a usable invalid tree")
	}
	if strings.Contains(err.Error(), "words:") {
		t.Errorf("normal Parse no longer stops at its first error: %v", err)
	}
	tree, failures = p.ParseDiagnostics(strings.ReplaceAll(input, "purple", "red"))
	if len(failures) > 0 {
		t.Fatalf("accepted values were refused: %v", failures)
	}
	if tree == nil {
		t.Fatal("valid input returned no tree")
	}
}

// TestParseDiagnosticsStopsAtStructuralErrors proves collection cannot skip
// an unknown field or an unsynchronized scalar statement to accept a file.
func TestParseDiagnosticsStopsAtStructuralErrors(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		count    int
		lastText string
	}{
		{"unknown field", "top { item a { color purple; unknown red; words purple; } }", 2, "unknown field in item: unknown"},
		{"missing terminator", "top { item a { color purple words red; } }", 1, `color: "purple" is not an accepted value`},
		{"refused key", "top { shade purple { label red; } item a { color purple; } }", 1, `shade purple: "purple" is not an accepted value`},
		{"extra terminator", "top { item a { words purple;; color purple; } }", 2, "expected keyword in item block, got SEMICOLON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, failures := NewParser(refusalSchema()).ParseDiagnostics(tc.input)
			if tree != nil {
				t.Fatal("structural failure returned a usable tree")
			}
			if len(failures) != tc.count {
				t.Fatalf("got %d failures, want %d: %v", len(failures), tc.count, failures)
			}
			if !strings.Contains(failures[len(failures)-1].Error(), tc.lastText) {
				t.Errorf("last failure does not name %q: %v", tc.lastText, failures)
			}
		})
	}
}

// TestParseDiagnosticsCollectsMissingValue checks the missing enum's
// unconsumed terminator and the next independent value refusal.
func TestParseDiagnosticsCollectsMissingValue(t *testing.T) {
	input := "top { item a { color; words purple; } }"
	tree, failures := NewParser(refusalSchema()).ParseDiagnostics(input)
	if tree != nil {
		t.Fatal("missing value returned a usable tree")
	}
	if len(failures) != 2 {
		t.Fatalf("got %d failures, want 2: %v", len(failures), failures)
	}
	if !strings.Contains(failures[0].Error(), "color: needs a value") {
		t.Errorf("missing-value refusal lost: %v", failures[0])
	}
	if !strings.Contains(failures[1].Error(), `words: "purple" is not an accepted value`) {
		t.Errorf("next refusal lost: %v", failures[1])
	}
}

// TestParseDiagnosticsCollectsInactiveScalars covers the leaf-list paths
// that parse inactive members without entering parseNode themselves.
func TestParseDiagnosticsCollectsInactiveScalars(t *testing.T) {
	input := "top { item a { inactive: color purple; inactive: tags purple; inactive: modes purple; } }"
	tree, failures := NewParser(refusalSchema()).ParseDiagnostics(input)
	if tree != nil {
		t.Fatal("inactive value failures returned a usable tree")
	}
	if len(failures) != 3 {
		t.Fatalf("got %d failures, want 3: %v", len(failures), failures)
	}
	for i, leaf := range []string{"color", "tags", "modes"} {
		var refusal *configyang.ValidationError
		if !errors.As(failures[i], &refusal) {
			t.Fatalf("failure %d lost its typed refusal: %v", i, failures[i])
		}
		if got := strings.Join(refusal.Segments(), " "); got != "top item a "+leaf {
			t.Errorf("inactive path = %q", got)
		}
	}
}

// TestParseSetDiagnosticsCollectsLines checks both set forms, their typed
// paths, a nil invalid tree and the strict parser's unchanged first refusal.
func TestParseSetDiagnosticsCollectsLines(t *testing.T) {
	for _, format := range []ConfigFormat{FormatSet, FormatSetMeta} {
		input := "set top item a color purple\nset top item a words purple\n"
		if format == FormatSetMeta {
			input = "#alice " + strings.ReplaceAll(input, "\nset ", "\n#alice set ")
		}
		tree, failures := ParseSetDiagnostics(refusalSchema(), input, format)
		if tree != nil {
			t.Fatal("set failures returned a usable tree")
		}
		if len(failures) != 2 {
			t.Fatalf("format %v: got %d failures, want 2: %v", format, len(failures), failures)
		}
		for i, leaf := range []string{"color", "words"} {
			var refusal *configyang.ValidationError
			if !errors.As(failures[i], &refusal) {
				t.Fatalf("failure %d lost its typed refusal: %v", i, failures[i])
			}
			if got := strings.Join(refusal.Segments(), " "); got != "top item a "+leaf {
				t.Errorf("path = %q", got)
			}
		}
		_, first := ParseSetFormat(refusalSchema(), input, format)
		if first == nil {
			t.Fatal("strict parser accepted an invalid value")
		}
		if strings.Contains(first.Error(), "words:") {
			t.Errorf("strict parser no longer stops at first failure: %v", first)
		}
		tree, failures = ParseSetDiagnostics(refusalSchema(), strings.ReplaceAll(input, "purple", "red"), format)
		if len(failures) > 0 {
			t.Fatalf("accepted values refused: %v", failures)
		}
		if tree == nil {
			t.Fatal("accepted set input returned no tree")
		}
	}
}

// TestParseSetDiagnosticsStopsAtStructuralErrors proves a syntax failure
// cannot be skipped to reach a later line, in either set form.
func TestParseSetDiagnosticsStopsAtStructuralErrors(t *testing.T) {
	for _, format := range []ConfigFormat{FormatSet, FormatSetMeta} {
		input := "set top item a color purple\nset top unknown red\nset top item a words purple\n"
		if format == FormatSetMeta {
			input = "#alice " + strings.ReplaceAll(input, "\nset ", "\n#alice set ")
		}
		tree, failures := ParseSetDiagnostics(refusalSchema(), input, format)
		if tree != nil {
			t.Fatal("set syntax failure returned a usable tree")
		}
		if len(failures) != 2 {
			t.Fatalf("format %v: got %d failures, want 2: %v", format, len(failures), failures)
		}
		var refusal *configyang.ValidationError
		if errors.As(failures[1], &refusal) {
			t.Fatalf("structural error was skipped for a later value failure: %v", failures)
		}
		if !strings.Contains(failures[1].Error(), "unknown") {
			t.Errorf("structural error lost: %v", failures[1])
		}
	}
}
