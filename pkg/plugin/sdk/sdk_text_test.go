package sdk

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// textEngineSide wraps two TextConns for the engine's perspective in text mode:
//   - serverA: reads stages 1,3,5 from Socket A, sends "ok" responses
//   - clientB: sends stages 2,4 on Socket B, reads "ok" responses
type textEngineSide struct {
	serverA *rpc.TextConn
	clientB *rpc.TextConn
}

// newTextTestPair creates a connected text-mode plugin SDK + engine side using net.Pipe.
func newTextTestPair(t *testing.T) (*Plugin, *textEngineSide) {
	t.Helper()

	// Socket A: Plugin → Engine
	aPlugin, aEngine := net.Pipe()
	// Socket B: Engine → Plugin
	bPlugin, bEngine := net.Pipe()

	t.Cleanup(func() {
		for _, c := range []net.Conn{aPlugin, aEngine, bPlugin, bEngine} {
			if err := c.Close(); err != nil {
				t.Logf("cleanup close: %v", err)
			}
		}
	})

	p := NewTextPlugin("test-text-plugin", aPlugin, bPlugin)

	engine := &textEngineSide{
		serverA: rpc.NewTextConn(aEngine, aEngine),
		clientB: rpc.NewTextConn(bEngine, bEngine),
	}

	return p, engine
}

// completeTextStartup runs the engine side of the 5-stage text handshake.
func completeTextStartup(t *testing.T, ctx context.Context, engine *textEngineSide) {
	t.Helper()

	// Stage 1: read registration from Socket A, respond "ok".
	regText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	_, err = rpc.ParseRegistrationText(regText)
	require.NoError(t, err)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))

	// Stage 2: send configure on Socket B, read "ok".
	configText, err := rpc.FormatConfigureText(rpc.ConfigureInput{})
	require.NoError(t, err)
	require.NoError(t, engine.clientB.WriteMessage(ctx, configText))
	resp, err := engine.clientB.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	// Stage 3: read capabilities from Socket A, respond "ok".
	capsText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	_, err = rpc.ParseCapabilitiesText(capsText)
	require.NoError(t, err)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))

	// Stage 4: send registry on Socket B, read "ok".
	registryText, err := rpc.FormatRegistryText(rpc.ShareRegistryInput{})
	require.NoError(t, err)
	require.NoError(t, engine.clientB.WriteMessage(ctx, registryText))
	resp, err = engine.clientB.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	// Stage 5: read ready from Socket A, respond "ok".
	readyText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	_, err = rpc.ParseReadyText(readyText)
	require.NoError(t, err)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))
}

// TestTextSDKStartup verifies the full 5-stage text startup protocol via SDK.
//
// VALIDATES: AC-7 (SDK text startup path), spec step 16.
// PREVENTS: Text mode declared but not wired into SDK Run().
func TestTextSDKStartup(t *testing.T) {
	t.Parallel()

	p, engine := newTextTestPair(t)

	reg := Registration{
		Families: []FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
		},
		Commands: []CommandDecl{
			{Name: "test-cmd", Description: "A test command"},
		},
	}

	configReceived := make(chan []ConfigSection, 1)
	registryReceived := make(chan []RegistryCommand, 1)

	p.OnConfigure(func(sections []ConfigSection) error {
		configReceived <- sections
		return nil
	})
	p.OnShareRegistry(func(commands []RegistryCommand) {
		registryReceived <- commands
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// === Stage 1: Engine reads text registration ===
	regText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	regInput, err := rpc.ParseRegistrationText(regText)
	require.NoError(t, err)
	assert.Equal(t, 1, len(regInput.Families))
	assert.Equal(t, "ipv4/unicast", regInput.Families[0].Name)
	assert.Equal(t, 1, len(regInput.Commands))
	assert.Equal(t, "test-cmd", regInput.Commands[0].Name)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))

	// === Stage 2: Engine sends text configure ===
	sections := []rpc.ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
	}
	configText, err := rpc.FormatConfigureText(rpc.ConfigureInput{Sections: sections})
	require.NoError(t, err)
	require.NoError(t, engine.clientB.WriteMessage(ctx, configText))
	resp, err := engine.clientB.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	select {
	case got := <-configReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "bgp", got[0].Root)
	case <-time.After(time.Second):
		t.Fatal("configure callback not called")
	}

	// === Stage 3: Engine reads text capabilities ===
	capsText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	_, err = rpc.ParseCapabilitiesText(capsText)
	require.NoError(t, err)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))

	// === Stage 4: Engine sends text registry ===
	commands := []rpc.RegistryCommand{
		{Name: "test-cmd", Plugin: "test-text-plugin", Encoding: "text"},
	}
	registryText, err := rpc.FormatRegistryText(rpc.ShareRegistryInput{Commands: commands})
	require.NoError(t, err)
	require.NoError(t, engine.clientB.WriteMessage(ctx, registryText))
	resp, err = engine.clientB.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	select {
	case got := <-registryReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "test-cmd", got[0].Name)
	case <-time.After(time.Second):
		t.Fatal("share-registry callback not called")
	}

	// === Stage 5: Engine reads text ready ===
	readyText, err := engine.serverA.ReadMessage(ctx)
	require.NoError(t, err)
	_, err = rpc.ParseReadyText(readyText)
	require.NoError(t, err)
	require.NoError(t, engine.serverA.WriteLine(ctx, "ok"))

	// === Shutdown: Engine sends bye on Socket B ===
	require.NoError(t, engine.clientB.WriteLine(ctx, "bye test-complete"))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestTextSDKEventDelivery verifies text event delivery via the text event loop.
//
// VALIDATES: Text-mode event delivery on Socket B.
// PREVENTS: Events lost when using text protocol.
func TestTextSDKEventDelivery(t *testing.T) {
	t.Parallel()

	p, engine := newTextTestPair(t)

	eventReceived := make(chan string, 1)
	p.OnEvent(func(event string) error {
		eventReceived <- event
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeTextStartup(t, ctx, engine)

	// Send a text event on Socket B.
	require.NoError(t, engine.clientB.WriteLine(ctx, "update peer 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24"))

	select {
	case got := <-eventReceived:
		assert.Equal(t, "update peer 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24", got)
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}

	// Shutdown.
	require.NoError(t, engine.clientB.WriteLine(ctx, "bye"))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestTextSDKByeWithReason verifies bye with reason in text mode.
//
// VALIDATES: bye reason is passed to OnBye handler.
// PREVENTS: Reason string lost in text mode.
func TestTextSDKByeWithReason(t *testing.T) {
	t.Parallel()

	p, engine := newTextTestPair(t)

	byeReason := make(chan string, 1)
	p.OnBye(func(reason string) {
		byeReason <- reason
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeTextStartup(t, ctx, engine)

	require.NoError(t, engine.clientB.WriteLine(ctx, "bye shutdown-requested"))

	select {
	case reason := <-byeReason:
		assert.Equal(t, "shutdown-requested", reason)
	case <-time.After(time.Second):
		t.Fatal("bye callback not called")
	}

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}
