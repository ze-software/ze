// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree cleanup policy
package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func sweepAbandoned(ctx context.Context, root, base string, deps dependencies) ([]string, []string, []CleanupFailure) {
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, []CleanupFailure{{Operation: "read abandoned worktrees", Message: err.Error()}}
	}
	markers, err := os.OpenRoot(base)
	if err != nil {
		return nil, nil, []CleanupFailure{{Operation: "open abandoned worktree root", Message: err.Error()}}
	}
	removed := make([]string, 0)
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
			diagnostics = append(diagnostics, text.Reset().Str("verify-worktree: ").Str(entry.Name()).
				Str(" was abandoned but holds uncommitted changes, so it is left alone").String())
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
	return removed, diagnostics, failures
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
