// Design: docs/architecture/api/process-protocol.md — ad-hoc plugin sessions
// Overview: server.go — Server struct and lifecycle
// Related: startup.go — handleProcessStartupRPC used for 5-stage handshake

package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

	// Build PluginConn from the raw reader/writer — same stack as InitConns
	// but without requiring net.Conn (SSH channels are not net.Conn).
	rpcConn := rpc.NewConn(reader, writer)
	mux := rpc.NewMuxConn(rpcConn)
	proc.SetConn(pluginipc.NewMuxPluginConn(mux))
	proc.SetRunning(true)

	// Run 5-stage handshake. With s.coordinator == nil (ad-hoc session,
	// not part of tier-ordered startup), all stageTransition calls return
	// true immediately — no barriers to wait on.
	savedCoordinator := s.coordinator
	s.coordinator = nil
	s.handleProcessStartupRPC(proc)
	s.coordinator = savedCoordinator

	if proc.Stage() < plugin.StageRunning {
		return fmt.Errorf("handshake incomplete: reached stage %d", proc.Stage())
	}

	// Enter runtime command loop (blocks until disconnect).
	s.handleSingleProcessCommandsRPC(proc)
	return nil
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
