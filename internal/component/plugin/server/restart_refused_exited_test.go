package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestRestartRefusalCanFollowPluginExit proves that ErrRestartRefused does not
// establish whether the old process still runs.
func TestRestartRefusalCanFollowPluginExit(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const name = "restart-refused-exited"
	registerCountedRestartPlugin(t, name, 0)
	s, _ := newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}); err != nil {
		t.Fatal(err)
	}
	pm := s.procManager.Load()
	if pm == nil {
		t.Fatal("startup did not install the process manager")
	}
	old := pm.GetProcess(name)
	if old == nil {
		t.Fatal("startup did not install the plugin process")
	}
	pm.RefuseRespawns()
	old.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := old.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if old.Running() {
		t.Fatal("joined plugin still reports running")
	}
	err := s.restartPlugin(name)
	if !errors.Is(err, process.ErrDaemonStopping) {
		t.Errorf("restart = %v, want daemon-stopping refusal", err)
	}
	if !errors.Is(err, transaction.ErrRestartRefused) {
		t.Errorf("restart = %v, want pre-stop refusal classification", err)
	}
	if pm.GetProcess(name) != old {
		t.Error("refused restart replaced the old process")
	}
	if old.Running() {
		t.Error("refused restart started the exited plugin")
	}
}
