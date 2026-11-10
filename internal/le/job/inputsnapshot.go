// Design: docs/contributing/running-commands.md -- the lint planning guard over its own population
// Design: docs/architecture/testing/verify-freshness-scope.md -- a scoped PASS judged by content
// Related: treehash.go -- the label input declaration and the content fingerprint this reuses
//
// InputHash answers whether two jobs judge the same inputs, so it describes
// everything a label could read. A caller that already knows WHICH paths its
// verdict depends on asks a narrower question: did any of those paths change
// since a start instant? Answering that with a whole-tree hash refuses the
// caller for an edit, or a commit, that it never reads. On a checkout that many
// sessions write, that refusal fired on every scoped lint
// (plan/journal/gate-fires-outside-its-population.md), and the same mistake
// made every scoped verify PASS stale. This file is the one answer both use.

package job

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// InputSnapshot records, at one instant, the commit the checkout sits on and
// the content fingerprint of every path that differs from it.
//
// The zero value is an unmeasured snapshot, and ChangedInputs refuses it:
// an empty manifest would otherwise read as "nothing was dirty" and match
// any tree.
type InputSnapshot struct {
	label string
	head  string
	dirty map[string]string
	// unreadable holds each dirty path whose content could not be measured.
	// It is raised only for a caller whose population reaches the path, so an
	// unreadable file nobody reads refuses nobody.
	unreadable map[string]error
}

// SnapshotInputs records the inputs label reads, as InputSnapshot describes.
// The trees labelIgnores declares for label are left out, as InputHash leaves
// them out, so the two answers agree on what the label reads.
//
// A failed Git query is an error, never a snapshot: a failed measurement is not
// a state any later measurement may match. An unreadable path is held in the
// snapshot and refused by ChangedInputs when a population reaches it.
func SnapshotInputs(root, label string) (InputSnapshot, error) {
	out, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return InputSnapshot{}, fmt.Errorf("snapshot %s inputs: read HEAD: %w", label, err)
	}
	head := strings.TrimSpace(string(out))
	if head == "" {
		return InputSnapshot{}, fmt.Errorf("snapshot %s inputs: git named no HEAD commit", label)
	}
	// The diff is taken against the commit just read, never against HEAD again,
	// so a commit landing between the two queries cannot pair one commit with
	// another commit's dirty set.
	paths, err := changedPaths(root, head)
	if err != nil {
		return InputSnapshot{}, fmt.Errorf("snapshot %s inputs: %w", label, err)
	}
	ignored := labelIgnores[label]
	snapshot := InputSnapshot{
		label:      label,
		head:       head,
		dirty:      make(map[string]string, len(paths)),
		unreadable: make(map[string]error),
	}
	for _, rel := range paths {
		if ignoredTree(rel, ignored) {
			continue
		}
		fingerprint, err := inputFingerprint(root, rel)
		if err != nil {
			snapshot.unreadable[rel] = err
			continue
		}
		snapshot.dirty[rel] = fingerprint
	}
	return snapshot, nil
}

// RecordedSnapshot rebuilds a snapshot an earlier run recorded: the commit it
// read and the manifest of paths dirty against that commit (DirtyManifest). It
// carries no label, so every path counts and the population decides.
func RecordedSnapshot(head string, manifest map[string]string) InputSnapshot {
	return InputSnapshot{head: head, dirty: manifest}
}

// ChangeKind says how ChangedInputs learned that a path changed. Zero is
// Unspecified, never a valid kind.
type ChangeKind uint8

const (
	// ChangeUnspecified is the zero value and names no change.
	ChangeUnspecified ChangeKind = iota
	// ChangedContent: the path was dirty at start and its content differs now.
	ChangedContent
	// ChangedDirty: the path matched its commit at start and differs from HEAD now.
	ChangedDirty
	// ChangedCommitted: the path matched its commit at both instants, and the
	// commits made between them changed it.
	ChangedCommitted
)

