// VALIDATES: the one split of an RFC's figures into the two bars' sections:
// a row a layer under Ze performs is OS verified or an OS gap at every level,
// never not owed, and every other exclusion kind is not owed by its own name.
// PREVENTS: a lower-layer row counted as not owed, which would move a row the
// stack owes out of the MUST bar (owner, 2026-10-31).

package rfc

import (
	"reflect"
	"strings"
	"testing"
)

// TestSplitExcludedKeepsTheOSRowsByKind proves the split answers the rows a
// lower layer performs apart from the rows not owed, each by its own
// annotation kind, and that the SHOULD sections count those rows OS verified
// or OS gap, never not owed (owner, 2026-10-31), while every other kind stays
// not owed.
//
// Method: one lower-layer and one not-applicable count; then the SHOULD split
// of figures whose optional exclusions hold both. It moved here from
// internal/le/site with the split itself, which the site now reads.
func TestSplitExcludedKeepsTheOSRowsByKind(t *testing.T) {
	excluded := map[string]int{AnnotationLowerLayer: 5, AnnotationNotApplicable: 4}
	if rows := osRowsOf(excluded); rows != 5 {
		t.Errorf("OS rows: %d, want the 5 lower-layer rows", rows)
	}
	if notOwed := notOwedByKind(excluded); !reflect.DeepEqual(notOwed, map[string]int{AnnotationNotApplicable: 4}) {
		t.Errorf("not-owed rows by kind: %v", notOwed)
	}
	figures := StemFigures{Stem: "rfc9999", Walked: true,
		Mandatory: Figure{Proven: 1, Total: 1, Attainable: 1},
		Optional: Figure{Proven: 1, Total: 10, Attainable: 5, Excluded: map[string]int{
			AnnotationNotApplicable: 2, AnnotationLowerLayer: 3,
		}},
		NotYet: 2, NotEver: 1, NotDecided: 1, OptionalOSVerified: 1, OptionalOSGap: 2}
	sections, err := SectionsOf("rfc9999", &figures)
	if err != nil {
		t.Fatal(err)
	}
	should := sections.Should
	want := map[string]int{AnnotationNotApplicable: 2}
	if should.NotOwed != 2 || !reflect.DeepEqual(should.NotOwedByKind, want) {
		t.Errorf("SHOULD not owed %d by kind %v, want 2 by kind %v: a lower-layer row is never not owed", should.NotOwed, should.NotOwedByKind, want)
	}
	if should.OSVerified != 1 || should.OSGap != 2 {
		t.Errorf("SHOULD OS verified %d and OS gap %d, want 1 and 2", should.OSVerified, should.OSGap)
	}
}

// TestSectionsOfRefusesAnUndeclaredKind proves a figure excluding rows under a
// kind the annotation declaration does not name is refused by stem: its
// meaning is unknown, so no section can hold it.
func TestSectionsOfRefusesAnUndeclaredKind(t *testing.T) {
	figures := StemFigures{Stem: "rfc9999", Walked: true,
		Mandatory: Figure{Total: 1, Excluded: map[string]int{"no-such-kind": 1}}}
	if _, err := SectionsOf("rfc9999", &figures); err == nil {
		t.Fatal("SectionsOf split a row excluded under an undeclared kind")
	}
}

// TestSectionsOfRefusesFiguresThatDoNotAddBack checks the contract that lets
// rollupOf and treeRow call sectionsOf unchecked. Goal: SectionsOf refuses
// every figure set whose sections do not add back to their own totals, each
// with the refusal of the check that caught it, and accepts the same figures
// once they do add back. Method: one figure set per refusal of
// checkMustSections and checkShouldSections, each consistent but for the one
// count it breaks, then a consistent set.
func TestSectionsOfRefusesFiguresThatDoNotAddBack(t *testing.T) {
	lowerLayer := func(rows int) map[string]int { return map[string]int{AnnotationLowerLayer: rows} }
	for _, one := range []struct {
		name    string
		figures StemFigures
		says    string
	}{
		{"MUST OS rows", StemFigures{Mandatory: Figure{Total: 2, Excluded: lowerLayer(2)}, OSVerified: 1},
			"the OS verified and OS gap MUST rows (1) differ from the MUST rows a lower layer performs (2)"},
		{"MUST proven above attainable", StemFigures{Mandatory: Figure{Total: 2, Attainable: 1, Proven: 2}},
			"the Ze-proven MUST rows (2) exceed the MUST rows Ze's code owes (1)"},
		{"MUST sections", StemFigures{Mandatory: Figure{Total: 5, Attainable: 2}},
			"the MUST sections (2) differ from the MUST rows counted (5)"},
		{"SHOULD OS rows", StemFigures{Optional: Figure{Total: 1, Excluded: lowerLayer(1)}},
			"the OS verified and OS gap SHOULD rows (0) differ from the SHOULD rows a lower layer performs (1)"},
		{"SHOULD intent", StemFigures{Optional: Figure{Total: 2, Attainable: 2}, NotYet: 1},
			"the not ever, not yet and not decided SHOULD rows (1) differ from the open SHOULD rows (2)"},
		{"SHOULD sections", StemFigures{Optional: Figure{Total: 5, Attainable: 2}, NotYet: 2},
			"the SHOULD sections (2) differ from the SHOULD rows counted (5)"},
	} {
		t.Run(one.name, func(t *testing.T) {
			figures := one.figures
			figures.Stem, figures.Walked = "rfc9999", true
			_, err := SectionsOf("rfc9999", &figures)
			if err == nil {
				t.Fatalf("SectionsOf split figures that do not add back: %+v", figures)
			}
			for _, says := range []string{"rfc9999", one.says} {
				if !strings.Contains(err.Error(), says) {
					t.Errorf("the refusal %q does not say %q", err, says)
				}
			}
		})
	}
	consistent := StemFigures{Stem: "rfc9999", Walked: true,
		Mandatory:  Figure{Total: 5, Attainable: 2, Proven: 1, Excluded: map[string]int{AnnotationLowerLayer: 2, AnnotationNotApplicable: 1}},
		OSVerified: 1, OSGap: 1,
		Optional:      Figure{Total: 3, Attainable: 2, Excluded: lowerLayer(1)},
		OptionalOSGap: 1, NotYet: 1, NotDecided: 1}
	if _, err := SectionsOf("rfc9999", &consistent); err != nil {
		t.Errorf("SectionsOf refused figures that add back: %v", err)
	}
}
