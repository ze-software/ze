// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Related: main.go -- runYANGConfig starts the window; boot recovers it
// Related: session_editor.go -- newSessionEditor hands the window to session editors

package hub

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// daemonConfirmWindow is the running daemon's confirmed-commit window, or nil
// before runYANGConfig starts it and after it stops. It is package state for
// the reason sessionReloadHolder is late-bound: the SSH session factory is
// registered before the reload the window reverts through exists. One process
// runs one daemon, so there is one window.
var daemonConfirmWindow atomic.Pointer[confirm.Window]

// startConfirmWindow starts the window for the daemon's config and publishes
// it to session editors. It answers nil, starting nothing, with no store or
// for a config read from stdin, which has nothing to revert to. The caller
// MUST call stopConfirmWindow with the answer.
func startConfirmWindow(store storage.Storage, configPath string, reload func() error) *confirm.Window {
	if store == nil {
		return nil
	}
	if configPath == "" || configPath == "-" {
		return nil
	}
	window := confirm.NewWindow(confirmRevert(store, configPath, reload), confirm.NewStoreRecorder(store, configPath))
	daemonConfirmWindow.Store(window)
	return window
}

// stopConfirmWindow withdraws the window from session editors and stops its
// worker. A window still open stays recorded, so the next start reverts it
// (AC-19). It MUST run before the commit reloads close, because a revert
// reloads. A nil window and a second call are no-ops.
func stopConfirmWindow(window *confirm.Window) {
	if window == nil {
		return
	}
	daemonConfirmWindow.CompareAndSwap(window, nil)
	window.Stop()
}

// confirmRevert is the running daemon's revert: the rollback is staged as the
// candidate and accepted through the same reload a commit uses, so the
// daemon runs it before it is promoted. Editor.Rollback is not used: it writes
// the config without the candidate and reload path (A-6).
func confirmRevert(store storage.Storage, configPath string, reload func() error) confirm.Reverter {
	return func(rollback []byte) error {
		if _, err := storage.WriteCandidateVersion(store, configPath, rollback, time.Now()); err != nil {
			return fmt.Errorf("revert unconfirmed commit: %w", err)
		}
		if err := reload(); err != nil {
			return errors.Join(fmt.Errorf("revert unconfirmed commit: reload: %w", err), storage.ClearCandidate(store, configPath))
		}
		return nil
	}
}

// recoverConfirmWindow reverts a window the last daemon left open, before the
// config is read, so the restored config is the one that boots (AC-19). It
// runs beside recoverFileCommit.
func recoverConfirmWindow(store storage.Storage, configPath string) error {
	if store == nil {
		return nil
	}
	if configPath == "" || configPath == "-" {
		return nil
	}
	recorder := confirm.NewStoreRecorder(store, configPath)
	record, err := recorder.Load()
	if err != nil {
		return err
	}
	_, err = confirm.RecoverOnStart(record, restoreConfirmOnStart(store, configPath), recorder)
	return err
}

// restoreConfirmOnStart is the boot revert: with no daemon running yet there
// is nothing to reload, so the rollback is published the way a recovered
// config is (publishRecoveredConfig), which writes the explicit file too when
// the daemon runs one. A candidate the stopped daemon left is the interrupted
// commit the revert undoes, so it is cleared first.
func restoreConfirmOnStart(store storage.Storage, configPath string) confirm.Reverter {
	return func(rollback []byte) error {
		if err := storage.ClearCandidate(store, configPath); err != nil {
			return fmt.Errorf("revert unconfirmed commit: %w", err)
		}
		current, err := storage.ReadConfigSource(store, configPath)
		if err != nil {
			return fmt.Errorf("revert unconfirmed commit: %w", err)
		}
		if err := publishRecoveredConfig(store, configPath, current, rollback); err != nil {
			return fmt.Errorf("revert unconfirmed commit: %w", err)
		}
		return nil
	}
}
