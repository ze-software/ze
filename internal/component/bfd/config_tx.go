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

	// stopDeferred says the accepted apply disabled the plugin and left its
	// loops running until the transaction commits (commitConfig). Stopping a
	// loop closes every protocol client's handle into it, and a client asks
	// again only at its next Established or route change, so a rollback that
	// started new loops would leave BGP and static routes without BFD until
	// then.
	stopDeferred bool
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
// another participant's apply fails.
//
// A disable is published at once, so no client gets a new session, but the
// loops it stops keep running until committedConfig: a rollback then finds
// every protocol client's session where the client left it. A stop still
// deferred when the next apply arrives belongs to a transaction that
// committed without telling this plugin, so it runs first. The caller MUST
// hold runtimeStateGuard.
func (r *runtimeState) commitConfig(cfg *pluginConfig) error {
	r.settleDeferredStop()
	previous := r.cfg
	if !cfg.enabled {
		r.cfg = cfg
		r.undo = &configUndo{previous: previous, stopDeferred: true}
		return nil
	}
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

// committedConfig makes the apply commitConfig accepted in the open
// transaction permanent: a disable stops the loops it left running. The
// caller MUST hold runtimeStateGuard.
func (r *runtimeState) committedConfig() {
	r.settleDeferredStop()
}

// settleDeferredStop runs the stop a committed disable deferred, and forgets
// the undo either way: once committed, no rollback is owed. The caller MUST
// hold runtimeStateGuard.
func (r *runtimeState) settleDeferredStop() {
	undo := r.undo
	r.undo = nil
	if undo == nil {
		return
	}
	if undo.stopDeferred {
		r.stopAll()
	}
}

// rollbackConfig undoes the apply commitConfig accepted in the open
// transaction, and does nothing when none is owed: a rollback answered once
// is not answered again. A disable it undoes stopped no loop, so every
// protocol client keeps its session. The caller MUST hold runtimeStateGuard.
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
// A stop that transaction deferred runs here. The caller MUST hold
// runtimeStateGuard.
func (r *runtimeState) dropUndo() {
	r.settleDeferredStop()
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
		return fmt.Errorf("bfd: the config running before this commit could not be restored, "+
			"so BFD runs part of each config: %w; "+
			"fix what that error names in the bfd section and run commit again", err)
	}
	return nil
}
