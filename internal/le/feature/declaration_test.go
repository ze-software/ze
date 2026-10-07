// Related: declaration.go -- the parser these cases drive
//
// VALIDATES: Parse refuses a missing required field and a value outside the
// vocabulary, naming the field and listing the vocabulary from its one
// declaration (AC-1, AC-2).
// PREVENTS: a misspelled or absent field read as an empty, passing one.

package feature

import (
	"strings"
	"testing"
)

func TestParseAcceptsTheFixture(t *testing.T) {
	declaration, err := Parse(passingDeclaration, "widget", "features/widget.md")
	if err != nil {
		t.Fatal(err)
	}
	if declaration.Level != LevelSupported {
		t.Fatalf("level %v", declaration.Level)
	}
	if len(declaration.RealPathTests) != 2 {
		t.Fatalf("real-path tests %v", declaration.RealPathTests)
	}
}

func TestParseDeclarationRefusesMissingField(t *testing.T) {
	cases := map[string]string{
		"| Kind | daemon |\n":                "Kind is required",
		"| Level | supported |\n":            "Level is required",
		"| Components | internal/widget |\n": "Components is required",
		"| Name | Widgets |\n":               "Name is required",
	}
	for row, want := range cases {
		_, err := Parse(strings.Replace(passingDeclaration, row, "", 1), "widget", "features/widget.md")
		if err == nil {
			t.Fatalf("removing %q was accepted", row)
		}
		if !strings.Contains(err.Error(), "features/widget.md: "+want) {
			t.Fatalf("removing %q: %v", row, err)
		}
	}
	partial := strings.Replace(passingDeclaration, "| Scope | complete |", "| Scope | partial |", 1)
	if _, err := Parse(partial, "widget", "w"); err == nil {
		t.Fatal("partial scope without Scope gaps was accepted")
	}
}

func TestParseDeclarationRefusesUnknownVocabulary(t *testing.T) {
	_, err := Parse(strings.Replace(passingDeclaration, "| Level | supported |", "| Level | great |", 1), "widget", "w")
	if err == nil {
		t.Fatal("an unknown level was accepted")
	}
	if !strings.Contains(err.Error(), "'great' is not one of: "+strings.Join(LevelNames(), ", ")) {
		t.Fatalf("refusal does not list the vocabulary: %v", err)
	}
	_, err = Parse(strings.Replace(passingDeclaration, "| Name |", "| Title |", 1), "widget", "w")
	if err == nil {
		t.Fatal("an unknown field was accepted")
	}
	if !strings.Contains(err.Error(), "unknown Meta field 'Title'") {
		t.Fatalf("refusal does not name the field: %v", err)
	}
}

func TestParseRefusesDateOnlyAttestation(t *testing.T) {
	text := strings.Replace(passingDeclaration, "2026-10-07: the page and the row prose read against Send", "2026-10-07", 1)
	if _, err := Parse(text, "widget", "w"); err == nil {
		t.Fatal("a date-only Doc review was accepted")
	}
}

func TestStatusLabelRendersPartial(t *testing.T) {
	if got := StatusLabel(ScopePartial, LevelExperimental); got != "Experimental (partial)" {
		t.Fatalf("got %q", got)
	}
	if got := StatusLabel(ScopeFuture, LevelUnspecified); got != "Future" {
		t.Fatalf("got %q", got)
	}
}
