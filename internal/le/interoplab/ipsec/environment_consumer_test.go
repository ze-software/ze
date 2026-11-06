// Design: docs/architecture/testing/interop.md -- IPsec peer environment.

package ipsec

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestPreparedIPsecPeerEnablesDebugLogging passes the prepared container's
// environment to the actual logger consumer. An unregistered lookalike such
// as ZE_LOG_LEVEL cannot enable the debug records the lab needs.
func TestPreparedIPsecPeerEnablesDebugLogging(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "scenario")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "ze.conf"), []byte("ike {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareScenario(root, interoplab.ScenarioSource{Name: "logger-environment", Directory: directory}, &scenarioState{}, "ze-logger", "swan-logger", "frr-logger", "nat-logger")
	if prepared.Cleanup != nil {
		t.Cleanup(func() {
			if err := prepared.Cleanup(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	// An inherited debug setting must not hide the misspelled lab key.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		canonical := strings.ToLower(strings.ReplaceAll(name, ".", "_"))
		if canonical == "ze_log" || strings.HasPrefix(canonical, "ze_log_") {
			t.Setenv(name, "")
		}
	}
	t.Cleanup(env.ResetCache)
	for _, peer := range prepared.Peers {
		if peer.Name != zePeer {
			continue
		}
		for _, variable := range peer.Environment {
			t.Setenv(variable.Name, variable.Value)
		}
		env.ResetCache()
		if !slogutil.Logger("ike").Enabled(t.Context(), slog.LevelDebug) {
			t.Fatalf("prepared Ze peer environment does not enable IKE debug logging: %v", peer.Environment)
		}
		return
	}
	t.Fatal("prepared IPsec scenario has no Ze peer")
}
