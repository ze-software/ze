//go:build ze_stripped

package system

import (
	"context"
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/host"
)

func TestStrippedBackendDisablesZeSelfUpdateWithoutURL(t *testing.T) {
	backend, err := NewBackend(host.PlatformSystemd, UpdateCheckConfig{}, BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	if backend.Name() != BackendZeSelfUpdate {
		t.Fatalf("backend name = %q, want %q", backend.Name(), BackendZeSelfUpdate)
	}
	status, err := backend.Check(context.Background())
	if !errors.Is(err, ErrFirmwareUnsupported) {
		t.Fatalf("Check() error = %v, want ErrFirmwareUnsupported", err)
	}
	if status.StatusText != "unsupported in ze-stripped" {
		t.Fatalf("StatusText = %q, want unsupported in ze-stripped", status.StatusText)
	}
	if status.Message != "self-update unavailable in ze-stripped" {
		t.Fatalf("Message = %q, want stripped unsupported message", status.Message)
	}
	if status.LastError != "" {
		t.Fatalf("LastError = %q, want empty", status.LastError)
	}
	if status.DownloadStatus != "unsupported" {
		t.Fatalf("DownloadStatus = %q, want unsupported", status.DownloadStatus)
	}
}

func TestStrippedBackendRejectsInvalidURL(t *testing.T) {
	_, err := NewBackend(host.PlatformSystemd, UpdateCheckConfig{URL: "http://example.com/version.json"}, BackendOptions{})
	if err == nil {
		t.Fatal("NewBackend() accepted invalid stripped update URL")
	}
}

func TestStrippedBackendRejectsInvalidSelfUpdateConfig(t *testing.T) {
	cfg := UpdateCheckConfig{URL: "https://update.example.com/version.json"}
	cfg.SelfUpdate.RestartImmediate = true
	cfg.SelfUpdate.RestartTime = "03:00"
	_, err := NewBackend(host.PlatformSystemd, cfg, BackendOptions{})
	if err == nil {
		t.Fatal("NewBackend() accepted invalid stripped self-update config")
	}
}
