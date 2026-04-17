// Design: docs/research/l2tpv2-ze-integration.md -- subsystem lifecycle
// Detail: config.go -- ExtractParameters / Parameters struct consumed by NewSubsystem
// Related: register.go -- blank import wiring for schema/ package
// Related: listener.go -- UDP transport owned by the subsystem
// Related: reactor.go -- dispatch goroutine owned by the subsystem

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// ifaceBackendFn returns the active iface backend wrapped in the small
// interface ppp.Driver consumes. Production wires iface.GetBackend();
// if no backend is loaded the subsystem skips PPP driver construction.
// Package-level var so a future test can swap it when a test-only
// fake iface backend is introduced; no injector exists today.
var ifaceBackendFn = defaultIfaceBackend

// defaultIfaceBackend returns iface.GetBackend() typed as ppp.IfaceBackend
// when one is loaded; nil when none. The PPP driver is only constructed
// when this returns non-nil so MTU-set on pppN is always reachable.
func defaultIfaceBackend() ppp.IfaceBackend {
	b := iface.GetBackend()
	if b == nil {
		return nil
	}
	return b
}

// Compile-time interface check: Subsystem must satisfy ze.Subsystem.
var _ ze.Subsystem = (*Subsystem)(nil)

// SubsystemName is the canonical identifier for the L2TP subsystem.
const SubsystemName = "l2tp"

// probeKernelModulesFn is the kernel module probe invoked at Start().
// Production uses probeKernelModules (Linux modprobe; no-op on other OS).
// Tests override this via export_test.go to run without root privileges.
var probeKernelModulesFn = probeKernelModules

// Subsystem is the ze.Subsystem implementation for L2TPv2.
//
// Phase 3 scope: UDP listener + reactor skeleton are wired. Tunnel state
// machines, timer goroutine, and full FSM transitions land in later
// phases. Start with Parameters whose Enabled=false is a no-op.
//
// Caller MUST call Stop when done if Start returned nil.
type Subsystem struct {
	params Parameters
	logger *slog.Logger

	mu            sync.Mutex
	started       bool
	listeners     []*UDPListener
	reactors      []*L2TPReactor
	timers        []*tunnelTimer
	kernelWorkers []*kernelWorker
	pppDrivers    []*ppp.Driver
}

// NewSubsystem constructs an L2TP subsystem from parsed Parameters. The returned
// value is inert until Start is called.
func NewSubsystem(p Parameters) *Subsystem {
	return &Subsystem{
		params: p,
		logger: slog.Default().With("subsystem", SubsystemName),
	}
}

// Name implements ze.Subsystem.
func (s *Subsystem) Name() string { return SubsystemName }

