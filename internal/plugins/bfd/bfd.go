// Design: rfc/short/rfc5880.md -- BFD plugin entry point
// Design: docs/research/bfd-implementation-guide.md -- ze plugin layout
// Detail: config.go -- parser for the YANG bfd container
//
// Package bfd is the plugin entry point for Bidirectional Forwarding
// Detection. The implementation lives in sub-packages:
//
//   - packet  -- 24-byte Control packet codec, auth header parser, pool.
//   - session -- per-session state machine, timer arithmetic, Poll/Final.
//   - transport -- UDP transports + in-memory loopback for tests.
//   - engine -- express-loop runtime, session registry, Service interface.
//   - api -- public types (SessionRequest, Key, StateChange, Service).
//   - schema -- YANG module ze-bfd-conf.
//
// This file holds the plugin runtime hook (RunBFDPlugin), the package-level
// logger, and the lifecycle that drives the engine package from parsed
// configuration.
package bfd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/engine"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// pluginLogger is the package-level logger. Set via UseLogger from
// the plugin registration callback.
var pluginLogger atomic.Pointer[slog.Logger]

func init() {
	pluginLogger.Store(slogutil.DiscardLogger())
}

// logger returns the current package-level logger.
func logger() *slog.Logger { return pluginLogger.Load() }

// UseLogger swaps in a new logger. Called from the plugin's CLI handler
// after the engine has resolved the per-plugin log level.
func UseLogger(l *slog.Logger) {
	if l != nil {
		pluginLogger.Store(l)
	}
}

// loopKey identifies an engine.Loop instance. Each VRF/hop-mode pair has
// its own express loop because the underlying transport.UDP is bound to
// exactly one port and one routing instance.
type loopKey struct {
	vrf  string
	mode api.HopMode
}

// runtimeState owns the live engine.Loop instances and the handles for
// each pinned session. It is mutated only from the SDK callback goroutine
// (verify/configure/apply run sequentially per the SDK contract) so the
// fields do not need their own lock.
type runtimeState struct {
	loops  map[loopKey]*engine.Loop
	pinned map[api.Key]api.SessionHandle
	cfg    *pluginConfig
}

func newRuntimeState() *runtimeState {
	return &runtimeState{
		loops:  make(map[loopKey]*engine.Loop),
		pinned: make(map[api.Key]api.SessionHandle),
	}
}

// loopFor returns the Loop for the given VRF/mode, creating and starting
// it if necessary. Stage 1 binds a real UDP transport for VRF "default";
// non-default VRFs are not yet supported because they require netns or
// SO_BINDTODEVICE plumbing which lands in spec-bfd-2-transport-hardening.
func (r *runtimeState) loopFor(key loopKey) (*engine.Loop, error) {
	if l, ok := r.loops[key]; ok {
		return l, nil
	}
	if key.vrf != "default" {
		return nil, fmt.Errorf("bfd: VRF %q not yet supported (Stage 1 binds VRF default only)", key.vrf)
	}

	udp := newUDPTransport(key.mode)
	loop := engine.NewLoop(udp, clock.RealClock{})
	if startErr := loop.Start(); startErr != nil {
		if stopErr := udp.Stop(); stopErr != nil {
			logger().Debug("bfd udp stop after failed start", "err", stopErr)
		}
		return nil, fmt.Errorf("bfd: start engine loop for %s: %w", key.mode, startErr)
	}
	r.loops[key] = loop
	logger().Info("bfd loop started", "vrf", key.vrf, "mode", key.mode.String())
	return loop, nil
}

// stopAll tears down every running loop and forgets every pinned handle.
// Called from RunBFDPlugin's deferred cleanup so a clean shutdown returns
// the UDP sockets to the kernel.
func (r *runtimeState) stopAll() {
	for key, loop := range r.loops {
		if err := loop.Stop(); err != nil {
			logger().Warn("bfd loop stop failed", "vrf", key.vrf, "mode", key.mode.String(), "err", err)
		}
	}
	r.loops = map[loopKey]*engine.Loop{}
	r.pinned = map[api.Key]api.SessionHandle{}
}

