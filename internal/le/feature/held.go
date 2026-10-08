// Design: docs/contributing/feature-maturity.md -- the ceiling, and a held Supported level
// Related: runrecord.go -- the run states a stale run is read from
// Related: check.go -- the verdicts the warnings land in
//
// Owner decision 2026-10-08, superseding the 2026-10-07 staleness rule: "a
// feature once supported needs to remain supported". Promotion to Supported
// still needs every counted run current. A run that goes stale afterwards,
// older than runAgeDaysMax days or its test or scenario changed, never lowers
// the level: the check accepts it and names the stale run as a warning.
//
// "Once supported" is read from HEAD. The check before a commit judges the
// working tree against HEAD, so the commit that raises Level to supported is
// judged as a promotion and owes current runs; once it lands, HEAD holds the
// level and a later stale run only warns. A declaration HEAD does not hold, or
// holds below Supported, is a promotion.

package feature

import (
	"strings"
)

// stale reports whether the answer is a recorded green run that stopped being
// current: its content changed or it aged. A test never run is not stale.
func (a runAnswer) stale() bool {
	switch a.state {
	case runStateChanged, runStateAged:
		return true
	case runStateUnspecified, runStateNotRun, runStateCurrent:
		return false
	}
	panic("BUG: a run answer holds an unknown run state")
}

// warnStale files reason as a warning of verdict, and answers true, when the
// run behind it is stale and HEAD already holds d at Supported. Otherwise it
// answers false and the caller files reason as unmet. A HEAD that cannot be
// read is a refusal, never a held level.
func (in *evidence) warnStale(d *Declaration, verdict *Verdict, answer runAnswer, reason string) bool {
	if !answer.stale() {
		return false
	}
	if d.Level != LevelSupported {
		return false
	}
	held, err := in.holdsSupported(d)
	if err != nil {
		verdict.Refusals = append(verdict.Refusals, "cannot read the Level HEAD holds: "+err.Error())
		return false
	}
	if !held {
		return false
	}
	verdict.Warnings = append(verdict.Warnings, reason)
	return true
}

// holdsSupported answers whether HEAD's copy of d declares Level supported.
// Only the Level row is read, so a HEAD copy written under an older field
// vocabulary still answers.
func (in *evidence) holdsSupported(d *Declaration) (bool, error) {
	if held, cached := in.held[d.ID]; cached {
		return held, nil
	}
	rel := declarationDir + "/" + d.ID + ".md"
	listed, err := gitOutput(in.tree, "ls-tree", "--name-only", "HEAD", "--", rel)
	if err != nil {
		return false, err
	}
	held := false
	if listed != "" {
		text, err := gitOutput(in.tree, "show", "HEAD:"+rel)
		if err != nil {
			return false, err
		}
		held = levelCell(text) == LevelSupported.String()
	}
	in.held[d.ID] = held
	return held, nil
}

// levelCell answers the Level cell of a declaration's Meta table, or "".
func levelCell(text string) string {
	section, found := sectionBody(text, "## Meta")
	if !found {
		return ""
	}
	for line := range strings.SplitSeq(section, "\n") {
		field, value, ok := tableRow(line)
		if !ok {
			continue
		}
		if field == fieldLevel {
			return value
		}
	}
	return ""
}
