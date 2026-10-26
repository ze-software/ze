// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc next`
// Related: priority.go -- the corpus, the declared tiers and the order this reads
// Related: tree.go -- the whole tree the next gap is the first gap of
//
// next.go answers the next gap the owner rule of 2026-10-25 orders: the
// lowest level of the required members holding an uncovered row, mandatory
// rows until their mandatory figure is complete, optional-register rows
// after. A row an updating RFC rewrites (`{updated-by}`) is ordered at the
// updater's level. Members of the optional tier never answer the gap: they
// are listed in their own section, not counted.
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two registers a gap is judged in.
const (
	registerMandatory = "mandatory"
	registerOptional  = "optional"
)

// NextRow is one uncovered requirement row.
type NextRow struct {
	ID    string `json:"id"`
	Level string `json:"level"`
	// Annotation is the row's coverage annotation, empty for none: a
	// `{not-applicable}` row stays owed and is never proven (owner ruling,
	// 2026-09-02), so it is named rather than hidden.
	Annotation string `json:"annotation"`
	// UpdatedBy is the updater's requirement id when the row carries
	// `{updated-by}`, which is what put it at the updater's level.
	UpdatedBy string `json:"updated-by"`
}

// NextGap is one RFC of the next gap and what it owes.
type NextGap struct {
	Stem    string `json:"stem"`
	Title   string `json:"title"`
	Summary bool   `json:"summary"`
	Walked  bool   `json:"walked"`
	// Uncovered is the rows of the register owed and not proven; for an
	// optional-tier member, the rows of both registers.
	Uncovered []NextRow `json:"uncovered"`
	// Profiles is the judged profiles listing the RFC in the gap's tier, and
	// Path the declaration that puts it in the first of them.
	Profiles []string `json:"profiles"`
	Path     string   `json:"path"`
}

// NextReport is the answer of `./le rfc next`.
type NextReport struct {
	// Profiles is the profiles judged: the named one, or every release one.
	Profiles []string `json:"profiles"`
	// Register is `mandatory` until the required members' mandatory figure
	// is complete, `optional` after.
	Register          string      `json:"register"`
	MandatoryComplete bool        `json:"mandatory-complete"`
	Complete          bool        `json:"complete"`
	Level             *int        `json:"level"`
	Component         []string    `json:"component"`
	Gaps              []NextGap   `json:"gaps"`
	Rollup            ScopeRollup `json:"rollup"`
	// Optional is the optional tier: its own rollup, and each member owing a
	// row, roots to leaves. It never answers the gap and never enters Rollup.
	Optional NextOptional `json:"optional"`
}

// NextOptional is the optional tier of the judged profiles, not counted.
type NextOptional struct {
	Rollup ScopeRollup `json:"rollup"`
	Gaps   []NextGap   `json:"gaps"`
}

// nextReport answers the next gap of one checkout, inside profile when it is
// not empty, across every release profile otherwise.
func nextReport(tree, profile string) (*NextReport, error) {
	corpus, err := loadPriorityCorpus(tree)
	if err != nil {
		return nil, err
	}
	return corpus.next(profile)
}

// gapKey places one candidate: its level, known or not, then its stem.
type gapKey struct {
	leveled bool
	level   int
	stem    string
}

// keyOf answers the key of stem owing at level.
func keyOf(level *int, stem string) gapKey {
	if level == nil {
		return gapKey{stem: stem}
	}
	return gapKey{leveled: true, level: *level, stem: stem}
}

// levelPointer answers the key's level, nil when it has none.
func (k gapKey) levelPointer() *int {
	if !k.leveled {
		return nil
	}
	level := k.level
	return &level
}