// applyPinned reconciles the live pinned-session set against cfg. Sessions
// missing from cfg are released; new entries are created. Existing keys
// have their shutdown bit re-applied so an operator flipping `shutdown
// true/false` on a reload takes effect immediately.
func (r *runtimeState) applyPinned(cfg *pluginConfig) error {
	wanted := make(map[api.Key]sessionConfig, len(cfg.sessions))
	for _, s := range cfg.sessions {
		req := s.toSessionRequest(cfg.profiles)
		wanted[req.Key()] = s
	}

	// Release sessions absent from the new config.
	for key, handle := range r.pinned {
		if _, keep := wanted[key]; keep {
			continue
		}
		loop := r.loops[loopKey{vrf: key.VRF, mode: key.Mode}]
		if loop != nil {
			if err := loop.ReleaseSession(handle); err != nil {
				logger().Warn("bfd release session failed", "key", key, "err", err)
			}
		}
		delete(r.pinned, key)
		logger().Info("bfd pinned session removed", "peer", key.Peer.String(), "mode", key.Mode.String())
	}

	// Create or refresh sessions present in the new config.
	for key, s := range wanted {
		loop, err := r.loopFor(loopKey{vrf: key.VRF, mode: key.Mode})
		if err != nil {
			return err
		}
		req := s.toSessionRequest(cfg.profiles)
		handle, ok := r.pinned[key]
		if !ok {
			h, ensureErr := loop.EnsureSession(req)
			if ensureErr != nil {
				return fmt.Errorf("bfd: ensure session %s: %w", key.Peer, ensureErr)
			}
			handle = h
			r.pinned[key] = handle
			logger().Info("bfd pinned session created", "peer", key.Peer.String(), "mode", key.Mode.String(), "vrf", key.VRF)
		}
		if s.shutdown {
			if err := handle.Shutdown(); err != nil {
				return fmt.Errorf("bfd: shutdown session %s: %w", key.Peer, err)
			}
		} else {
			if err := handle.Enable(); err != nil {
				return fmt.Errorf("bfd: enable session %s: %w", key.Peer, err)
			}
		}
	}

	r.cfg = cfg
	return nil
}

// newUDPTransport allocates a transport.UDP for the given hop mode on
// the canonical RFC port. Stage 1 binds the unspecified IPv4 address; the
// transport accepts IPv4 packets only. Stage 2 will dual-bind v4 and v6.
//
// This function does not start the transport: the caller passes the
// returned UDP to engine.NewLoop, then calls Loop.Start which is
// responsible for binding the socket. Bind failures surface there.
func newUDPTransport(mode api.HopMode) *transport.UDP {
	port := transport.UDPPortSingleHopControl
	if mode == api.MultiHop {
		port = transport.UDPPortMultiHopControl
	}
	bind := netip.AddrPortFrom(netip.IPv4Unspecified(), port)
	return &transport.UDP{Bind: bind, Mode: mode, VRF: "default"}
}

// runtimeStateGuard serializes access to the package-level runtime when
// the SDK delivers verify/configure/apply on different goroutines. The
// SDK currently calls them sequentially but we keep the guard so a
// future change cannot silently introduce a race.
var runtimeStateGuard sync.Mutex

// RunBFDPlugin is the engine entry point. It uses the SDK 5-stage protocol
// to receive configuration, drives engine.Loop instances per (VRF, mode)
// for any pinned sessions, and blocks until shutdown.
func RunBFDPlugin(conn net.Conn) int {
	log := logger()
	log.Debug("bfd plugin starting")

	p := sdk.NewWithConn("bfd", conn)
	defer func() { _ = p.Close() }()

	state := newRuntimeState()
	defer func() {
		runtimeStateGuard.Lock()
		defer runtimeStateGuard.Unlock()
		state.stopAll()
	}()

	// pendingCfg holds the validated config between OnConfigVerify and
	// OnConfigApply. The SDK guarantees Configure runs before Verify on
	// startup; on reload, Verify runs and Apply consumes the result.
	var pendingCfg *pluginConfig

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		pendingCfg = cfg
		log.Debug("bfd config verified",
			"profiles", len(cfg.profiles),
			"sessions", len(cfg.sessions),
			"enabled", cfg.enabled)
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return fmt.Errorf("bfd: configure: %w", err)
		}
		runtimeStateGuard.Lock()
		defer runtimeStateGuard.Unlock()
		if !cfg.enabled {
			log.Info("bfd plugin disabled by config")
			state.cfg = cfg
			return nil
		}
		if applyErr := state.applyPinned(cfg); applyErr != nil {
			return applyErr
		}
		log.Info("bfd plugin configured",
			"profiles", len(cfg.profiles),
			"pinned-sessions", len(cfg.sessions))
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			return nil
		}
		runtimeStateGuard.Lock()
		defer runtimeStateGuard.Unlock()
		if !cfg.enabled {
			state.stopAll()
			state.cfg = cfg
			log.Info("bfd plugin disabled via reload")
			return nil
		}
		if err := state.applyPinned(cfg); err != nil {
			return fmt.Errorf("bfd: apply: %w", err)
		}
		log.Info("bfd plugin reloaded",
			"profiles", len(cfg.profiles),
			"pinned-sessions", len(cfg.sessions))
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		log.Info("bfd plugin running")
		return nil
	})

	if err := p.Run(context.Background(), sdk.Registration{
		WantsConfig:  []string{"bfd"},
		VerifyBudget: 1,
		ApplyBudget:  2,
	}); err != nil {
		log.Error("bfd plugin failed", "error", err)
		return 1
	}
	return 0
}
