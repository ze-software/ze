package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestTreeDiffGutterHighlightsLineBelowComments checks that, with the diff
// gutter on, a validation error lands on the statement it names when comments
// sit above that statement. Validation line numbers index the working content,
// which Serialize prints with every comment, while the tree diff prints none:
// counting diff lines as working lines shifted the highlight one line up for
// every comment line above the statement. Method: build the working content
// the editor holds (parse, then Serialize, of a commented configuration with
// one change against the baseline), put an error on the working line of
// `peer A {`, run the gutter and the highlighter as setViewportData does, and
// require that exactly the display line holding `peer A {` is styled.
func TestTreeDiffGutterHighlightsLineBelowComments(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	original := "bgp {\n\trouter-id 192.0.2.1\n\tpeer A {\n\t}\n}\n"
	source := "# head\n\nbgp { # open\n\t# lead one\n\t# lead two\n\trouter-id 192.0.2.2 # trail\n\t# about peer A\n\tpeer A { # entry\n\t}\n\t# closing\n} # after\n"
	tree, err := config.NewParser(schema).Parse(source)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	working := config.Serialize(tree, schema)

	errorLine := 0
	for i, line := range strings.Split(working, "\n") {
		if strings.Contains(line, "peer A {") {
			errorLine = i + 1
		}
	}
	if errorLine == 0 {
		t.Fatalf("working content holds no `peer A {` line:\n%s", working)
	}

	content, lineMapping := annotateContentWithTreeDiff(original, working, schema)
	errs := []ConfigValidationError{{Message: "boom", Severity: severityError, Line: errorLine}}
	highlighted := strings.Split(highlightValidationIssues(content, errs, nil, lineMapping, false), "\n")
	plain := strings.Split(content, "\n")
	if len(highlighted) != len(plain) {
		t.Fatalf("highlighting changed the line count: %d, want %d", len(highlighted), len(plain))
	}

	styled := 0
	for i := range plain {
		if highlighted[i] == plain[i] {
			continue
		}
		styled++
		if !strings.Contains(plain[i], "peer A {") {
			t.Errorf("error for working line %d styled display line %d %q, want the `peer A {` line\ndisplay:\n%s\nworking:\n%s",
				errorLine, i+1, plain[i], content, working)
		}
	}
	if styled != 1 {
		t.Errorf("%d display lines styled, want 1\ndisplay:\n%s\nworking:\n%s", styled, content, working)
	}
}
