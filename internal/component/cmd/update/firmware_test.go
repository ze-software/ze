// Design: plan/spec-unified-update-backend.md -- firmware backend dispatch tests

package update

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestFirmwareCheckGokrazyUnsupported(t *testing.T) {
	// VALIDATES: AC-7 update system firmware check on gokrazy returns structured unsupported response.
	// PREVENTS: gokrazy firmware commands returning the old update-checker-not-configured string.
	backend, err := system.NewBackend(host.PlatformGokrazy, system.UpdateCheckConfig{}, system.BackendOptions{})
	if err != nil {
		t.Fatalf("NewUpdateBackend() error = %v", err)
	}
	system.SetActiveBackend(backend)
	t.Cleanup(func() { system.SetActiveBackend(nil) })

	resp, err := handleFirmwareCheck(nil, nil)
	if err != nil {
		t.Fatalf("handleFirmwareCheck() error = %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("Status = %q, want %q", resp.Status, plugin.StatusError)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data = %T, want map[string]any", resp.Data)
	}
	if got := data["backend"]; got != string(system.BackendGokrazyAB) {
		t.Fatalf("backend = %v, want %q", got, system.BackendGokrazyAB)
	}
	if got := data["status"]; got != "unsupported" {
		t.Fatalf("status = %v, want unsupported", got)
	}
	if got := data["message"]; got != "updates managed by gokrazy" {
		t.Fatalf("message = %v, want updates managed by gokrazy", got)
	}
}
