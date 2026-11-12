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
