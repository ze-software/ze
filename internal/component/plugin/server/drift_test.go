// VALIDATES: Stage 1 drift-subsystems validation, daemon drift-check fan-out
// over both MuxConn (forked) and DirectBridge (internal), finding cap at 64,
// timeout/error reporting, and clean registration/unregistration on Start/Stop.
// PREVENTS: forked plugins failing drift checks, unbounded findings, held locks
// across RPC calls, or stale drift registrations leaking across restarts.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func runSDKPluginForDriftTest(ctx context.Context, p *sdk.Plugin, reg *sdk.Registration, errCh chan<- error) {
	errCh <- p.Run(ctx, reg)
}

func runHandshakeForDriftTest(ctx context.Context, sink *engineStartupSink, errCh chan<- error) {
	errCh <- runStartupHandshake(ctx, sink)
}

func serveBridgeDriftCallbacks(bridge *rpc.DirectBridge) {
	for cb := range bridge.CallbackCh() {
		if cb.Method != "ze-plugin-callback:drift-check" {
			cb.Result <- rpc.BridgeCallbackResult{Err: errors.New("unexpected method: " + cb.Method)}
			continue
		}
		var in rpc.DriftCheckInput
		if err := json.Unmarshal(cb.Params, &in); err != nil {
			cb.Result <- rpc.BridgeCallbackResult{Err: err}
			continue
		}
		raw, err := json.Marshal(&rpc.DriftCheckOutput{
			Subsystem: in.Subsystem,
			Status:    string(drift.StatusClean),
			Checked:   7,
			Drifted:   0,
			Findings:  []rpc.DriftFinding{},
		})
		cb.Result <- rpc.BridgeCallbackResult{Data: raw, Err: err}
	}
}

func TestValidateDriftSubsystemDecls(t *testing.T) {
	t.Parallel()

	t.Run("valid declarations", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateDriftSubsystemDecls(nil))
		require.NoError(t, validateDriftSubsystemDecls([]string{}))
		require.NoError(t, validateDriftSubsystemDecls([]string{"firewall", "fib-kernel", "interface", "dp0"}))
		require.NoError(t, validateDriftSubsystemDecls([]string{strings.Repeat("a", maxDriftSubsystemNameLen)}))

		maxAllowed := make([]string, maxDriftSubsystemsPerPlugin)
		for i := range maxAllowed {
			maxAllowed[i] = "sub-" + strconv.Itoa(i)
		}
		require.NoError(t, validateDriftSubsystemDecls(maxAllowed))
	})

	t.Run("invalid declarations", func(t *testing.T) {
		t.Parallel()
		tooMany := make([]string, maxDriftSubsystemsPerPlugin+1)
		for i := range tooMany {
			tooMany[i] = "sub-" + strconv.Itoa(i)
		}

		cases := []struct {
			name string
			in   []string
		}{
			{name: "too many", in: tooMany},
			{name: "empty name", in: []string{""}},
			{name: "too long", in: []string{strings.Repeat("x", maxDriftSubsystemNameLen+1)}},
			{name: "uppercase", in: []string{"Firewall"}},
			{name: "space", in: []string{"bad name"}},
			{name: "tab", in: []string{"bad\tname"}},
			{name: "control char", in: []string{"bad\x1bname"}},
			{name: "underscore", in: []string{"fib_kernel"}},
			{name: "duplicate", in: []string{"firewall", "firewall"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				require.Error(t, validateDriftSubsystemDecls(tc.in))
			})
		}
	})
}

