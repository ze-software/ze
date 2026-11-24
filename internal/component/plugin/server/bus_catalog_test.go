// VALIDATES: Stage 1 bus declaration checks (including separate-process agreement),
// Stage 2 BusCatalog delivery (compiled-in + external declared sends), and Stage 5
// onReady subscription activation from Stage 1 Receives.
// PREVENTS: malformed Stage 1 declarations, missing external sends in the Stage 2
// catalog, or Stage 1 Receives failing to subscribe on ready.
package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: Stage 1 rejects control chars and missing namespaces in bus
// declarations, and rejects a separate-process internal plugin whose declared
// Sends/Receives disagree with its compiled registry.Registration (spec R-5).
// PREVENTS: unvalidated wire declarations or drifted separate-process metadata.
func TestStage1BusDeclarationChecksAndAgreement(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	const compiledName = "stage1-bus-compiled-probe"
	require.NoError(t, registry.Register(registry.Registration{
		Name:        compiledName,
		Description: "test compiled plugin with bus declarations",
		Sends:       []rpc.MessageSendDecl{{Message: "bgp:update"}},
		Receives:    []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)

	// Separate-process internal plugin (Internal: true + Run: "ze plugin <compiledName>") with matching declaration passes.
	sepCfg := plugin.PluginConfig{Name: "my-sep-alias", Run: "ze plugin " + compiledName, Internal: true}
	procOK := process.NewProcess(sepCfg)
	sinkOK := &engineStartupSink{s: s, proc: procOK}
	err := sinkOK.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Sends:           []rpc.MessageSendDecl{{Message: "bgp:update"}},
		Receives:        []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
	})
	require.NoError(t, err)
	s.registry.Unregister(sepCfg.Name)
	s.unregisterDeclaredBusSends(sepCfg.Name)

	// Separate-process internal plugin with mismatched Sends is refused.
	procMismatch := process.NewProcess(sepCfg)
	sinkMismatch := &engineStartupSink{s: s, proc: procMismatch}
	err = sinkMismatch.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Sends:           []rpc.MessageSendDecl{{Message: "bgp:raw"}},
		Receives:        []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disagree with compiled registration")

	// Foreign external plugin with control character in send is refused.
	extProc := process.NewProcess(plugin.PluginConfig{Name: "ext-bad-ctrl", Run: "/usr/local/bin/ext"})
	extSink := &engineStartupSink{s: s, proc: extProc}
	err = extSink.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Sends:           []rpc.MessageSendDecl{{Message: "acme:bad\x1bmsg"}},
	})
	require.Error(t, err)

	// Foreign external plugin with missing namespace in receive is refused.
	err = extSink.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Receives:        []rpc.MessageReceiveDecl{{Message: "no-namespace"}},
	})
	require.Error(t, err)
}

