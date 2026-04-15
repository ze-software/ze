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

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Compile-time interface check: Subsystem must satisfy ze.Subsystem.
var _ ze.Subsystem = (*Subsystem)(nil)

// SubsystemName is the canonical identifier for the L2TP subsystem.
const SubsystemName = "l2tp"

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

	mu        sync.Mutex
	started   bool
	listeners []*UDPListener
	reactors  []*L2TPReactor
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

	// Bind every configured listen endpoint and launch a reactor for each.
	// On any bind failure, unwind the partial state so a retry is safe.
	for _, addr := range s.params.ListenAddrs {
		ln := NewUDPListener(addr, s.logger)
		if err := ln.Start(ctx); err != nil {
			s.unwindLocked()
			return fmt.Errorf("l2tp: bind %s: %w", addr, err)
		}
		reactor := NewL2TPReactor(ln, s.logger, ReactorParams{
			MaxTunnels: s.params.MaxTunnels,
			Defaults: TunnelDefaults{
				// HostName left empty; reactor applies "ze" default.
				// Phase 7 will wire a YANG leaf for operator-controlled hostname.
				FramingCapabilities: 0x00000003, // sync + async per RFC 2661 S4.4.3
				BearerCapabilities:  0,
				RecvWindow:          16,
				SharedSecret:        s.params.SharedSecret,
			},
		})
		if err := reactor.Start(); err != nil {
			reactorErr := fmt.Errorf("l2tp: start reactor for %s: %w", addr, err)
			if stopErr := ln.Stop(); stopErr != nil {
				reactorErr = errors.Join(reactorErr, fmt.Errorf("l2tp: close listener %s: %w", addr, stopErr))
			}
			s.unwindLocked()
			return reactorErr
		}
		s.listeners = append(s.listeners, ln)
		s.reactors = append(s.reactors, reactor)
		s.logger.Info("L2TP listener bound", "address", ln.Addr().String())
	}
	s.started = true
	return nil
}

// unwindLocked stops any partially-started reactors and listeners. Must be
// called with s.mu held. Errors are joined so the caller can surface them
// all without suppressing any.
func (s *Subsystem) unwindLocked() {
	var errs []error
	for _, r := range s.reactors {
		r.Stop()
	}
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
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
	for _, r := range s.reactors {
		r.Stop()
	}
	for _, l := range s.listeners {
		if err := l.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
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
