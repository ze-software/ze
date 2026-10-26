// Design: docs/contributing/rfc-conformance-gates.md -- the priority tree, `./le rfc next`
// Related: priority.go -- the corpus, the closure and the order this reads
// Related: tree.go -- the whole tree the next gap is the first gap of
//
// next.go answers the next gap the owner rule of 2026-10-25 orders: the
// lowest level of the closure holding an uncovered row, mandatory rows until
// the closure's mandatory figure is complete, optional rows after. A row an
// updating RFC rewrites (`{updated-by}`) is ordered at the updater's level.
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
	Absent  bool   `json:"absent"`
	// Uncovered is the rows of the register owed and not proven.
	Uncovered []NextRow `json:"uncovered"`
	// Profiles is the judged profiles whose closure holds the RFC, and Path
	// why the first of them does.
	Profiles []string `json:"profiles"`
	Path     string   `json:"path"`
}

// NextReport is the answer of `./le rfc next`.
type NextReport struct {
	// Profiles is the profiles judged: the named one, or every release one.
	Profiles []string `json:"profiles"`
	// Register is `mandatory` until the closure's mandatory figure is
	// complete, `optional` after.
	Register          string      `json:"register"`
	MandatoryComplete bool        `json:"mandatory-complete"`
	Complete          bool        `json:"complete"`
	Level             *int        `json:"level"`
	Component         []string    `json:"component"`
	Gaps              []NextGap   `json:"gaps"`
	Absent            []string    `json:"absent"`
	Rollup            ScopeRollup `json:"rollup"`
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
		Absent: absentStems(scopes), Rollup: c.rollupOfScopes(scopes)}
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
		if report.Register == registerMandatory && len(report.Absent) != 0 {
			for _, stem := range report.Absent {
				report.Gaps = append(report.Gaps, NextGap{Stem: stem, Absent: true,
					Uncovered: []NextRow{}, Profiles: scopesHolding(scopes, stem), Path: absentPath})
			}
			return report, nil
		}
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

// gapCandidates answers every RFC of the scopes that owes something in the
// register, keyed by the level it is owed at. In the mandatory register a
// node with no summary and a summary with no extraction sign-off owe their
// walk at their own level, whatever their rows say: the rollup is incomplete
// while either exists. A row is owed at its own RFC's level, or at its
// updater's when it carries `{updated-by}`.
func (c *priorityCorpus) gapCandidates(scopes []profileScope, mandatory bool) map[gapKey]*NextGap {
	union := map[string]bool{}
	for index := range scopes {
		for stem := range scopes[index].reached {
			union[stem] = true
		}
	}
	absent := absentStemSet(scopes)
	out := map[gapKey]*NextGap{}
	at := func(level *int, stem string) *NextGap {
		key := keyOf(level, stem)
		if gap, held := out[key]; held {
			return gap
		}
		figures, summarized := c.figures[stem]
		gap := &NextGap{Stem: stem, Title: c.metas[stem].Title, Absent: absent[stem], Summary: summarized, Walked: figures.Walked,
			Uncovered: []NextRow{}, Profiles: scopesHolding(scopes, stem), Path: firstPath(scopes, stem)}
		out[key] = gap
		return gap
	}
	if mandatory {
		for _, stem := range sortedSet(union) {
			figures, summarized := c.figures[stem]
			if !summarized || !figures.Walked {
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
		owed, met := rowCounts(requirement, c.proven)
		if !owed {
			continue
		}
		if met {
			continue
		}
		row := NextRow{ID: requirement.RID, Level: requirement.Level}
		if requirement.Annotation != nil {
			row.Annotation = requirement.Annotation.Kind
		}
		level := c.level(requirement.RFC)
		if requirement.UpdatedBy != nil {
			row.UpdatedBy = requirement.UpdatedBy.Target
			if updater, shaped := ridStem(requirement.UpdatedBy.Target); shaped {
				level = c.level(updater)
			}
		}
		gap := at(level, requirement.RFC)
		gap.Uncovered = append(gap.Uncovered, row)
	}
	return out
}

// scopesHolding answers the names of the scopes whose closure holds stem or
// lists it absent.
func scopesHolding(scopes []profileScope, stem string) []string {
	out := []string{}
	for index := range scopes {
		if _, held := scopes[index].reached[stem]; held {
			out = append(out, scopes[index].name)
			continue
		}
		if slices.Contains(absentStems(scopes[index:index+1]), stem) {
			out = append(out, scopes[index].name)
		}
	}
	return out
}

// firstPath answers why the first scope holding stem holds it.
func firstPath(scopes []profileScope, stem string) string {
	for index := range scopes {
		if _, held := scopes[index].reached[stem]; held {
			return scopes[index].path(stem)
		}
	}
	return ""
}

// Text renders the next gap for a reader at a terminal.
func (r *NextReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("next RFC gap for ").Str(strings.Join(r.Profiles, ", ")).Str(": ").Str(r.Rollup.String()).Byte('\n')
	if r.Complete {
		tb.Str("every mandatory and optional row the closure reaches is proven: nothing is owed\n")
		return tb.String()
	}
	if r.MandatoryComplete {
		tb.Str("mandatory is complete: every mandatory row is proven, so the next gap is optional\n")
	}
	// The fallback lists only absent RFCs and sets no level; an absent RFC a
	// closure reaches is a gap at its own level, among the others.
	if len(r.Gaps) != 0 && r.Gaps[0].Absent && r.Level == nil {
		tb.Str("every RFC in the corpus is covered; these RFCs are absent from it at 0%: ").
			Str("fetch each text into rfc/full/ and summarize it (ai/skills/ze-rfc.md)\n")
	} else {
		tb.Str(r.Register).Str(" gap at ").Str(levelCell(r.Level))
		if len(r.Component) != 0 {
			tb.Str(", citation cycle ").Str(strings.Join(r.Component, ", "))
		}
		tb.Byte('\n')
	}
	for index := range r.Gaps {
		writeNextGap(&tb, &r.Gaps[index])
	}
	if len(r.Absent) != 0 {
		tb.Str("absent at 0%: ").Str(strings.Join(r.Absent, ", ")).Byte('\n')
	}
	return tb.String()
}

// absentPath is the path of an absent RFC no closure reaches: the profile
// lists it, and no edge leads to it.
const absentPath = "listed under Absent"

// writeNextGap renders one RFC of the gap and what it owes.
func writeNextGap(tb *textbuf.Buffer, gap *NextGap) {
	tb.Str("  ").Str(gap.Stem)
	if gap.Title != "" {
		tb.Str("  ").Str(gap.Title)
	}
	tb.Byte('\n')
	if gap.Absent && gap.Path == absentPath {
		tb.Str("      serves ").Str(strings.Join(gap.Profiles, ", ")).Byte('\n')
		return
	}
	switch {
	case gap.Absent:
		tb.Str("      absent from the corpus at 0%: write rfc/short/").Str(gap.Stem).Str(".md (ai/skills/ze-rfc.md)\n")
	case !gap.Summary:
		tb.Str("      no summary: write rfc/short/").Str(gap.Stem).Str(".md (ai/skills/ze-rfc.md)\n")
	case !gap.Walked:
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
