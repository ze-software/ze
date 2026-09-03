// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree cleanup policy
package verify

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// worktreeEntriesMax bounds the size walk of one preserved worktree. This
// repository is 22,439 tracked files, so 200,000 entries is an order of
// magnitude above a healthy worktree and is reached only by a tree somebody grew
// by hand. The walk stops there and reports the size as a floor, because an
// operator deciding whether to remove 8 GiB needs the order of magnitude rather
// than the byte.
const worktreeEntriesMax = 200_000

// sweep is what one pass over the abandoned worktrees found.
type sweep struct {
	Removed     []string
	Preserved   []PreservedWorktree
	Diagnostics []string
	Failures    []CleanupFailure
}

func sweepAbandoned(ctx context.Context, root, base string, deps dependencies) sweep {
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sweep{}
		}
		return sweep{Failures: []CleanupFailure{{Operation: "read abandoned worktrees", Message: err.Error()}}}
	}
	markers, err := os.OpenRoot(base)
	if err != nil {
		return sweep{Failures: []CleanupFailure{{Operation: "open abandoned worktree root", Message: err.Error()}}}
	}
	removed := make([]string, 0)
	preserved := make([]PreservedWorktree, 0)
	diagnostics := make([]string, 0)
	failures := make([]CleanupFailure, 0)
	var text textbuf.Buffer
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name())
		if owner, readErr := markers.ReadFile(text.Reset().Str(entry.Name()).Str(".owner").Slice()); readErr == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(owner)))
			if parseErr == nil && deps.alive(pid) {
				continue
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			diagnostics = append(diagnostics, text.Reset().
				Str("verify-worktree: cannot read owner of ").Str(entry.Name()).
				Str(", treating it as abandoned: ").Err(readErr).String())
		}

		status, statusErr := deps.git(ctx, gitTimeoutWorktree, path, "status", "--porcelain")
		if statusErr != nil || status.Code != 0 {
			message := commandFailure("git status", status, statusErr)
			diagnostics = append(diagnostics, text.Reset().Str("verify-worktree: ").Str(entry.Name()).
				Str(" was abandoned but its status is unknown, so it is left alone: ").Str(message).String())
			failures = append(failures, CleanupFailure{
				Operation: text.Reset().Str("inspect abandoned ").Str(entry.Name()).String(),
				Message:   message,
			})
			continue
		}
		if strings.TrimSpace(status.Output) != "" {
			kept, measureErr := describePreserved(path, status.Output, deps.now())
			preserved = append(preserved, kept)
			diagnostics = append(diagnostics, preservedLine(entry.Name(), kept))
			if measureErr != nil {
				diagnostics = append(diagnostics, text.Reset().Str("verify-worktree: ").Str(entry.Name()).
					Str(" was kept but could not be measured: ").Err(measureErr).String())
			}
			continue
		}
		cleanup := removeWorktree(context.WithoutCancel(ctx), root, path, deps.git)
		if len(cleanup) != 0 {
			for _, failure := range cleanup {
				failure.Operation = text.Reset().Str("sweep ").Str(entry.Name()).Str(": ").
					Str(failure.Operation).String()
				failures = append(failures, failure)
			}
			continue
		}
		removed = append(removed, path)
	}
	if err := markers.Close(); err != nil {
		failures = append(failures, CleanupFailure{Operation: "close abandoned worktree root", Message: err.Error()})
	}
	return sweep{Removed: removed, Preserved: preserved, Diagnostics: diagnostics, Failures: failures}
}

// describePreserved measures one worktree the sweep is about to keep, and
// answers the reason it could not when the tree would not read.
//
// The age is the last change to the worktree directory itself, which for a
// detached checkout is when git created it or last rewrote its top level.
func describePreserved(path, status string, now time.Time) (PreservedWorktree, error) {
	kept := PreservedWorktree{Path: path}
	countDirt(&kept, status)
	info, err := os.Stat(path)
	if err != nil {
		return kept, fmt.Errorf("inspect preserved worktree: %w", err)
	}
	bytes, floor, walkErr := directorySize(path)
	if walkErr != nil {
		return kept, walkErr
	}
	kept.Measured = true
	kept.AgeSeconds = int64(now.Sub(info.ModTime()).Seconds())
	kept.SizeBytes = bytes
	kept.SizeFloor = floor
	return kept, nil
}

// countDirt classifies each porcelain line of a preserved worktree's status.
//
// The shape decides what an operator does with the tree. Modified or untracked
// content exists only here, so removing the worktree destroys it. Deletions
// alone are a tree somebody emptied, and git restores every one of them from the
// commit the worktree is detached at. The 8.27 GiB worktree measured on
// 2026-09-03 was entirely of the second kind: 264 deletions, no modified path
// and no untracked path.
func countDirt(kept *PreservedWorktree, status string) {
	for line := range strings.SplitSeq(status, "\n") {
		if len(line) < 3 {
			continue
		}
		index, work := line[0], line[1]
		switch {
		case index == '?' && work == '?':
			kept.Untracked++
		case index == 'D' || work == 'D':
			kept.Deleted++
		default:
			kept.Modified++
		}
	}
}

