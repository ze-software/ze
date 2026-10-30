// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc tree`
// Related: priority.go -- the corpus, the declared tiers and the order this renders
// Related: next.go -- the next gap, read from the same tiers
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// noSummaryCell is the cell a node with a text and no summary renders in
// place of a figure: nobody has listed its requirements. Only the whole-corpus
// tree and the candidates hold such a node; no profile member is one.
const noSummaryCell = "no summary"

// candidatesHeading introduces the candidates, and says why none is owed.
const candidatesHeading = "candidates, not owed: one builds-on or updates edge from a member, listed in " +
	"neither tier. A normative citation is never an obligation; a summary's `Use cases` row makes one owed\n"

// TreeRow is one node of the tree: one RFC or draft with both figures.
type TreeRow struct {
	Stem  string `json:"stem"`
	Title string `json:"title"`
	// Level is the graph level, nil for a summary whose text is not in the
	// corpus.
	Level *int `json:"level"`
	// Component is the other members of the stem's citation cycle, which
	// share its level.
	Component []string `json:"component"`
	Aggregate string   `json:"aggregate"`
	BuildsOn  []string `json:"builds-on"`
	UpdatedBy []string `json:"updated-by"`
	// Summary is whether rfc/short/ holds a summary. A node without one has
	// a text the graph reached and no requirement list.
	Summary             bool           `json:"summary"`
	Walked              bool           `json:"walked"`
	MandatoryProven     int            `json:"mandatory-proven"`
	MandatoryTotal      int            `json:"mandatory-total"`
	OptionalProven      int            `json:"optional-proven"`
	OptionalTotal       int            `json:"optional-total"`
	MandatoryAttainable int            `json:"mandatory-attainable"`
	OptionalAttainable  int            `json:"optional-attainable"`
	MandatoryExcluded   map[string]int `json:"mandatory-excluded"`
	OptionalExcluded    map[string]int `json:"optional-excluded"`
	// Profiles is the use-case profiles the node's summary is required by,
	// and OptionalProfiles those listing it as optional.
	Profiles         []string `json:"profiles"`
	OptionalProfiles []string `json:"optional-profiles"`
	// Path is the declaration that puts the node in the named profile, empty
	// without one.
	Path string `json:"path"`
}

// TreeReport is the answer of `./le rfc tree`.
type TreeReport struct {
	// Profile is the profile the tree is limited to, empty for the corpus.
	Profile string `json:"profile"`
	// Title is the profile file's Title, and Heading an aggregate's Heading
	// (empty for a use case); both are empty for the whole corpus.
	Title   string `json:"title"`
	Heading string `json:"heading"`
	Kind    string `json:"kind"`
	Release bool   `json:"release"`
	// Rollup sums the profile's required members, the only tier its
	// percentage counts; nil for the whole corpus, which is no set the owner
	// rule judges.
	Rollup *ScopeRollup `json:"rollup"`
	Rows   []*TreeRow   `json:"rows"`
	// Optional is a profile's optional tier with its own rollup: a use-case
	// profile's `Optional use cases` members, or an aggregate's members whose
	// `Aggregate tier` is optional. Nil for the whole corpus.
	Optional *TreeTier `json:"optional"`
	// Candidates is the RFCs one edge from a member of a use-case profile
	// that neither tier lists: advisory, never owed and never counted. Empty
	// for the whole corpus and for an aggregate.
	Candidates []TreeCandidate `json:"candidates"`
}

// TreeTier is one tier of a profile: its rollup and its rows.
type TreeTier struct {
	Rollup ScopeRollup `json:"rollup"`
	Rows   []*TreeRow  `json:"rows"`
}

// TreeCandidate is one RFC a member leans on that no tier lists, with the
// first member and edge that name it.
type TreeCandidate struct {
	Stem    string `json:"stem"`
	Title   string `json:"title"`
	Level   *int   `json:"level"`
	Summary bool   `json:"summary"`
	// Via is the member and the edge, such as `rfc7454 builds-on rfc5082`.
	Via string `json:"via"`
}

