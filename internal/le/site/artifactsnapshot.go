// Design: website/AI.md -- a build stages sources into an isolated Pages artifact
// Overview: build.go -- snapshotArtifact, the build step that takes and releases a snapshot

package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// artifactSnapshotPrefix names the scratch copies of the published artifact a
// build makes beside it. Only directories with this prefix are ever swept.
const artifactSnapshotPrefix = ".site-artifact-"

// artifactSnapshot is a scratch copy of the published artifact held by the
// build that made it. The hold is an exclusive flock on the directory itself,
// which the kernel drops when the process exits, however it exits: a build
// killed before its release leaves a copy that no process holds, and the next
// build removes it. Not safe for concurrent use; one build owns one snapshot.
type artifactSnapshot struct {
	path   string
	handle *os.File
}

// takeArtifactSnapshot creates an empty snapshot directory under parent and
// holds it. The caller MUST call release when the build no longer reads it.
func takeArtifactSnapshot(parent string) (*artifactSnapshot, error) {
	path, err := os.MkdirTemp(parent, artifactSnapshotPrefix)
	if err != nil {
		return nil, fmt.Errorf("create site artifact snapshot: %w", err)
	}
	handle, held, err := holdDirectory(path)
	if err != nil {
		_ = os.RemoveAll(path) //nolint:errcheck // The hold failure is the verdict returned.
		return nil, fmt.Errorf("hold site artifact snapshot %s: %w", path, err)
	}
	if !held {
		// Only another build's sweep can hold a directory this build just
		// made, and it holds it to remove it.
		_ = handle.Close() //nolint:errcheck // Nothing was locked through this handle.
		return nil, fmt.Errorf("site artifact snapshot %s was claimed by another build's clean-up; "+
			"two builds write %s at once: let the other finish, then run ./le site build again", path, parent)
	}
	return &artifactSnapshot{path: path, handle: handle}, nil
}

// release removes the snapshot, then drops the hold. Removing first means a
// sweep never meets a half-removed copy that it could take for an abandoned
// one. A failed removal leaves the copy unheld, so the next build sweeps it;
// it must not fail a build that otherwise succeeded.
func (s *artifactSnapshot) release() {
	_ = os.RemoveAll(s.path) //nolint:errcheck // The next build's sweep removes what is left.
	_ = s.handle.Close()     //nolint:errcheck // Closing drops the flock; process exit would too.
}

// removeAbandonedSnapshots deletes every snapshot under parent that no live
// build holds. A copy whose hold is taken belongs to a build still running and
// is left alone, so two builds beside one artifact never remove each other's.
func removeAbandonedSnapshots(parent string) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return fmt.Errorf("list the site artifact snapshots in %s: %w", parent, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), artifactSnapshotPrefix) {
			continue
		}
		if err := removeAbandonedSnapshot(filepath.Join(parent, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// removeAbandonedSnapshot removes one snapshot when no live build holds it.
func removeAbandonedSnapshot(path string) error {
	handle, held, err := holdDirectory(path)
	if err != nil {
		return fmt.Errorf("check whether a build still uses the site artifact snapshot %s: %w; "+
			"if no ./le site build is running, remove it with: rm -rf %s", path, err, path)
	}
	defer handle.Close() //nolint:errcheck // Closing drops the flock; the removal owns the verdict.
	if !held {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove the abandoned site artifact snapshot %s: %w; remove it with: rm -rf %s",
			path, err, path)
	}
	return nil
}

// holdDirectory opens a directory and tries to take an exclusive flock on it
// without waiting. held is false when another open file holds the lock; the
// handle is then still open and the caller MUST close it.
func holdDirectory(path string) (handle *os.File, held bool, err error) {
	handle, err = os.Open(path)
	if err != nil {
		return nil, false, err
	}
	err = syscall.Flock(int(handle.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return handle, true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return handle, false, nil
	}
	_ = handle.Close() //nolint:errcheck // The flock failure is the verdict returned.
	return nil, false, err
}
