// Design: docs/contributing/rfc-conformance-gates.md -- clause-scoped evidence.
// Related: render.go -- RequirementRows renders each polarity's actual citations.
package rfc

import (
	"strings"
	"testing"
)

// Partial scope qualifies existing citations; it must not turn a missing
// polarity into evidence. Render authored fixtures with either, both or
// neither polarity and check the citation and empty-cell boundaries together.
func TestPartialCitationCellsKeepMissingPolaritiesEmpty(t *testing.T) {
	for _, test := range []struct {
		name     string
		positive bool
		negative bool
	}{
		{name: "positive_only", positive: true},
		{name: "negative_only", negative: true},
		{name: "both", positive: true, negative: true},
		{name: "neither"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := partialFiles()
			var tags []string
			if test.positive {
				tags = append(tags, "# RFC requirement: "+selftestRIDSend+" positive -- receipt accepts zero")
			}
			if test.negative {
				tags = append(tags, "# RFC requirement: "+selftestRIDSend+" negative -- receipt ignores nonzero")
			}
			files[selftestCIPath] = strings.Join(tags, "\n") + "\nexpect=stdout:contains=OK\n"
			root := checkFixtureTree(t, files)
			collected, err := Collect(root)
			if err != nil {
				t.Fatal(err)
			}
			in, err := NewRenderInput(root, collected, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			rows := RequirementRows(in)[selftestStem]
			if len(rows) != 1 {
				t.Fatalf("rendered %d requirement rows, want one", len(rows))
			}
			for _, polarity := range []struct {
				name    string
				cell    string
				present bool
			}{
				{name: PolarityPositive, cell: rows[0].Positive, present: test.positive},
				{name: PolarityNegative, cell: rows[0].Negative, present: test.negative},
			} {
				if !polarity.present {
					if polarity.cell != "--" {
						t.Errorf("missing %s citation renders %q, want --", polarity.name, polarity.cell)
					}
					continue
				}
				if !strings.HasPrefix(polarity.cell, "scoped evidence: `"+selftestCIPath+"`") {
					t.Errorf("present %s citation lost its scope or source: %q", polarity.name, polarity.cell)
				}
			}
		})
	}
}