// directorySize sums the regular files under path, and answers whether the entry
// budget stopped the walk before the end.
//
// A symbolic link is never followed and its target is never counted, which is
// what keeps the shared Go build cache out of the total: cache/ is a link into
// the per-user target (internal/le/scratch, EnsureCache) and it belongs to every
// checkout on the machine rather than to this worktree.
func directorySize(path string) (uint64, bool, error) {
	var bytes uint64
	entries := 0
	floor := false
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > worktreeEntriesMax {
			floor = true
			return filepath.SkipAll
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() > 0 {
			bytes += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("measure preserved worktree: %w", err)
	}
	return bytes, floor, nil
}

// preservedLine reports one worktree the sweep kept, in the terms that decide
// what an operator does next.
func preservedLine(name string, kept PreservedWorktree) string {
	extent := preservedAgeAndSize(kept)
	var text textbuf.Buffer
	return text.Str("verify-worktree: ").Str(name).
		Str(" was abandoned but holds uncommitted changes, so it is left alone: ").
		Str(kept.Path).Str(", ").Str(extent).
		Str(", ").Int(int64(kept.Modified)).Str(" modified, ").
		Int(int64(kept.Untracked)).Str(" untracked, ").
		Int(int64(kept.Deleted)).Str(" deleted").String()
}

// preservedAgeAndSize renders how old a preserved worktree is and how much disk
// it holds, or says that neither was measured.
//
// It never renders a number it did not measure. A zero age beside a zero size is
// what an unreadable directory produces, and an operator reads that as a
// worktree created this second and holding nothing.
func preservedAgeAndSize(kept PreservedWorktree) string {
	if !kept.Measured {
		return "age and size unknown"
	}
	var text textbuf.Buffer
	age := time.Duration(kept.AgeSeconds) * time.Second
	text.Str(age.Round(time.Minute).String()).Str(" old, ")
	if kept.SizeFloor {
		text.Str("at least ")
	}
	return text.Str(diskspace.GiB(kept.SizeBytes)).String()
}

// reclaimWorktree removes a worktree that a failed or abandoned add left on
// disk. Git creates the worktree directory before it checks anything out, so an
// add that created no directory left nothing to reclaim, and a removal there
// would report a failure of its own.
func reclaimWorktree(ctx context.Context, root, path string, git gitRunner) []CleanupFailure {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return removeWorktree(ctx, root, path, git)
}

func removeWorktree(ctx context.Context, root, path string, git gitRunner) []CleanupFailure {
	failures := make([]CleanupFailure, 0)
	if err := os.Remove(ownerMarker(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		failures = append(failures, CleanupFailure{Operation: "remove owner marker", Message: err.Error()})
	}

	// A killed "git worktree add" leaves git's own creation lock behind, and git
	// refuses to remove a locked worktree with one --force. Nothing but this
	// package creates a worktree under tmp/verify-worktree, so the second --force
	// can only override an add this tool did not finish.
	removed, err := git(ctx, gitTimeoutWorktree, root, "worktree", "remove", "--force", "--force", path)
	if err != nil || removed.Code != 0 {
		failures = append(failures, CleanupFailure{Operation: "git worktree remove", Message: commandFailure("git worktree remove", removed, err)})
	}
	if _, statErr := os.Stat(path); statErr == nil {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			failures = append(failures, CleanupFailure{Operation: "remove worktree directory", Message: removeErr.Error()})
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		failures = append(failures, CleanupFailure{Operation: "inspect worktree directory", Message: statErr.Error()})
	}
	if prune := pruneWorktrees(ctx, root, git); prune != nil {
		failures = append(failures, CleanupFailure{Operation: "git worktree prune", Message: prune.Error()})
	}
	return failures
}

func pruneWorktrees(ctx context.Context, root string, git gitRunner) error {
	result, err := git(ctx, gitTimeoutMetadata, root, "worktree", "prune", "--expire", "now")
	if err != nil || result.Code != 0 {
		return errors.New(commandFailure("git worktree prune", result, err))
	}
	return nil
}

func saveLogs(root, worktree, name string) (string, error) {
	source := filepath.Join(worktree, "tmp", "verify")
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect stage logs: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("stage log path is not a directory: %s", source)
	}
	destination := filepath.Join(root, "tmp", "verify-worktree-logs", name)
	if err := os.RemoveAll(destination); err != nil {
		return "", fmt.Errorf("replace saved logs: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return "", fmt.Errorf("create saved log parent: %w", err)
	}
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		return "", fmt.Errorf("copy stage logs: %w", err)
	}
	return destination, nil
}

func cleanupMessage(failures []CleanupFailure) string {
	var text textbuf.Buffer
	for index, failure := range failures {
		if index != 0 {
			text.Str("; ")
		}
		text.Str(failure.Operation).Str(": ").Str(failure.Message)
	}
	return text.String()
}
