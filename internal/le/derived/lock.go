// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
// Related: derived.go -- the registry whose rebuilds and removals this lock serializes
package derived

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/filelock"
)

// LockPath is the lock every rebuild and every removal of a derived artifact
// holds, relative to the checkout root, in slash form.
//
// It lives under tmp/ for three reasons. The checkout ignores tmp/, so the lock
// is never a file a commit could carry. No artifact is registered under tmp/,
// so no invalidation removes the lock with the directory it sits in. And tmp/
// is where every session of the checkout already keeps its own state
// (internal/le/verify's parts record holds its lock there too), so a scratch
// checkout a test creates gets the directory created like any other.
const LockPath = "tmp/derived-artifacts.lock"

// lockWaiting is a TEST SEAM and nil in production: when set, it is called
// once when the lock is found held, just before the caller starts to wait, so
// a test can order a peer's release after the wait began without a sleep.
var lockWaiting func()

// LockCause says why the lock could not be taken. Each cause has its own fix,
// so a refusal that lumped them together would name a command that fixes only
// one of them.
type LockCause uint8

const (
	// LockCauseUnspecified is the zero value and is never a cause.
	LockCauseUnspecified LockCause = iota
	// LockCauseBusy: another session held the lock past the caller's deadline.
	LockCauseBusy
	// LockCauseNotDirectory: tmp/ is a regular file, so nothing can be made in it.
	LockCauseNotDirectory
	// LockCauseDanglingLink: tmp/ is a link to a directory that does not exist.
	LockCauseDanglingLink
	// LockCausePermission: the lock file is absent and tmp/ refuses this user
	// the create.
	LockCausePermission
	// LockCauseFileRefused: the lock file exists, and its owner or its mode
	// refuses this user the open. A writable tmp/ does not fix it.
	LockCauseFileRefused
	// LockCauseRemoved: something removed the lock file repeatedly while waiting.
	LockCauseRemoved
	// LockCauseOther: any other failure, carried in Err.
	LockCauseOther
)

// LockError reports that the derived-artifact lock could not be taken, so the
// rebuild or the removal that needed it did not run.
type LockError struct {
	// Path is LockPath joined to the checkout root.
	Path string
	// Cause selects the fix Operator names.
	Cause LockCause
	// Err is the underlying failure.
	Err error
}

// Error is the text of Operator: the lock that cannot be taken, why for this
// cause with the underlying failure, then the fix alone on the last line. So
// every reporter that prints the error prints the fix too, and the two hooks
// put why it matters before it and add nothing after it.
func (e *LockError) Error() string {
	return e.Operator().Error()
}

// Unwrap answers the cause, so errors.Is sees an absent or unwritable path.
func (e *LockError) Unwrap() error {
	return e.Err
}

// Operator answers the refusal as an operator error, the form a command hands
// leaction.ReportError (ai/rules/user-facing-errors.md). Each cause names its
// own fix: ./le scratch ensure fixes only an absent or dangling tmp/ link, and
// for a real path at tmp/ it prints SKIP (internal/le/scratch ensureSymlink).
func (e *LockError) Operator() *opererr.Error {
	problem := "the derived-artifact lock " + e.Path + " cannot be taken"
	tmp := filepath.Dir(e.Path)
	switch e.Cause {
	case LockCauseBusy:
		return opererr.Wrap(e.Err, problem,
			"another session is rebuilding or removing a derived artifact and held it past the wait",
			"run the same command again once that session is done")
	case LockCauseNotDirectory:
		return opererr.Wrap(e.Err, problem,
			"tmp/ is a file, not a directory, so the lock file cannot be made in it; move it aside and let le recreate the link",
			"mv "+tmp+" "+tmp+".aside && ./le scratch ensure")
	case LockCauseDanglingLink:
		return opererr.Wrap(e.Err, problem,
			"tmp/ links to a directory that does not exist; recreate its target",
			"./le scratch ensure")
	case LockCausePermission:
		// The trailing slash makes chmod act on the directory a tmp/ link
		// resolves to, and it needs no search permission inside tmp/.
		return opererr.Wrap(e.Err, problem,
			"this user cannot create the lock file in tmp/; let this user enter and write the directory it resolves to",
			"chmod u+rwx "+tmp+"/")
	case LockCauseFileRefused:
		// Not removed: a session holding the lock keeps it on the removed
		// file, and the next taker locks a new file beside it. The owner and
		// the mode change in place, on the file every holder shares.
		return opererr.Wrap(e.Err, problem,
			"the lock file exists, but its owner or its mode refuses this user; make it this user's own, readable and writable",
			"sudo chown \"$(id -un)\" "+e.Path+" && chmod u+rw "+e.Path)
	case LockCauseRemoved:
		return opererr.Wrap(e.Err, problem,
			"something removes the lock file in a loop; find the process removing it, then try again",
			"ls -l "+e.Path)
	case LockCauseUnspecified, LockCauseOther:
		return opererr.Wrap(e.Err, problem,
			"le does not classify this failure; fix what it names, then try again",
			"ls -ld "+tmp)
	}
	return opererr.Wrap(e.Err, problem,
		"le does not classify this failure; fix what it names, then try again",
		"ls -ld "+tmp)
}

