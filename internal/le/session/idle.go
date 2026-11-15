// Design: docs/contributing/running-commands.md -- conservative cleanup of ended sessions
// Related: reap.go -- the judgement that reads IdleAge for a session it cannot see
// Related: ../scratch/livetrim.go -- the store trim, which reads TreeIdle for every store
package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// IdleAge is how long a session the liveness proof cannot see must have
// written nothing before the judgement may call it dead. The proof reads only
// a Claude CLI, a pid pin, an argv and a transcript, so a harness session that
// took its identity from the environment, a session under another
// CLAUDE_CONFIG_DIR, and a session on another machine sharing the checkout all
// show it nothing while they run: their recent writes are the one sign of them
// the judgement can read. A fixed constant, not a setting (the store trim's
// owner decision D-3).
const IdleAge = 24 * time.Hour

// TreeIdle answers whether nothing at path was written for age before now: a
// regular file by its own mtime, a directory by the newest mtime anywhere in
// its tree. The walk stops at the first young entry. A symbolic link, or
// anything else that is neither a file nor a directory, is never idle: the
// callers remove only what a session, a build or an agent made. An entry a
// peer already removed is not idle, and not an error. It is the one copy the
// session judgement and the store trim (internal/le/scratch) both read.
func TreeIdle(path string, now time.Time, age time.Duration) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode().IsRegular() {
		return idleSince(info.ModTime(), now, age), nil
	}
	if !info.IsDir() {
		return false, nil
	}
	idle := true
	walkErr := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		entryInfo, infoErr := entry.Info()
		if errors.Is(infoErr, fs.ErrNotExist) {
			return nil
		}
		if infoErr != nil {
			return infoErr
		}
		if idleSince(entryInfo.ModTime(), now, age) {
			return nil
		}
		idle = false
		return fs.SkipAll
	})
	if walkErr != nil {
		return false, walkErr
	}
	return idle, nil
}

// idleSince reports whether modified lies at least age before now.
func idleSince(modified, now time.Time, age time.Duration) bool {
	return now.Sub(modified) >= age
}
