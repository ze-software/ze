// Design: plan/spec-unified-update-backend.md -- show update backend field test

package show

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
)

func TestShowSystemUpdateBackendField(t *testing.T) {
	// VALIDATES: AC-4 show system update output includes backend identity.
	// PREVENTS: CLI/API consumers having no stable way to distinguish update backends.
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
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data = %T, want map[string]any", resp.Data)
	}
	if got := data["backend"]; got != string(system.BackendZeSelfUpdate) {
		t.Fatalf("backend = %v, want %q", got, system.BackendZeSelfUpdate)
	}
}
