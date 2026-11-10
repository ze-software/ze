// Design: docs/architecture/testing/tracked-build-gate.md -- the one `git archive` export every le tool uses
//
// Package export materializes a commit into a directory, so a tool can judge
// what git holds rather than what the working tree holds. It is a helper
// library and registers no command.
//
// A revision is resolved with objects.Resolve (internal/le/repo/objects), the
// one revision resolver le's committed-source readers share; this package
// takes the commit that resolver answered.
package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Commit extracts commit's tree into dest, streaming `git archive` straight
// into `tar` so no second copy of the archive is held in memory or on disk.
//
// `git archive` rather than a worktree: it reads the object store and writes
// nothing into .git/. A worktree registration outlives a killed run and then
// needs pruning by hand, and several sessions share this checkout.
//
// vendor/ is INCLUDED: the module is vendored, so a tree without vendor/
// cannot build at all.
//
// dest MUST be an existing, empty directory. `tar -x` overwrites archived
// paths but never removes extras, so a non-empty directory CAN put a file back
// into the view that the commit deleted; Commit refuses one. On an error dest
// may hold a partial tree, and the caller that created dest removes it.
//
// commit is passed to git as one argv element, never through a shell, and a
// value that git would read as an option is refused. ctx bounds both children.
func Commit(ctx context.Context, repo, commit, dest string) error {
	if strings.HasPrefix(commit, "-") {
		return fmt.Errorf("export %q: a revision cannot start with '-'; pass the full sha objects.Resolve answered", commit)
	}
	if err := emptyDirectory(dest); err != nil {
		return err
	}

	archive := exec.CommandContext(ctx, "git", "-C", repo, "archive", "--format=tar", commit) //nolint:gosec // commit is one argv element and cannot be an option
	untar := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", dest)                          //nolint:gosec // dest is the caller's empty scratch directory

	pipe, err := archive.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe git archive: %w", err)
	}
	untar.Stdin = pipe
	var archiveErr, untarErr textbuf.Buffer
	archive.Stderr = &archiveErr
	untar.Stderr = &untarErr

	if err := archive.Start(); err != nil {
		return fmt.Errorf("start git archive: %w", err)
	}
	if err := untar.Start(); err != nil {
		startErr := fmt.Errorf("start tar: %w", err)
		if killErr := archive.Process.Kill(); killErr != nil {
			startErr = errors.Join(startErr, fmt.Errorf("kill git archive: %w", killErr))
		}
		archive.Wait() //nolint:errcheck // reaps the child just killed; its exit status is the kill, already reported
		return startErr
	}

	// tar is waited on first: it is the reader. Then the pipe is drained --
	// THIS PROCESS still holds its read end, so a tar that died early would not
	// give `git archive` an EPIPE and it would block on a full pipe forever.
	tarWait := untar.Wait()
	if _, drainErr := io.Copy(io.Discard, pipe); drainErr != nil && tarWait == nil {
		return fmt.Errorf("drain git archive output: %w", drainErr)
	}
	gitWait := archive.Wait()

	if gitWait != nil {
		return fmt.Errorf("git archive %s: %w: %s", commit, gitWait, strings.TrimSpace(archiveErr.String()))
	}
	if tarWait != nil {
		return fmt.Errorf("tar -x into %s: %w: %s", dest, tarWait, strings.TrimSpace(untarErr.String()))
	}
	return nil
}

// emptyDirectory refuses a dest that is missing, not a directory, or holds
// anything, naming the fix.
func emptyDirectory(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return fmt.Errorf("export into %s: %w; create an empty directory first", dest, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("export into %s: the directory holds %d entries; export into an empty directory, because tar never removes a file the commit deleted", dest, len(entries))
	}
	return nil
}