// Start implements ze.Subsystem. It is a no-op when Enabled=false or when
// no listener addresses are configured. Phase 3 logs the intent; phase 2
// of the tunnel work wires the actual UDP listener.
//
// MUST be called before Stop/Reload.
func (s *Subsystem) Start(ctx context.Context, _ ze.EventBus, _ ze.ConfigProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("l2tp: subsystem already started")
	}

	if !s.params.Enabled {
		s.logger.Info("L2TP subsystem disabled in config, skipping start")
		s.started = true
		return nil
	}
	if len(s.params.ListenAddrs) == 0 {
		s.logger.Warn("L2TP subsystem enabled but no listener configured, skipping start")
		s.started = true
		return nil
	}

	// spec-l2tp-6b-auth Phase 9: surface the effective PPP periodic
	// re-auth interval at startup so the clamp WARN (or the "disabled"
	// parse warning) fires once, before any session connects, rather
	// than only on first successful kernel setup in handleKernelSuccess.
	// handleKernelSuccess still re-reads the env per session so operators
	// can change the value on reload for new sessions.
	if d := clampReauthInterval(s.logger, env.Get("ze.l2tp.auth.reauth-interval")); d > 0 {
		s.logger.Info("l2tp: periodic PPP re-auth enabled", "interval", d)
	}

	// Phase 5: probe kernel modules before binding listeners.
	// AC-1/AC-2: on Linux, modprobe l2tp_ppp or pppol2tp must succeed.
	// On non-Linux, probeKernelModules() is a no-op (returns nil).
	// RFC 2661 Section 24.23: fail startup if module probe fails.
	if err := probeKernelModulesFn(); err != nil {
		return fmt.Errorf("l2tp: %w", err)
	}

	// Bind every configured listen endpoint and launch a reactor + timer
	// + kernel worker for each. On any bind failure, unwind the partial
	// state so a retry is safe.
	for _, addr := range s.params.ListenAddrs {
		ln := NewUDPListener(addr, s.logger)
		if err := ln.Start(ctx); err != nil {
			s.unwindLocked()
			return fmt.Errorf("l2tp: bind %s: %w", addr, err)
		}
		reactor := NewL2TPReactor(ln, s.logger, ReactorParams{
			MaxTunnels:    s.params.MaxTunnels,
			MaxSessions:   s.params.MaxSessions,
			HelloInterval: s.params.HelloInterval,
			Defaults: TunnelDefaults{
				// HostName left empty; reactor applies "ze" default.
				// Phase 7 will wire a YANG leaf for operator-controlled hostname.
				FramingCapabilities: 0x00000003, // sync + async per RFC 2661 S4.4.3
				BearerCapabilities:  0,
				RecvWindow:          16,
				SharedSecret:        s.params.SharedSecret,
			},
		})

		// Phase 5: wire the kernel worker BEFORE starting the reactor so
		// SetKernelWorker's writes happen-before the reactor goroutine
		// first reads kernelErrCh. Worker may be nil on non-Linux or when
		// genl resolve fails -- the reactor handles that gracefully.
		//
		// errCh and successCh each have a single sender (the worker) and a
		// single reader (the reactor's run loop). They are never closed: GC
		// reclaims them when both goroutines exit during Stop. Closing
		// would race with the worker's report selects.
		errCh := make(chan kernelSetupFailed, 16)
		successCh := make(chan kernelSetupSucceeded, 16)
		worker := newSubsystemKernelWorker(errCh, successCh, s.logger)
		reactor.SetKernelWorker(worker, errCh, successCh)

		// Phase 6a: construct a PPP driver if an iface backend is loaded.
		// The driver owns per-session goroutines that drive LCP and (in
		// later specs) auth + NCPs. Skip when no backend is available
		// (test paths, non-Linux, init order); the reactor logs a warning
		// when a kernelSetupSucceeded arrives without a driver.
		var pppDriver *ppp.Driver
		if backend := ifaceBackendFn(); backend != nil {
			pppDriver = ppp.NewProductionDriver(s.logger.With("component", "ppp"), backend)
			reactor.SetPPPDriver(pppDriver)
		}

		// Start ordering: PPP driver before the kernel worker so any
		// success event the worker emits has a consumer ready, and both
		// before the reactor so its select arms have live channels.
		if pppDriver != nil {
			if err := pppDriver.Start(); err != nil {
				startErr := fmt.Errorf("l2tp: start PPP driver for %s: %w", addr, err)
				if stopErr := ln.Stop(); stopErr != nil {
					startErr = errors.Join(startErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
				}
				s.unwindLocked()
				return startErr
			}
		}
		if worker != nil {
			worker.Start()
		}

		if err := reactor.Start(); err != nil {
			if worker != nil {
				worker.Stop()
			}
			if pppDriver != nil {
				pppDriver.Stop()
			}
			reactorErr := fmt.Errorf("l2tp: start reactor for %s: %w", addr, err)
			if stopErr := ln.Stop(); stopErr != nil {
				reactorErr = errors.Join(reactorErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
			}
			s.unwindLocked()
			return reactorErr
		}
		timer := newTunnelTimer(reactor.tickCh, reactor.updateCh)
		if err := timer.Start(); err != nil {
			reactor.Stop()
			if worker != nil {
				worker.Stop()
			}
			if pppDriver != nil {
				pppDriver.Stop()
			}
			timerErr := fmt.Errorf("l2tp: start timer for %s: %w", addr, err)
			if stopErr := ln.Stop(); stopErr != nil {
				timerErr = errors.Join(timerErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
			}
			s.unwindLocked()
			return timerErr
		}
		s.listeners = append(s.listeners, ln)
		s.reactors = append(s.reactors, reactor)
		s.timers = append(s.timers, timer)
		s.kernelWorkers = append(s.kernelWorkers, worker)
		s.pppDrivers = append(s.pppDrivers, pppDriver)
		s.logger.Info("L2TP listener bound", "address", ln.Addr().String())
	}
	s.started = true
	return nil
}

// unwindLocked stops any partially-started reactors and listeners. Must be
// called with s.mu held. Errors are joined so the caller can surface them
// all without suppressing any.
//
// Order matters. Stop timers and reactors BEFORE the PPP drivers so no new
// ppp.StartSession writes land on pppDriver.SessionsIn() mid-teardown.
// Stop the PPP drivers BEFORE the kernel workers: the kernel worker owns
// the fds, and pppDriver.Stop closes them from the PPP side; the kernel
// worker's TeardownAll is idempotent against double-close via closeFD
// error logging. Then TeardownAll drains kernel state and Stop signals
// the worker goroutine to exit. The listener is closed last because the
// kernel data plane (programmed via the worker's socketFD) holds a
// kernel-side reference until tunnel delete completes.
func (s *Subsystem) unwindLocked() {
	var errs []error
	// Timers first: they send on reactor channels, so stop them before
	// the reactors close those channels.
	for _, t := range s.timers {
		t.Stop()
	}
	// Reactors next: after this returns, no new packets are dispatched,
	// no new kernelSetupEvents are enqueued, and no new ppp.StartSession
	// writes land on pppDriver.SessionsIn().
	for _, r := range s.reactors {
		r.Stop()
	}
	// PPP drivers: close every active session's chan fd (blocking reads
	// return EBADF, per-session goroutines exit), wait for them.
	for _, d := range s.pppDrivers {
		if d != nil {
			d.Stop()
		}
	}
	// Kernel workers: SignalStop first to break any in-flight
	// setupSession out of its successCh/errCh channel-send select BEFORE
	// TeardownAll acquires w.mu; otherwise a blocked report would hold
	// w.mu forever. Then TeardownAll drains kernel state, and Stop
	// finally reaps the worker goroutine.
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.SignalStop()
		}
	}
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.TeardownAll()
			kw.Stop()
		}
	}
	// Listeners last: kernel tunnel/session delete commands carry a
	// reference to the UDP socket; close after the worker drains.
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	s.pppDrivers = nil
	s.kernelWorkers = nil
	s.timers = nil
	s.reactors = nil
	s.listeners = nil
	if len(errs) > 0 {
		s.logger.Warn("L2TP partial-start unwind encountered errors", "error", errors.Join(errs...).Error())
	}
}

