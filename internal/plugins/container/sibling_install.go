// Design: docs/architecture/container/runtime.md -- the sibling seam: the order of the siblings' apply and the supervisor's
// Related: sibling.go -- the siblings' registration
// Related: supervise.go -- the supervisor's apply and begin, which this install orders after the siblings'
// Related: container.go -- the engine entry, which hands this install to the config transaction

package container

import (
	"context"
	"errors"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// configInstaller is the config transaction's install. It applies each
// sibling's part of a config, in registration order, then the supervisor's,
// so what a sibling makes (the network's bridges and tables) exists before a
// container of the new config starts. The supervisor's apply returns before
// any container stops, so a sibling removes what an old container used at
// that container's ended call, never at its apply.
//
// The boot configure arrives before the engine answers the plugin's
// requests, so an install then only records the config and makes the
// supervisor's apply, which records too. begin applies the siblings' part of
// the recorded config, then lets the supervisor start the containers.
//
// install is called by the config transaction under its own lock, begin by
// the engine's started notice; both hold mu, so a sibling's apply runs one at
// a time. Safe for concurrent use.
type configInstaller struct {
	ctx      context.Context
	siblings []configSibling
	sup      *supervisor

	mu sync.Mutex
	// started is set by begin. Before it no sibling apply is made.
	started bool
	// installed is the config the last install made running. The siblings
	// run their part of it once started is set.
	installed containerConfig
}

// newConfigInstaller returns the install of the plugin's config. ctx is the
// engine entry's: a sibling's apply ends with it.
func newConfigInstaller(ctx context.Context, sup *supervisor, siblings []configSibling) *configInstaller {
	return &configInstaller{ctx: ctx, siblings: siblings, sup: sup}
}

// install makes cfg the running config, as sdk.ConfigInstaller requires: on
// an error the running config is what it was, or the error wraps
// sdk.ErrRunningConfigLost.
func (i *configInstaller) install(cfg containerConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.started {
		if err := i.applySiblingsLocked(cfg, i.installed); err != nil {
			return err
		}
	}
	if err := i.sup.apply(cfg); err != nil {
		if !i.started {
			return err
		}
		if restoreErr := i.restoreSiblingsLocked(len(i.siblings), i.installed); restoreErr != nil {
			return sdk.RunningConfigLost(errors.Join(err, restoreErr), "", "")
		}
		return err
	}
	i.installed = cfg
	return nil
}

// begin is the engine's notice that the plugin started. It applies the
// siblings' part of the installed config, then wakes the supervisor's
// workers. A sibling that refuses fails the plugin's startup, as a boot
// configure the plugin cannot install does: no container starts. It MUST be
// called once.
func (i *configInstaller) begin() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.applySiblingsLocked(i.installed, containerConfig{}); err != nil {
		return err
	}
	i.started = true
	i.sup.begin()
	return nil
}

// applySiblingsLocked applies each sibling's part of target, in registration
// order. When one refuses, the siblings that applied before it get their part
// of previous back, in reverse order, and the refusal is the answer. When a
// put-back fails too, the answer wraps sdk.ErrRunningConfigLost, so the
// transaction's rollback installs the running config again. MUST be called
// with mu held.
func (i *configInstaller) applySiblingsLocked(target, previous containerConfig) error {
	for index := range i.siblings {
		sibling := &i.siblings[index]
		err := i.applySibling(sibling, sibling.valueIn(target))
		if err == nil {
			continue
		}
		if restoreErr := i.restoreSiblingsLocked(index, previous); restoreErr != nil {
			return sdk.RunningConfigLost(errors.Join(err, restoreErr), "", "")
		}
		return err
	}
	return nil
}

// restoreSiblingsLocked applies previous to the first count siblings, the
// last first, and answers every failure. MUST be called with mu held.
func (i *configInstaller) restoreSiblingsLocked(count int, previous containerConfig) error {
	var failures []error
	for index := count - 1; index >= 0; index-- {
		sibling := &i.siblings[index]
		if err := i.applySibling(sibling, sibling.valueIn(previous)); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// applySibling makes one sibling apply value, bounded as a participant's call
// is.
func (i *configInstaller) applySibling(sibling *configSibling, value any) error {
	ctx, cancel := context.WithTimeout(i.ctx, participantCallTimeout)
	defer cancel()
	return sibling.apply(ctx, value)
}
