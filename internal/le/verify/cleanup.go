// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree cleanup policy
package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// orphaned reports that git resolves this directory to a DIFFERENT tree, so
// anything read there describes that tree rather than this one.
//
// It answers false whenever git cannot say, which keeps the conservative arm
// below for a directory whose state is unknown: only a toplevel git NAMED, and
// named as somewhere else, licenses the sweep.
func orphaned(ctx context.Context, deps dependencies, path string) bool {
	answer, err := deps.git(ctx, gitTimeoutWorktree, path, "rev-parse", "--show-toplevel")
	if err != nil || answer.Code != 0 {
		return false
	}
	top := strings.TrimSpace(answer.Output)
	if top == "" {
		return false
	}
	here, statErr := os.Stat(path)
	if statErr != nil {
		return false
	}
	there, statErr := os.Stat(top)
	if statErr != nil {
		return false
	}
	return !os.SameFile(here, there)
}

// Reclaim removes abandoned verification artifacts without creating a checkout
// or running a verification stage. The caller supplies the checkout root.
func Reclaim(ctx context.Context, root string) Report {
	report := Report{Action: "reclaim"}
	if !filepath.IsAbs(root) {
		report.Code = 2
		report.Diagnostics = []string{"reclaim worktrees: checkout root must be absolute; run from a Ze checkout"}
		return report
	}
	swept := sweepAbandoned(ctx, root, filepath.Join(root, "tmp", "verify-worktree"), defaultDependencies())
	if err := reclaimRegistrations(ctx, root, defaultDependencies()); err != nil {
		swept.Failures = append(swept.Failures, CleanupFailure{Operation: "reclaim stale registrations", Message: err.Error()})
	}
	report.Swept = swept.Removed
	report.Preserved = swept.Preserved
	report.Cleanup = swept.Failures
	report.Diagnostics = swept.Diagnostics
	for _, path := range swept.Removed {
		report.Diagnostics = append(report.Diagnostics, "reclaim worktrees: removed "+path)
	}
	for _, failure := range swept.Failures {
		report.Diagnostics = append(report.Diagnostics, "reclaim worktrees: "+failure.Operation+": "+failure.Message)
	}
	if len(swept.Failures) != 0 {
		report.Code = 1
	}
	if len(report.Diagnostics) == 0 {
		report.Diagnostics = []string{"reclaim worktrees: no abandoned artifacts to remove"}
	}
	return report
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
		name := entry.Name()
		directoryless := false
		// ownerDead is set only when a marker named a process that is gone. A
		// missing marker leaves it false, so a worktree nobody claimed is never
		// read as a finished run's interrupted removal.
		ownerDead := false
		if strings.HasSuffix(name, ".owner") {
			if entry.IsDir() {
				continue
			}
			name = strings.TrimSuffix(name, ".owner")
			if name == "" {
				continue
			}
			if _, statErr := markers.Lstat(name); statErr == nil {
				continue
			} else if !errors.Is(statErr, os.ErrNotExist) {
				failures = append(failures, CleanupFailure{Operation: "inspect owner directory " + name, Message: statErr.Error()})
				continue
			}
			directoryless = true
		} else if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, name)
		owner, readErr := markers.ReadFile(name + ".owner")
		if readErr != nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				failures = append(failures, CleanupFailure{Operation: "read owner of " + name, Message: readErr.Error()})
				continue
			}
			if directoryless {
				// The snapshot can list a marker already removed with its directory.
				continue
			}
		} else {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(owner)))
			if parseErr != nil {
				failures = append(failures, CleanupFailure{Operation: "read owner of " + name, Message: "invalid owner identity; preserve the resource and investigate its owner"})
				continue
			}
			if pid < 0 {
				failures = append(failures, CleanupFailure{Operation: "read owner of " + name, Message: "negative owner identity; preserve the resource and investigate its owner"})
				continue
			}
			if deps.alive(pid) {
				continue
			}
			ownerDead = true
		}
		if directoryless {
			reclaimed, err := removeMissingRegistration(ctx, root, path, deps.git, deps.alive)
			if err != nil {
				failures = append(failures, CleanupFailure{Operation: "unregister directoryless worktree " + name, Message: err.Error()})
				continue
			}
			if !reclaimed {
				continue
			}
			if err := markers.Remove(name + ".owner"); err != nil {
				failures = append(failures, CleanupFailure{Operation: "remove directoryless owner of " + name, Message: err.Error()})
				continue
			}
			removed = append(removed, path)
			continue
		}

		// A directory that is no longer a worktree ROOT is swept on its
		// contents alone, because its status describes SOMEBODY ELSE.
		//
		// `git worktree remove` unregisters the checkout and a failure after
		// that leaves the files with no `.git` of their own. `git -C <dir>
		// status` then walks up to the enclosing checkout and answers about
		// THAT tree, naming its paths relative to <dir>: the main checkout's
		// one untracked file arrives as `?? ../../../continue`, the dirt count
		// reads 1, and the directory is preserved for a change it does not
		// hold. Nothing ever removes it. Measured 2026-09-19: two such
		// directories held 22G between them and the run that met them died of
		// `no space left on device`.
		if orphaned(ctx, deps, path) {
			cleanup := removeOrphan(context.WithoutCancel(ctx), root, path, deps.git)
			if len(cleanup) != 0 {
				for _, failure := range cleanup {
					failure.Operation = text.Reset().Str("sweep orphaned ").Str(entry.Name()).Str(": ").
						Str(failure.Operation).String()
					failures = append(failures, failure)
				}
				continue
			}
			diagnostics = append(diagnostics, text.Reset().Str("verify-worktree: swept orphaned ").
				Str(entry.Name()).Str(", which git no longer reports as a worktree root").String())
			removed = append(removed, path)
			continue
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

		// A dead run's worktree whose only dirt is deletions is a removal that
		// stopped part way: `git worktree remove` deletes the files before it
		// drops the registration, so a kill in between leaves a registered tree
		// of thousands of ` D` lines. Nothing in it exists anywhere but the
		// commit HEAD names, so finishing the removal destroys nothing. Two
		// such trees (1.6G, 2026-10-18) were otherwise kept as "holds
		// uncommitted changes" until removed by hand.
		if ownerDead && onlyDeletions(status.Output) && headResolves(ctx, deps, path) {
			cleanup := removeWorktree(context.WithoutCancel(ctx), root, path, deps.git)
			if len(cleanup) != 0 {
				for _, failure := range cleanup {
					failure.Operation = text.Reset().Str("finish interrupted removal of ").Str(entry.Name()).
						Str(": ").Str(failure.Operation).String()
					failures = append(failures, failure)
				}
				continue
			}
			diagnostics = append(diagnostics, text.Reset().Str("verify-worktree: finished interrupted removal of ").
				Str(entry.Name()).Str(", whose owner is gone and whose only changes were deletions").String())
			removed = append(removed, path)
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
// and no untracked path. A dead owner's tree of that kind no longer reaches this
// function: sweepAbandoned finishes it as an interrupted removal, so a kept
// tree of deletions alone has no owner marker, or a HEAD that does not resolve.
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

// onlyDeletions answers whether a porcelain status names at least one path and
// every path it names is a plain deletion, ` D` (the work tree lost it) or `D `
// (the index dropped it too).
//
// `MD` and `AD` are refused: the index holds content no commit carries, and
// removing the worktree would destroy it. So are untracked, renamed and
// modified paths, and any line too short to carry a status.
func onlyDeletions(status string) bool {
	deletions := 0
	for line := range strings.SplitSeq(status, "\n") {
		if line == "" {
			continue
		}
		if len(line) < 3 {
			return false
		}
		code := line[:2]
		if code != " D" && code != "D " {
			return false
		}
		deletions++
	}
	return deletions != 0
}

// headResolves answers whether the worktree's HEAD names a commit, the one git
// restores every deleted path from. A HEAD that does not resolve, or a git that
// cannot answer, leaves the worktree to the preserving arm.
func headResolves(ctx context.Context, deps dependencies, path string) bool {
	answer, err := deps.git(ctx, gitTimeoutWorktree, path, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	// refuse-by-default: guards sweepAbandoned -- false keeps an unanswered HEAD in the preserving arm
	if err != nil {
		return false
	}
	return answer.Code == 0 && strings.TrimSpace(answer.Output) != ""
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

// reclaimWorktree reclaims both halves of this run's failed add. A directoryless
// checkout MUST be unregistered before its retry marker is removed.
func reclaimWorktree(ctx context.Context, root, path string, git gitRunner) []CleanupFailure {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if _, err := removeMissingRegistration(ctx, root, path, git, nil); err != nil {
			return []CleanupFailure{{Operation: "git worktree remove", Message: err.Error()}}
		}
		if err := os.Remove(ownerMarker(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return []CleanupFailure{{Operation: "remove directoryless owner marker", Message: err.Error()}}
		}
		return nil
	}
	return removeWorktree(ctx, root, path, git)
}

// removeOrphan removes files whose Git root is already gone, then explicitly
// unregisters any surviving entry before discarding the ownership marker.
func removeOrphan(ctx context.Context, root, path string, git gitRunner) []CleanupFailure {
	if err := os.RemoveAll(path); err != nil {
		return []CleanupFailure{{Operation: "remove orphaned directory", Message: err.Error()}}
	}
	return reclaimWorktree(ctx, root, path, git)
}

func removeWorktree(ctx context.Context, root, path string, git gitRunner) []CleanupFailure {
	failures := make([]CleanupFailure, 0)

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
	if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) && len(failures) == 0 {
		if err := os.Remove(ownerMarker(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, CleanupFailure{Operation: "remove owner marker", Message: err.Error()})
		}
	}
	return failures
}

// reclaimRegistrations preserves legacy markerless stale checkouts only inside
// this tool's direct-child area. Global pruning bypasses ownership refusals and
// can erase other sessions' registrations, including paths outside that area.
func reclaimRegistrations(ctx context.Context, root string, deps dependencies) error {
	paths, err := worktreeRegistrations(ctx, root, deps.git)
	if err != nil {
		return err
	}
	base := filepath.Join(root, "tmp", "verify-worktree")
	var failures error
	for _, path := range paths {
		if filepath.Dir(path) != base {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				failures = errors.Join(failures, fmt.Errorf("inspect registered worktree %q: %w", path, err))
			}
			continue
		}
		if _, err := removeMissingRegistration(ctx, root, path, deps.git, deps.alive); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}

// worktreeRegistrations reads raw NUL records: paths may contain spaces or
// newlines, so neither line splitting nor shell unquoting identifies a checkout.
func worktreeRegistrations(ctx context.Context, root string, git gitRunner) ([]string, error) {
	result, err := git(ctx, gitTimeoutMetadata, root, "worktree", "list", "--porcelain", "-z")
	if err != nil || result.Code != 0 {
		return nil, errors.New(commandFailure("git worktree list", result, err))
	}
	var paths []string
	for field := range strings.SplitSeq(result.Output, "\x00") {
		if path, ok := strings.CutPrefix(field, "worktree "); ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// removeMissingRegistration MUST precede marker deletion. It answers false for
// a live owner, which the caller MUST preserve. alive is required for abandoned
// resources; nil is reserved for the current run's failed add.
// Snapshot/rechecks refuse changed ownership, a recreated directory or a target
// outside the direct-child area before Git can mutate its registration.
func removeMissingRegistration(ctx context.Context, root, path string, git gitRunner, alive func(int) bool) (bool, error) {
	if filepath.Clean(path) != path {
		return false, fmt.Errorf("refuse noncanonical registration path: %q", path)
	}
	if filepath.Dir(path) != filepath.Join(root, "tmp", "verify-worktree") {
		return false, fmt.Errorf("refuse registration outside verification area: %q", path)
	}
	owner, ownerErr := os.ReadFile(ownerMarker(path))
	if ownerErr != nil && !errors.Is(ownerErr, os.ErrNotExist) {
		return false, fmt.Errorf("read registration owner %q: %w", path, ownerErr)
	}
	paths, err := worktreeRegistrations(ctx, root, git)
	if err != nil {
		return false, err
	}
	registered := slices.Contains(paths, path)
	current, currentErr := os.ReadFile(ownerMarker(path))
	if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) {
		return false, fmt.Errorf("recheck registration owner %q: %w", path, currentErr)
	}
	if errors.Is(currentErr, os.ErrNotExist) != errors.Is(ownerErr, os.ErrNotExist) {
		return false, fmt.Errorf("registration owner changed while reclaiming %q", path)
	}
	if !bytes.Equal(current, owner) {
		return false, fmt.Errorf("registration owner changed while reclaiming %q", path)
	}
	if alive != nil && currentErr == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(current)))
		if err != nil {
			return false, fmt.Errorf("invalid registration owner for %q: %w", path, err)
		}
		if pid < 0 {
			return false, fmt.Errorf("negative registration owner for %q", path)
		}
		if alive(pid) {
			return false, nil
		}
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return false, fmt.Errorf("inspect missing registration %q: %w", path, err)
		}
		return false, fmt.Errorf("worktree reappeared while reclaiming %q", path)
	}
	if !registered {
		return true, nil
	}
	result, err := git(ctx, gitTimeoutWorktree, root, "worktree", "remove", "--force", "--force", path)
	if err != nil || result.Code != 0 {
		return false, errors.New(commandFailure("git worktree remove", result, err))
	}
	return true, nil
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
