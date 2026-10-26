// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc tree`
// Related: priority.go -- the corpus, the closure and the order this renders
// Related: next.go -- the next gap, read from the same closure
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// noSummaryCell is the cell a node with a text and no summary renders in
// place of a figure: nobody has listed its requirements.
const noSummaryCell = "no summary"

// TreeRow is one node of the tree: one RFC or draft with both figures.
type TreeRow struct {
	Stem  string `json:"stem"`
	Title string `json:"title"`
	// Level is the graph level, nil for a summary whose text is not in the
	// corpus and for an absent RFC.
	Level *int `json:"level"`
	// Component is the other members of the stem's citation cycle, which
	// share its level.
	Component []string `json:"component"`
	Aggregate string   `json:"aggregate"`
	BuildsOn  []string `json:"builds-on"`
	UpdatedBy []string `json:"updated-by"`
	// Summary is whether rfc/short/ holds a summary. A node without one has
	// a text the graph reached and no requirement list.
	Summary         bool `json:"summary"`
	Absent          bool `json:"absent"`
	Walked          bool `json:"walked"`
	MandatoryProven int  `json:"mandatory-proven"`
	MandatoryTotal  int  `json:"mandatory-total"`
	OptionalProven  int  `json:"optional-proven"`
	OptionalTotal   int  `json:"optional-total"`
	// Profiles is the use-case profiles whose closure holds the node.
	Profiles []string `json:"profiles"`
	// Path is why the node is in the named profile, empty without one.
	Path string `json:"path"`

	figures StemFigures
}

// TreeReport is the answer of `./le rfc tree`.
type TreeReport struct {
	// Profile is the profile the tree is limited to, empty for the corpus.
	Profile string `json:"profile"`
	Kind    string `json:"kind"`
	Release bool   `json:"release"`
	// Rollup sums the profile's members; nil for the whole corpus, which is
	// no set the owner rule judges.
	Rollup *ScopeRollup `json:"rollup"`
	Rows   []*TreeRow   `json:"rows"`
}

// treeReport answers the tree of one checkout, limited to profile when it is
// not empty.
func treeReport(tree, profile string) (*TreeReport, error) {
	corpus, err := loadPriorityCorpus(tree)
	if err != nil {
		return nil, err
	}
	return corpus.tree(profile)
}

// tree answers the rows, ordered roots to leaves. Without a profile every
// node of the graph and every summary is a row, and the nodes some release
// profile reaches come first; with one, its closure and its absent RFCs are,
// and the absent ones come last because they have no text to level.
func (c *priorityCorpus) tree(profile string) (*TreeReport, error) {
	useCases := c.useCaseScopes(false)
	report := &TreeReport{}
	var stems []string
	var named *profileScope
	if profile == "" {
		union := map[string]bool{}
		for _, stem := range c.graph.Nodes() {
			union[stem] = true
		}
		for stem := range c.figures {
			union[stem] = true
		}
		stems = sortedSet(union)
	} else {
		scope, err := c.scope(profile)
		if err != nil {
			return nil, err
		}
		named = &scope
		report.Profile, report.Kind, report.Release = scope.name, scope.kind, scope.release
		rollup := c.rollupOfScopes([]profileScope{scope})
		report.Rollup = &rollup
		stems = sortedKeysOf(scope.reached)
	}
	inRelease := map[string]bool{}
	for _, stem := range stems {
		row := c.treeRow(stem, useCases)
		for index := range useCases {
			if useCases[index].release && slices.Contains(row.Profiles, useCases[index].name) {
				inRelease[stem] = true
			}
		}
		if named != nil {
			row.Path = named.path(stem)
		}
		report.Rows = append(report.Rows, row)
	}
	slices.SortFunc(report.Rows, func(left, right *TreeRow) int {
		if profile == "" && inRelease[left.Stem] != inRelease[right.Stem] {
			if inRelease[left.Stem] {
				return -1
			}
			return 1
		}
		if order := compareLevels(left.Level, right.Level); order != 0 {
			return order
		}
		return statusRowCompare(left.Stem, right.Stem)
	})
	if named != nil {
		for _, number := range named.absent {
			absent := absentFigures(number)
			report.Rows = append(report.Rows, &TreeRow{Stem: absent.Stem, Absent: true,
				Component: []string{}, BuildsOn: []string{}, UpdatedBy: []string{},
				Profiles: []string{named.name}, Path: named.name + ": listed under Absent",
				figures: absent})
		}
	}
	return report, nil
}

// treeRow answers one node's row, without its path.
func (c *priorityCorpus) treeRow(stem string, useCases []profileScope) *TreeRow {
	figures, summarized := c.figures[stem]
	row := &TreeRow{Stem: stem, Title: c.metas[stem].Title, Level: c.level(stem),
		Component: []string{}, Aggregate: c.metas[stem].Aggregate,
		BuildsOn: append([]string{}, c.buildsOn[stem]...), UpdatedBy: append([]string{}, c.updaters[stem]...),
		Summary: summarized, Walked: figures.Walked,
		MandatoryProven: figures.Mandatory.Proven, MandatoryTotal: figures.Mandatory.Total,
		OptionalProven: figures.Optional.Proven, OptionalTotal: figures.Optional.Total,
		Profiles: []string{}, figures: figures}
	if members, held := c.graph.Component(stem); held {
		for _, member := range members {
			if member != stem {
				row.Component = append(row.Component, member)
			}
		}
	}
	for index := range useCases {
		if _, held := useCases[index].reached[stem]; held {
			row.Profiles = append(row.Profiles, useCases[index].name)
		}
	}
	return row
}

// cells answers the row's two figure cells in the state the node is in.
func (r *TreeRow) cells() (mandatory, optional string) {
	if !r.Summary && !r.Absent {
		return noSummaryCell, noSummaryCell
	}
	return r.figures.mandatoryCell(), r.figures.optionalCell()
}

// Text renders the tree, one line per node, for a reader at a terminal.
func (r *TreeReport) Text() string {
	var tb textbuf.Buffer
	if r.Profile == "" {
		tb.Str("RFC dependency tree, every text and summary; nodes a release profile reaches first\n")
	} else {
		tb.Str("RFC tree of ").Str(r.Kind).Str(" profile ").Str(r.Profile)
		if r.Release {
			tb.Str(" (release)")
		}
		tb.Str(": ").Str(r.Rollup.String())
		if r.Rollup.MandatoryComplete {
			tb.Str("; mandatory complete")
		}
		tb.Byte('\n')
	}
	for index := range r.Rows {
		row := r.Rows[index]
		mandatory, optional := row.cells()
		tb.Str("  ").Str(levelCell(row.Level)).Byte(' ').Str(row.Stem)
		if len(row.Component) != 0 {
			tb.Str(" (cycle with ").Str(strings.Join(row.Component, ", ")).Byte(')')
		}
		tb.Str("  mandatory ").Str(mandatory).Str(", optional ").Str(optional)
		if row.Aggregate != "" {
			tb.Str("  [").Str(row.Aggregate).Byte(']')
		}
		if row.Title != "" {
			tb.Str("  ").Str(row.Title)
		}
		if row.Path != "" {
			tb.Str("\n      via ").Str(row.Path)
		}
		tb.Byte('\n')
	}
	return tb.String()
}

// levelCell renders a level, `L?` for a node with no text to level.
func levelCell(level *int) string {
	var tb textbuf.Buffer
	if level == nil {
		return "L?"
	}
	return tb.Byte('L').Int(int64(*level)).String()
}
