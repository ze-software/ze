package fixture

import (
	"slices"
	"strings"
	"testing"
)

// leTreeSectionsFailure runs body and answers the fixture failure it raised,
// or "" when it raised none.
func leTreeSectionsFailure(t *testing.T, body func()) (message string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			failure, ok := recovered.(leRFCAnswersFailure)
			if !ok {
				panic(recovered)
			}
			message = failure.message
		}
	}()
	body()
	return ""
}

// leTreeSectionsSample is two section lines in the shape rfc.Sections.String
// renders, a rollup line and a row line, with the bar's names as given.
func leTreeSectionsSample(must string) string {
	clause := "MUST 9 kept: " + must + "; MUST not owed 1, not-applicable 1; SHOULD 4: proven 1, not built 3; SHOULD not owed 0"
	return "RFC tree of use-case profile x: 2 walked; " + clause + "\n  L0 rfc1  " + clause + "  [agg]\n  L1 rfc2  not walked\n"
}

// TestLeTreeSectionWordsAreReadFromTheTreeText proves the fixture derives its
// vocabulary from the tree's text: the MUST and SHOULD names in their printed
// order, with the counts and the not-owed clauses left out.
func TestLeTreeSectionWordsAreReadFromTheTreeText(t *testing.T) {
	var words leTreeSectionWords
	if failure := leTreeSectionsFailure(t, func() {
		words = leTreeSectionWordsOf(leTreeSectionsSample("Ze proven 2, OS verified 0, Proof pending 7"))
	}); failure != "" {
		t.Fatalf("a well-formed tree was refused: %s", failure)
	}
	if want := []string{"Ze proven", "OS verified", "Proof pending"}; !slices.Equal(words.must, want) {
		t.Fatalf("MUST words %q, want %q", words.must, want)
	}
	if want := []string{"proven", "not built"}; !slices.Equal(words.should, want) {
		t.Fatalf("SHOULD words %q, want %q", words.should, want)
	}
}

// TestLeTreeSectionWordsRefuseLinesThatDisagree proves two section lines that
// name different MUST sections are refused, as is a tree with no section line.
func TestLeTreeSectionWordsRefuseLinesThatDisagree(t *testing.T) {
	text := leTreeSectionsSample("Ze proven 2, Not built 7") + "  L2 rfc3  " +
		"MUST 1 kept: Ze proven 1; MUST not owed 0; SHOULD 0: proven 0; SHOULD not owed 0\n"
	if failure := leTreeSectionsFailure(t, func() { leTreeSectionWordsOf(text) }); !strings.Contains(failure, "MUST sections") {
		t.Fatalf("disagreeing lines were not refused: %q", failure)
	}
	if failure := leTreeSectionsFailure(t, func() { leTreeSectionWordsOf("  L0 rfc1  not walked\n") }); failure == "" {
		t.Fatal("a tree with no section line was accepted")
	}
}

// TestLeTreeHelpMustNameEveryWordInOrder proves the help check accepts a page
// naming every word in the tree's order, whatever its case, and refuses one
// that omits a word, names two out of order, or names a SHOULD state only
// inside a MUST word.
func TestLeTreeHelpMustNameEveryWordInOrder(t *testing.T) {
	words := leTreeSectionWords{must: []string{"Ze proven", "Not built"}, should: []string{"proven", "not built"}}
	cases := []struct {
		page   string
		accept bool
	}{
		{"the MUST bar (ze proven, not built) and the SHOULD states (proven, not built)", true},
		// "proven" inside "Ze proven" sits before the MUST words end, so it
		// cannot stand for the SHOULD state.
		{"the MUST bar (Ze proven, Not built) and the SHOULD states (not built)", false},
		{"the MUST bar (Ze proven) and the SHOULD states (proven)", false},
		{"the MUST bar (Not built, Ze proven) and the SHOULD states (proven, not built)", false},
	}
	for _, c := range cases {
		failure := leTreeSectionsFailure(t, func() { leTreeRequireNamedInOrder(c.page, words) })
		if (failure == "") != c.accept {
			t.Errorf("page %q: accept=%v, failure %q", c.page, c.accept, failure)
		}
	}
}
