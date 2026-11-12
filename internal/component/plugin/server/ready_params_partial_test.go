package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// readyPartialShape is the payload type of the event the ready-params tests
// subscribe to.
type readyPartialShape struct{ Name string }

// readyPartialNamespace is the namespace of that event, registered by these
// tests alone so no other package's event types decide what subscribes.
const readyPartialNamespace = "test-ns-ready-partial"

// runReadyHandshake runs the engine's own startup handshake for a plugin named
// name whose ready parameters are params, and answers its process and the
// server that ran it, as TestReadyParamsRefusedRecordsDrop does.
func runReadyHandshake(t *testing.T, name string, params json.RawMessage) (*Server, *process.Process) {
	t.Helper()
	engineConn, pluginMux := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	proc.SetConn(engineConn)
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.coordinator = plugin.NewStartupCoordinator(1)

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		drivePluginHandshakeWith(ctx, pluginMux,
			&rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion}, params)
	}()
	s.handleProcessStartupRPC(proc)
	<-pluginDone
	return s, proc
}

// TestReadyParamsPartlyParsedSubscribeNothing sends a ready object whose
// subscribe block is valid and whose transport field has the wrong type.
//
// VALIDATES: runStartupHandshake publishes the ready input only when it
// decodes, so a refused one leaves the plugin subscribed to nothing, as the
// drop it records says (startup_driver.go, the Stage 5 decode).
// PREVENTS: encoding/json, which goes on past a field of the wrong type,
// registering the subscriptions that did decode while `ze doctor` reports
// them dropped because the ready parameters did not parse.
// METHOD: handleProcessStartupRPC over net.Pipe, as
// TestReadyParamsRefusedRecordsDrop. The control run sends the same subscribe
// block with no bad field and must register it, which proves the probe can
// see a subscription. Red: the driver decoding straight into the value it
// passes to onReady.
func TestReadyParamsPartlyParsedSubscribeNothing(t *testing.T) {
	events.Register[*readyPartialShape](readyPartialNamespace, "partial-up")
	subscribe := `"subscribe":{"namespace":"` + readyPartialNamespace + `","events":["partial-up"]}`

	t.Run("control", func(t *testing.T) {
		s, proc := runReadyHandshake(t, "ready-whole", json.RawMessage(`{`+subscribe+`}`))
		require.Equal(t, plugin.StageRunning, proc.Stage())
		require.NotZero(t, s.subscriptions.Count(proc), "a valid ready subscribe block registered nothing")
		require.Empty(t, proc.DoctorRecord(time.Now()).SubscriptionDrops)
	})

	t.Run("wrong-type field", func(t *testing.T) {
		s, proc := runReadyHandshake(t, "ready-partial", json.RawMessage(`{`+subscribe+`,"transport":1}`))
		require.Equal(t, plugin.StageRunning, proc.Stage(), "unparsable ready parameters stopped the startup")
		require.Zero(t, s.subscriptions.Count(proc),
			"a refused ready object still registered the subscriptions that decoded")
		drops := proc.DoctorRecord(time.Now()).SubscriptionDrops
		require.Len(t, drops, 1, "the dropped startup subscriptions were not recorded")
		require.Equal(t, process.SubscriptionDropReadyParamsUnparsable, drops[0].Cause)
	})
}
