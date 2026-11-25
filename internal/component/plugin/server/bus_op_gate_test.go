// Design: docs/architecture/api/process-protocol.md -- bus operation gate, refusal shapes, and command profile publication
// Related: bus_op_gate.go -- engine-op bus gate, ValidateAllEngineOps, ReportBusSendRefusal
// Related: dispatch_registry.go -- built-in engineOp BusMessage / BusExemptReason declarations

package server

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestValidateAllEngineOpsRejectsUndeclared verifies every built-in engine
// operation declares its bus message or written exemption reason, and that
// ValidateEngineOpEntry and ValidateAllEngineOps fail on omitted or duplicate
// declarations.
//
// VALIDATES: AC-32, Q-T1 -- every built-in and owner-registered engine op is
// walked and undeclared entries fail validation.
// PREVENTS: a new engine operation shipping ungated by omission.
func TestValidateAllEngineOpsRejectsUndeclared(t *testing.T) {
	require.NoError(t, ValidateAllEngineOps(), "all built-in engineOps must declare BusMessage or BusExemptReason")

	// Negative cases on ValidateEngineOpEntry.
	require.Error(t, ValidateEngineOpEntry("ze-test:missing", "", ""), "neither message nor reason must fail")
	require.Error(t, ValidateEngineOpEntry("ze-test:blank", "   ", "\t"), "whitespace-only message and reason must fail")
	require.Error(t, ValidateEngineOpEntry("ze-test:both", "bgp:update", "some reason"), "declaring both must fail")

	// Positive cases on ValidateEngineOpEntry.
	require.NoError(t, ValidateEngineOpEntry("ze-test:msg", "bgp:update", ""))
	require.NoError(t, ValidateEngineOpEntry("ze-test:exempt", "", "acts on caller's own records"))

	// Negative case on the full walk: an owner-registered op with neither fails.
	const undeclaredMethod = "ze-plugin:undeclared-walk-test"
	registerTestEngineOperation(t, EngineOperation{
		Method:  undeclaredMethod,
		Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			return nil, nil //nolint:nilnil // no result payload
		},
	})
	err := ValidateAllEngineOps()
	require.Error(t, err, "ValidateAllEngineOps must reject an owner op with neither BusMessage nor BusExemptReason")
	assert.Contains(t, err.Error(), undeclaredMethod)
}

