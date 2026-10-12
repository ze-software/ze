// Design: docs/architecture/config/transaction-protocol.md -- Rollback
// Related: bfd.go -- applyConfig, which every step here runs

package bfd

import (
	"errors"
	"fmt"
)

// configUndo is the config a rollback of the open transaction restores: the
// one this plugin ran before its accepted apply, nil when it ran none.
type configUndo struct {
	previous *pluginConfig
}

// commitConfig applies cfg as this plugin's step of a config transaction,
// which is all or nothing. When the apply fails, the running config is the
// one before the commit, and the engine sends this plugin no rollback: the
// SDK's rollback gate stays shut for a plugin whose own apply failed
// (docs/architecture/config/transaction-protocol.md, "Rollback"). So the
// apply's partial work is undone here, by applying the config the plugin ran
// before: cfg stops being published, every client takes the request its
// profile resolved to before, and pinned sessions return to that set. When
// the apply succeeds, that config is kept for rollbackConfig, which runs when
// another participant's apply fails. The caller MUST hold runtimeStateGuard.
func (r *runtimeState) commitConfig(cfg *pluginConfig) error {
	previous := r.cfg
	r.undo = nil
	err := r.applyConfig(cfg)
	if err == nil {
		r.undo = &configUndo{previous: previous}
		return nil
	}
	if restoreErr := r.restoreConfig(previous); restoreErr != nil {
		return errors.Join(err, restoreErr)
	}
	return err
}

// rollbackConfig undoes the apply commitConfig accepted in the open
// transaction, and does nothing when none is owed: a rollback answered once
// is not answered again. The caller MUST hold runtimeStateGuard.
func (r *runtimeState) rollbackConfig() error {
	undo := r.undo
	r.undo = nil
	if undo == nil {
		return nil
	}
	return r.restoreConfig(undo.previous)
}

// dropUndo forgets the undo of an earlier transaction, for an accepted apply
// that changed nothing: the SDK runs the rollback handler after every
// accepted apply, and the undo of a committed transaction is not this one's.
// The caller MUST hold runtimeStateGuard.
func (r *runtimeState) dropUndo() {
	r.undo = nil
}

// restoreConfig makes the runtime match previous again, the config this
// plugin ran before a commit; nil means it ran none, so every session stops.
// The caller MUST hold runtimeStateGuard.
func (r *runtimeState) restoreConfig(previous *pluginConfig) error {
	if previous == nil {
		r.stopAll()
		r.cfg = nil
		return nil
	}
	if err := r.applyConfig(previous); err != nil {
		return fmt.Errorf("bfd: restore the config running before this commit: %w", err)
	}
	return nil
}