// IsAggregate answers whether the report is a technology aggregate's, whose
// members are the RFCs naming it in `Support` or `Aggregate`.
func (r *TreeReport) IsAggregate() bool { return r.Kind == profileKindTechnology }

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
// node of the graph and every summary is a row, and the rows some release
// profile requires come first. With a use-case profile the rows are its
// required members, then its optional tier and its candidates; with an
// aggregate, the summaries naming it.
func (c *priorityCorpus) tree(profile string) (*TreeReport, error) {
	useCases := c.useCaseScopes(false)
	report := &TreeReport{Candidates: []TreeCandidate{}}
	if profile == "" {
		union := map[string]bool{}
		for _, stem := range c.graph.Nodes() {
			union[stem] = true
		}
		for stem := range c.figures {
			union[stem] = true
		}
		inRelease := map[string]bool{}
		for index := range useCases {
			if !useCases[index].release {
				continue
			}
			for stem := range useCases[index].members {
				inRelease[stem] = true
			}
		}
		report.Rows = c.allRows(sortedSet(union), useCases, inRelease)
		return report, nil
	}
	scope, err := c.scope(profile, "tree")
	if err != nil {
		return nil, err
	}
	report.Profile, report.Kind, report.Release = scope.name, scope.kind, scope.release
	report.Title, report.Heading = c.profileTitles(profile)
	rollup := c.rollupOfScopes([]profileScope{scope})
	report.Rollup = &rollup
	report.Rows = c.treeRows(useCases, &scope, false)
	report.Optional = &TreeTier{Rollup: c.optionalRollupOfScopes([]profileScope{scope}),
		Rows: c.treeRows(useCases, &scope, true)}
	if scope.kind == profileKindTechnology {
		return report, nil
	}
	report.Candidates = c.candidates(&scope)
	return report, nil
}

// treeRows answers the rows of the tier of named that optional selects, each
// with the declaration that puts it there, ordered by level, then by number.
// The stems are read out of the tier, so every path names a real listing.
func (c *priorityCorpus) treeRows(useCases []profileScope, named *profileScope, optional bool) []*TreeRow {
	stems := sortedSet(named.tier(optional))
	rows := make([]*TreeRow, 0, len(stems))
	for _, stem := range stems {
		row := c.treeRow(stem, useCases)
		row.Path = named.path(stem, optional)
		rows = append(rows, row)
	}
	sortTreeRows(rows, nil)
	return rows
}

// allRows answers the rows of stems with no profile named, so with no path,
// ordered rows inRelease holds first, then by level, then by number.
func (c *priorityCorpus) allRows(stems []string, useCases []profileScope, inRelease map[string]bool) []*TreeRow {
	rows := make([]*TreeRow, 0, len(stems))
	for _, stem := range stems {
		rows = append(rows, c.treeRow(stem, useCases))
	}
	sortTreeRows(rows, inRelease)
	return rows
}

