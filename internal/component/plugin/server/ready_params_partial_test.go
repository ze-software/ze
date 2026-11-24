package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

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
// wire delivery fields are valid and whose transport field has the wrong type.
//
// VALIDATES: runStartupHandshake publishes the ready input only when it
// decodes, so a refused one applies none of the partially decoded wire
// delivery options (format, encoding, envelope), matching the drop it records
// (startup_driver.go, the Stage 5 decode).
// PREVENTS: encoding/json, which goes on past a field of the wrong type,
// applying format/encoding/envelope fields that did decode while `ze doctor`
// reports the ready parameters refused because they did not parse.
// METHOD: handleProcessStartupRPC over net.Pipe, as
// TestReadyParamsRefusedRecordsDrop. The control run sends the same delivery
// fields with no bad field and must apply them, which proves the probe can
// observe the ready options. Red: the driver decoding straight into the value
// it passes to onReady.
func TestReadyParamsPartlyParsedSubscribeNothing(t *testing.T) {
	deliveryOpts := `"format":"full","encoding":"text","envelope":true`

	t.Run("control", func(t *testing.T) {
		_, proc := runReadyHandshake(t, "ready-whole", json.RawMessage(`{`+deliveryOpts+`}`))
		require.Equal(t, plugin.StageRunning, proc.Stage())
		require.Equal(t, plugin.FormatFull, proc.Format(), "a valid ready payload did not apply format")
		require.Equal(t, plugin.EncodingText, proc.Encoding(), "a valid ready payload did not apply encoding")
		require.True(t, proc.Envelope(), "a valid ready payload did not apply envelope")
		require.Empty(t, proc.DoctorRecord(time.Now()).SubscriptionDrops)
	})

	t.Run("wrong-type field", func(t *testing.T) {
		_, proc := runReadyHandshake(t, "ready-partial", json.RawMessage(`{`+deliveryOpts+`,"transport":1}`))
		require.Equal(t, plugin.StageRunning, proc.Stage(), "unparsable ready parameters stopped the startup")
		require.Equal(t, plugin.FormatParsed, proc.Format(),
			"a refused ready object still applied the format that decoded")
		require.Equal(t, plugin.EncodingJSON, proc.Encoding(),
			"a refused ready object still applied the encoding that decoded")
		require.False(t, proc.Envelope(),
			"a refused ready object still applied the envelope flag that decoded")
		drops := proc.DoctorRecord(time.Now()).SubscriptionDrops
		require.Len(t, drops, 1, "the dropped startup ready parameters were not recorded")
		require.Equal(t, process.SubscriptionDropReadyParamsUnparsable, drops[0].Cause)
	})
}
