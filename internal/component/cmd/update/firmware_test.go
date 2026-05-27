// Design: plan/spec-unified-update-backend.md -- firmware backend dispatch tests

package update

import (
	"strings"
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
	if resp.Error == "" {
		t.Fatal("expected non-empty Error field")
	}
	if !strings.Contains(resp.Error, "unsupported") {
		t.Fatalf("Error = %q, want contains 'unsupported'", resp.Error)
	}
}
