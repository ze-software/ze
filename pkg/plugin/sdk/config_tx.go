// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: sdk_callbacks.go -- OnConfigure, OnConfigVerify, OnConfigApply, OnConfigRollback, which Register binds
// Related: config_tx_gate.go -- the gate that lets a rollback reach only a plugin that applied

package sdk

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// ErrConfigApplyUnverified refuses a config-apply that no accepted
// config-verify of the same transaction preceded. Answering OK would report a
// commit the plugin never took.
var ErrConfigApplyUnverified = errors.New("config apply requires a verified candidate")

// ErrRunningConfigLost marks an install error after which the running config
// is no longer what runs: the installer failed and could not put the running
// config back either (a listener of the running config could not be bound
// again, say). An installer wraps it, with %w, only when it cannot meet the
// ConfigInstaller contract. The transaction then treats the running config as
// not installed: the rollback of the failed transaction installs it again and,
// when that fails too, answers an error so the engine restarts the plugin
// (CodeBroken), and a later apply of the same config installs it rather than
// skipping it as unchanged.
var ErrRunningConfigLost = errors.New("the running config could not be put back, so it is not fully running")

// RunningConfigLostNext is what an error that wraps ErrRunningConfigLost for
// a lost listener tells the operator after the facts: what the engine does
// next, and the operator's own fix. The error reaches the operator only after
// the rollback ran, and the engine appends the rollback's outcome to it
// (transaction.TxCoordinator), so this text claims neither outcome. A commit
// of the running config is not the fix: it has no diff, so no apply reaches
// the plugin.
const RunningConfigLostNext = "ze rolls the commit back: the rollback binds the running config again, " +
	"and restarts the plugin when that fails too; the Rollback line of the reload answer says which happened.\n" +
	"Fix: stop the program that holds the address and port named above, or give the interface its address back. " +
	"If the Rollback line says ze restarted the plugin, its listeners on a port that was still in use stay down; " +
	"once the port is free, run: ze signal restart"

// ConfigParser reads a plugin's config from the sections of one delivery. It
// reports present false when the sections carry nothing the plugin reads (its
// root is absent), which a reload means as "unchanged". It MUST NOT change
// any state: a verify that is refused, and a CLI validate, run it too.
type ConfigParser[T any] func(sections []ConfigSection) (cfg T, present bool, err error)

// ConfigInstaller makes cfg the plugin's running config. When it returns an
// error it MUST leave the running config as it was, because the transaction's
// rollback does not reach a plugin whose own apply failed. An installer that
// cannot (it tried and failed to put the running config back) MUST wrap
// ErrRunningConfigLost in its error, which hands the repair to the rollback.
type ConfigInstaller[T any] func(cfg T) error

// ConfigTx is the config transaction of a plugin whose whole config is one
// value it parses and installs: Stage 2 installs at once, a reload's verify
// parses into a pending candidate, apply installs it and keeps the replaced
// config, and rollback installs that back.
//
// Apply installs nothing when the candidate deeply equals the running config
// (reflect.DeepEqual), so a plugin on a shared root (bgp, service) restarts
// nothing on an edit outside its own subtree. A config value holding a func
// or a channel never compares equal and is installed on every apply.
//
// An install that fails with ErrRunningConfigLost leaves the transaction
// broken: Apply compares nothing and installs, and Rollback installs the
// running config again, failing (so the engine restarts the plugin) when
// that install fails too.
//
// Safe for concurrent use.
type ConfigTx[T any] struct {
	parse   ConfigParser[T]
	install ConfigInstaller[T]

	mu      sync.Mutex
	running T
	// pending is the candidate of a verify that accepted it, nil when no
	// verified transaction is open. A verify whose sections carried nothing
	// for the plugin still opens one, with present false.
	pending *configCandidate[T]
	// previous is what the last accepted apply replaced, nil when that apply
	// changed nothing or a rollback already put it back.
	previous *T
	// broken is true while running is not what runs: an install failed with
	// ErrRunningConfigLost and no install of running has succeeded since. It
	// is the guard that stops Apply skipping running as unchanged.
	broken bool
}

// configCandidate is one verified candidate: its config, and whether the
// verify carried the plugin's root at all.
type configCandidate[T any] struct {
	cfg     T
	present bool
}

// NewConfigTx returns the transaction of a plugin whose config parse reads
// and install makes running. unconfigured MUST describe what the plugin runs
// before any install: a rollback of the first apply installs it.
func NewConfigTx[T any](unconfigured T, parse ConfigParser[T], install ConfigInstaller[T]) *ConfigTx[T] {
	return &ConfigTx[T]{parse: parse, install: install, running: unconfigured}
}

// Register binds the transaction to the plugin's Stage 2 configure and its
// reload verify, apply and rollback callbacks. It MUST be called before Run.
func (tx *ConfigTx[T]) Register(p *Plugin) {
	p.OnConfigure(tx.Configure)
	p.OnConfigVerify(tx.Verify)
	p.OnConfigApply(func(_ []ConfigDiffSection) error { return tx.Apply() })
	p.OnConfigRollback(func(_ string) error { return tx.Rollback() })
}

// Configure is the boot delivery (Stage 2): the parsed config is installed at
// once. Sections that carry nothing for the plugin leave it unconfigured.
func (tx *ConfigTx[T]) Configure(sections []ConfigSection) error {
	cfg, present, err := tx.parse(sections)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if err := tx.install(cfg); err != nil {
		return err
	}
	tx.running, tx.previous, tx.broken = cfg, nil, false
	return nil
}

// Verify parses a reload candidate and holds it for apply. A refused
// candidate leaves no verified transaction open, so a following apply is
// refused too.
func (tx *ConfigTx[T]) Verify(sections []ConfigSection) error {
	cfg, present, err := tx.parse(sections)
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.pending = nil
	if err != nil {
		return err
	}
	tx.pending = &configCandidate[T]{cfg: cfg, present: present}
	return nil
}

// Apply installs the verified candidate and keeps the config it replaced for
// Rollback. A candidate that is absent or equal to the running config
// installs nothing and drops the undo set of an earlier apply, which belongs
// to a transaction that already committed. A broken transaction (see
// ConfigTx) skips the comparison, so committing the running config again
// installs it.
func (tx *ConfigTx[T]) Apply() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	candidate := tx.pending
	tx.pending, tx.previous = nil, nil
	if candidate == nil {
		return ErrConfigApplyUnverified
	}
	if !candidate.present {
		return nil
	}
	if !tx.broken && reflect.DeepEqual(candidate.cfg, tx.running) {
		return nil
	}
	if err := tx.install(candidate.cfg); err != nil {
		if errors.Is(err, ErrRunningConfigLost) {
			tx.broken = true
		}
		return err
	}
	replaced := tx.running
	tx.running, tx.previous, tx.broken = candidate.cfg, &replaced, false
	return nil
}

// Rollback installs back the config the last accepted apply replaced, when
// that apply changed one. In a broken transaction it installs the running
// config again instead. An error here leaves the plugin's state unknown; the
// engine then restarts the plugin, which reads the committed config at
// Stage 2.
func (tx *ConfigTx[T]) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.pending = nil
	if tx.broken {
		if err := tx.install(tx.running); err != nil {
			return fmt.Errorf("the running config could not be installed again, so the plugin must restart: %w", err)
		}
		tx.broken = false
		return nil
	}
	if tx.previous == nil {
		return nil
	}
	if err := tx.install(*tx.previous); err != nil {
		return err
	}
	tx.running, tx.previous = *tx.previous, nil
	return nil
}