// TestRefusalNamesItsReason proves all three bus refusal shapes name the exact
// reason, increment ze_bus_send_refused_total with {participant, message}
// labels, and raise CodeDoctorPluginRequestRefused on the report bus.
//
// VALIDATES: AC-26, AC-27, AC-28 -- three refusal shapes, single counter, and
// report-bus doctor code.
// PREVENTS: silent refusals, missing fix lines, or wrong counter labels.
func TestRefusalNamesItsReason(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	s := &Server{dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	t.Cleanup(s.cancel)

	// Reason 1: participant registration does not declare the message.
	undeclared := process.NewProcess(plugin.PluginConfig{Name: "gate-undeclared"})
	undeclared.SetRegistration(&plugin.PluginRegistration{})
	undeclaredPath := plugin.EndpointPath(undeclared.Name())
	beforeUndeclared := BusSendRefusedCount(undeclaredPath, busMsgSystemRIBInstall)

	_, err := s.dispatchPluginRPCDirect(t.Context(), undeclared, rpc.MethodRouteInstall, json.RawMessage(`{"routes":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), undeclaredPath)
	assert.Contains(t, err.Error(), busMsgSystemRIBInstall)
	assert.Contains(t, err.Error(), "registration does not declare")
	assert.Contains(t, err.Error(), "show config running | implied")
	assert.Contains(t, err.Error(), "show plugin conformance")
	assert.NotContains(t, err.Error(), "set bus endpoint", "undeclared refusal must not suggest a config allow line")
	assert.Equal(t, beforeUndeclared+1, BusSendRefusedCount(undeclaredPath, busMsgSystemRIBInstall))

	// Verify CodeDoctorPluginRequestRefused was raised on the report bus.
	foundDoctor := false
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginRequestRefused && issue.Subject == undeclaredPath {
			foundDoctor = true
			assert.Contains(t, issue.Message, busMsgSystemRIBInstall)
			break
		}
	}
	assert.True(t, foundDoctor, "refusal must raise CodeDoctorPluginRequestRefused on the report bus")

	// Reason 2: declared by registration, but removed by send allow-list.
	ops := process.NewProcess(plugin.PluginConfig{Name: "ops"})
	ops.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{
			{Message: busMsgBGPUpdate},
			{Message: busMsgSystemRIBInstall},
			{Message: "cli:clear bgp peer"},
		},
	})
	s.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"ops": {
				Send: plugin.BusDirectionBlock{
					AllowPresent: true,
					Allow:        []string{busMsgBGPUpdate},
				},
			},
		},
	})
	opsPath := plugin.EndpointPath("ops")
	beforeCLI := BusSendRefusedCount(opsPath, "cli:clear bgp peer")
	beforeRIB := BusSendRefusedCount(opsPath, busMsgSystemRIBInstall)

	_, err = s.dispatchPluginRPCDirect(t.Context(), ops, rpc.MethodDispatchCommand, json.RawMessage(`{"command":"clear bgp peer"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), opsPath)
	assert.Contains(t, err.Error(), "cli:clear bgp peer")
	assert.Contains(t, err.Error(), "send allow-list")
	assert.Contains(t, err.Error(), `set bus endpoint ops send allow "cli:clear bgp peer"`)
	assert.Equal(t, beforeCLI+1, BusSendRefusedCount(opsPath, "cli:clear bgp peer"))

	_, err = s.dispatchPluginRPCDirect(t.Context(), ops, rpc.MethodRouteInstall, json.RawMessage(`{"routes":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), opsPath)
	assert.Contains(t, err.Error(), busMsgSystemRIBInstall)
	assert.Contains(t, err.Error(), "send allow-list")
	assert.Contains(t, err.Error(), "set bus endpoint ops send allow system-rib:route-install")
	assert.Equal(t, beforeRIB+1, BusSendRefusedCount(opsPath, busMsgSystemRIBInstall))

	// Reason 3: target not covered by to/from list.
	targetErr := FormatBusTargetRefusal("/bus/endpoint/rs", "/proto/entry/edge1", busMsgBGPUpdate, "bus > endpoint rs > send > to")
	require.NotNil(t, targetErr)
	assert.Contains(t, targetErr.Error(), "/bus/endpoint/rs")
	assert.Contains(t, targetErr.Error(), "/proto/entry/edge1")
	assert.Contains(t, targetErr.Error(), busMsgBGPUpdate)
	assert.Contains(t, targetErr.Error(), "bus > endpoint rs > send > to")
}

// TestEmitNeedsARegisteredSend proves emit-event fails closed when the caller
// did not declare the message, succeeds once declared, overwrites any spoofed
// sender field in the payload with the authenticated participant path, and
// honors receive > from filters against the stamped sender.
//
// VALIDATES: AC-30, AC-35, S25, S37.
// PREVENTS: undeclared event emission and sender spoofing in event payloads.
func TestEmitNeedsARegisteredSend(t *testing.T) {
	registerTestBusMessages(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	s := &Server{
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
		dispatcher:        NewDispatcher(),
	}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	t.Cleanup(s.cancel)

	// Subscriber listening for acme:alert.
	receiver, batches := newDeliveryProbe(t, "gate-collector")
	t.Cleanup(func() { s.subscriptions.clearProcess(receiver) })
	s.subscriptions.Add(receiver, &Subscription{
		Namespace: events.LookupNamespaceID(testNSAcme),
		EventType: events.LookupEventTypeID("alert"),
		Direction: events.DirBoth,
	})

	// Undeclared emitter is refused and nothing is delivered.
	unauth := process.NewProcess(plugin.PluginConfig{Name: "gate-unauth-emitter"})
	unauth.SetRegistration(&plugin.PluginRegistration{})
	unauthPath := plugin.EndpointPath(unauth.Name())
	beforeRefused := BusSendRefusedCount(unauthPath, testMsgAcmeAlert)

	delivered, err := s.deliverEvent(unauth, testNSAcme, "alert", "", "", `{"detail":"unauth"}`)
	require.Error(t, err)
	assert.Zero(t, delivered)
	assert.Contains(t, err.Error(), testMsgAcmeAlert)
	assert.Equal(t, beforeRefused+1, BusSendRefusedCount(unauthPath, testMsgAcmeAlert))

	// Declared emitter succeeds and any spoofed "sender" field is overwritten
	// with the authenticated endpoint path.
	senderB := process.NewProcess(plugin.PluginConfig{Name: "gate-emitter-b"})
	senderB.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
	})
	delivered, err = s.deliverEvent(senderB, testNSAcme, "alert", "", "", `{"sender":"/bus/endpoint/trusted-a","detail":"spoof-attempt"}`)
	require.NoError(t, err)
	assert.Equal(t, 1, delivered)

	batch := waitBatch(t, batches)
	require.Len(t, batch, 1)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(batch[0]), &got))
	assert.Equal(t, plugin.EndpointPath("gate-emitter-b"), got["sender"], "bus must stamp authenticated sender path over spoofed sender")
	assert.Equal(t, "spoof-attempt", got["detail"])

	// With a CompiledBus where gate-collector only allows receive > from
	// [ /bus/endpoint/trusted-a ], gate-emitter-b spoofing trusted-a delivers 0.
	err = s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{
		{
			Path:         "/bus/endpoint/trusted-a",
			EndpointName: "trusted-a",
			Sends:        []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
		},
		{
			Path:         plugin.EndpointPath("gate-emitter-b"),
			EndpointName: "gate-emitter-b",
			Sends:        []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
		},
		{
			Path:         plugin.EndpointPath("gate-collector"),
			EndpointName: "gate-collector",
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgAcmeAlert}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/bus/endpoint/trusted-a"},
				},
			},
		},
	})
	require.NoError(t, err)

	delivered, err = s.deliverEvent(senderB, testNSAcme, "alert", "", "", `{"sender":"/bus/endpoint/trusted-a","detail":"blocked-spoof"}`)
	require.NoError(t, err)
	assert.Zero(t, delivered, "receive > from must reject spoofed sender after bus stamping")
}

// TestUpdateRouteRestrictsToUpdateSubcommands proves update-route checks
// bgp:update (and bgp:refresh for borr/eorr markers), refuses raw or arbitrary
// subcommand smuggling on both JSON and DirectBridge rails, and succeeds under
// real fail-closed authz when send > allow holds only bgp:update (no cli:send
// bgp required).
//
// VALIDATES: AC-40, Q-T4(b), S36.
// PREVENTS: raw/arbitrary command smuggling through update-route and breaking
// bgp:update-only allow-lists with a redundant cli:send bgp requirement.
func TestUpdateRouteRestrictsToUpdateSubcommands(t *testing.T) {
	family.RegisterTestFamilies()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	d := NewDispatcher()
	d.SetAuthorizer(authz.StoreAuthorizer{Store: authz.NewStore()})

	dispatched := 0
	require.NoError(t, d.RegisterWithOptions("send bgp", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		dispatched++
		return &plugin.Response{Status: plugin.StatusDone, Data: &plugin.RouteResult{Announced: 1}}, nil
	}, "send test route", RegisterOptions{AcceptsDestinationSession: true}))
	require.NoError(t, d.RegisterWithOptions("request peer", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		dispatched++
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "request test marker", RegisterOptions{AcceptsDestinationSession: true}))

	s := &Server{dispatcher: d, subscriptions: newSubscriptionManager()}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	t.Cleanup(s.cancel)

	// Endpoint declares bgp:update + cli:show version, and bus config narrows
	// send > allow to [ bgp:update ] ONLY (no cli:send bgp, no bgp:refresh).
	proc := process.NewProcess(plugin.PluginConfig{Name: "update-only"})
	proc.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{
			{Message: busMsgBGPUpdate},
			{Message: "cli:show version"},
		},
	})
	s.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"update-only": {
				Send: plugin.BusDirectionBlock{
					AllowPresent: true,
					Allow:        []string{busMsgBGPUpdate},
				},
			},
		},
	})
	require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{{
		Path:         plugin.EndpointPath("update-only"),
		EndpointName: "update-only",
		Sends: []rpc.MessageSendDecl{
			{Message: busMsgBGPUpdate},
			{Message: "cli:show version"},
		},
	}}))
	t.Cleanup(func() { withdrawPluginProfileServer(s) })

	// 1. Valid UPDATE subcommand succeeds under fail-closed authz WITHOUT cli:send bgp!
	_, err := s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodUpdateRoute,
		json.RawMessage(`{"peer-selector":"192.0.2.1","command":"update text nlri add 203.0.113.0/24"}`))
	require.NoError(t, err)
	assert.Equal(t, 1, dispatched)

	// 2. Non-UPDATE subcommands (raw, clear) are refused before dispatch on both JSON and typed bridge.
	for _, smuggled := range []string{"raw hex DEADBEEF", "clear bgp peer"} {
		rawParams, marshalErr := json.Marshal(rpc.UpdateRouteInput{PeerSelector: "192.0.2.1", Command: smuggled})
		require.NoError(t, marshalErr)
		_, err = s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodUpdateRoute, rawParams)
		require.Error(t, err, "smuggled subcommand %q must be refused", smuggled)
		assert.Contains(t, err.Error(), "refuses subcommand")

		_, err = s.handleUpdateRouteSelResultDirect(t.Context(), proc, selector.ParseDefault("192.0.2.1"), smuggled, nil, 0)
		require.Error(t, err, "typed bridge must also refuse smuggled subcommand %q", smuggled)
		assert.Contains(t, err.Error(), "refuses subcommand")
	}
	assert.Equal(t, 1, dispatched, "smuggled subcommands must never reach the dispatcher")

	// 3. cli:show version is refused by the bgp:update-only allow-list (AC-40 / S36).
	_, err = s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodDispatchCommand,
		json.RawMessage(`{"command":"show version"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send allow-list")

	// 4. borr/eorr markers require bgp:refresh, not bgp:update.
	deadline := time.Now().Add(10 * time.Second).UnixMilli()
	markerParams, err := json.Marshal(rpc.UpdateRouteInput{
		PeerSelector:       "192.0.2.1",
		PeerMarker:         "borr",
		MarkerFamily:       "ipv4/unicast",
		DestinationOwner:   17,
		DestinationSession: 71,
		DeadlineMS:         deadline,
	})
	require.NoError(t, err)
	_, err = s.dispatchPluginRPCDirect(t.Context(), proc, rpc.MethodUpdateRoute, markerParams)
	require.Error(t, err, "borr marker must fail when bgp:refresh is not declared")
	assert.Contains(t, err.Error(), busMsgBGPRefresh)

	// Once bgp:refresh is declared and allowed, borr succeeds.
	refreshProc := process.NewProcess(plugin.PluginConfig{Name: "refresh-capable"})
	refreshProc.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{{Message: busMsgBGPRefresh}},
	})
	_, err = s.dispatchPluginRPCDirect(t.Context(), refreshProc, rpc.MethodUpdateRoute, markerParams)
	require.NoError(t, err)
	assert.Equal(t, 2, dispatched)
}

// TestBatchValidateGatedOnBothTransports proves batch-validate is gated on
// cli:request bgp adj-rib-in batch-validate on both socket and in-daemon
// DirectBridge transports, and that a DirectBridge with DispatchRPC active
// refuses the legacy ungated SetBatchValidate shortcut.
//
// VALIDATES: AC-26, AC-28, AC-32, Q-T3(a), S27.
// PREVENTS: in-daemon batch-validate bypassing bus send authorization via the
// legacy DirectBridge shortcut.
func TestBatchValidateGatedOnBothTransports(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	for _, direct := range []bool{false, true} {
		mode := "socket"
		if direct {
			mode = "direct"
		}
		t.Run(mode, func(t *testing.T) {
			d := NewDispatcher()
			d.SetAuthorizer(authz.StoreAuthorizer{Store: authz.NewStore()})
			s := &Server{dispatcher: d, subscriptions: newSubscriptionManager()}
			s.ctx, s.cancel = context.WithCancel(t.Context())
			t.Cleanup(s.cancel)
			t.Cleanup(func() { withdrawPluginProfileServer(s) })

			done := registerExecuteCommandTarget(t, d, rpc.BatchValidateCommand, 1,
				func(_ int, _ *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
					return &rpc.ExecuteCommandOutput{
						Status: rpc.StatusDone,
						Data:   json.RawMessage(`{"accepted":1,"rejected":0,"early":0}`),
					}, nil
				})

			// Helper wires a real SDK client on either socket or DirectBridge.
			newCaller := func(proc *process.Process, requests int) *sdk.Plugin {
				client, engine := net.Pipe()
				if direct {
					bridge := rpc.NewDirectBridge()
					// Even if legacy SetBatchValidate is set, DispatchRPC disables the shortcut.
					bridge.SetBatchValidate(func([]rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
						t.Error("legacy ungated DirectBridge.BatchValidate shortcut must not run when DispatchRPC is active")
						return &rpc.BatchValidateResult{Accepted: 999}, nil
					})
					bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
						return s.dispatchPluginRPCDirect(ctx, proc, method, params)
					})
					bridge.SetReady()
					require.False(t, bridge.HasBatchValidate(), "HasBatchValidate must be false when DispatchRPC is wired")
					_, shortcutErr := bridge.BatchValidate(nil)
					require.Error(t, shortcutErr, "direct BatchValidate call must fail when DispatchRPC is wired")
					p := sdk.NewWithConn(proc.Name(), rpc.NewBridgedConn(client, bridge))
					t.Cleanup(func() { _ = p.Close(); _ = engine.Close() })
					return p
				}
				conn := plugipc.NewPluginConn(engine, engine)
				served := make(chan struct{})
				go func() {
					defer close(served)
					for range requests {
						req, readErr := conn.ReadRequest(s.ctx)
						if readErr != nil {
							return
						}
						s.dispatchPluginRPC(proc, conn, req)
					}
				}()
				p := sdk.NewWithConn(proc.Name(), client)
				t.Cleanup(func() { _ = p.Close(); _ = engine.Close(); <-served })
				return p
			}

			decision := []rpc.ValidationDecision{{
				Accept: true, PeerAddr: "192.0.2.1", Family: "ipv4/unicast", Prefix: "10.0.0.0/24", ValState: 1,
			}}

			// 1. Undeclared caller is refused on both transports.
			unauthProc := process.NewProcess(plugin.PluginConfig{Name: "bv-unauth"})
			unauthProc.SetRegistration(&plugin.PluginRegistration{})
			unauthCaller := newCaller(unauthProc, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			_, err := unauthCaller.BatchValidate(ctx, decision)
			require.Error(t, err)
			assert.Contains(t, err.Error(), busMsgBatchValidate)

			// 2. Declared caller passes both bus gate and authz PluginCommandProfile.
			authProc := process.NewProcess(plugin.PluginConfig{Name: "bv-auth"})
			authProc.SetRegistration(&plugin.PluginRegistration{
				Sends: []rpc.MessageSendDecl{{Message: busMsgBatchValidate}},
			})
			require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{{
				Path:         plugin.EndpointPath("bv-auth"),
				EndpointName: "bv-auth",
				Sends:        []rpc.MessageSendDecl{{Message: busMsgBatchValidate}},
			}}))

			// Verify profile published to aaa lookup.
			prof, ok := aaa.LookupPluginCommandProfile("bv-auth")
			require.True(t, ok)
			require.False(t, prof.AllowNone)
			require.Contains(t, prof.AllowedPrefixes, rpc.BatchValidateCommand)

			authCaller := newCaller(authProc, 2)
			res, err := authCaller.BatchValidate(ctx, decision)
			require.NoError(t, err)
			require.Equal(t, &rpc.BatchValidateResult{Accepted: 1}, res)

			// 3. Live BusConfig narrowing + RefreshDeliveryGraph denies without restarting endpoint (S27).
			s.SetBusConfig(plugin.BusConfig{
				Endpoints: map[string]plugin.BusBlock{
					"bv-auth": {
						Send: plugin.BusDirectionBlock{
							AllowPresent: true,
							AllowNone:    true,
						},
					},
				},
			})
			require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{{
				Path:         plugin.EndpointPath("bv-auth"),
				EndpointName: "bv-auth",
				Sends:        []rpc.MessageSendDecl{{Message: busMsgBatchValidate}},
			}}))
			prof, ok = aaa.LookupPluginCommandProfile("bv-auth")
			require.True(t, ok)
			require.True(t, prof.AllowNone)

			_, err = authCaller.BatchValidate(ctx, decision)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "send allow-list")

			require.NoError(t, <-done)
		})
	}
}

