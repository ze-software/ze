// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc tree`
// Related: priority.go -- the corpus, the declared tiers and the order this renders
// Related: next.go -- the next gap, read from the same tiers
package rfc

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// noSummaryCell is the cell a node with a text and no summary renders in
// place of a figure: nobody has listed its requirements. Only the whole-corpus
// tree and the candidates hold such a node; no profile member is one.
const noSummaryCell = "no summary"

// noRowsCell is the state of a figure that counts no row: 0 of 0 is neither
// complete nor empty, so no number is printed for it.
const noRowsCell = "no rows"

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
	Summary bool `json:"summary"`
	// StemFigures is the stem's figures held whole, as figuresByStem answered
	// them, so no count can be dropped between the measurement and the row.
	// It is promoted (row.Walked, row.Mandatory) and published through
	// MarshalJSON, never by its own field names.
	StemFigures `json:"-"`
	// Sections is the stem's two bars (sectionsOf over StemFigures), nil for
	// a node with no summary or an unwalked one, which publish no figure.
	// MarshalJSON publishes it under `sections`, and a row read back from
	// JSON holds it as published.
	Sections *Sections `json:"-"`
	// Profiles is the use-case profiles the node's summary is required by,
	// and OptionalProfiles those listing it as optional.
	Profiles         []string `json:"profiles"`
	OptionalProfiles []string `json:"optional-profiles"`
	// Path is the declaration that puts the node in the named profile, empty
	// without one.
	Path string `json:"path"`
}

// treeRowFields is TreeRow without its methods, so the JSON forms below
// encode its own keys without recursing into MarshalJSON.
type treeRowFields TreeRow

// treeRowJSON is the published form of a TreeRow: its own keys, then the
// figure keys the site and `./le rfc tree | json` read. The section counts are
// published whole under `sections`; the figures' Attainable and Excluded, and
// the OS and intent splits they come from, are not published apart, so a row
// decoded from JSON holds them zero and no TreeRow method reads them.
type treeRowJSON struct {
	*treeRowFields
	Walked          bool      `json:"walked"`
	MandatoryProven int       `json:"mandatory-proven"`
	MandatoryTotal  int       `json:"mandatory-total"`
	OptionalProven  int       `json:"optional-proven"`
	OptionalTotal   int       `json:"optional-total"`
	Sections        *Sections `json:"sections"`
	// MandatoryByKind is the proven mandatory rows by the strongest evidence
	// kind behind each, summing to MandatoryProven. Unmet is the ids the
	// summary or an audit says are unmet, published walked or not.
	MandatoryByKind map[string]int `json:"mandatory-by-kind"`
	Unmet           []string       `json:"unmet"`
}

// MarshalJSON publishes the row with its figures under the flat keys.
func (r *TreeRow) MarshalJSON() ([]byte, error) {
	unmet := r.Unmet
	if unmet == nil {
		unmet = []string{}
	}
	return json.Marshal(treeRowJSON{treeRowFields: (*treeRowFields)(r), Walked: r.Walked,
		MandatoryProven: r.Mandatory.Proven, MandatoryTotal: r.Mandatory.Total,
		OptionalProven: r.Optional.Proven, OptionalTotal: r.Optional.Total,
		Sections: r.Sections, MandatoryByKind: r.MandatoryByKind, Unmet: unmet})
}

// UnmarshalJSON reads a published row back, so its cells render as
// `./le rfc tree` printed them.
func (r *TreeRow) UnmarshalJSON(data []byte) error {
	wire := treeRowJSON{treeRowFields: (*treeRowFields)(r)}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.StemFigures = StemFigures{Stem: r.Stem, Walked: wire.Walked,
		Mandatory:       Figure{Proven: wire.MandatoryProven, Total: wire.MandatoryTotal},
		Optional:        Figure{Proven: wire.OptionalProven, Total: wire.OptionalTotal},
		MandatoryByKind: wire.MandatoryByKind, Unmet: wire.Unmet}
	r.Sections = wire.Sections
	return nil
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
func (r *TreeReport) IsAggregate() bool { return r.Kind == ProfileKindTechnology }

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
	if scope.kind == ProfileKindTechnology {
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
		Summary: summarized, StemFigures: figures,
		Profiles: []string{}, OptionalProfiles: []string{}}
	if summarized && figures.Walked {
		sections := sectionsOf(&figures)
		row.Sections = &sections
	}
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

// sectionsCell renders the row's sections, or the state that publishes none:
// `no summary`, `not walked`, or `no rows` for a walked RFC that counts no
// MUST and no SHOULD row. A bar of zeros would read as a measurement, so the
// cell says what the site's rfc-figures.js says for the same RFC.
func sectionsCell(row *TreeRow) string {
	if !row.Summary {
		return noSummaryCell
	}
	if row.Sections == nil {
		return notWalked
	}
	if row.Sections.Must.Total+row.Sections.Should.Total == 0 {
		return noRowsCell
	}
	return row.Sections.String()
}

// writeTreeRow renders one row and its path.
func writeTreeRow(tb *textbuf.Buffer, row *TreeRow) {
	tb.Str("  ").Str(levelCell(row.Level)).Byte(' ').Str(row.Stem)
	if len(row.Component) != 0 {
		tb.Str(" (cycle with ").Str(strings.Join(row.Component, ", ")).Byte(')')
	}
	tb.Str("  ").Str(sectionsCell(row))
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

// ProofCell answers the Proof cell docs/features/rfc-status.md prints for
// this RFC: `N of M MUST proven`, or `not walked`.
func (r *TreeRow) ProofCell() string { return proofCell(r.StemFigures) }

// EvidenceCell answers the Evidence cell: the proven mandatory rows by
// strongest evidence kind, or `-`.
func (r *TreeRow) EvidenceCell() string { return evidenceCell(r.StemFigures) }

// UnmetCell answers the Unmet cell: the unmet requirement ids, or `-`.
func (r *TreeRow) UnmetCell() string { return unmetCell(r.StemFigures) }