func TestDriftCheckOverMuxConn(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.StartWithContext(t.Context()))
	t.Cleanup(s.Stop)

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() {
		_ = engineSide.Close()
		_ = pluginSide.Close()
	})

	conn := muxPluginConn(t, engineSide)
	proc := process.NewProcess(plugin.PluginConfig{Name: "ext-dp-plugin", Internal: false})
	markProcessRunning(t, proc)
	proc.SetConn(conn)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess(proc.Name(), proc))
	s.procManager.Store(pm)

	p := sdk.NewWithConn("ext-dp-plugin", pluginSide)
	p.OnDriftCheck(func(subsystem string) (rpc.DriftCheckOutput, error) {
		switch subsystem {
		case "test-dp":
			return rpc.DriftCheckOutput{
				Subsystem: "test-dp",
				Status:    string(drift.StatusDrifted),
				Checked:   4,
				Drifted:   1,
				Findings: []rpc.DriftFinding{
					{
						Subject:  "table ze_filter",
						Kind:     string(drift.KindMissing),
						Expected: "present",
						Actual:   "absent",
						Detail:   "missing in kernel",
					},
				},
			}, nil
		case "test-cap":
			findings := make([]rpc.DriftFinding, drift.MaxFindingsPerSubsystem+10)
			for i := range findings {
				findings[i] = rpc.DriftFinding{
					Subject: "item-" + strconv.Itoa(i),
					Kind:    string(drift.KindExtra),
				}
			}
			return rpc.DriftCheckOutput{
				Subsystem: "test-cap",
				Status:    string(drift.StatusDrifted),
				Checked:   len(findings),
				Drifted:   len(findings),
				Findings:  findings,
			}, nil
		case "test-err":
			return rpc.DriftCheckOutput{}, errors.New("backend unavailable")
		case "test-slow":
			time.Sleep(300 * time.Millisecond)
			return rpc.DriftCheckOutput{Subsystem: "test-slow", Status: string(drift.StatusClean)}, nil
		default:
			return rpc.DriftCheckOutput{Subsystem: subsystem, Status: string(drift.StatusUnavailable)}, nil
		}
	})

	runCtx, cancelRun := context.WithCancel(t.Context())
	defer cancelRun()

	pluginErrCh := make(chan error, 1)
	go runSDKPluginForDriftTest(runCtx, p, &sdk.Registration{
		DriftSubsystems: []string{"test-dp", "test-cap", "test-err", "test-slow"},
	}, pluginErrCh)

	handshakeCtx, cancelHandshake := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelHandshake()
	sink := &engineStartupSink{s: s, proc: proc, startup: handshakeCtx}
	handshakeErrCh := make(chan error, 1)
	go runHandshakeForDriftTest(handshakeCtx, sink, handshakeErrCh)

	require.NoError(t, <-handshakeErrCh)

	// Subsystems lister wired via drift.RegisterRemoteLister reports sorted names.
	subsystems := drift.Subsystems()
	assert.Contains(t, subsystems, "test-dp")
	assert.Contains(t, subsystems, "test-cap")
	assert.Contains(t, subsystems, "test-err")
	assert.Contains(t, subsystems, "test-slow")

	// Direct SendDriftCheck and drift.Check round-trip findings over MuxConn.
	rep, found := drift.Check(t.Context(), "test-dp")
	require.True(t, found)
	assert.Equal(t, "test-dp", rep.Subsystem)
	assert.Equal(t, drift.StatusDrifted, rep.Status)
	assert.Equal(t, 4, rep.Checked)
	assert.Equal(t, 1, rep.Drifted)
	assert.False(t, rep.Truncated)
	require.Len(t, rep.Findings, 1)
	assert.Equal(t, drift.Finding{
		Subject:  "table ze_filter",
		Kind:     drift.KindMissing,
		Expected: "present",
		Actual:   "absent",
		Detail:   "missing in kernel",
	}, rep.Findings[0])

	// Findings past drift.MaxFindingsPerSubsystem are capped and marked Truncated.
	capRep, capFound := drift.Check(t.Context(), "test-cap")
	require.True(t, capFound)
	assert.Equal(t, drift.StatusDrifted, capRep.Status)
	assert.True(t, capRep.Truncated)
	assert.Len(t, capRep.Findings, drift.MaxFindingsPerSubsystem)

	// Plugin callback error produces drift.StatusError with non-nil empty findings.
	errRep, errFound := drift.Check(t.Context(), "test-err")
	require.True(t, errFound)
	assert.Equal(t, "test-err", errRep.Subsystem)
	assert.Equal(t, drift.StatusError, errRep.Status)
	assert.NotNil(t, errRep.Findings)
	assert.Empty(t, errRep.Findings)
	assert.Contains(t, errRep.Error, "backend unavailable")

	// Caller deadline shorter than 5s bounds the RPC and returns drift.StatusError.
	shortCtx, cancelShort := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelShort()
	slowReports := s.collectPluginDrift(shortCtx, "test-slow")
	require.Len(t, slowReports, 1)
	assert.Equal(t, "test-slow", slowReports[0].Subsystem)
	assert.Equal(t, drift.StatusError, slowReports[0].Status)
	assert.NotNil(t, slowReports[0].Findings)
	assert.Empty(t, slowReports[0].Findings)
	assert.NotEmpty(t, slowReports[0].Error)

	cancelRun()
	_ = engineSide.Close()
	_ = pluginSide.Close()
	<-pluginErrCh

	// Stopping the server unregisters the remote collector and lister cleanly.
	s.Stop()
	assert.NotContains(t, drift.Subsystems(), "test-dp")
}

