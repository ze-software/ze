// VALIDATES: the registered bgp-nlri-srpolicy engine answers the plugin
// server's Stage 1 and declares both SR Policy families.
// PREVENTS: the daemon auto-loading this plugin for a configured
// `ipv4 sr-policy` family (startup_autoload.go, getUnclaimedFamilyPlugins) and
// failing its startup at Registration with "stage 1 read: mux conn closed",
// because the engine returned before it spoke the protocol.

package srpolicy

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEngineDeclaresFamiliesAtStage1 drives the registered RunEngine over a
// pipe, playing the engine side, and reads the Stage 1 registration it sends.
// The stub it replaces returned 0 at once, so the pipe closed before any
// request arrived and this test fails waiting for it.
func TestEngineDeclaresFamiliesAtStage1(t *testing.T) {
	reg := registry.Lookup("bgp-nlri-srpolicy")
	require.NotNil(t, reg, "bgp-nlri-srpolicy must be registered")
	require.NotNil(t, reg.RunEngine)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan struct{})
	go func() {
		reg.RunEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("srpolicy engine did not exit after its transport closed")
		}
	})

	var req *rpc.Request
	select {
	case req = <-mux.Requests():
	case <-done:
		t.Fatal("srpolicy engine returned before sending its Stage 1 registration")
	case <-ctx.Done():
		t.Fatalf("waiting for Stage 1 registration: %v", ctx.Err())
	}
	require.NotNil(t, req, "srpolicy engine closed its transport before Stage 1")
	require.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var input rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	require.ElementsMatch(t, []rpc.FamilyDecl{
		{Name: IPv4SRPolicy.String(), Mode: "decode", AFI: 1, SAFI: 73},
		{Name: IPv6SRPolicy.String(), Mode: "decode", AFI: 2, SAFI: 73},
	}, input.Families)
	require.NoError(t, mux.SendOK(ctx, req.ID))
}