// next answers the next gap.
func (c *priorityCorpus) next(profile string) (*NextReport, error) {
	scopes := c.useCaseScopes(true)
	if profile != "" {
		scope, err := c.scope(profile, "next")
		if err != nil {
			return nil, err
		}
		scopes = []profileScope{scope}
	}
	report := &NextReport{Profiles: []string{}, Component: []string{}, Gaps: []NextGap{},
		Rollup:   c.rollupOfScopes(scopes),
		Optional: NextOptional{Rollup: c.optionalRollupOfScopes(scopes), Gaps: c.optionalGaps(scopes)}}
	for index := range scopes {
		report.Profiles = append(report.Profiles, scopes[index].name)
	}
	report.MandatoryComplete = report.Rollup.MandatoryComplete
	report.Register = registerMandatory
	if report.MandatoryComplete {
		report.Register = registerOptional
	}
	candidates := c.gapCandidates(scopes, report.Register == registerMandatory)
	if len(candidates) == 0 {
		report.Complete = true
		return report, nil
	}
	keys := make([]gapKey, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right gapKey) int {
		if order := compareLevels(left.levelPointer(), right.levelPointer()); order != 0 {
			return order
		}
		return statusRowCompare(left.stem, right.stem)
	})
	first := keys[0]
	report.Level = first.levelPointer()
	component, leveled := c.graph.Component(first.stem)
	if !leveled {
		component = []string{first.stem}
	}
	if len(component) > 1 {
		report.Component = component
	}
	for _, key := range keys {
		if key.leveled != first.leveled {
			continue
		}
		if key.level != first.level {
			continue
		}
		if !slices.Contains(component, key.stem) {
			continue
		}
		report.Gaps = append(report.Gaps, *candidates[key])
	}
	return report, nil
}

// gapCandidates answers every required member of the scopes that owes
// something in the register, keyed by the level it is owed at. In the
// mandatory register a summary with no extraction sign-off owes its walk at
// its own level, whatever its rows say: the rollup is incomplete while one
// exists. A row is owed at its own RFC's level, or at its updater's when it
// carries `{updated-by}`.
func (c *priorityCorpus) gapCandidates(scopes []profileScope, mandatory bool) map[gapKey]*NextGap {
	union := requiredUnion(scopes)
	out := map[gapKey]*NextGap{}
	at := func(level *int, stem string) *NextGap {
		key := keyOf(level, stem)
		if gap, held := out[key]; held {
			return gap
		}
		gap := c.nextGap(stem, scopes, requiredTier)
		out[key] = gap
		return gap
	}
	if mandatory {
		for _, stem := range sortedSet(union) {
			if !c.figures[stem].Walked {
				at(c.level(stem), stem)
			}
		}
	}
	for index := range c.requirements {
		requirement := &c.requirements[index]
		if !union[requirement.RFC] {
			continue
		}
		if requirement.Gated() != mandatory {
			continue
		}
		row, owed := c.uncoveredRow(requirement)
		if !owed {
			continue
		}
		level := c.level(requirement.RFC)
		if requirement.UpdatedBy != nil {
			if updater, shaped := ridStem(requirement.UpdatedBy.Target); shaped {
				level = c.level(updater)
			}
		}
		gap := at(level, requirement.RFC)
		gap.Uncovered = append(gap.Uncovered, row)
	}
	return out
}

// optionalGaps answers every optional-tier member that owes something, in
// either register: unwalked, or holding an uncovered row. Roots to leaves,
// then RFC number.
func (c *priorityCorpus) optionalGaps(scopes []profileScope) []NextGap {
	union := optionalUnion(scopes)
	byStem := map[string]*NextGap{}
	for _, stem := range sortedSet(union) {
		if !c.figures[stem].Walked {
			byStem[stem] = c.nextGap(stem, scopes, optionalTier)
		}
	}
	for index := range c.requirements {
		requirement := &c.requirements[index]
		if !union[requirement.RFC] {
			continue
		}
		row, owed := c.uncoveredRow(requirement)
		if !owed {
			continue
		}
		gap, held := byStem[requirement.RFC]
		if !held {
			gap = c.nextGap(requirement.RFC, scopes, optionalTier)
			byStem[requirement.RFC] = gap
		}
		gap.Uncovered = append(gap.Uncovered, row)
	}
	stems := sortedKeysOf(byStem)
	slices.SortFunc(stems, func(left, right string) int {
		if order := compareLevels(c.level(left), c.level(right)); order != 0 {
			return order
		}
		return statusRowCompare(left, right)
	})
	out := make([]NextGap, 0, len(stems))
	for _, stem := range stems {
		out = append(out, *byStem[stem])
	}
	return out
}

// memberTier names which of a scope's two tiers a gap is read from.
type memberTier uint8

