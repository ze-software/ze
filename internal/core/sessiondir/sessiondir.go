// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns

// Package sessiondir names, finds and creates the per-session directories
// under tmp/session/, and it is the one copy of that rule.
//
// The native `./le` session tools (internal/le/le/path) and the test artifact
// router (internal/test/sessionpath) both call it. They held two copies of the
// lookup before, and the copies drifted, so neither may grow its own again.
//
// A directory is named <YYYY-MM-DD>-<HHMMSS> in local time at the moment it is
// first created, with -2, -3, ... appended when another session took the same
// second. The name carries no session id. The owner is recorded in the file
// <dir>/.session-id, which holds the full session id and a newline, and that
// file is the only source of truth for which session owns a directory.
//
// Because the name depends on when the directory was made, it is looked up,
// never recomputed: the first resolution for an id creates the directory and
// every later one finds it by its .session-id, after midnight included. A
// directory named in the earlier <YYYY-MM-DD>-<session-id> shape is found the
// same way once its .session-id exists; Find writes that file the first time
// its own session looks for it.
//
// Safe for concurrent use by several processes: creation runs under an
// exclusive lock on tmp/session/.create.lock, and .session-id is written to a
// temporary file and renamed, so no reader sees a partial id.
package sessiondir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IDFile is the file inside a session directory that names its owner.
const IDFile = ".session-id"

// lockFile serializes directory creation among every process of every session.
const lockFile = ".create.lock"

// nameLayout is the time layout of a new directory's name.
const nameLayout = "2006-01-02-150405"

// datedPrefix is the <YYYY-MM-DD>- prefix every session directory starts with.
const datedPrefix = len("2006-01-02-")

// collisionMax bounds the -N suffixes tried for one second. Each try is one
// session that created its directory in that same second, so the bound is far
// above any real collision and only stops a loop on a broken filesystem.
const collisionMax = 1000

// Dated reports whether name has the <YYYY-MM-DD>- prefix of a session
// directory. It accepts both the <date>-<HHMMSS> shape and the earlier
// <date>-<session-id> shape, which still exist on disk.
func Dated(name string) bool {
	if len(name) <= datedPrefix {
		return false
	}
	if name[4] != '-' || name[7] != '-' || name[10] != '-' {
		return false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if name[index] < '0' || name[index] > '9' {
			return false
		}
	}
	return true
}

// Owner returns the session id recorded in dir/.session-id. A missing file is
// returned as an error wrapping os.ErrNotExist, so a caller can tell "no
// owner recorded" from an unreadable or malformed one.
func Owner(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, IDFile)) //nolint:gosec // dir is a session directory under the checkout
	if err != nil {
		return "", err
	}
	id := strings.TrimSuffix(string(data), "\n")
	if id == "" {
		return "", fmt.Errorf("%s: empty session id", filepath.Join(dir, IDFile))
	}
	if strings.ContainsAny(id, "\n/\\") {
		return "", fmt.Errorf("%s: malformed session id %q", filepath.Join(dir, IDFile), id)
	}
	return id, nil
}

// Find returns the base name of the directory under sessionRoot that id owns,
// and false when there is none. A missing sessionRoot is no directory.
//
// os.ReadDir answers in name order and dated names sort by creation, so a tree
// that somehow holds two directories for one id answers the older.
func Find(sessionRoot, id string) (string, bool, error) {
	entries, err := os.ReadDir(sessionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read session directory %s: %w", sessionRoot, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !Dated(name) {
			continue
		}
		dir := filepath.Join(sessionRoot, name)
		owner, ownerErr := Owner(dir)
		// A directory in the earlier <date>-<session-id> shape gets its
		// .session-id the first time its own session looks for it.
		if errors.Is(ownerErr, os.ErrNotExist) && name[datedPrefix:] == id {
			if err := writeOwner(dir, id); err != nil {
				return "", false, err
			}
			return name, true, nil
		}
		// refuse-by-default: guards Find -- an owner that cannot be read is not provably id, so the directory is not id's
		if ownerErr != nil {
			continue
		}
		if owner == id {
			return name, true, nil
		}
	}
	return "", false, nil
}

