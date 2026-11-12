// Design: docs/architecture/api/process-protocol.md — ad-hoc plugin sessions
// Overview: server.go — Server struct and lifecycle
// Related: startup.go — handleProcessStartupRPC used for 5-stage handshake

package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// HandleAdHocPluginSession runs the 5-stage plugin handshake and runtime
// command loop over an arbitrary bidirectional stream (e.g., an SSH channel).
// The session uses coordinator == nil, so all stage barriers are skipped.
// Blocks until the connection closes or the server shuts down.
// Caller MUST close reader and writer after this returns.
func (s *Server) HandleAdHocPluginSession(reader io.ReadCloser, writer io.WriteCloser) error {
	name, err := generateAdHocName()
	if err != nil {
		return fmt.Errorf("generate session name: %w", err)
	}

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     name,
		Internal: true, // No external process to manage.
	})
	// An ad-hoc session is never in the process manager, so nothing else
	// clears what it raised: every return ends the session.
	defer s.endAdHocSession(name)

	// Build PluginConn from the raw reader/writer — same stack as InitConns
	// but without requiring net.Conn (SSH channels are not net.Conn).
	rpcConn := rpc.NewConn(reader, writer)
	// SSH channels do not implement SetWriteDeadline, so writes here are
	// governed by the rpc write watchdog; label it for watchdog logs/metrics.
	rpcConn.SetLabel(name)
	mux := rpc.NewMuxConn(rpcConn)
	proc.SetConn(pluginipc.NewMuxPluginConn(mux))
	proc.SetRunning(true)

	// Run the 5-stage handshake outside any tier barrier: an ad-hoc session is
	// not part of tier-ordered startup, so every stageTransition returns true
	// immediately (runUnbarrieredStartupHandshake, restart.go).
	s.runUnbarrieredStartupHandshake(s.ctx, proc)

	if proc.Stage() < plugin.StageRunning {
		return fmt.Errorf("handshake incomplete: reached stage %d", proc.Stage())
	}

	// Enter runtime command loop (blocks until disconnect).
	s.handleSingleProcessCommandsRPC(proc)
	return nil
}

// endAdHocSession forgets an ad-hoc session that ended. Its active warnings
// leave the report bus: a managed plugin's go when the process manager
// removes it or records its crash (report.ClearSource), and an ad-hoc
// session's name is random and never comes back, so they would outlive the
// session and fill the bus as sessions come and go. Its report-issue rate
// window goes too (forgetPluginReports), on every return, a failed handshake
// included.
func (s *Server) endAdHocSession(name string) {
	report.ClearSource(name)
	s.forgetPluginReports(name)
}

// generateAdHocName creates a unique name for an ad-hoc plugin session.
// Uses "cli-debug-" prefix with 8 random hex chars to avoid collisions
// with real plugin names.
func generateAdHocName() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "cli-debug-" + hex.EncodeToString(b[:]), nil
}
