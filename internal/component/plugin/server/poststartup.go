// Design: docs/architecture/api/process-protocol.md — the post-startup callback
// Overview: startup.go — signalStartupComplete, the once-per-daemon fan-out
// Related: startup_claims.go — the DECLARATIVE channel this callback must not replace
// Related: startup_autoload.go — autoLoadForNewConfigPaths, the mid-life fan-out

package server

import (
	"context"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// postStartupTimeout bounds the wait for a plugin to process the post-startup
// callback. Plugins with expensive OnAllPluginsReady handlers (e.g., those
// that issue a DispatchCommand to another plugin) must complete within this
// window or the callback is abandoned; this prevents one slow handler from
// blocking engine bookkeeping.
const postStartupTimeout = 10 * time.Second

// sendPostStartupToAll delivers the post-startup callback to every running
// plugin. Each delivery runs in its own goroutine with a bounded timeout, so
// one slow or broken plugin cannot delay notification to the rest. Errors are
// logged at Debug level because they are expected during shutdown races
// (connection closed before callback arrives).
//
// It deliberately does NOT wait. Waiting was tried (2026-07-25) to make
// OnAllPluginsReady handlers ordered before peer startup, so that a handler
// configuring how peer-up is processed could not lose a race against session
// establishment. It DEADLOCKS: this function is called immediately before
// SignalPluginStartupComplete -> StartPeers, and a handler that waits on peer
// activity (a test observer waiting for routes to reach Adj-RIB-In, for
// instance) then blocks the very peers it is waiting for, until the
// postStartupTimeout fires. Three functional tests failed that way.
//
// So the ordering between a post-startup handler and peer startup is NOT
// guaranteed, and anything that needs to be in place before the first peer-up
// must not rely on this callback. That state is DECLARED instead: a plugin puts
// it in its registration (registry.Registration.Claims) and the engine delivers
// the resolved set on the Stage-2 configure callback, which is part of the
// sequential handshake and therefore completes before peers start. See
// startup_claims.go. Do not move such a decision back onto this fan-out.
func (s *Server) sendPostStartupToAll() {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	for _, proc := range pm.AllProcesses() {
		s.sendPostStartupTo(proc)
	}
}

// sendPostStartupToNames delivers the post-startup callback to the named
// plugins and to nobody else.
//
// It exists for the plugins a config reload starts MID-LIFE
// (autoLoadForNewConfigPaths). Those plugins reach no other post-startup
// delivery: sendPostStartupToAll runs once, from signalStartupComplete, and a
// later phase does not call it. Their OnAllPluginsReady handlers therefore never
// ran, and the handler is where a plugin that takes an exclusive role over from
// another one tells the plugin that already holds it (bgp-rs
// claimReplayOwnership, bgp/plugins/rs/server_handlers.go). The declarative
// Stage-2 channel cannot carry that direction: Stage 2 runs per handshake, so a
// plugin configured before the claimant joined is never re-told.
//
// The names, not every running process, because a second delivery re-runs the
// OnAllPluginsReady handler of a plugin that already ran it. Those handlers are
// written for a single call.
//
// The deadlock sendPostStartupToAll records does not reach here: peers are
// already running when a reload lands, so a handler that waits on peer activity
// waits on peers nothing is holding back.
func (s *Server) sendPostStartupToNames(names []string) {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	for _, name := range names {
		s.sendPostStartupTo(pm.GetProcess(name))
	}
}

// sendPostStartupTo delivers the callback to one plugin, on its own goroutine
// with a bounded timeout. A process that is nil, stopped, or without a
// connection is skipped: there is nothing to deliver to.
func (s *Server) sendPostStartupTo(proc *process.Process) {
	if proc == nil || !proc.Running() {
		return
	}
	conn := proc.Conn()
	if conn == nil {
		return
	}
	name := proc.Name()
	go func(c *plugipc.PluginConn, pluginName string) {
		ctx, cancel := context.WithTimeout(s.ctx, postStartupTimeout)
		defer cancel()
		if err := c.SendPostStartup(ctx); err != nil {
			logger().Debug("post-startup callback failed", "plugin", pluginName, "error", err)
		}
	}(conn, name)
}