// InputChange is one reached path whose content differs from the start.
type InputChange struct {
	Path string
	Kind ChangeKind
}

// ErrCommitsIncomparable says the start commit can no longer be diffed against
// HEAD, which is what a commit that left the checkout in a rebase looks like.
// The caller has then learned nothing about the paths.
var ErrCommitsIncomparable = errors.New("the starting commit can no longer be compared with HEAD")

// ChangedInputs names, sorted by path, every path reaches accepts whose
// content now differs from what start recorded.
//
// The question is asked about CONTENT, never about the commit. A commit that
// absorbs a dirty path unchanged moves HEAD and the dirty set, yet the path
// holds what it held. So a path dirty at start is compared by its content; a
// path clean at start and dirty now has changed; and a path clean at both
// instants changed only when the commits between the two name it.
//
// reaches is the caller's population. A path it refuses is never named and
// never measured, whatever happened to it.
func ChangedInputs(root string, start InputSnapshot, reaches func(rel string) bool) ([]InputChange, error) {
	if start.head == "" {
		return nil, errors.New("the starting input snapshot was never measured, so no change can be ruled out")
	}
	now, err := SnapshotInputs(root, start.label)
	if err != nil {
		return nil, err
	}
	for _, unreadable := range []map[string]error{start.unreadable, now.unreadable} {
		for rel, cause := range unreadable {
			if reaches(rel) {
				return nil, cause
			}
		}
	}
	changed := make(map[string]ChangeKind)
	for rel, fingerprint := range start.dirty {
		if !reaches(rel) {
			continue
		}
		current, dirty := now.dirty[rel]
		if !dirty {
			current, err = inputFingerprint(root, rel)
			if err != nil {
				return nil, err
			}
		}
		if current != fingerprint {
			changed[rel] = ChangedContent
		}
	}
	for rel := range now.dirty {
		if _, recorded := start.dirty[rel]; recorded {
			continue
		}
		if reaches(rel) {
			changed[rel] = ChangedDirty
		}
	}
	if now.head != start.head {
		moved, err := PathsChangedBetween(root, start.head, now.head)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCommitsIncomparable, err)
		}
		for _, rel := range moved {
			if !cleanAtBoth(rel, start, now) {
				continue
			}
			if ignoredTree(rel, labelIgnores[start.label]) {
				continue
			}
			if reaches(rel) {
				changed[rel] = ChangedCommitted
			}
		}
	}
	changes := make([]InputChange, 0, len(changed))
	for rel, kind := range changed {
		changes = append(changes, InputChange{Path: rel, Kind: kind})
	}
	slices.SortFunc(changes, func(a, b InputChange) int { return strings.Compare(a.Path, b.Path) })
	return changes, nil
}

// cleanAtBoth answers whether rel matched its commit at both instants, which is
// the one case where only the commits between them can say it changed. A path
// dirty, or unreadable, at either instant was already judged by its content.
func cleanAtBoth(rel string, start, now InputSnapshot) bool {
	for _, snapshot := range []InputSnapshot{start, now} {
		if _, dirty := snapshot.dirty[rel]; dirty {
			return false
		}
		if _, unreadable := snapshot.unreadable[rel]; unreadable {
			return false
		}
	}
	return true
}

// changedPaths lists, sorted and once each, every path that differs from
// commit: the tracked changes and the untracked files, in Git's -z spelling,
// which never quotes a path.
func changedPaths(root, commit string) ([]string, error) {
	tracked, err := git(root, "diff", commit, "--name-only", "-z")
	if err != nil {
		return nil, fmt.Errorf("list tracked changes: %w", err)
	}
	untracked, err := git(root, "ls-files", "-o", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("list untracked files: %w", err)
	}
	paths := nulPaths(tracked)
	paths = append(paths, nulPaths(untracked)...)
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

// nulPaths splits Git's -z output into its paths.
func nulPaths(out []byte) []string {
	return slices.DeleteFunc(strings.Split(string(out), "\x00"), func(rel string) bool { return rel == "" })
}