func TestDriftCheckOverDirectBridgeAndClosedConn(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.StartWithContext(t.Context()))
	t.Cleanup(s.Stop)

	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() {
		_ = engineSide.Close()
		_ = pluginSide.Close()
	})

	conn := ipc.NewPluginConn(engineSide, engineSide)
	bridge := rpc.NewDirectBridge()
	t.Cleanup(bridge.CloseCallbacks)
	go serveBridgeDriftCallbacks(bridge)
	bridge.SetReady()
	conn.SetBridge(bridge)

	intProc := process.NewProcess(plugin.PluginConfig{Name: "int-fw", Internal: true})
	markProcessRunning(t, intProc)
	intProc.SetConn(conn)

	deadProc := process.NewProcess(plugin.PluginConfig{Name: "disconnected-dp", Internal: true})
	markProcessRunning(t, deadProc)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess(intProc.Name(), intProc))
	require.NoError(t, pm.AddProcess(deadProc.Name(), deadProc))
	s.procManager.Store(pm)

	require.NoError(t, s.registerPluginDriftSubsystems(intProc, []string{"bridge-fw"}))
	require.NoError(t, s.registerPluginDriftSubsystems(deadProc, []string{"dead-dp"}))

	reports := s.collectPluginDrift(t.Context(), "")
	require.Len(t, reports, 2)

	assert.Equal(t, "bridge-fw", reports[0].Subsystem)
	assert.Equal(t, drift.StatusClean, reports[0].Status)
	assert.Equal(t, 7, reports[0].Checked)
	assert.NotNil(t, reports[0].Findings)
	assert.Empty(t, reports[0].Findings)

	assert.Equal(t, "dead-dp", reports[1].Subsystem)
	assert.Equal(t, drift.StatusError, reports[1].Status)
	assert.NotNil(t, reports[1].Findings)
	assert.Empty(t, reports[1].Findings)
	assert.Contains(t, reports[1].Error, ErrPluginConnectionClosed.Error())
}

func TestDriftSubsystemsCrossPluginConflictAndRestart(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)

	procA := process.NewProcess(plugin.PluginConfig{Name: "dp-owner-a", Internal: true})
	markProcessRunning(t, procA)
	procB := process.NewProcess(plugin.PluginConfig{Name: "dp-owner-b", Internal: true})
	markProcessRunning(t, procB)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess(procA.Name(), procA))
	require.NoError(t, pm.AddProcess(procB.Name(), procB))
	s.procManager.Store(pm)

	sinkA := &engineStartupSink{s: s, proc: procA, startup: t.Context()}
	require.NoError(t, sinkA.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		DriftSubsystems: []string{"shared-dp", "only-a"},
	}))
	assert.Equal(t, []string{"only-a", "shared-dp"}, s.listPluginDriftSubsystems())

	// Second live plugin claiming an already-owned subsystem is refused at Stage 1
	// and leaves zero residual drift or registry state.
	sinkB := &engineStartupSink{s: s, proc: procB, startup: t.Context()}
	err := sinkB.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show dp-owner-b status", ShortHelp: "test"}},
		DriftSubsystems: []string{"only-b", "shared-dp"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared-dp")
	assert.Contains(t, err.Error(), "dp-owner-a")
	assert.Equal(t, []string{"only-a", "shared-dp"}, s.listPluginDriftSubsystems())
	assert.Empty(t, s.registry.LookupCommand("show dp-owner-b status"))

	// A plugin whose Stage 1 fails before drift registration (e.g. invalid family)
	// must never leave drift subsystems behind.
	procFail := process.NewProcess(plugin.PluginConfig{Name: "dp-fail-family", Internal: true})
	markProcessRunning(t, procFail)
	require.NoError(t, pm.AddProcess(procFail.Name(), procFail))
	sinkFail := &engineStartupSink{s: s, proc: procFail, startup: t.Context()}
	err = sinkFail.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		DriftSubsystems: []string{"never-registered"},
		Families:        []rpc.FamilyDecl{{Name: "invalid-family-name", Mode: "both", AFI: 1, SAFI: 204}},
	})
	require.Error(t, err)
	assert.NotContains(t, s.listPluginDriftSubsystems(), "never-registered")

	// After releasing the first plugin's registrations, a replacement can claim
	// the subsystem cleanly.
	procA.RunRuntimeCleanupOnce(func() {})
	s.releasePluginRegistrations(procA)
	assert.Empty(t, s.listPluginDriftSubsystems())

	require.NoError(t, sinkB.onRegistration(&rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Commands:        []rpc.CommandDecl{{Name: "show dp-owner-b status", ShortHelp: "test"}},
		DriftSubsystems: []string{"only-b", "shared-dp"},
	}))
	assert.Equal(t, []string{"only-b", "shared-dp"}, s.listPluginDriftSubsystems())
	assert.Equal(t, "dp-owner-b", s.registry.LookupCommand("show dp-owner-b status"))
}