// VALIDATES: Stage 1 external Sends join the bus catalog before Stage 2
// configure, and Stage 2 delivers both compiled-in messages and declared
// external sends to sdk.Plugin.BusCatalog() before OnConfigure runs.
// PREVENTS: Stage 2 BusCatalog omitting external sends or arriving after OnConfigure.
func TestStage2BusCatalogDelivery(t *testing.T) {
	const extMsg = "acme-stage2-cat:alert"

	s, _ := newLifecycleStartupServer(t)

	// Register an external sender at Stage 1 first ("Sends before receptions").
	senderProc := process.NewProcess(plugin.PluginConfig{Name: "acme-sender", Run: "/usr/local/bin/acme-sender"})
	senderSink := &engineStartupSink{s: s, proc: senderProc}
	require.NoError(t, senderSink.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Sends: []rpc.MessageSendDecl{{
			Message: extMsg,
			Tags: []rpc.MessageTagDecl{{
				Name:   "severity",
				Values: []string{"warn", "crit"},
			}},
		}},
	}))
	t.Cleanup(func() {
		s.registry.Unregister("acme-sender")
		s.unregisterDeclaredBusSends("acme-sender")
	})

	// Now start a second plugin over a real net.Pipe and verify it observes both
	// compiled-in messages and acme-stage2-cat:alert inside OnConfigure via p.BusCatalog().
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
	})

	recvProc := process.NewProcess(plugin.PluginConfig{Name: "acme-receiver", Run: "/usr/local/bin/acme-receiver"})
	recvProc.SetConn(ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	gotCatalogCh := make(chan []sdk.BusCatalogEntry, 1)
	pluginErrCh := make(chan error, 1)
	go func() {
		p := sdk.NewWithConn("acme-receiver", pluginEnd)
		defer func() { _ = p.Close() }()
		p.OnConfigure(func(_ []sdk.ConfigSection) error {
			gotCatalogCh <- p.BusCatalog()
			return nil
		})
		pluginErrCh <- p.Run(ctx, &sdk.Registration{
			Receives: []sdk.MessageReceiveDecl{{Message: extMsg}},
		})
	}()

	sink := &engineStartupSink{s: s, proc: recvProc, startup: ctx}
	require.NoError(t, runStartupHandshake(ctx, sink))
	cancel()

	var catalog []sdk.BusCatalogEntry
	select {
	case catalog = <-gotCatalogCh:
	case <-time.After(2 * time.Second):
		t.Fatal("OnConfigure did not receive BusCatalog")
	}

	var foundExt *sdk.BusCatalogEntry
	var foundCompiled bool
	for i := range catalog {
		if catalog[i].Message == extMsg {
			foundExt = &catalog[i]
		}
		if catalog[i].Message == "resolve:dns" {
			foundCompiled = true
		}
	}
	require.NotNil(t, foundExt, "Stage 2 BusCatalog missing external declared message %q", extMsg)
	assert.Equal(t, []string{"acme-sender"}, foundExt.SenderKinds)
	require.Len(t, foundExt.Tags, 1)
	assert.Equal(t, "severity", foundExt.Tags[0].Name)
	assert.Equal(t, []string{"warn", "crit"}, foundExt.Tags[0].Values)
	assert.True(t, foundCompiled, "Stage 2 BusCatalog missing compiled-in message resolve:dns")

	<-pluginErrCh
}

// VALIDATES: onReady activates subscriptions from Stage 1 Receives (including
// direction tag mapping) without ReadyInput.Subscribe, delivers matching
// direction events, filters non-matching directions, and keeps dormant
// receptions without recording a subscription drop.
// PREVENTS: Stage 1 Receives being ignored at Stage 5 or treating dormant
// receptions as errors.
func TestStage1ReceivesActivatesSubscriptionsOnReady(t *testing.T) {
	const testNS = "receives-ready-ns"
	require.NoError(t, events.RegisterMessage(testNS, "update", events.DirectionTagDef()))

	s := newSubscriptionServer()
	s.dispatcher = NewDispatcher()
	proc, batches := newDeliveryProbe(t, "receives-probe")
	proc.SetRegistration(&plugin.PluginRegistration{
		Name: "receives-probe",
		Receives: []rpc.MessageReceiveDecl{
			{
				Message: testNS + ":update",
				Tags: []rpc.MessageTagDecl{{
					Name:   events.TagDirection,
					Values: []string{events.DirectionReceived},
				}},
			},
			{
				// Dormant reception in an absent namespace must not raise a drop.
				Message: "absent-dormant-ns:event",
			},
		},
	})

	sink := &engineStartupSink{s: s, proc: proc}
	require.NoError(t, sink.onReady(&rpc.ReadyInput{Format: "full"}))

	assert.Equal(t, "full", proc.Format())
	assert.Empty(t, proc.DoctorRecord(time.Now()).SubscriptionDrops,
		"dormant reception must not record a subscription drop")

	// Emit a "sent" event: must NOT match the direction=received declaration.
	_, err := s.deliverEvent(nil, testNS, "update", events.DirectionSent, "", `{"dir":"sent"}`)
	require.NoError(t, err)

	// Emit a "received" event: MUST match and be delivered.
	delivered, err := s.deliverEvent(nil, testNS, "update", events.DirectionReceived, "", `{"dir":"received"}`)
	require.NoError(t, err)
	assert.Equal(t, 1, delivered)

	batch := waitBatch(t, batches)
	require.Len(t, batch, 1)
	assert.Equal(t, `{"dir":"received"}`, batch[0])
}