const (
	// tierUnspecified is the zero value, which no gap is built with.
	tierUnspecified memberTier = iota
	requiredTier
	optionalTier
)

// nextGap answers stem's gap with no row yet, naming the scopes that list it
// in tier and the declaration of the first of them.
func (c *priorityCorpus) nextGap(stem string, scopes []profileScope, tier memberTier) *NextGap {
	figures, summarized := c.figures[stem]
	gap := &NextGap{Stem: stem, Title: c.metas[stem].Title, Summary: summarized, Walked: figures.Walked,
		Uncovered: []NextRow{}, Profiles: []string{}}
	for index := range scopes {
		var listed bool
		switch tier {
		case requiredTier:
			listed = scopes[index].members[stem]
		case optionalTier:
			listed = scopes[index].optional[stem]
		case tierUnspecified:
			panic("BUG: nextGap asked for " + stem + " with no tier")
		}
		if !listed {
			continue
		}
		if gap.Path == "" {
			gap.Path = scopes[index].path(stem)
		}
		gap.Profiles = append(gap.Profiles, scopes[index].name)
	}
	return gap
}

// uncoveredRow answers requirement's row when it is owed and not proven.
func (c *priorityCorpus) uncoveredRow(requirement *Requirement) (NextRow, bool) {
	owed, met := rowCounts(requirement, c.proven)
	if !owed {
		return NextRow{}, false
	}
	if met {
		return NextRow{}, false
	}
	row := NextRow{ID: requirement.RID, Level: requirement.Level}
	if requirement.Annotation != nil {
		row.Annotation = requirement.Annotation.Kind
	}
	if requirement.UpdatedBy != nil {
		row.UpdatedBy = requirement.UpdatedBy.Target
	}
	return row, true
}

// Text renders the next gap for a reader at a terminal, then the optional
// tier under its own rollup.
func (r *NextReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("next RFC gap for ").Str(strings.Join(r.Profiles, ", ")).Str(": ").Str(r.Rollup.String()).Byte('\n')
	switch {
	case r.Complete:
		tb.Str("every mandatory and optional row of the required members is proven: nothing is owed\n")
	case r.MandatoryComplete:
		tb.Str("mandatory is complete: every mandatory row is proven, so the next gap is optional\n")
	}
	if !r.Complete {
		tb.Str(r.Register).Str(" gap at ").Str(levelCell(r.Level))
		if len(r.Component) != 0 {
			tb.Str(", citation cycle ").Str(strings.Join(r.Component, ", "))
		}
		tb.Byte('\n')
	}
	for index := range r.Gaps {
		writeNextGap(&tb, &r.Gaps[index])
	}
	if r.Optional.Rollup.RFCs == 0 {
		return tb.String()
	}
	tb.Str("optional, not counted: ").Str(r.Optional.Rollup.String()).Byte('\n')
	for index := range r.Optional.Gaps {
		writeNextGap(&tb, &r.Optional.Gaps[index])
	}
	return tb.String()
}

// writeNextGap renders one RFC of the gap and what it owes.
func writeNextGap(tb *textbuf.Buffer, gap *NextGap) {
	tb.Str("  ").Str(gap.Stem)
	if gap.Title != "" {
		tb.Str("  ").Str(gap.Title)
	}
	tb.Byte('\n')
	if !gap.Walked {
		tb.Str("      not walked: no valid extraction sign-off, run ./le rfc extraction create stem ").
			Str(gap.Stem).Byte('\n')
	}
	if len(gap.Uncovered) != 0 {
		tb.Str("      ").Int(int64(len(gap.Uncovered))).Str(" uncovered: ")
		for index := range gap.Uncovered {
			if index > 0 {
				tb.Str(", ")
			}
			row := &gap.Uncovered[index]
			tb.Str(row.ID)
			if row.Annotation != "" {
				tb.Str(" {").Str(row.Annotation).Byte('}')
			}
			if row.UpdatedBy != "" {
				tb.Str(" (updated by ").Str(row.UpdatedBy).Byte(')')
			}
		}
		tb.Byte('\n')
	}
	tb.Str("      serves ").Str(strings.Join(gap.Profiles, ", ")).Str("; via ").Str(gap.Path).Byte('\n')
}
