// Design: docs/architecture/api/process-protocol.md — the post-startup callback
// Overview: startup.go — signalStartupComplete, the once-per-daemon fan-out
// Related: startup_claims.go — the DECLARATIVE channel this callback must not replace
// Related: startup_autoload.go — autoLoadForNewConfigPaths, the mid-life fan-out

package server

import (
	"context"
	"errors"
	"time"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// postStartupTimeout bounds the wait for a plugin to process the post-startup
// callback. Plugins with expensive OnAllPluginsReady handlers (e.g., those
// that issue a DispatchCommand to another plugin) must complete within this
// window or the callback is abandoned; this prevents one slow handler from
// blocking engine bookkeeping.
const postStartupTimeout = 10 * time.Second

// sendPostStartupToAll delivers the post-startup callback to every running
// plugin. Each delivery runs in its own goroutine with a bounded timeout, so
// one slow or broken plugin cannot delay notification to the rest. A failure
// is reported by recordPostStartupOutcome: at Warn while the daemon runs, at
// Debug only once it is stopping, where a closed connection is expected.
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
// with a bounded timeout. A process that is nil or stopped is skipped: there
// is nothing to deliver to. A running process with no connection is recorded
// undelivered, because it will never get the callback.
func (s *Server) sendPostStartupTo(proc *process.Process) {
	if proc == nil || !proc.Running() {
		return
	}
	conn := proc.Conn()
	if conn == nil {
		s.recordPostStartupOutcome(proc, errPostStartupNoConn)
		return
	}
	proc.SetPostStartup(process.PostStartupPending, nil)
	go s.deliverPostStartup(conn, proc)
}

// errPostStartupNoConn is the post-startup outcome of a running plugin with
// no connection to send the callback on.
var errPostStartupNoConn = errors.New("the plugin has no connection to send the post-startup callback on")

// deliverPostStartup sends the callback to one plugin and records the
// outcome. It is the body of the one goroutine sendPostStartupTo starts per
// plugin, bounded by postStartupTimeout.
func (s *Server) deliverPostStartup(conn *plugipc.PluginConn, proc *process.Process) {
	ctx, cancel := context.WithTimeout(s.ctx, postStartupTimeout)
	defer cancel()
	s.recordPostStartupOutcome(proc, conn.SendPostStartup(ctx))
}

// recordPostStartupOutcome records whether a plugin received its post-startup
// callback, for the doctor (AC-17), and says when it did not. While the daemon
// runs a failure is a Warn naming the plugin, the cause and where to look: a
// plugin that misses the callback never starts the work it defers to
// post-startup, and a Debug line left the operator nothing at the default
// level. Once the server context is done the daemon is stopping, a closed
// connection is the expected race, and the line stays at Debug with nothing
// recorded.
func (s *Server) recordPostStartupOutcome(proc *process.Process, err error) {
	if err == nil {
		proc.SetPostStartup(process.PostStartupDelivered, nil)
		return
	}
	if s.ctx.Err() != nil {
		logger().Debug("post-startup callback not delivered: the daemon is stopping",
			"plugin", proc.Name(), "error", err)
		return
	}
	proc.SetPostStartup(process.PostStartupUndelivered, err)
	logger().Warn("plugin did not receive its post-startup callback, so work it starts after "+
		"startup is not running; check it is running with `ze show plugin list` and read its log",
		"plugin", proc.Name(), "error", err, "code", diagnostic.CodeDoctorPluginPostStartupUndelivered)
}
