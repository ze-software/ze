// Design: rfc/short/rfc5880.md -- BFD plugin entry point
// Design: docs/research/bfd-implementation-guide.md -- ze plugin layout
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
// This file holds only the plugin runtime hook (RunBFDPlugin) and the
// package-level logger. It is intentionally tiny: the plugin is not yet
// wired into the engine startup path; that hook lands in a follow-up
// commit once the spec is ready to merge.
package bfd

import (
	"log/slog"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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

// RunBFDPlugin is the engine entry point. It is wired through
// registry.Registration.RunEngine but currently is a no-op stub: the
// plugin compiles, the codec and FSM tests run, but the runtime hook
// that opens UDP sockets and dispatches client requests is left to a
// follow-up commit so this skeleton stays merge-conflict-free for the
// other concurrent work in flight.
//
// Next session: see docs/architecture/bfd.md "Next session: start here"
// for the exact file-by-file edit sequence for Stage 1 (wiring). The
// pattern to copy is internal/plugins/sysrib/sysrib.go's lifecycle
// hooks (OnConfigVerify / OnConfigure / OnConfigApply / OnStarted).
//
// When wiring lands, RunBFDPlugin will:
//
//  1. Parse the YANG ze-bfd-conf section from sdk.OnConfigure.
//  2. Construct a transport.UDP per (VRF, hop-mode, address-family).
//  3. Construct an engine.Loop per VRF.
//  4. Expose api.Service via the plugin RPC surface so BGP/OSPF/static
//     clients can call EnsureSession.
//  5. Subscribe to iface events for interface up/down handling.
func RunBFDPlugin(_ net.Conn) int {
	logger().Info("bfd plugin stub: not yet wired into engine startup; see docs/architecture/bfd.md")
	return 0
}