// Resolve returns the base name of id's directory under sessionRoot, creating
// it, and sessionRoot, with mode when id owns none. now names a new
// directory; an existing one is returned whatever the clock says.
func Resolve(sessionRoot, id string, now time.Time, mode os.FileMode) (string, error) {
	if name, found, err := Find(sessionRoot, id); err != nil || found {
		return name, err
	}
	if err := os.MkdirAll(sessionRoot, mode); err != nil {
		return "", fmt.Errorf("create session root %s: %w", sessionRoot, err)
	}

	unlock, err := lock(sessionRoot)
	if err != nil {
		return "", err
	}
	defer unlock()

	// Another process of this session may have created it while we waited.
	if name, found, err := Find(sessionRoot, id); err != nil || found {
		return name, err
	}
	return create(sessionRoot, id, now, mode)
}

// create makes a new directory for id. os.Mkdir is the exclusive step: a name
// that already exists answers ErrExist and the next suffix is tried, so two
// sessions never share a directory, whether or not they took the lock.
func create(sessionRoot, id string, now time.Time, mode os.FileMode) (string, error) {
	base := now.Format(nameLayout)
	for attempt := 1; attempt <= collisionMax; attempt++ {
		name := base
		if attempt > 1 {
			name = base + "-" + strconv.Itoa(attempt)
		}
		dir := filepath.Join(sessionRoot, name)
		err := os.Mkdir(dir, mode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create session directory %s: %w", dir, err)
		}
		if err := writeOwner(dir, id); err != nil {
			_ = os.Remove(dir) //nolint:errcheck // the directory was made empty above; the write error is the one reported
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("create session directory under %s: every name from %s to %s-%d is taken",
		sessionRoot, base, base, collisionMax)
}

// writeOwner records id in dir/.session-id through a temporary file and a
// rename, so a concurrent reader sees either no file or the whole id.
func writeOwner(dir, id string) error {
	temporary, err := os.CreateTemp(dir, IDFile+".tmp-*")
	if err != nil {
		return fmt.Errorf("record session owner in %s: %w", dir, err)
	}
	name := temporary.Name()
	_, writeErr := temporary.WriteString(id + "\n")
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(name) //nolint:errcheck // the write error is the one reported
		return fmt.Errorf("record session owner in %s: %w", dir, err)
	}
	// Every agent account on the host reads the owner, as it shares the tree.
	if err := os.Chmod(name, 0o644); err != nil { //nolint:gosec // the owner id is not secret and other accounts read it
		_ = os.Remove(name) //nolint:errcheck // the chmod error is the one reported
		return fmt.Errorf("record session owner in %s: %w", dir, err)
	}
	if err := os.Rename(name, filepath.Join(dir, IDFile)); err != nil {
		_ = os.Remove(name) //nolint:errcheck // the rename error is the one reported
		return fmt.Errorf("record session owner in %s: %w", dir, err)
	}
	return nil
}

// lock takes the creation lock under sessionRoot and returns its release,
// which the caller MUST call. A filesystem that does not support flock
// (some network mounts) answers ENOTSUP or ENOLCK: creation then runs
// unlocked, and os.Mkdir still keeps two sessions out of one directory; only
// two processes of one session racing their first resolution can then make
// two directories, and Find answers the older for both after that.
func lock(sessionRoot string) (func(), error) {
	path := filepath.Join(sessionRoot, lockFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666) //nolint:gosec // shared by every agent account on the host
	if err != nil {
		return nil, fmt.Errorf("open session creation lock %s: %w", path, err)
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX) //nolint:gosec // a file descriptor fits an int
	if errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENOLCK) {
		return func() { _ = file.Close() }, nil //nolint:errcheck // closing a lock file that holds no lock
	}
	if err != nil {
		_ = file.Close() //nolint:errcheck // the flock error is the one reported
		return nil, fmt.Errorf("lock session creation %s: %w", path, err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck,gosec // closing releases the lock anyway
		_ = file.Close()                                   //nolint:errcheck // nothing was written to the lock file
	}, nil
}
