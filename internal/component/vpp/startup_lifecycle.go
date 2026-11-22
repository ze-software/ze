// Design: docs/research/vpp-deployment-reference.md -- startup configuration transactions
// Related: register.go -- SDK lifecycle and configuration transaction wiring

package vpp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// vppLifecycle owns exactly one manager worker. Start opens its boot lifetime;
// Install cancels and joins that worker before replacing it. Stop MUST join
// the final worker before the plugin returns. Safe for concurrent use.
type vppLifecycle struct {
	mu       sync.Mutex
	ctx      context.Context
	run      func(context.Context, *VPPSettings, bool, chan<- error) error
	settings *VPPSettings
	active   *vppLifetime
	// started guards the SDK phase boundary: Configure records settings, but
	// only OnStarted may create the manager and make runtime engine calls.
	started   bool
	everReady bool
}

type vppLifetime struct {
	cancel context.CancelFunc
	done   chan struct{}
	result error // published by closing done, then immutable
	// cleanup is installed only after the original worker has joined. Its
	// receipt keeps the original manager alive through explicit cleanup retry.
	cleanup *nativeCleanupError
}

const startupReadyTimeout = 45 * time.Second

// newVPPLifecycle starts no worker. The caller MUST Stop it before returning.
// Unresolved cleanup MUST block replacement for this lifecycle's lifetime.
func newVPPLifecycle(ctx context.Context,
	run func(context.Context, *VPPSettings, bool, chan<- error) error) *vppLifecycle {
	return &vppLifecycle{ctx: ctx, run: run, settings: &VPPSettings{Enabled: false}}
}

func newVPPConfigTx(runtime *vppLifecycle) *sdk.ConfigTx[*VPPSettings] {
	return sdk.NewConfigTx(&VPPSettings{Enabled: false}, parseVPPTransaction, runtime.Install)
}

func parseVPPTransaction(sections []sdk.ConfigSection) (*VPPSettings, bool, error) {
	if len(sections) == 0 {
		return nil, false, nil
	}
	settings, err := ParseConfigSections(sections)
	if err != nil {
		return nil, false, err
	}
	if err := settings.Validate(); err != nil {
		return nil, false, err
	}
	return settings, true, nil
}

func (r *vppLifecycle) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("VPP lifecycle already started")
	}
	r.started = true
	return r.replace(r.settings)
}

// Install MUST restore the old manager after a failed candidate's confirmed
// cleanup. Unresolved cleanup MUST retain the original owner and block both
// candidate and restoration. ConfigTx's broken-state retry keeps the old
// settings as its target when the running configuration could not be restored.
func (r *vppLifecycle) Install(settings *VPPSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		r.settings = settings
		return nil
	}
	previous := r.settings
	if err := r.replace(settings); err != nil {
		if r.active != nil {
			return sdk.RunningConfigLost(err,
				"VPP could not restore its original native resources", "ze doctor")
		}
		if restoreErr := r.replace(previous); restoreErr != nil {
			return sdk.RunningConfigLost(errors.Join(err, restoreErr),
				"VPP could not apply the candidate configuration or restore its running configuration", "ze doctor")
		}
		return opererr.Wrap(err, "VPP could not apply the candidate configuration",
			"The previous VPP configuration has been installed again.", "ze doctor")
	}
	r.settings = settings
	return nil
}

func (r *vppLifecycle) replace(settings *VPPSettings) error {
	if err := r.stop(); err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan struct{})
	ready := make(chan error, 1)
	active := &vppLifetime{cancel: cancel, done: done}
	r.active = active
	// One worker owns one complete managed or external connection lifetime.
	// Every result path below cancels and joins it before a replacement starts.
	go func() {
		defer close(done)
		active.result = r.run(ctx, settings, r.everReady, ready)
	}()
	timer := time.NewTimer(startupReadyTimeout)
	defer timer.Stop()
	var failure error
	select {
	case failure = <-ready:
		if failure == nil {
			r.everReady = true
			return nil
		}
	case <-done:
		failure = active.result
		if failure == nil {
			failure = errors.New("VPP manager ended before publishing readiness")
		}
	case <-r.ctx.Done():
		failure = r.ctx.Err()
	case <-timer.C:
		failure = errors.New("VPP did not publish readiness within 45 seconds")
	}
	return errors.Join(failure, r.stop())
}

// Stop MUST be called before the owning plugin returns; it cancels and joins
// the worker rather than letting a replaced process outlive its owner. An
// unresolved result retains its owner; the caller MUST treat that cleanup as
// unfinished unless a later Stop against this same lifecycle confirms it.
func (r *vppLifecycle) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stop()
}

func (r *vppLifecycle) stop() error {
	if r.active == nil {
		return nil
	}
	active := r.active
	if active.cleanup != nil {
		if active.cleanup.receipt == nil {
			return active.result
		}
		if err := active.cleanup.receipt.retry(); err != nil {
			return err
		}
		r.active = nil
		return nil
	}
	active.cancel()
	<-active.done
	if cleanup, ok := errors.AsType[nativeCleanupError](active.result); ok {
		active.cleanup = &cleanup
		return active.result
	}
	r.active = nil
	return active.result
}