// Stop implements ze.Subsystem. Idempotent. Reactors are stopped first so
// no more dispatch occurs, then listeners are closed to free the UDP
// sockets.
func (s *Subsystem) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}
	s.logger.Info("L2TP subsystem stopping")

	var errs []error
	// Same order as unwindLocked. Reactors stop before PPP drivers and
	// workers so no new kernelSetupEvents / ppp.StartSession dispatches
	// land after TeardownAll, satisfying AC-14: every kernel resource is
	// torn down before Stop() returns.
	for _, t := range s.timers {
		t.Stop()
	}
	for _, r := range s.reactors {
		r.Stop()
	}
	for _, d := range s.pppDrivers {
		if d != nil {
			d.Stop()
		}
	}
	// Same SignalStop-first pattern as unwindLocked: release w.mu holders
	// before TeardownAll acquires the lock.
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.SignalStop()
		}
	}
	for _, kw := range s.kernelWorkers {
		if kw != nil {
			kw.TeardownAll()
			kw.Stop()
		}
	}
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	s.pppDrivers = nil
	s.kernelWorkers = nil
	s.timers = nil
	s.reactors = nil
	s.listeners = nil
	s.started = false
	return errors.Join(errs...)
}

// Reload implements ze.Subsystem. Phase 3 is restart-only; Reload accepts the
// new ConfigProvider but does not apply changes until phase 7 wires config
// transaction participation.
func (s *Subsystem) Reload(_ context.Context, _ ze.ConfigProvider) error {
	s.logger.Debug("L2TP Reload received (config transaction participation added in phase 7)")
	return nil
}
