// Design: docs/contributing/gh-pages.md -- Quality pages
// Related: rfcreader.go -- the glossary under test

package site

import (
	"strings"
	"testing"
)

// TestGlossaryDefinesBarTerms proves the reader glossary defines every word
// the two bars show exactly once, and carries no retired ceiling entry
// (spec-rfc-figures-sectioned-presentation, AC-14). Method: walk rfcBars, the
// one declaration of both bars, and count the glossary terms that name each
// segment label; then refuse a meaning for a segment no bar draws, and any
// term or meaning that still speaks of a ceiling.
func TestGlossaryDefinesBarTerms(t *testing.T) {
	glossary := rfcGlossary()
	drawn := map[string]bool{}
	for _, segment := range append(append([]rfcBarSegment{}, rfcBars.Must...), rfcBars.Overview...) {
		drawn[segment.Segment] = true
		meaning := rfcSegmentMeanings[segment.Segment]
		if meaning == "" {
			t.Errorf("bar segment %q (%q) has no glossary meaning", segment.Segment, segment.Label)
		}
		named := 0
		for _, term := range glossary {
			for label := range strings.SplitSeq(strings.TrimSuffix(term.Term, " (bar segment)"), ", ") {
				if label == segment.Label {
					named++
					if term.Meaning != meaning {
						t.Errorf("term %q carries %q, want the meaning of segment %q", term.Term, term.Meaning, segment.Segment)
					}
				}
			}
		}
		if named != 1 {
			t.Errorf("label %q is named by %d glossary terms, want exactly 1", segment.Label, named)
		}
	}
	for name := range rfcSegmentMeanings {
		if !drawn[name] {
			t.Errorf("glossary defines segment %q, which neither bar draws", name)
		}
	}
	for _, term := range glossary {
		if strings.Contains(strings.ToLower(term.Term+" "+term.Meaning), "ceiling") {
			t.Errorf("glossary term %q still speaks of the retired ceiling", term.Term)
		}
	}
}

// TestGlossaryStatesOSBlockedNeverCountsAgainstZe proves the linux-limit
// entry says an OS-blocked row is never counted against Ze (AC-14), the owner
// rule the old "stays in the denominator" wording contradicted.
func TestGlossaryStatesOSBlockedNeverCountsAgainstZe(t *testing.T) {
	for _, term := range rfcGlossary() {
		if term.Term != "Linux cannot do this" {
			continue
		}
		for _, want := range []string{"OS blocked", "never counted against Ze"} {
			if !strings.Contains(term.Meaning, want) {
				t.Errorf("linux-limit entry %q lacks %q", term.Meaning, want)
			}
		}
		return
	}
	t.Fatal("the glossary has no linux-limit entry")
}
