// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: model_commit_window.go -- the SSH session editor's commit through the window
// Related: ../config/confirm/confirm.go -- the daemon's window worker
// Related: ../web/cli_terminal.go -- the web editor's commit through the window

package cli

import (
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

var (
	// errCommitConfirmedNeedsDaemon refuses a session `commit confirmed` with
	// no daemon window to own the countdown: a session editor of a config the
	// daemon does not run, or a daemon that does not run it from its store.
	errCommitConfirmedNeedsDaemon = errors.New("commit confirmed needs a daemon that runs this configuration from its store")
	// ErrCommitNotApplied is what a WindowCommit's apply returns when the
	// commit did not happen (validation, a conflict, nothing pending, or the
	// reload refused it): the window then opens nothing, and the caller shows
	// the commit's own answer.
	ErrCommitNotApplied = errors.New("commit not applied")
	// errCommitVerifyNotWindowAction refuses `commit verify` here: it changes
	// no config, so each editor answers it before reaching the window.
	errCommitVerifyNotWindowAction = errors.New("commit verify does not go through the confirmed-commit window")
)

// WindowCommit is one session user's commit through the daemon's
// confirmed-commit window. The SSH session editor and the web editor both run
// `commit now`, `commit confirmed`, `commit accept` and `commit abort` through
// Run, so the window's owner rules (AC-17, AC-18, AC-23) have one code path.
type WindowCommit struct {
	// Window is the daemon's window. Nil means no daemon runs this config
	// from its store: `commit now` applies directly, and the window
	// subcommands are refused.
	Window *confirm.Window
	// User owns any window this commit opens.
	User string
	// Store and ConfigPath name the config a confirmed commit snapshots for
	// its revert. A nil Store holds no history to revert to (AC-21).
	Store      storage.Storage
	ConfigPath string
}

// Run runs req. apply is the commit `commit now` and `commit confirmed` make;
// it runs on the window's worker, so it never interleaves with a revert, and
// it returns ErrCommitNotApplied when nothing was committed.
func (c WindowCommit) Run(req contract.CommitRequest, apply func() error) error {
	switch req.Action {
	case contract.CommitNow:
		if c.Window == nil {
			return apply()
		}
		return c.Window.Now(c.User, confirm.Commit{Apply: apply})
	case contract.CommitConfirmed:
		if c.Window == nil {
			return errCommitConfirmedNeedsDaemon
		}
		// AC-21: the window's revert restores a config version, so a store
		// with no history refuses before anything is written.
		if c.Store == nil {
			return errCommitConfirmedNeedsHistory
		}
		return c.Window.Confirmed(c.User, time.Duration(req.Seconds)*time.Second, req.Force, confirm.Commit{
			Snapshot: func() ([]byte, error) { return storage.ReadActiveConfig(c.Store, c.ConfigPath) },
			Apply:    apply,
		})
	case contract.CommitAccept:
		if c.Window == nil {
			return confirm.ErrNoWindow
		}
		return c.Window.Accept(c.User)
	case contract.CommitAbort:
		if c.Window == nil {
			return confirm.ErrNoWindow
		}
		return c.Window.Abort(c.User)
	case contract.CommitVerify:
		return errCommitVerifyNotWindowAction
	case contract.CommitActionUnspecified:
		panic("BUG: commit request carries no action")
	}
	panic("BUG: unknown commit action")
}
