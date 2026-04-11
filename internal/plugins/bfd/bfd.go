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
	"sort"
	"strings"
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
// it if necessary. Stage 2 extends the transport with SO_BINDTODEVICE
// support, so non-default VRFs bind the socket to the VRF device name.
// Single-hop loops also bind to a session interface when every pinned
// session in that (vrf, single-hop) pair names the same interface; a
// mismatch between pinned interfaces falls back to "no bind-to-device"
// and logs a warning so the operator notices the reduced GTSM depth.
func (r *runtimeState) loopFor(key loopKey, device string) (*engine.Loop, error) {
	if l, ok := r.loops[key]; ok {
		return l, nil
	}

	udp := newUDPTransport(key.mode, key.vrf, device)
	loop := engine.NewLoop(udp, clock.RealClock{})
	if startErr := loop.Start(); startErr != nil {
		if stopErr := udp.Stop(); stopErr != nil {
			logger().Debug("bfd udp stop after failed start", "err", stopErr)
		}
		return nil, fmt.Errorf("bfd: start engine loop for %s (vrf=%s device=%s): %w", key.mode, key.vrf, device, startErr)
	}
	r.loops[key] = loop
	logger().Info("bfd loop started",
		"vrf", key.vrf,
		"mode", key.mode.String(),
		"device", device)
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

	// Compute a per-loop device name from all wanted sessions sharing
	// the (vrf, mode) pair. If every single-hop pinned session in the
	// same VRF names the same interface, the loop binds to it;
	// otherwise Device stays empty and the engine-side TTL gate is the
	// only protection.
	deviceForLoop := resolveLoopDevices(wanted)

	// Create or refresh sessions present in the new config.
	for key, s := range wanted {
		lk := loopKey{vrf: key.VRF, mode: key.Mode}
		loop, err := r.loopFor(lk, deviceForLoop[lk])
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

// resolveLoopDevices computes the SO_BINDTODEVICE name for each loop key
// based on the pinned sessions mapped to that key.
//
//   - Non-default VRF: the device is the VRF name (Linux binds the socket
//     to the VRF master device). On Linux SO_BINDTODEVICE can only name
//     one device, so any session `interface` leaf under a non-default
//     VRF is dropped and the override is logged once per affected loop.
//   - Default VRF single-hop with one or more pinned sessions all naming
//     the same interface: the device is that interface name.
//   - Default VRF single-hop with a mix of interfaces (or no interface
//     specified): empty device; the engine TTL gate still enforces GTSM
//     but no kernel-level pinning applies. The full set of conflicting
//     interfaces is logged once per loop.
//   - Default VRF multi-hop: empty device (multi-hop does not bind to a
//     specific interface).
//
// The function iterates a Go map and therefore observes sessions in
// nondeterministic order. To keep log output deterministic and readable,
// all state-building runs first, then warnings are emitted once per loop
// with a sorted list of the conflicting interfaces.
func resolveLoopDevices(wanted map[api.Key]sessionConfig) map[loopKey]string {
	type loopState struct {
		ifaces          map[string]struct{} // distinct single-hop interface names in default VRF
		sawEmptyIface   bool                // at least one session omitted its interface
		overriddenByVRF []string            // interface leaves dropped because non-default VRF wins
		vrfBind         string              // non-empty when VRF overrides
	}
	states := make(map[loopKey]*loopState)
	getState := func(lk loopKey) *loopState {
		st, ok := states[lk]
		if !ok {
			st = &loopState{ifaces: make(map[string]struct{})}
			states[lk] = st
		}
		return st
	}

	for key, s := range wanted {
		lk := loopKey{vrf: key.VRF, mode: key.Mode}
		st := getState(lk)
		if key.VRF != defaultVRFName {
			st.vrfBind = key.VRF
			if s.iface != "" {
				st.overriddenByVRF = append(st.overriddenByVRF, s.iface)
			}
			continue
		}
		if s.mode != api.SingleHop {
			continue
		}
		if s.iface == "" {
			st.sawEmptyIface = true
			continue
		}
		st.ifaces[s.iface] = struct{}{}
	}

	out := make(map[loopKey]string, len(states))
	for lk, st := range states {
		if st.vrfBind != "" {
			out[lk] = st.vrfBind
			if len(st.overriddenByVRF) > 0 {
				sort.Strings(st.overriddenByVRF)
				logger().Info("bfd non-default VRF binds to VRF device; session interface leaves ignored",
					"vrf", lk.vrf,
					"mode", lk.mode.String(),
					"device", st.vrfBind,
					"dropped-interfaces", strings.Join(dedupSorted(st.overriddenByVRF), ","))
			}
			continue
		}
		// Default VRF single-hop: bind only when every session agrees
		// on one interface name AND no session omitted it.
		if st.sawEmptyIface || len(st.ifaces) != 1 {
			if len(st.ifaces) > 1 || (len(st.ifaces) >= 1 && st.sawEmptyIface) {
				logger().Warn("bfd single-hop loop interface mismatch, skipping SO_BINDTODEVICE",
					"vrf", lk.vrf,
					"interfaces", strings.Join(sortedKeys(st.ifaces), ","),
					"sessions-without-interface", st.sawEmptyIface)
			}
			out[lk] = ""
			continue
		}
		for only := range st.ifaces {
			out[lk] = only
		}
	}
	return out
}

// sortedKeys returns the keys of a set as a sorted slice so log output
// is deterministic across Go map iteration orders.
func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedupSorted removes consecutive duplicates from an already-sorted
// string slice. Used to collapse repeated interface names in the
// "dropped under VRF" log line.
func dedupSorted(s []string) []string {
	if len(s) <= 1 {
		return s
	}
	out := s[:1]
	for _, v := range s[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// newUDPTransport allocates a transport.UDP for the given hop mode on
// the canonical RFC port. Stage 2 passes through the VRF label and the
// Linux network device for SO_BINDTODEVICE. IPv6 dual-bind is explicitly
// deferred (see plan/deferrals.md spec-bfd-2b-ipv6-transport).
//
// This function does not start the transport: the caller passes the
// returned UDP to engine.NewLoop, then calls Loop.Start which is
// responsible for binding the socket. Bind failures surface there.
func newUDPTransport(mode api.HopMode, vrf, device string) *transport.UDP {
	port := transport.UDPPortSingleHopControl
	if mode == api.MultiHop {
		port = transport.UDPPortMultiHopControl
	}
	bind := netip.AddrPortFrom(netip.IPv4Unspecified(), port)
	return &transport.UDP{
		Bind:   bind,
		Mode:   mode,
		VRF:    vrf,
		Device: device,
	}
}

// runtimeStateGuard serializes access to the package-level runtime when
// the SDK delivers verify/configure/apply on different goroutines. The
// SDK currently calls them sequentially but we keep the guard so a
// future change cannot silently introduce a race.
//
// Stage 3 (BGP opt-in) also acquires this mutex from pluginService so a
// BGP peer's EnsureSession cannot race a config reload that is creating
// or tearing down the same loop.
var runtimeStateGuard sync.Mutex

// pluginService is the in-process implementation of api.Service that the
// bfd plugin publishes via api.SetService. BGP (and any future client)
// calls api.GetService().EnsureSession to obtain a SessionHandle; the
// dispatch picks the (vrf, mode) loop on runtimeState, lazily creating
// it if no pinned session had already triggered loopFor.
//
// Concurrency: every call takes runtimeStateGuard so that EnsureSession
// / ReleaseSession do not race a config reload. The runtimeState itself
// is not safe for concurrent use -- the lock is the contract.
//
// Device selection for a BGP-driven loop mirrors resolveLoopDevices's
// per-request rules without the multi-session conflict detection:
// non-default VRF wins (socket binds to VRF device), otherwise a
// single-hop session's Interface leaf is used, otherwise the loop runs
// without SO_BINDTODEVICE and the engine TTL gate is the only
// protection. The FIRST caller to create a loop locks in the device --
// later callers share the socket regardless of their own Interface
// because one UDP socket can only bind to one device.
type pluginService struct {
	state *runtimeState
}

// EnsureSession dispatches the request to the correct engine.Loop,
// creating the loop on demand. The returned SessionHandle is owned by
// the caller; callers MUST call ReleaseSession when finished.
func (s *pluginService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	runtimeStateGuard.Lock()
	defer runtimeStateGuard.Unlock()
	vrf := req.VRF
	if vrf == "" {
		vrf = defaultVRFName
	}
	device := ""
	if vrf != defaultVRFName {
		device = vrf
	} else if req.Mode == api.SingleHop {
		device = req.Interface
	}
	lk := loopKey{vrf: vrf, mode: req.Mode}
	loop, err := s.state.loopFor(lk, device)
	if err != nil {
		return nil, err
	}
	normalized := req
	normalized.VRF = vrf
	return loop.EnsureSession(normalized)
}

// ReleaseSession hands the handle back to the owning loop. If the loop
// has already been torn down (config reload cleared it) the call is a
// no-op: the handle was already invalidated by Loop.Stop closing every
// subscriber channel.
func (s *pluginService) ReleaseSession(h api.SessionHandle) error {
	if h == nil {
		return nil
	}
	runtimeStateGuard.Lock()
	defer runtimeStateGuard.Unlock()
	key := h.Key()
	vrf := key.VRF
	if vrf == "" {
		vrf = defaultVRFName
	}
	loop, ok := s.state.loops[loopKey{vrf: vrf, mode: key.Mode}]
	if !ok {
		return nil
	}
	return loop.ReleaseSession(h)
}

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
		// Clear the in-process Service publication FIRST so new
		// clients see nil and skip BFD wiring instead of receiving a
		// handle whose underlying loop is about to tear down.
		api.SetService(nil)
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
		// Publish the in-process Service so BGP (and any future
		// client) can reach the engine via api.GetService() without
		// importing internal/plugins/bfd/engine. Published last,
		// after all configured loops have started, so an early BGP
		// EnsureSession does not race the loop creation.
		api.SetService(&pluginService{state: state})
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