// TestBusOpGateWave1Security proves the Wave 1 security invariants:
//   - S1: peer-scoped emit-event from a plugin enforces the plugin's compiled
//     send > to and receiver's receive > from rather than spoofing the peer;
//   - S2: send { to none; } refuses cli: sends in both the bus gate and
//     compiledFilterToCommandProfile, and PermitSenderPeer enforces send > to
//     on peer-selecting CLI verbs;
//   - S3: update-route rejects PeerSelector whitespace/pipes and command pipes;
//   - S5: wildcard subscribe-events ["*"] expands only to compiled allowed
//     receptions and engine-emitted events enforce compiled receive filters.
func TestBusOpGateWave1Security(t *testing.T) {
	t.Run("S1_peer_scoped_emit_cannot_spoof_peer", func(t *testing.T) {
		s := &Server{dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
		s.ctx, s.cancel = context.WithCancel(t.Context())
		t.Cleanup(s.cancel)

		spoofer := process.NewProcess(plugin.PluginConfig{Name: "spoofer"})
		spoofer.SetRegistration(&plugin.PluginRegistration{
			Sends: []rpc.MessageSendDecl{{Message: "bgp:state"}},
		})
		victim := process.NewProcess(plugin.PluginConfig{Name: "victim"})
		victim.SetRegistration(&plugin.PluginRegistration{
			Receives: []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
		})
		s.subscriptions.Add(victim, &Subscription{
			Namespace: events.LookupNamespaceID("bgp"),
			EventType: events.LookupEventTypeID("state"),
			Direction: events.DirBoth,
		})

		// Peer 10.0.0.1 declares sending bgp:state; victim only accepts bgp:state
		// from /bgp/ip/10.0.0.1, NOT from /bus/endpoint/spoofer.
		s.SetBusConfig(plugin.BusConfig{
			Endpoints: map[string]plugin.BusBlock{
				"victim": {
					Receive: plugin.BusDirectionBlock{
						TargetsPresent: true,
						Targets:        []string{"/bgp/ip/10.0.0.1"},
					},
				},
			},
		})
		require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("bgp", []DeliveryPeer{{
			Addr:           "10.0.0.1",
			Name:           "p1",
			Path:           "/bgp/peer/p1",
			SecondaryPaths: []string{"/bgp/ip/10.0.0.1"},
		}}, []BusParticipantInput{
			{
				Path:         plugin.EndpointPath("spoofer"),
				EndpointName: "spoofer",
				Sends:        []rpc.MessageSendDecl{{Message: "bgp:state"}},
			},
			{
				Path:         plugin.EndpointPath("victim"),
				EndpointName: "victim",
				Receives:     []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
			},
		}))

		delivered, err := s.deliverEvent(spoofer, "bgp", "state", "", "10.0.0.1", `{"state":"down"}`)
		require.NoError(t, err)
		assert.Equal(t, 0, delivered, "plugin emit-event with peerAddress must not bypass receive > from by spoofing the peer")
	})

	t.Run("S2_cli_to_none_and_peer_target_enforcement", func(t *testing.T) {
		s := &Server{dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
		s.ctx, s.cancel = context.WithCancel(t.Context())
		t.Cleanup(s.cancel)

		noneProc := process.NewProcess(plugin.PluginConfig{Name: "ops-none"})
		noneProc.SetRegistration(&plugin.PluginRegistration{
			Sends: []rpc.MessageSendDecl{{Message: "cli:request peer"}},
		})
		scopedProc := process.NewProcess(plugin.PluginConfig{Name: "ops-scoped"})
		scopedProc.SetRegistration(&plugin.PluginRegistration{
			Sends: []rpc.MessageSendDecl{{Message: "cli:request peer"}},
		})
		s.SetBusConfig(plugin.BusConfig{
			Endpoints: map[string]plugin.BusBlock{
				"ops-none": {
					Send: plugin.BusDirectionBlock{
						TargetsPresent: true,
						TargetsNone:    true,
					},
				},
				"ops-scoped": {
					Send: plugin.BusDirectionBlock{
						TargetsPresent: true,
						Targets:        []string{"/bgp/peer/p1"},
					},
				},
			},
		})
		require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("bgp", []DeliveryPeer{
			{Addr: "10.0.0.1", Name: "p1", Path: "/bgp/peer/p1", SecondaryPaths: []string{"/bgp/ip/10.0.0.1"}},
			{Addr: "10.0.0.2", Name: "p2", Path: "/bgp/peer/p2", SecondaryPaths: []string{"/bgp/ip/10.0.0.2"}},
		}, []BusParticipantInput{
			{
				Path:         plugin.EndpointPath("ops-none"),
				EndpointName: "ops-none",
				Sends:        []rpc.MessageSendDecl{{Message: "cli:request peer"}},
			},
			{
				Path:         plugin.EndpointPath("ops-scoped"),
				EndpointName: "ops-scoped",
				Sends:        []rpc.MessageSendDecl{{Message: "cli:request peer"}},
			},
		}))

		// send { to none; } refuses cli: send and produces AllowNone profile.
		err := checkEngineOpBusSendCLI(s, noneProc, "cli:request peer 10.0.0.1 teardown", "cli:request peer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot send")
		prof, ok := s.lookupPluginCommandProfile("ops-none")
		require.True(t, ok)
		assert.True(t, prof.AllowNone)

		// send { to [ "/bgp/peer/p1" ]; } allows p1 and refuses p2.
		cmdCtx := &CommandContext{
			Server:              s,
			Sender:              plugin.ProcessSender("ops-scoped"),
			BusCLIMessage:       "cli:request peer 10.0.0.1 teardown",
			BusCLIReportMessage: "cli:request peer",
		}
		assert.False(t, SenderCLIToAll(cmdCtx))
		_, err = PermitSenderPeer(cmdCtx, netip.MustParseAddr("10.0.0.1"))
		require.NoError(t, err)
		_, err = PermitSenderPeer(cmdCtx, netip.MustParseAddr("10.0.0.2"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/bgp/ip/10.0.0.2")
	})

	t.Run("S3_update_route_rejects_smuggling", func(t *testing.T) {
		_, err := updateRouteDispatchCommand(rpc.UpdateRouteInput{Command: "announce route 192.0.2.0/24 next-hop 192.0.2.1"}, "10.0.0.1 raw deadbeef")
		require.Error(t, err, "PeerSelector with whitespace must be rejected")

		_, err = updateRouteDispatchCommand(rpc.UpdateRouteInput{Command: "announce route 192.0.2.0/24 next-hop 192.0.2.1 | text"}, "10.0.0.1")
		require.Error(t, err, "update-route command with pipe must be rejected")

		_, err = updateRouteDispatchCommand(rpc.UpdateRouteInput{Command: "announce route 192.0.2.0/24 next-hop 192.0.2.1\nrequest peer * teardown"}, "10.0.0.1")
		require.Error(t, err, "update-route command with newline must be rejected")

		cmd, err := updateRouteDispatchCommand(rpc.UpdateRouteInput{Command: "announce route 192.0.2.0/24 next-hop 192.0.2.1"}, "10.0.0.1")
		require.NoError(t, err)
		assert.Equal(t, "send bgp 10.0.0.1 announce route 192.0.2.0/24 next-hop 192.0.2.1", cmd)
	})

	t.Run("S5_wildcard_and_engine_receive_filtering", func(t *testing.T) {
		s := &Server{dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
		s.ctx, s.cancel = context.WithCancel(t.Context())
		t.Cleanup(s.cancel)

		sub := process.NewProcess(plugin.PluginConfig{Name: "sub-only-state"})
		sub.SetRegistration(&plugin.PluginRegistration{
			Receives: []rpc.MessageReceiveDecl{
				{Message: "bgp:state"},
				{Message: "bgp:open"},
			},
		})
		s.SetBusConfig(plugin.BusConfig{
			Endpoints: map[string]plugin.BusBlock{
				"sub-only-state": {
					Receive: plugin.BusDirectionBlock{
						AllowPresent: true,
						Allow:        []string{"bgp:state"},
					},
				},
			},
		})
		require.NoError(t, s.UpdateDeliveryGraphWithEndpoints("bgp", nil, []BusParticipantInput{{
			Path:         plugin.EndpointPath("sub-only-state"),
			EndpointName: "sub-only-state",
			Receives: []rpc.MessageReceiveDecl{
				{Message: "bgp:state"},
				{Message: "bgp:open"},
			},
		}}))

		s.registerSubscriptions(sub, &rpc.SubscribeEventsInput{
			Namespace: "bgp",
			Events:    []string{"*"},
		})

		bgpNS := events.LookupNamespaceID("bgp")
		stateET := events.LookupEventTypeID("state")
		openET := events.LookupEventTypeID("open")
		assert.Len(t, s.subscriptions.getMatching(bgpNS, stateET, events.DirBoth, "", ""), 1)
		assert.Empty(t, s.subscriptions.getMatching(bgpNS, openET, events.DirBoth, "", ""),
			"wildcard subscription must not register events excluded by receive > allow")

		// Even if a subscription exists directly, engine-emitted delivery enforces compiled receive filter.
		s.subscriptions.Add(sub, &Subscription{Namespace: bgpNS, EventType: openET, Direction: events.DirBoth})
		delivered, err := s.deliverEvent(nil, "bgp", "open", "", "", `{"asn":65001}`)
		require.NoError(t, err)
		assert.Equal(t, 0, delivered, "engine-emitted event must respect receive > allow")
	})
}
