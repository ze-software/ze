// Design: docs/architecture/api/process-protocol.md — Stage 2 config delivery
// Related: sdk_dispatch.go — serveOne, handleConfigure
// Related: internal/component/plugin/server/startup.go — isConfigRefusal, the engine that reads it

package sdk

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestConfigureHandlerErrorReachesEngineAsAResponse is the plugin half of the
// refusal, and the engine half is worthless without it.
//
// VALIDATES: an OnConfigure handler that returns an error puts an error
// RESPONSE on the wire, which the engine end parses into an *rpc.RPCCallError
// carrying the handler's own message.
// PREVENTS: a refusal arriving at the engine in a shape no transport failure
// can be told apart from. The engine decides whether a Stage 2 failure stops
// ze by the type of what comes back (isConfigRefusal,
// internal/component/plugin/server/startup.go), and only parseRPCError builds
// that type, only from a response line. A serveOne that returned the handler
// error up the stack instead of answering with it would drop the connection,
// and the engine would then see a transport failure and start ze on a
// configuration this plugin rejected.
func TestConfigureHandlerErrorReachesEngineAsAResponse(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer pluginEnd.Close() //nolint:errcheck // test cleanup
	defer engineEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const reason = `address family "ipv6-unicat" is not a family`

	p := NewWithConn("test-refusing-plugin", pluginEnd)
	p.OnConfigure(func([]ConfigSection) error {
		return errors.New(reason)
	})

	go func() {
		_ = p.Run(ctx, Registration{}) //nolint:errcheck // the refusal ends the run
	}()

	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	defer engineMux.Close() //nolint:errcheck // test cleanup

	// Stage 1: the plugin declares itself.
	req := readMuxRequest(t, ctx, engineMux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))

	// Stage 2: the engine delivers the configuration and the plugin refuses it.
	_, err := engineMux.CallRPC(ctx, "ze-plugin-callback:configure", struct {
		Sections []ConfigSection `json:"sections"`
	}{})

	require.Error(t, err, "a refusing handler must fail the configure call")
	var callErr *rpc.RPCCallError
	require.ErrorAs(t, err, &callErr,
		"the refusal must arrive as a response the plugin wrote, not as a broken transport")
	assert.Contains(t, callErr.Message, reason,
		"the engine tells the operator what the plugin refused")
}