// lock takes the derived-artifact lock of the checkout at root and answers the
// open lock file. The caller MUST close it, which releases the lock. A zero
// deadline waits without a bound; any other deadline answers a LockError with
// LockCauseBusy once it passes.
//
// One lock per checkout, not one per artifact, because the RFC generator
// publishes six artifacts from one render: a lock per artifact would let a
// removal of rfc/requirements run while a rebuild that holds the lock of
// rfc/enrolled.txt writes the shards.
//
// The hooks pass a deadline inside their own harness timeout: a hook the
// harness kills while it waits says nothing, and a removal that never ran
// leaves a stale artifact that reads as current. An in-process caller with no
// such timeout (EnsureAll, a generator command through WithLock) waits for as
// long as the peer's render takes.
func lock(root string, deadline time.Time) (*os.File, error) {
	path := filepath.Join(root, filepath.FromSlash(LockPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, &LockError{Path: path, Cause: directoryCause(filepath.Dir(path), err), Err: err}
	}
	file, err := filelock.Open(path, deadline, lockWaiting)
	if err != nil {
		return nil, &LockError{Path: path, Cause: openCause(path, err), Err: err}
	}
	return file, nil
}

// directoryCause classifies a failure to create the lock's directory.
func directoryCause(dir string, err error) LockCause {
	if errors.Is(err, fs.ErrPermission) {
		return LockCausePermission
	}
	info, lstatErr := os.Lstat(dir)
	if lstatErr != nil {
		return LockCauseOther
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
			return LockCauseDanglingLink
		}
		return LockCauseOther
	}
	if !info.IsDir() {
		return LockCauseNotDirectory
	}
	return LockCauseOther
}

// openCause classifies a failure of filelock.Open on path. A permission error
// has two causes with two fixes, told apart by whether the lock file exists: a
// file that is there and refused the open belongs to another user or has a
// mode that shuts this one out, while an absent file means the directory
// refused the create.
func openCause(path string, err error) LockCause {
	if errors.Is(err, filelock.ErrBusy) {
		return LockCauseBusy
	}
	if errors.Is(err, filelock.ErrRemoved) {
		return LockCauseRemoved
	}
	if !errors.Is(err, fs.ErrPermission) {
		return LockCauseOther
	}
	// A stat that fails here failed on the directory side too: the file is
	// absent, or tmp/ refuses this user even the search, and a directory the
	// user may enter and write fixes both.
	if _, statErr := os.Lstat(path); statErr == nil {
		return LockCauseFileRefused
	}
	return LockCausePermission
}

// WithLock runs write under the derived-artifact lock of the checkout at
// root, waiting without a bound. A command that writes a registered artifact
// directly (`./le rfc update`, `./le doc index update`) wraps its generator
// call in it, so a hook's removal never cuts through that write either.
//
// The command action takes it, never the generator function that a Rebuild
// calls: Ensure already holds the lock around a Rebuild, and the lock is not
// reentrant, so a generator that locked would wait for itself.
func WithLock(root string, write func() error) error {
	held, err := lock(root, time.Time{})
	if err != nil {
		return err
	}
	defer held.Close() //nolint:errcheck // closing the descriptor releases the lock, and nothing was written through it
	return write()
}

// Ensure builds the artifact when the tree at root does not hold it whole, and
// leaves it alone when it does. It answers the Rebuild error unwrapped, a
// *LockError when the lock could not be taken by deadline (zero waits without
// a bound), and a wrapped error when the artifact could not be read.
//
// The question and the build both run under the checkout's derived-artifact
// lock, so a removal never cuts through a batch the rebuild is publishing. The
// question is asked again under the lock rather than trusted from before it: a
// caller that waited while a peer rebuilt the same artifact finds it whole and
// reuses that build rather than running the render a second time.
//
// A caller that wants a whole artifact to cost one stat asks Whole first and
// calls this only when the answer is no, as EnsureAll and the materialize hook
// do. A Rebuild MUST NOT call Ensure, Remove, EnsureAll or WithLock: the lock
// is not reentrant, and a second descriptor of this process waits for the
// first.
func (a Artifact) Ensure(root string, deadline time.Time) error {
	held, err := lock(root, deadline)
	if err != nil {
		return err
	}
	defer held.Close() //nolint:errcheck // closing the descriptor releases the lock, and nothing was written through it

	whole, err := a.Whole(root)
	if err != nil {
		return fmt.Errorf("read %s: %w", a.Path, err)
	}
	if whole {
		return nil
	}
	return a.Rebuild(root)
}

// Remove deletes the artifact from the tree at root, a file or a whole
// directory, under the checkout's derived-artifact lock. It answers an error
// that errors.Is reads as os.ErrNotExist when the tree does not hold it, and a
// *LockError when the lock could not be taken by deadline (zero waits without
// a bound).
//
// The lock is what keeps the removal out of a rebuild in flight. Without it, a
// removal of rfc/requirements between a batch's first temporary and its last
// rename failed that whole batch, and the reader that asked for the build was
// refused for a tree nothing was wrong with. A removal that waits for the
// build removes what the build wrote, which is correct: the write that asked
// for the removal may have landed after the render read its input.
//
// An absent artifact still takes the lock. Absent is exactly the state in
// which a rebuild is likely in flight, about to publish what it rendered from
// the inputs before this write, so skipping the lock there would leave that
// stale build in place. An uncontended lock costs one open and one flock.
//
// A generator that writes a DIRECTORY of files registers the directory, because
// the set of files inside it is derived as well: rfc/requirements holds one
// shard per summary, and a summary that stops declaring requirements leaves a
// file the generator no longer owns. os.Remove refuses a directory that holds
// anything, so such an artifact would be reported unremovable on every edit and
// would sit there stale.
//
// Lstat rather than Stat, so a symlink is unlinked rather than followed into
// whatever it points at.
func (a Artifact) Remove(root string, deadline time.Time) error {
	held, err := lock(root, deadline)
	if err != nil {
		return err
	}
	defer held.Close() //nolint:errcheck // closing the descriptor releases the lock, and nothing was written through it

	path := filepath.Join(root, filepath.FromSlash(a.Path))
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
