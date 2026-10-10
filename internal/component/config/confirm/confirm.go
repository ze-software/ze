// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Related: ../storage/pointer.go -- the store keys the pending record sits beside

// Package confirm owns the daemon's confirmed-commit window: one window per
// daemon and config, which a `commit confirmed <seconds>` opens, `commit
// accept` closes, and `commit abort` or the deadline reverts. The window
// belongs to the user who opened it, not to a session, so it survives a
// dropped SSH connection and any session of that user may act on it.
package confirm

import (
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var confirmLog = slogutil.Logger("config.confirm")

var (
	// ErrPending refuses the owner's `commit now [force]` and a plain nested
	// `commit confirmed <seconds>` while the owner's window is open (AC-18 a).
	ErrPending = errors.New("a confirmed commit is pending: use '" + contract.CommitCommand(contract.CommitAccept) +
		"' to keep it, '" + contract.CommitCommand(contract.CommitAbort) + "' to revert it, or '" +
		contract.CommitCommand(contract.CommitConfirmed) + " " + contract.CommitForce +
		"' to add changes and reset the countdown")
	// ErrNoWindow refuses `commit accept` and `commit abort` with no window open.
	ErrNoWindow = errors.New("no confirmed commit is pending")
	// ErrStopped answers every call once the window's worker has stopped.
	ErrStopped = errors.New("the confirmed-commit window is stopped")
)

// OtherUserError refuses a user who does not own the open window (AC-18 b).
// Revert is the error of the window's last failed deadline revert, nil while
// it counts down.
type OtherUserError struct {
	Owner  string
	Left   time.Duration
	Revert error
}

func (e *OtherUserError) Error() string {
	var tb textbuf.Buffer
	if e.Revert != nil {
		return tb.Str("a confirmed commit by ").Str(e.Owner).Str(" passed its deadline and its revert failed (").
			Err(e.Revert).Str("): every other commit is refused until it reverts, or until ").Str(e.Owner).
			Str(" runs '").Str(contract.CommitCommand(contract.CommitAbort)).Str("' to retry the revert or '").
			Str(contract.CommitCommand(contract.CommitAccept)).Str("' to keep it").String()
	}
	return tb.Str("a confirmed commit by ").Str(e.Owner).Str(" is pending with ").
		Int(wholeSeconds(e.Left)).
		Str(" seconds left: wait for its deadline, or have ").Str(e.Owner).
		Str(" run '").Str(contract.CommitCommand(contract.CommitAccept)).Str("' or '").
		Str(contract.CommitCommand(contract.CommitAbort)).Str("'").String()
}

// wholeSeconds rounds left to whole seconds, and a deadline already passed to
// zero: a countdown never shows a negative number.
func wholeSeconds(left time.Duration) int64 {
	if left > 0 {
		return int64(left.Round(time.Second) / time.Second)
	}
	return 0
}

// Pending is the persisted record of an open window: who opened it, when it
// reverts, and the config to restore, which is the config from before the
// FIRST unconfirmed commit of the window (AC-23).
type Pending struct {
	User     string
	Deadline time.Time
	Rollback []byte
}

// Commit is one commit the window runs on its worker, so a commit and a
// revert never interleave. Snapshot returns the config the daemon runs before
// Apply; it is called only when the commit opens a window. Apply writes and
// reloads the commit; an error leaves the window as it was.
type Commit struct {
	Snapshot func() ([]byte, error)
	Apply    func() error
}

// Reverter restores rollback as the daemon's config and reloads it.
type Reverter func(rollback []byte) error

// Recorder persists the pending record, so a daemon restarted during a window
// reverts at start (RecoverOnStart).
type Recorder interface {
	Save(p Pending) error
	Clear() error
}

// revertRetry bounds the retries of a failed deadline revert: the first
// retry waits first, each next one doubles the wait up to max, and after
// attemptsMax reverts in all the worker stops retrying and waits for the
// owner's abort or accept, or a restart, which reverts from the record.
type revertRetry struct {
	first       time.Duration
	max         time.Duration
	attemptsMax int
}

// defaultRevertRetry tries a failed revert eight times over about two minutes.
var defaultRevertRetry = revertRetry{first: time.Second, max: time.Minute, attemptsMax: 8}

// wait is the delay before the retry that follows the attempts-th failure.
func (r revertRetry) wait(attempts int) time.Duration {
	wait := r.first
	for range attempts - 1 {
		wait *= 2
		if wait >= r.max {
			return r.max
		}
	}
	return wait
}

// Window is the daemon's confirmed-commit window. NewWindow starts its one
// worker goroutine, which owns the state and the deadline timer and runs every
// request in order; Stop ends it. Safe for concurrent use: every method that
// changes the window hands its work to the worker and waits for the answer.
// Status and Timeouts never wait for the worker: they read what it published,
// because an editor polls them from its UI loop, and a worker busy reloading
// the daemon must not freeze, or wait on, that loop.
type Window struct {
	revert   Reverter
	record   Recorder
	retry    revertRetry
	requests chan func()
	quit     chan struct{}
	done     chan struct{}

	// Written by the worker, read by Status and Timeouts.
	shown    atomic.Pointer[Status]
	timeouts atomic.Uint64

	// Owned by the worker goroutine alone. revertErr and revertAttempts
	// describe the failed deadline reverts of the open window: while
	// revertErr is set, the timer, when set, is the next retry.
	pending        *Pending
	timer          *time.Timer
	revertErr      error
	revertAttempts int
}

// Status is what an editor shows of an open window: its owner, and the
// deadline it reverts at. RevertFailed is the error of the last failed
// deadline revert, nil while the window counts down; Retry is when the worker
// tries the revert again, zero once its retries are spent.
type Status struct {
	User         string
	Deadline     time.Time
	RevertFailed error
	Retry        time.Time
}

// Left is the time until the window reverts, and zero once the deadline has
// passed.
func (s Status) Left() time.Duration {
	left := time.Until(s.Deadline)
	if left > 0 {
		return left
	}
	return 0
}

// Line is the status line an editor of viewer shows for the window: the
// owner's countdown, another user's notice, or, after a failed revert, what
// failed and what each user may do. While a revert is failing, every user but
// the owner stays refused, because a commit they made would be wiped by the
// revert that later succeeds; the owner may retry it with `commit abort` or
// keep the configuration with `commit accept`.
func (s Status) Line(viewer string) string {
	var tb textbuf.Buffer
	if s.RevertFailed != nil {
		tb.Str("The deadline revert of the confirmed commit by ").Str(s.User).Str(" failed: ").Err(s.RevertFailed).Str(". ")
		if s.Retry.IsZero() {
			tb.Str("No retry is left. ")
		} else {
			tb.Str("Retrying in ").Int(wholeSeconds(time.Until(s.Retry))).Str("s. ")
		}
		if viewer == s.User {
			return tb.Str("Use '").Str(contract.CommitCommand(contract.CommitAbort)).Str("' to retry the revert or '").
				Str(contract.CommitCommand(contract.CommitAccept)).Str("' to keep this configuration.").String()
		}
		return tb.Str("Commits are refused until it reverts or ").Str(s.User).Str(" accepts it.").String()
	}
	if viewer == s.User {
		return contract.ConfirmWithin(wholeSeconds(s.Left()))
	}
	return tb.Str("A confirmed commit by ").Str(s.User).Str(" is pending: ").Int(wholeSeconds(s.Left())).Str("s left.").String()
}

// NewWindow starts the window's worker. The caller MUST call Stop.
func NewWindow(revert Reverter, record Recorder) *Window {
	return newWindow(revert, record, defaultRevertRetry)
}

// newWindow is NewWindow with the retry bounds of a failed revert.
func newWindow(revert Reverter, record Recorder, retry revertRetry) *Window {
	w := &Window{
		revert:   revert,
		record:   record,
		retry:    retry,
		requests: make(chan func()),
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go w.run()
	return w
}

// Stop ends the worker and waits for it. A window still open stays recorded,
// so the next start reverts it (AC-19). Calling Stop twice is safe.
func (w *Window) Stop() {
	select {
	case <-w.quit:
	default:
		close(w.quit)
	}
	<-w.done
}

// run is the worker. It loops until Stop: the loop has no other bound because
// the window lives as long as the daemon.
func (w *Window) run() {
	defer close(w.done)
	for {
		var deadline <-chan time.Time
		if w.timer != nil {
			deadline = w.timer.C
		}
		select {
		case <-w.quit:
			if w.timer != nil {
				w.timer.Stop()
			}
			return
		case request := <-w.requests:
			request()
		case <-deadline:
			w.timer = nil
			w.deadlineRevert()
		}
	}
}

// deadlineRevert reverts a window whose deadline, or whose revert retry, is
// due. A failed revert keeps the window, its record and its refusals, and the
// worker retries it within w.retry; the failure is logged at each attempt and
// published in Status, so every editor shows it. Called on the worker.
func (w *Window) deadlineRevert() {
	err := w.restore()
	if err == nil {
		w.timeouts.Add(1)
		confirmLog.Info("confirmed commit not accepted: reverted to the previous configuration")
		return
	}
	w.revertErr = err
	w.revertAttempts++
	shown := Status{User: w.pending.User, Deadline: w.pending.Deadline, RevertFailed: err}
	if w.revertAttempts < w.retry.attemptsMax {
		wait := w.retry.wait(w.revertAttempts)
		w.timer = time.NewTimer(wait)
		shown.Retry = time.Now().Add(wait)
		confirmLog.Error("confirmed commit not accepted, and the revert failed: retrying",
			"user", w.pending.User, "attempt", w.revertAttempts, "retry-in", wait, "error", err)
	} else {
		confirmLog.Error("confirmed commit not accepted, and every revert failed: the owner must abort or accept it, or a restart reverts it",
			"user", w.pending.User, "attempts", w.revertAttempts, "error", err)
	}
	w.shown.Store(&shown)
}

// do runs fn on the worker and returns its answer, or ErrStopped.
func (w *Window) do(fn func() error) error {
	answer := make(chan error, 1)
	select {
	case w.requests <- func() { answer <- fn() }:
		return <-answer
	case <-w.quit:
		return ErrStopped
	}
}

// refuse answers whether user may not act while a window is open: nil with no
// window, ErrPending for the owner, OtherUserError for anyone else.
func (w *Window) refuse(user string) error {
	if w.pending == nil {
		return nil
	}
	if w.pending.User == user {
		return ErrPending
	}
	return w.otherUserError()
}

// otherUserError is the refusal of anyone but the open window's owner. The
// window MUST be open.
func (w *Window) otherUserError() *OtherUserError {
	return &OtherUserError{Owner: w.pending.User, Left: time.Until(w.pending.Deadline), Revert: w.revertErr}
}

// WriteOutside runs apply, a config write that no editor makes (an API
// config session, a data restore, a raw-source commit), on window's worker:
// refused with OtherUserError while any window is open, whoever asks, because
// the window's revert would wipe the write. A nil window, a daemon with no
// window, runs apply directly.
func WriteOutside(window *Window, apply func() error) error {
	if window == nil {
		return apply()
	}
	return window.do(func() error {
		if window.pending != nil {
			return window.otherUserError()
		}
		return apply()
	})
}

// Now runs `commit now [force]`: refused while any window is open (AC-18).
func (w *Window) Now(user string, commit Commit) error {
	return w.do(func() error {
		if err := w.refuse(user); err != nil {
			return err
		}
		return commit.Apply()
	})
}

// Confirmed runs `commit confirmed <seconds> [force]`. With no window it
// snapshots the running config, applies, and opens a window for user. Inside
// the owner's window it is refused unless force, which applies, keeps the
// first rollback, and restarts the countdown at seconds (AC-23).
func (w *Window) Confirmed(user string, seconds time.Duration, force bool, commit Commit) error {
	return w.do(func() error {
		if w.pending == nil {
			return w.open(user, seconds, commit)
		}
		if w.pending.User != user {
			return w.refuse(user)
		}
		if !force {
			return ErrPending
		}
		previous := *w.pending
		next := previous
		next.Deadline = time.Now().Add(seconds)
		if err := w.record.Save(next); err != nil {
			return err
		}
		if err := commit.Apply(); err != nil {
			return errors.Join(err, w.record.Save(previous))
		}
		w.arm(next)
		return nil
	})
}

// open snapshots the running config and opens a window over commit. The
// record is saved BEFORE the commit applies, so no commit is ever applied
// with no revert behind it: a failed save applies nothing, and a daemon that
// dies after the apply boots the rollback (AC-19). A commit that fails to
// apply clears the record again. Called on the worker.
func (w *Window) open(user string, seconds time.Duration, commit Commit) error {
	rollback, err := commit.Snapshot()
	if err != nil {
		return err
	}
	p := Pending{User: user, Deadline: time.Now().Add(seconds), Rollback: rollback}
	if err := w.record.Save(p); err != nil {
		return err
	}
	if err := commit.Apply(); err != nil {
		return errors.Join(err, w.record.Clear())
	}
	w.arm(p)
	return nil
}

// arm makes p, already recorded, the open window and sets the timer to its
// deadline; a failed revert of the previous deadline is forgotten, because p
// has a deadline of its own. Called on the worker.
func (w *Window) arm(p Pending) {
	if w.timer != nil {
		w.timer.Stop()
	}
	w.pending = &p
	w.revertErr = nil
	w.revertAttempts = 0
	w.shown.Store(&Status{User: p.User, Deadline: p.Deadline})
	w.timer = time.NewTimer(time.Until(p.Deadline))
}

// Accept runs `commit accept`: the owner's open window closes and keeps what
// it applied; nothing else is applied (AC-24).
func (w *Window) Accept(user string) error {
	return w.do(func() error {
		if err := w.owned(user); err != nil {
			return err
		}
		if err := w.record.Clear(); err != nil {
			return err
		}
		w.close()
		return nil
	})
}

// Abort runs `commit abort`: the owner's open window reverts at once.
func (w *Window) Abort(user string) error {
	return w.do(func() error {
		if err := w.owned(user); err != nil {
			return err
		}
		return w.restore()
	})
}

// owned answers whether user may accept or abort. Called on the worker.
func (w *Window) owned(user string) error {
	if w.pending == nil {
		return ErrNoWindow
	}
	if w.pending.User != user {
		return w.refuse(user)
	}
	return nil
}

// restore reverts the open window to its rollback and closes it. A failed
// revert keeps the window and its record, so a restart still reverts.
func (w *Window) restore() error {
	if err := w.revert(w.pending.Rollback); err != nil {
		return err
	}
	if err := w.record.Clear(); err != nil {
		return err
	}
	w.close()
	return nil
}

// close forgets the open window. Called on the worker.
func (w *Window) close() {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.pending = nil
	w.revertErr = nil
	w.revertAttempts = 0
	w.shown.Store(nil)
}

// Status reports the open window, and false when none is open or the worker
// has stopped. It does not wait for the worker.
func (w *Window) Status() (Status, bool) {
	select {
	case <-w.done:
		return Status{}, false
	default:
	}
	shown := w.shown.Load()
	if shown == nil {
		return Status{}, false
	}
	return *shown, true
}

// Timeouts counts the windows the deadline reverted, so an editor that saw a
// window open can tell, once it is gone, a timeout from an accept or an abort
// another session of the owner ran. It does not wait for the worker; a
// stopped worker answers ErrStopped.
func (w *Window) Timeouts() (uint64, error) {
	select {
	case <-w.done:
		return 0, ErrStopped
	default:
	}
	return w.timeouts.Load(), nil
}

// RecoverOnStart reverts a window a stopped daemon left open (AC-19). It runs
// before the daemon reads its config, so the reverted config is the one that
// boots. A nil record reverts nothing and answers false.
func RecoverOnStart(p *Pending, revert Reverter, record Recorder) (bool, error) {
	if p == nil {
		return false, nil
	}
	if err := revert(p.Rollback); err != nil {
		return false, err
	}
	if err := record.Clear(); err != nil {
		return true, err
	}
	confirmLog.Info("reverted an unconfirmed commit left pending at shutdown", slog.String("user", p.User))
	return true, nil
}
