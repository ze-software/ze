// Design: docs/contributing/spec-workflow.md -- what each closure gate reads
// Related: specpath.go -- HistoricalBucket and Stem, the layout this judgement reads
// Related: ../../commit/review.go -- closedSpecStem, the commit gate that applies it
// Related: ../status/closed.go -- the record of closed specs read from git history

package specpath

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// Side answers a path's text on one side of a commit, whether that side
// holds the path, and the reason the read failed. present is what tells an
// absent path from an empty file.
type Side func(path string) (text string, present bool, problem string)

// Sides is the pair of texts a rename is judged over: Before is the commit
// the change starts from, and After is the content the change carries.
type Sides struct {
	Before Side
	After  Side
}

// ClosedStems answers the stems of the specs one commit's file population
// closes, sorted, from the population and the two sides of the commit that
// sides reads. paths is every path the commit carries, removed every path it
// deletes.
//
// A REMOVED spec file is the closure, and it is the only signal. A removal that
// relocatedSpecs or renamedSpecs explains is no closure.
//
// This is the one statement of the rule. The commit gate applies it to the
// commit in hand and to a commit a discharge re-derives months later, and the
// record of closed specs (internal/le/spec/status) applies it to every commit
// of the history, so none of them holds a copy. Each caller supplies the sides
// it can see: a prepared commit reads HEAD and the working tree, a landed
// commit reads its parent and itself, and the rename judgement is the same
// over either pair. The gate's own refusal of a second closure in one commit
// is the gate's, not the rule's: history holds commits that closed several.
func ClosedStems(paths, removed []string, sides Sides) ([]string, error) {
	moved := relocatedSpecs(paths, removed)
	renamed, err := renamedSpecs(paths, removed, moved, sides)
	if err != nil {
		return nil, err
	}
	var stems []string
	for _, path := range removed {
		if _, isSpec := HistoricalBucket(path); !isSpec {
			continue
		}
		if moved[filepath.Base(path)] {
			continue
		}
		if renamed[path] {
			continue
		}
		if stem := Stem(path); !slices.Contains(stems, stem) {
			stems = append(stems, stem)
		}
	}
	slices.Sort(stems)
	return stems, nil
}

// relocatedSpecs answers the spec file names this commit MOVES between release
// buckets rather than closes: the same name removed from one bucket directory
// and carried in another in the same commit. A DIFFERENT name is a closure
// unless renamedSpecs pairs it with the spec it was renamed to.
//
// The distinction is the whole of it. A closure retires a spec because its work
// is done. A relocation retires nothing: the spec is still open, still
// unimplemented, and has only been re-filed under the release that owes it.
// Counting a relocation as a closure made the commit gate refuse every batch of
// them, which is the shape the 2026-08-24 row in
// plan/journal/gate-fires-outside-its-population.md asks for: the question is
// not "does this commit remove a spec" but "does this commit CLOSE it".
//
// The rule read "removed from plan/ and added under plan/future/" until the
// release buckets replaced plan/future/. It now reads over every pair of
// buckets, in both directions, because a triage sweep re-files work upward as
// often as downward.
func relocatedSpecs(paths, removed []string) map[string]bool {
	removedFrom := make(map[string]string, len(removed))
	for _, path := range removed {
		if bucket, ok := HistoricalBucket(path); ok {
			removedFrom[filepath.Base(path)] = bucket
		}
	}

	moved := make(map[string]bool)
	for _, path := range paths {
		bucket, ok := HistoricalBucket(path)
		if !ok {
			continue
		}
		base := filepath.Base(path)
		from, wasRemoved := removedFrom[base]
		if !wasRemoved {
			continue
		}
		if from != bucket {
			moved[base] = true
		}
	}

	return moved
}

// renamedSpecs answers the removed spec paths this commit RENAMES rather than
// closes (Thomas, 2026-10-18: a rename is allowed and is not a closure).
//
// A removed spec is renamed only when the commit adds exactly one NEW spec, one
// its parent does not hold, whose text equals the removed spec's text at the
// parent with every occurrence of the old stem replaced by the new stem, and
// that new spec matches no other removal. The replacement covers the title and
// every self-reference, which is all a rename rewrites. Any other edit, a
// missing parent blob, an added spec that already existed, or a match that is
// not one to one leaves the removal a closure, because the failure this guards
// is a real closure passing as a rename: a closure deletes a spec and carries
// its work elsewhere, never its text verbatim under a new name.
//
// A relocation (moved) is judged before this and is never a rename candidate
// on either side.
func renamedSpecs(paths, removed []string, moved map[string]bool, sides Sides) (map[string]bool, error) {
	gone := make([]string, 0, len(removed))
	removedSet := make(map[string]bool, len(removed))
	for _, path := range removed {
		removedSet[path] = true
		if _, isSpec := HistoricalBucket(path); !isSpec {
			continue
		}
		if !moved[filepath.Base(path)] {
			gone = append(gone, path)
		}
	}
	added, err := addedSpecTexts(paths, removedSet, moved, sides)
	if err != nil {
		return nil, err
	}
	renamed := make(map[string]bool)
	if len(gone) == 0 {
		return renamed, nil
	}
	if len(added) == 0 {
		return renamed, nil
	}

	matches := make(map[string][]string, len(gone))
	claims := make(map[string]int, len(added))
	for _, path := range gone {
		oldText, present, problem := sides.Before(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if !present {
			continue
		}
		oldStem := Stem(path)
		if oldStem == "" {
			continue
		}
		for newPath, newText := range added {
			if strings.ReplaceAll(oldText, oldStem, Stem(newPath)) == newText {
				matches[path] = append(matches[path], newPath)
				claims[newPath]++
			}
		}
	}

	for path, targets := range matches {
		if len(targets) != 1 {
			continue
		}
		if claims[targets[0]] == 1 {
			renamed[path] = true
		}
	}
	return renamed, nil
}

// addedSpecTexts answers the text of every spec the commit ADDS: named in the
// population, not removed, not a relocation's target, and absent from the
// parent.
func addedSpecTexts(paths []string, removed, moved map[string]bool, sides Sides) (map[string]string, error) {
	added := make(map[string]string)
	for _, path := range paths {
		if _, isSpec := HistoricalBucket(path); !isSpec {
			continue
		}
		if removed[path] {
			continue
		}
		if moved[filepath.Base(path)] {
			continue
		}
		if _, ok := added[path]; ok {
			continue
		}
		_, existed, problem := sides.Before(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if existed {
			continue
		}
		text, present, problem := sides.After(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if present {
			added[path] = text
		}
	}
	return added, nil
}
