// Design: docs/contributing/feature-maturity.md -- `./le feature report` and `./le feature check`
// Related: check.go -- the verdicts these payloads carry
// Related: actions.go -- the verbs that answer them

package feature

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ReportEntry is one feature in `./le feature report`, with kebab-case keys.
type ReportEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Scope    string   `json:"scope"`
	Level    string   `json:"level,omitempty"`
	Ceiling  string   `json:"ceiling,omitempty"`
	Status   string   `json:"status"`
	Next     string   `json:"next-level,omitempty"`
	NextGaps []string `json:"next-level-unmet,omitempty"`
	// Promotion is true when the evidence supports a level above the declared
	// one (AC-12): reported, never refused.
	Promotion bool     `json:"promotion-candidate"`
	Refusals  []string `json:"refusals,omitempty"`
	Bounds    []string `json:"bounds,omitempty"`
	// Warnings name the stale recorded runs of a Supported level HEAD holds:
	// the level stands, and the runs want re-recording.
	Warnings []string `json:"warnings,omitempty"`
}

// CheckReport is the answer of `./le feature check`.
type CheckReport struct {
	Declarations int      `json:"declarations"`
	Refused      []string `json:"refused,omitempty"`
}

// entryOf renders one verdict as a report entry.
func entryOf(verdict *Verdict) ReportEntry {
	d := &verdict.Declaration
	entry := ReportEntry{
		ID: d.ID, Name: d.Name, Kind: d.Kind.String(), Scope: d.Scope.String(),
		Status: StatusLabel(d.Scope, d.Level), Refusals: verdict.Refusals, Bounds: verdict.Bounds,
		Warnings: verdict.Warnings,
	}
	if !d.Scope.Implemented() {
		return entry
	}
	entry.Level = d.Level.String()
	entry.Ceiling = verdict.Ceiling.String()
	entry.Promotion = verdict.Ceiling > d.Level
	if d.Level < LevelSupported {
		next := d.Level + 1
		entry.Next = next.String()
		entry.NextGaps = verdict.Unmet[next]
	}
	return entry
}

// Report answers one entry per declaration, or the one named by id.
func Report(tree, id string) ([]ReportEntry, error) {
	verdicts, problems, err := Check(tree)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	entries := make([]ReportEntry, 0, len(verdicts))
	for i := range verdicts {
		if id != "" {
			if verdicts[i].Declaration.ID != id {
				continue
			}
		}
		entries = append(entries, entryOf(&verdicts[i]))
	}
	if id != "" {
		if len(entries) == 0 {
			return nil, errors.New("no declaration features/" + id + ".md")
		}
	}
	return entries, nil
}

// Judge answers the check report and whether anything was refused. A tree with
// no declaration is refused: an empty population proves nothing, and reading
// it as "every feature passes" is the silent zero (ai/rules/principles.md).
func Judge(tree string) (CheckReport, error) {
	verdicts, problems, err := Check(tree)
	if err != nil {
		return CheckReport{}, err
	}
	report := CheckReport{Declarations: len(verdicts)}
	for _, problem := range problems {
		report.Refused = append(report.Refused, problem.Error())
	}
	for i := range verdicts {
		for _, reason := range verdicts[i].Refusals {
			var tb textbuf.Buffer
			report.Refused = append(report.Refused,
				tb.Str(verdicts[i].Declaration.Source).Str(": ").Str(reason).String())
		}
	}
	if len(verdicts)+len(problems) == 0 {
		report.Refused = append(report.Refused, declarationDir+"/ holds no declaration")
	}
	return report, nil
}
