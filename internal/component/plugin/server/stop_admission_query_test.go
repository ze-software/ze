// Design: docs/architecture/api/commands.md -- runtime owners share stop admission.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServerAdmissionClosesBeforeStopFinishes holds an actual stop hook open
// and proves a cached runtime owner cannot admit work during that drain.
func TestServerAdmissionClosesBeforeStopFinishes(t *testing.T) {
	server, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	require.NoError(t, server.CheckAdmission())
	entered := make(chan struct{})
	release := make(chan struct{})
	stopped := make(chan struct{})
	_, err = server.OnStop(func() error { close(entered); <-release; return nil })
	require.NoError(t, err)
	go func() { server.Stop(); close(stopped) }()
	<-entered
	refusal := server.CheckAdmission()
	close(release)
	<-stopped
	require.ErrorIs(t, refusal, errServerStopped)
	require.ErrorIs(t, server.CheckAdmission(), errServerStopped)
	var absent *Server
	require.ErrorIs(t, absent.CheckAdmission(), errServerStopped)
}
