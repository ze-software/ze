//go:build ze_distro

// Design: plan/learned/909-unified-update-backend.md -- update backend tests

package system

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/host"
)

func testConfig() UpdateCheckConfig {
	return UpdateCheckConfig{URL: "http://localhost/version.json", Interval: 60}
}

func TestBackendSelectionGokrazy(t *testing.T) {
	// VALIDATES: AC-1 Platform is gokrazy, update-check config present selects backend gokrazy-ab.
	// PREVENTS: gokrazy appliances starting the Ze binary updater instead of reporting external management.
	backend, err := NewBackend(host.PlatformGokrazy, testConfig(), BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	if got := backend.Name(); got != BackendGokrazyAB {
		t.Fatalf("backend.Name() = %q, want %q", got, BackendGokrazyAB)
	}
}

func TestBackendSelectionSelfUpdate(t *testing.T) {
	// VALIDATES: AC-2 Platform is Linux with auto-apply config selects ze-self-update wrapping SelfUpdater.
	// PREVENTS: auto-apply config falling back to passive version checking only.
	cfg := testConfig()
	cfg.SelfUpdate.AutoApply = true

	backend, err := NewBackend(host.PlatformPlainLinux, cfg, BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	zb, ok := backend.(*zeBackend)
	if !ok {
		t.Fatalf("backend type = %T, want *zeBackend", backend)
	}
	if zb.updater == nil {
		t.Fatal("expected SelfUpdater wrapper")
	}
	if zb.checker != nil {
		t.Fatal("checker should be nil when self-updater is selected")
	}
}

func TestBackendSelectionChecker(t *testing.T) {
	// VALIDATES: AC-3 Platform is Linux without auto-apply selects ze-self-update wrapping UpdateChecker.
	// PREVENTS: passive update-check configs accidentally enabling binary replacement state.
	backend, err := NewBackend(host.PlatformSystemd, testConfig(), BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	zb, ok := backend.(*zeBackend)
	if !ok {
		t.Fatalf("backend type = %T, want *zeBackend", backend)
	}
	if zb.checker == nil {
		t.Fatal("expected UpdateChecker wrapper")
	}
	if zb.updater != nil {
		t.Fatal("updater should be nil when passive checker is selected")
	}
}

func TestBackendStatusIncludesName(t *testing.T) {
	// VALIDATES: AC-4 show/status data source carries backend identity ze-self-update.
	// PREVENTS: callers having to infer the backend from missing fields or implementation type.
	backend, err := NewBackend(host.PlatformPlainLinux, UpdateCheckConfig{}, BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	st := backend.Status()
	if st.Backend != BackendZeSelfUpdate {
		t.Fatalf("Status().Backend = %q, want %q", st.Backend, BackendZeSelfUpdate)
	}
}

func TestGokrazyBackendStatus(t *testing.T) {
	// VALIDATES: AC-5 show status on gokrazy returns managed-by-gokrazy status and explanatory message.
	// PREVENTS: gokrazy appliances reporting firmware updates as not configured.
	backend, err := NewBackend(host.PlatformGokrazy, UpdateCheckConfig{}, BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	st := backend.Status()
	if st.Backend != BackendGokrazyAB {
		t.Fatalf("Status().Backend = %q, want %q", st.Backend, BackendGokrazyAB)
	}
	if st.StatusText != "managed by gokrazy" {
		t.Fatalf("StatusText = %q, want managed by gokrazy", st.StatusText)
	}
	if st.Message == "" {
		t.Fatal("expected explanatory message")
	}
}

func TestGokrazyProbeReachable(t *testing.T) {
	// VALIDATES: AC-6 reachable gokrazy management endpoint reports reachability and features.
	// PREVENTS: a live gokrazy management socket being flattened to a generic managed status.
	socketPath := startProbeServer(t)
	backend, err := NewBackend(host.PlatformGokrazy, UpdateCheckConfig{}, BackendOptions{GokrazySocketPath: socketPath})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}

	st := backend.Status()
	if !st.GokrazyReachable {
		t.Fatal("expected gokrazy reachable")
	}
	if !hasFeature(st.GokrazyFeatures, "root") || !hasFeature(st.GokrazyFeatures, "serial-console") {
		t.Fatalf("features = %v, want root and serial-console", st.GokrazyFeatures)
	}
	if hasFeature(st.GokrazyFeatures, "disabled") {
		t.Fatalf("features = %v, disabled feature should not be reported", st.GokrazyFeatures)
	}
}

func TestGokrazyProbeUnreachable(t *testing.T) {
	// VALIDATES: AC-6 unreachable gokrazy management endpoint reports unreachable without leaking socket paths.
	// PREVENTS: operator output exposing local runtime paths from probe errors.
	socketPath := filepath.Join(t.TempDir(), "missing.sock")
	backend, err := NewBackend(host.PlatformGokrazy, UpdateCheckConfig{}, BackendOptions{GokrazySocketPath: socketPath})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}

	st := backend.Status()
	if st.GokrazyReachable {
		t.Fatal("expected gokrazy unreachable")
	}
	if st.LastError == "" {
		t.Fatal("expected sanitized last error")
	}
	if st.LastError == socketPath {
		t.Fatal("last error leaked socket path")
	}
}

func TestGokrazyFirmwareUnsupported(t *testing.T) {
	// VALIDATES: AC-7 and AC-8 firmware operations on gokrazy return structured unsupported results.
	// PREVENTS: gokrazy firmware commands falling through to the old not-configured string.
	backend, err := NewBackend(host.PlatformGokrazy, UpdateCheckConfig{}, BackendOptions{})
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	checks := []struct {
		name string
		fn   func(context.Context) (FirmwareResult, error)
	}{
		{name: "download", fn: backend.Download},
		{name: "apply", fn: backend.Apply},
	}
	for _, tt := range checks {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.fn(context.Background())
			if !errors.Is(err, ErrFirmwareUnsupported) {
				t.Fatalf("error = %v, want ErrFirmwareUnsupported", err)
			}
			assertUnsupported(t, res)
		})
	}

	if res, err := backend.Check(context.Background()); !errors.Is(err, ErrFirmwareUnsupported) {
		t.Fatalf("check error = %v, want ErrFirmwareUnsupported", err)
	} else if res.Backend != BackendGokrazyAB || res.StatusText != "managed by gokrazy" {
		t.Fatalf("check result = %+v, want gokrazy managed status", res)
	}

	if res, err := backend.Restart(); !errors.Is(err, ErrFirmwareUnsupported) {
		t.Fatalf("restart error = %v, want ErrFirmwareUnsupported", err)
	} else {
		assertUnsupported(t, res)
	}

	if res, err := backend.Rollback(); !errors.Is(err, ErrFirmwareUnsupported) {
		t.Fatalf("rollback error = %v, want ErrFirmwareUnsupported", err)
	} else {
		assertUnsupported(t, res)
	}
}

// probeSocketDir returns a directory short enough to hold a Unix socket path.
//
// t.TempDir() embeds the test name, and sun_path is 104 bytes including the NUL
// on darwin (108 on linux). The names here happen to still fit, so this is a
// latent trap rather than a live failure: renaming a caller to something longer
// makes bind() fail with EINVAL ("invalid argument") before any assertion runs.
// The identical construction in internal/component/gokrazy DID overflow.
// Keep the prefix short so the length of a test name can never matter.
func probeSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gk")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func startProbeServer(t *testing.T) string {
	t.Helper()
	socketPath := filepath.Join(probeSocketDir(t), "gokrazy.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/update/features":
			_ = json.NewEncoder(w).Encode(map[string]any{"root": true, "serial-console": true, "disabled": false})
		default:
			http.NotFound(w, r)
		}
	})}

	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Shutdown(context.Background())
		_ = ln.Close()
		_ = os.Remove(socketPath)
	})
	return socketPath
}

func hasFeature(features []string, want string) bool {
	return slices.Contains(features, want)
}

func assertUnsupported(t *testing.T, res FirmwareResult) {
	t.Helper()
	if res.Backend != BackendGokrazyAB {
		t.Fatalf("Backend = %q, want %q", res.Backend, BackendGokrazyAB)
	}
	if res.Status != "unsupported" {
		t.Fatalf("Status = %q, want unsupported", res.Status)
	}
	if res.Message != "updates managed by gokrazy" {
		t.Fatalf("Message = %q, want updates managed by gokrazy", res.Message)
	}
}