// sortTreeRows orders rows inRelease holds first, then by level, then by
// number. A nil inRelease holds nothing.
func sortTreeRows(rows []*TreeRow, inRelease map[string]bool) {
	slices.SortFunc(rows, func(left, right *TreeRow) int {
		if inRelease[left.Stem] != inRelease[right.Stem] {
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
}

// candidates answers the builds-on targets and updaters of every member of
// scope, in either tier, that neither tier lists: one hop, each once with the
// first member (in RFC number order) and edge naming it, sorted by level and
// then by number. Transitive reach is the over-reach the declared tiers
// replaced, so a candidate's own edges are not followed.
func (c *priorityCorpus) candidates(scope *profileScope) []TreeCandidate {
	listed := map[string]bool{}
	for stem := range scope.members {
		listed[stem] = true
	}
	for stem := range scope.optional {
		listed[stem] = true
	}
	members := sortedSet(listed)
	slices.SortFunc(members, statusRowCompare)
	out := []TreeCandidate{}
	named := map[string]bool{}
	add := func(stem, via string) {
		if listed[stem] {
			return
		}
		if named[stem] {
			return
		}
		named[stem] = true
		_, summarized := c.figures[stem]
		out = append(out, TreeCandidate{Stem: stem, Title: c.metas[stem].Title, Level: c.level(stem),
			Summary: summarized, Via: via})
	}
	for _, member := range members {
		for _, target := range c.buildsOn[member] {
			add(target, member+" builds-on "+target)
		}
		for _, updater := range c.updaters[member] {
			add(updater, member+" updated-by "+updater)
		}
	}
	slices.SortFunc(out, func(left, right TreeCandidate) int {
		if order := compareLevels(left.Level, right.Level); order != 0 {
			return order
		}
		return statusRowCompare(left.Stem, right.Stem)
	})
	return out
}

// profileTitles answers a profile's Title and, for an aggregate, its Heading.
// MUST be called with a name scope accepted.
func (c *priorityCorpus) profileTitles(name string) (title, heading string) {
	if useCase, held := c.profiles.UseCases[name]; held {
		return useCase.Title, ""
	}
	aggregate, held := c.profiles.Aggregates[name]
	if !held {
		panic("BUG: priorityCorpus.profileTitles asked for " + name + ", which scope did not accept")
	}
	return aggregate.Title, aggregate.Heading
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
		MandatoryAttainable: figures.Mandatory.Attainable, OptionalAttainable: figures.Optional.Attainable,
		MandatoryExcluded: figures.Mandatory.Excluded, OptionalExcluded: figures.Optional.Excluded,
		Profiles: []string{}, OptionalProfiles: []string{}}
	if members, held := c.graph.Component(stem); held {
		for _, member := range members {
			if member != stem {
				row.Component = append(row.Component, member)
			}
		}
	}
	for index := range useCases {
		if useCases[index].members[stem] {
			row.Profiles = append(row.Profiles, useCases[index].name)
		}
		if useCases[index].optional[stem] {
			row.OptionalProfiles = append(row.OptionalProfiles, useCases[index].name)
		}
	}
	return row
}

// Cells answers the row's two figure cells in the state the node is in. It
// reads only the exported fields, so a row read back from JSON (the site's
// published ledger) renders the cells `./le rfc tree` prints.
func (r *TreeRow) Cells() (mandatory, optional string) {
	if !r.Summary {
		return noSummaryCell, noSummaryCell
	}
	figures := r.figures()
	return figures.mandatoryCell(), figures.optionalCell()
}

// CeilingCells answers ceiling cells with the same measurement boundary as Cells.
func (r *TreeRow) CeilingCells() (mandatory, optional string) {
	if !r.Summary {
		return noSummaryCell, noSummaryCell
	}
	if !r.Walked {
		return notWalked, notWalked
	}
	figures := r.figures()
	return figures.Mandatory.CeilingString(), figures.Optional.CeilingString()
}

// figures reads the counts carried by a row, including one decoded from JSON.
func (r *TreeRow) figures() StemFigures {
	return StemFigures{Stem: r.Stem, Walked: r.Walked,
		Mandatory: Figure{Proven: r.MandatoryProven, Total: r.MandatoryTotal,
			Attainable: r.MandatoryAttainable, Excluded: r.MandatoryExcluded},
		Optional: Figure{Proven: r.OptionalProven, Total: r.OptionalTotal,
			Attainable: r.OptionalAttainable, Excluded: r.OptionalExcluded}}
}

// Text renders the tree, one line per node, for a reader at a terminal: the
// required rows under the profile's rollup, then the optional tier under its
// own rollup, then the candidates.
func (r *TreeReport) Text() string {
	var tb textbuf.Buffer
	if r.Profile == "" {
		tb.Str("RFC dependency tree, every text and summary; rows a release profile requires first\n")
	} else {
		tb.Str("RFC tree of ").Str(r.Kind).Str(" profile ").Str(r.Profile)
		if r.Release {
			tb.Str(" (release)")
		}
		writeScopeRollup(&tb, ": ", r.Rollup)
	}
	for _, row := range r.Rows {
		writeTreeRow(&tb, row)
	}
	if r.Optional != nil {
		writeScopeRollup(&tb, "optional members, not counted: ", &r.Optional.Rollup)
		for _, row := range r.Optional.Rows {
			writeTreeRow(&tb, row)
		}
	}
	if len(r.Candidates) != 0 {
		tb.Str(candidatesHeading)
		for index := range r.Candidates {
			candidate := &r.Candidates[index]
			tb.Str("  ").Str(levelCell(candidate.Level)).Byte(' ').Str(candidate.Stem).
				Str("  via ").Str(candidate.Via)
			if candidate.Title != "" {
				tb.Str("  ").Str(candidate.Title)
			}
			tb.Byte('\n')
		}
	}
	return tb.String()
}

// writeScopeRollup renders one rollup line after label.
func writeScopeRollup(tb *textbuf.Buffer, label string, rollup *ScopeRollup) {
	tb.Str(label).Str(rollup.String())
	if rollup.MandatoryComplete {
		tb.Str("; mandatory complete")
	}
	tb.Byte('\n')
}

// writeTreeRow renders one row and its path.
func writeTreeRow(tb *textbuf.Buffer, row *TreeRow) {
	mandatory, optional := row.Cells()
	tb.Str("  ").Str(levelCell(row.Level)).Byte(' ').Str(row.Stem)
	if len(row.Component) != 0 {
		tb.Str(" (cycle with ").Str(strings.Join(row.Component, ", ")).Byte(')')
	}
	tb.Str("  mandatory ").Str(mandatory).Str(", optional ").Str(optional)
	mandatoryCeiling, optionalCeiling := row.CeilingCells()
	tb.Str("; ceiling mandatory ").Str(mandatoryCeiling).Str(", optional ").Str(optionalCeiling)
	if row.Summary && row.Walked {
		figures := row.figures()
		writeFigureExclusions(tb, "mandatory", figures.Mandatory)
		writeFigureExclusions(tb, "optional", figures.Optional)
	}
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

// levelCell renders a level, `L?` for a node with no text to level.
func levelCell(level *int) string {
	var tb textbuf.Buffer
	if level == nil {
		return "L?"
	}
	return tb.Byte('L').Int(int64(*level)).String()
}
