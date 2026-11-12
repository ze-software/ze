package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRefusedReloadRaisesEvent proves that a reload the server refuses
// reaches the report bus under doctor-config-reload-refused, with the reload
// as its subject, the refusal in its message and the fix the refusal names,
// so the recent-events doctor check reports it and not only the log. Method:
// drive a real refused reload through ReloadConfig, then read the bus.
//
// VALIDATES: AC-40 of spec-doctor-coverage-12-plugin-infra (R-7).
// PREVENTS: a refused reload whose only record is a log line `ze doctor`
// never reads.
func TestRefusedReloadRaisesEvent(t *testing.T) {
	t.Run("transaction-refused", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
		newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
		reactor := &mockReloadReactor{tree: oldTree}
		plugins := []pluginDef{{
			name:       "rib",
			roots:      []string{"bgp"},
			verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "invalid router-id"},
		}}
		s := newTestReloadServer(t, reactor, plugins)

		// Red: the raiseReloadRefused call after "config reload: transaction
		// failed" in reloadConfig.
		if err := s.ReloadConfig(context.Background(), newTree); err == nil {
			t.Fatal("a reload a plugin's verify refused answered no error")
		}
		event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused)
		if event == nil {
			t.Fatalf("no reload-refused event on the bus: %v", report.Errors(0))
		}
		if event.Subject != configReloadSubject {
			t.Errorf("subject %q, want %q", event.Subject, configReloadSubject)
		}
		if !strings.Contains(event.Message, "invalid router-id") {
			t.Errorf("message %q does not carry the plugin's refusal", event.Message)
		}
		if !strings.Contains(event.Message, "keeps the configuration it was running") {
			t.Errorf("message %q does not say what ze kept", event.Message)
		}
		// The refusal came as text over the plugin connection, so it names no
		// fix of its own.
		if event.Fix != fixShowErrors {
			t.Errorf("fix %q, want %q", event.Fix, fixShowErrors)
		}
	})

	t.Run("auto-load-dependency-refused", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		if err := registry.Register(registry.Registration{
			Name:         "auto-load-owner",
			Description:  "test auto-load owner",
			ConfigRoots:  []string{"autopath"},
			Dependencies: []string{"missing-dep"},
			RunEngine:    func(net.Conn) int { return 0 },
			CLIHandler:   func([]string) int { return 0 },
		}); err != nil {
			t.Fatalf("register the auto-load owner: %v", err)
		}
		reactor := &mockReloadReactor{tree: map[string]any{}}
		s := newTestReloadServer(t, reactor, nil)
		newTree := map[string]any{"autopath": map[string]any{"enabled": "true"}}

		// Red: the raiseReloadRefused call after "config reload: dependency
		// resolution failed" in selectAutoLoad.
		err := s.ReloadConfig(context.Background(), newTree)
		if err == nil {
			t.Fatal("a reload whose plugin has a missing dependency answered no error")
		}
		event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused)
		if event == nil {
			t.Fatalf("no reload-refused event on the bus: %v", report.Errors(0))
		}
		if event.Subject != configReloadSubject {
			t.Errorf("subject %q, want %q", event.Subject, configReloadSubject)
		}
		if !strings.Contains(event.Message, "auto-load-owner") {
			t.Errorf("message %q does not name the plugin the configuration needs", event.Message)
		}
		if !strings.Contains(event.Message, "missing-dep") {
			t.Errorf("message %q does not carry the refusal", event.Message)
		}
		// The registry's refusal is an operator error, so the event carries
		// its fix, not the generic one. Red: reloadRefusalFix answering
		// fixShowErrors whatever the chain holds.
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("the dependency refusal carries no operator error: %v", err)
		}
		if event.Fix != refusal.Fix() {
			t.Errorf("fix %q, want the refusal's own fix %q", event.Fix, refusal.Fix())
		}
	})
}

// startupRefusingReactor is a mock reactor whose SignalPluginStartupComplete
// answers refusal, the reactor refusing the configuration once the plugins a
// reload auto-loaded have started (startAutoLoaded). Every other method is
// mockReactor's.
type startupRefusingReactor struct {
	*mockReactor
	refusal error
}

func (r *startupRefusingReactor) SignalPluginStartupComplete() error { return r.refusal }

// registerAutoLoadOwner registers name as the plugin that owns the config root
// root, run by run, in a registry the caller has already snapshotted.
func registerAutoLoadOwner(t *testing.T, name, root string, run func(net.Conn) int) {
	t.Helper()
	if err := registry.Register(registry.Registration{
		Name:        name,
		Description: "test auto-load owner",
		ConfigRoots: []string{root},
		RunEngine:   run,
		CLIHandler:  func([]string) int { return 0 },
	}); err != nil {
		t.Fatalf("register the auto-load owner %s: %v", name, err)
	}
}

