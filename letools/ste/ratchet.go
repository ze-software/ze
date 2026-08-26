// Design: docs/architecture/core-design.md -- the gate: each changed file against its own HEAD
// Overview: ste.go -- the checker this gate runs
//
// HEAD is the PER-FILE baseline. A committed global baseline failed in two
// ways:
//
//   - It reported "markdown run-ons 24186 -> 24188" without a filename. Authors
//     did not know which sentence caused the finding.
//   - It reported sibling work. One session changed no Go file while another
//     session added 42 findings in internal/component/mcp/*.go. A gate that
//     fails for a colleague's edits soon gets disabled.
//
// Per-file comparison also makes untouched legacy prose free. No 44000-finding
// baseline or `--bless` operation is necessary.
package ste

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxReport bounds the findings one growth row prints. It is the script's
// `--max-report` default, and the flag is not ported: the payload carries every
// finding, so `| json` is what shows the rest.
const maxReport = 15

// Growth is one habit that grew in one file, and the findings that are new.
type Growth struct {
	File string `json:"file"`
	// Habit is the slug, and Number is the same habit as the number
	// `ai/rules/writing.md` lists it under.
	Habit    string    `json:"habit"`
	Number   int       `json:"habit-number"`
	Was      int       `json:"was"`
	Now      int       `json:"now"`
	Findings []Finding `json:"findings"`
}

// Ratchet answers all habits that grew in changed files and the examined-file
// count.
//
// named limits the run to repository-relative paths for commit-time checks.
//
// An unreadable file is an error, not a skip. The script skipped such files, so
// they contributed no findings. A ratchet that counts nothing reports no
// growth, and unreadable input would cause a false pass. See
// `plan/journal/zero-value-as-valid-answer.md`.
func Ratchet(root string, named []string) (growth []Growth, examined int, err error) {
	candidates, err := Candidates(root, named)
	if err != nil {
		return nil, 0, err
	}

	for _, candidate := range candidates {
		surface, ok := surfaceOf(candidate.Current)
		if !ok {
			continue
		}

		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(candidate.Current))) // #nosec G304 -- a path git named inside the checkout
		if readErr != nil {
			var tb textbuf.Buffer
			return nil, 0, errors.Join(
				errors.New(tb.Str("ste: cannot read the changed file ").Str(candidate.Current).
					Str(", so no habit in it can be measured").String()),
				readErr)
		}

		found, skipReason := Review(candidate.Current, string(body), surface)
		if skipReason != "" {
			continue
		}
		examined++

		before, _ := Review(candidate.Before, HeadText(root, candidate.Before), surface)
		growth = append(growth, grewIn(candidate.Current, found, before)...)
	}
	return growth, examined, nil
}

// grewIn answers one row for each habit whose count in this file is higher than
// it was at HEAD.
func grewIn(file string, found, before []Finding) []Growth {
	now := countByHabit(found)
	was := countByHabit(before)

	habits := make([]string, 0, len(now))
	for habit := range now {
		habits = append(habits, habit)
	}
	sort.Slice(habits, func(i, j int) bool {
		return habitNumber(habits[i]) < habitNumber(habits[j])
	})

	var out []Growth
	for _, habit := range habits {
		if now[habit] <= was[habit] {
			continue
		}
		fresh := addedSince(found, before, habit)
		if len(fresh) > maxReport {
			fresh = fresh[:maxReport]
		}
		out = append(out, Growth{
			File: file, Habit: habit, Number: habitNumber(habit),
			Was: was[habit], Now: now[habit], Findings: fresh,
		})
	}
	return out
}

// countByHabit counts findings by habit slug.
func countByHabit(findings []Finding) map[string]int {
	counts := make(map[string]int, len(habitNames))
	for _, finding := range findings {
		counts[finding.Habit]++
	}
	return counts
}

// addedSince answers the findings of one habit that are new since HEAD.
//
// Matched on CONTENT, not on line number: an edit higher in the file moves
// every line below it. Printing all 34 findings of a file that grew by 2 tells
// the author nothing about which 2 sentences to rewrite.
func addedSince(found, before []Finding, habit string) []Finding {
	type key struct{ detail, excerpt string }

	seen := make(map[key]int)
	for _, finding := range before {
		if finding.Habit == habit {
			seen[key{finding.Detail, finding.Excerpt}]++
		}
	}

	var fresh []Finding
	for _, finding := range found {
		if finding.Habit != habit {
			continue
		}
		at := key{finding.Detail, finding.Excerpt}
		if seen[at] > 0 {
			seen[at]--
			continue
		}
		fresh = append(fresh, finding)
	}
	return fresh
}
