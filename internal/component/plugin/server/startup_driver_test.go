package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// recordingSink is a startupSink stand-in that records the order in which the
// shared driver invokes its hooks. It delivers nil payloads and runs without a
// barrier (Transition always succeeds), like the hub sink, so a real net.Pipe
// plugin can complete the handshake against it.
type recordingSink struct {
	pc          *ipc.PluginConn
	order       []string
	transitions [][2]plugin.PluginStage
	regErr      error // when non-nil, onRegistration returns it
}

func (r *recordingSink) conn() *ipc.PluginConn {
	r.order = append(r.order, "Conn")
	return r.pc
}

func (r *recordingSink) onRegistration(*rpc.DeclareRegistrationInput) error {
	r.order = append(r.order, "OnRegistration")
	return r.regErr
}

func (r *recordingSink) deliverConfig(ctx context.Context) error {
	r.order = append(r.order, "DeliverConfig")
	return r.pc.SendConfigure(ctx, nil)
}

func (r *recordingSink) onCapabilities(*rpc.DeclareCapabilitiesInput) error {
	r.order = append(r.order, "OnCapabilities")
	return nil
}

func (r *recordingSink) deliverRegistry(ctx context.Context) error {
	r.order = append(r.order, "DeliverRegistry")
	return r.pc.SendShareRegistry(ctx, nil)
}

func (r *recordingSink) onReady(*rpc.ReadyInput) error {
	r.order = append(r.order, "OnReady")
	return nil
}

func (r *recordingSink) onRunning()                { r.order = append(r.order, "OnRunning") }
func (r *recordingSink) postReady(*rpc.ReadyInput) { r.order = append(r.order, "PostReady") }

func (r *recordingSink) transition(from, to plugin.PluginStage) bool {
	r.order = append(r.order, "Transition")
	r.transitions = append(r.transitions, [2]plugin.PluginStage{from, to})
	return true
}

// newDriverPipe wires an engine-side PluginConn to a plugin-side MuxConn over an
// in-memory net.Pipe, returning both. Cleanup closes everything.
func newDriverPipe(t *testing.T) (*ipc.PluginConn, *rpc.MuxConn) {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		pluginMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	return ipc.NewMuxPluginConn(engineMux), pluginMux
}

// drivePluginHandshake plays the plugin side of the 5-stage handshake: it sends
// the three plugin-initiated requests and answers the two engine-initiated
// callbacks. Runs in its own goroutine.
func drivePluginHandshake(ctx context.Context, pluginMux *rpc.MuxConn) {
	if _, err := pluginMux.CallRPC(ctx, methodDeclareRegistration, &rpc.DeclareRegistrationInput{
		Commands: []rpc.CommandDecl{{Name: "widget show"}},
	}); err != nil {
		return
	}
	// Stage 2: receive configure, respond OK.
	select {
	case req := <-pluginMux.Requests():
		if err := pluginMux.SendOK(ctx, req.ID); err != nil {
			return
		}
	case <-ctx.Done():
		return
	}
	if _, err := pluginMux.CallRPC(ctx, methodDeclareCapabilities, &rpc.DeclareCapabilitiesInput{}); err != nil {
		return
	}
	// Stage 4: receive share-registry, respond OK.
	select {
	case req := <-pluginMux.Requests():
		if err := pluginMux.SendOK(ctx, req.ID); err != nil {
			return
		}
	case <-ctx.Done():
		return
	}
	if _, err := pluginMux.CallRPC(ctx, methodReady, nil); err != nil {
		return
	}
}

// TestSharedStartupDriverSinkDispatch verifies the shared driver invokes each
// sink hook in the correct stage order, with the barrier transitions
// interleaved exactly between the read/respond and callback-delivery steps.
//
// VALIDATES: AC-7 -- one shared driver drives the 5-stage choreography; the
// caller effects are dispatched through the injected sink in stage order.
// PREVENTS: A future edit reordering a stage, dropping a hook, or moving a
// barrier relative to a callback delivery.
func TestSharedStartupDriverSinkDispatch(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		drivePluginHandshake(ctx, pluginMux)
	}()

	sink := &recordingSink{pc: engineConn}
	require.NoError(t, runStartupHandshake(ctx, sink))

	assert.Equal(t, []string{
		"Conn",
		"OnRegistration",
		"Transition",
		"DeliverConfig",
		"Transition",
		"OnCapabilities",
		"Transition",
		"DeliverRegistry",
		"Transition",
		"OnReady",
		"Transition",
		"OnRunning",
		"PostReady",
	}, sink.order)

	assert.Equal(t, [][2]plugin.PluginStage{
		{plugin.StageRegistration, plugin.StageConfig},
		{plugin.StageConfig, plugin.StageCapability},
		{plugin.StageCapability, plugin.StageRegistry},
		{plugin.StageRegistry, plugin.StageReady},
		{plugin.StageReady, plugin.StageRunning},
	}, sink.transitions)

	<-done
}

// TestSharedStartupDriverMethodMismatch verifies the driver rejects a wrong
// first method with the exact error string both callers relied on, and never
// advances past Stage 1.
//
// VALIDATES: AC-6 -- shared driver returns the same "expected ... got ..."
// message for a wrong method at a stage.
// PREVENTS: Silent acceptance of an out-of-sequence plugin request.
func TestSharedStartupDriverMethodMismatch(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		// Plugin sends the wrong method for Stage 1; expects an error response.
		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:not-registration", &rpc.DeclareRegistrationInput{}); err != nil {
			return
		}
	}()

	sink := &recordingSink{pc: engineConn}
	err := runStartupHandshake(ctx, sink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected declare-registration, got ze-plugin-engine:not-registration")
	// Only Conn was called; no stage hook ran.
	assert.Equal(t, []string{"Conn"}, sink.order)
}

// TestSharedStartupDriverRegistrationErrorAborts verifies that a sink stage
// error aborts the handshake: the driver relays the sink's message to the
// plugin via SendError and runs no later hook.
//
// VALIDATES: AC-6 -- a sink stage error is delivered to the plugin verbatim and
// stops the handshake.
// PREVENTS: A rejected registration silently proceeding to config/capability.
func TestSharedStartupDriverRegistrationErrorAborts(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callErr := make(chan error, 1)
	go func() {
		_, err := pluginMux.CallRPC(ctx, methodDeclareRegistration, &rpc.DeclareRegistrationInput{})
		callErr <- err
	}()

	boom := errors.New("registration conflict: taken")
	sink := &recordingSink{pc: engineConn, regErr: boom}
	err := runStartupHandshake(ctx, sink)
	require.ErrorIs(t, err, boom)

	// Plugin received the sink's message as an error response.
	select {
	case perr := <-callErr:
		require.Error(t, perr)
		assert.Contains(t, perr.Error(), "registration conflict: taken")
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not receive error response")
	}

	// Conn + OnRegistration ran; nothing after.
	assert.Equal(t, []string{"Conn", "OnRegistration"}, sink.order)
}
