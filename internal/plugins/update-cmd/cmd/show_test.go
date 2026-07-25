package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/host"
)

func TestShowSystemUpdateBackendField(t *testing.T) {
	backend, err := system.NewBackend(host.PlatformPlainLinux, system.UpdateCheckConfig{}, system.BackendOptions{})
	if err != nil {
		t.Fatalf("NewUpdateBackend() error = %v", err)
	}
	system.SetActiveBackend(backend)
	t.Cleanup(func() { system.SetActiveBackend(nil) })

	resp, err := handleShowSystemUpdate(nil, nil)
	if err != nil {
		t.Fatalf("handleShowSystemUpdate() error = %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data = %T, want map[string]any", resp.Data)
	}
	if got := data["backend"]; got != string(system.BackendZeSelfUpdate) {
		t.Fatalf("backend = %v, want %q", got, system.BackendZeSelfUpdate)
	}
}
