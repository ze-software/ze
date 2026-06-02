//go:build ze_stripped

// Design: plan/spec-unified-update-backend.md -- stripped firmware command dispatch tests

package update

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestStrippedFirmwareHandlersReachBackend(t *testing.T) {
	// VALIDATES: ze_stripped keeps update system firmware RPC handlers wired to the stripped backend.
	// PREVENTS: manual firmware commands disappearing instead of returning the stripped unsupported result.
	backend, err := system.NewBackend(host.PlatformSystemd, system.UpdateCheckConfig{}, system.BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	system.SetActiveBackend(backend)
	t.Cleanup(func() { system.SetActiveBackend(nil) })

	tests := []struct {
		name    string
		handler func() (*plugin.Response, error)
	}{
		{name: "check", handler: func() (*plugin.Response, error) { return handleFirmwareCheck(nil, nil) }},
		{name: "download", handler: func() (*plugin.Response, error) { return handleFirmwareDownload(nil, nil) }},
		{name: "apply", handler: func() (*plugin.Response, error) { return handleFirmwareApply(nil, nil) }},
		{name: "restart", handler: func() (*plugin.Response, error) { return handleFirmwareRestart(nil, nil) }},
		{name: "rollback", handler: func() (*plugin.Response, error) { return handleFirmwareRollback(nil, nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.handler()
			if err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if resp.Status != plugin.StatusError {
				t.Fatalf("Status = %q, want %q", resp.Status, plugin.StatusError)
			}
			if !strings.Contains(resp.Error, "self-update unavailable in ze-stripped") {
				t.Fatalf("Error = %q, want stripped unsupported message", resp.Error)
			}
		})
	}
}