// requireReloadRefusedEvent fails t when the bus holds no
// doctor-config-reload-refused event, when its subject is not the reload, or
// when its message lacks one of the fragments in want.
func requireReloadRefusedEvent(t *testing.T, want ...string) {
	t.Helper()
	event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused)
	if event == nil {
		t.Fatalf("no reload-refused event on the bus: %v", report.Errors(0))
	}
	if event.Subject != configReloadSubject {
		t.Errorf("subject %q, want %q", event.Subject, configReloadSubject)
	}
	for _, fragment := range want {
		if !strings.Contains(event.Message, fragment) {
			t.Errorf("message %q does not say %q", event.Message, fragment)
		}
	}
}

// TestRefusedAutoLoadStartRaisesEvent proves that the two refusals a reload's
// auto-load meets after it has chosen its plugins reach the report bus under
// doctor-config-reload-refused: an auto-loaded plugin that does not start, and
// the reactor refusing the configuration once the plugins have started. Each
// event names the plugin, says what refused, and says ze keeps the
// configuration it was running. Method: drive autoLoadForNewConfigPaths, the
// reload's auto-load step, over a registered plugin owning a new config root,
// then read the bus.
//
// VALIDATES: AC-40 of spec-doctor-coverage-12-plugin-infra (R-7), the two
// startAutoLoaded raises TestRefusedReloadRaisesEvent does not reach.
// PREVENTS: either raise deleted from startAutoLoaded with every test green,
// leaving that refused reload a log line `ze doctor` never reads.
func TestRefusedAutoLoadStartRaisesEvent(t *testing.T) {
	t.Run("plugin-did-not-start", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		const name = "auto-load-start-fails"
		const root = "autoloadstartfails"
		// The engine ends before Stage 1, so the plugin never starts.
		registerAutoLoadOwner(t, name, root, func(net.Conn) int { return 1 })
		s, _ := newLifecycleStartupServer(t)

		// Red: the raiseReloadRefused call after "config reload: auto-load
		// plugin startup failed" in startAutoLoaded.
		if _, err := s.autoLoadForNewConfigPaths(context.Background(),
			map[string]any{root: map[string]any{}}, []string{root}); err == nil {
			t.Fatal("an auto-load whose plugin never started answered no error")
		}
		requireReloadRefusedEvent(t, name, "did not all start",
			"keeps the configuration it was running")
	})

	// A recovery start restores the configuration ze was running, so its
	// failure leaves that restore incomplete, and the event must not claim
	// the configuration ze kept.
	t.Run("recovery-plugin-did-not-start", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		const name = "auto-load-recovery-fails"
		const root = "autoloadrecoveryfails"
		registerAutoLoadOwner(t, name, root, func(net.Conn) int { return 1 })
		s, _ := newLifecycleStartupServer(t)
		recovery := context.WithValue(context.Background(), removalRecoveryContextKey{}, true)

		// Red: autoLoadRefusalProblem answering the reload's own text
		// whatever recovery says.
		if _, err := s.autoLoadForNewConfigPaths(recovery,
			map[string]any{root: map[string]any{}}, []string{root}); err == nil {
			t.Fatal("a recovery auto-load whose plugin never started answered no error")
		}
		requireReloadRefusedEvent(t, name, "to restore the configuration it was running",
			"the restore did not complete")
		if event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused); event != nil &&
			strings.Contains(event.Message, "keeps the configuration it was running") {
			t.Errorf("message %q claims ze kept its configuration while the restore failed", event.Message)
		}
	})

	t.Run("reactor-refused-after-start", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		const name = "auto-load-reactor-refuses"
		const root = "autoloadreactorrefuses"
		registerAutoLoadOwner(t, name, root, func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		})
		s, _ := newLifecycleStartupServer(t)
		// Set before the server runs anything, so no goroutine reads the field.
		s.reactor = &startupRefusingReactor{
			mockReactor: &mockReactor{},
			refusal:     errors.New("the reactor refused the filter insertions"),
		}

		// Red: the raiseReloadRefused call after "config reload: the reactor
		// refused the config after the auto-loaded plugins started" in
		// startAutoLoaded.
		if _, err := s.autoLoadForNewConfigPaths(context.Background(),
			map[string]any{root: map[string]any{}}, []string{root}); err == nil {
			t.Fatal("an auto-load the reactor refused answered no error")
		}
		requireReloadRefusedEvent(t, name, "ze then refused the configuration",
			"the reactor refused the filter insertions", "keeps the configuration it was running")
	})
}
