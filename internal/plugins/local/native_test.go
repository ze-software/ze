//go:build linux

package local

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/cmd/ze/install"
)

type fakeNativeService struct {
	calls        *[]string
	preflightErr error
	stopErr      error
	available    bool
}

func (f *fakeNativeService) Available() (bool, error) { return f.available, nil }
func (f *fakeNativeService) Preflight(string, string) error {
	*f.calls = append(*f.calls, "preflight")
	return f.preflightErr
}
func (f *fakeNativeService) Stop(string, string) (install.StoppedService, error) {
	*f.calls = append(*f.calls, "stop")
	if f.stopErr != nil {
		return nil, f.stopErr
	}
	return fakeStoppedService{service: f}, nil
}

type fakeStoppedService struct{ service *fakeNativeService }

func (stopped fakeStoppedService) Install() int { return stopped.service.Install("", "") }
func (stopped fakeStoppedService) Restore() error {
	*stopped.service.calls = append(*stopped.service.calls, "service-restore")
	return nil
}
func (f *fakeNativeService) Install(string, string) int {
	*f.calls = append(*f.calls, "service-install")
	return 0
}
func (f *fakeNativeService) Uninstall(string, string) int {
	*f.calls = append(*f.calls, "service-uninstall")
	if f.stopErr != nil {
		return 1
	}
	return 0
}

func nativeFixture(t *testing.T) (*nativeRuntime, *fakeNativeService, string, *[]string, *bytes.Buffer) {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "prefix")
	source := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(source, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	calls := new([]string)
	service := &fakeNativeService{calls: calls, available: true}
	rt := &nativeRuntime{
		source: source,
		stdin:  strings.NewReader("y\n"), stdout: &output, stderr: &output,
		service: service,
		bootstrap: func(dir string) int {
			*calls = append(*calls, "bootstrap")
			if err := os.MkdirAll(filepath.Join(dir, "database"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "database", "credentials"), []byte("original-secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "database.lock"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return 0
		},
	}
	return rt, service, prefix, calls, &output
}

func TestNativeInstallBootstrapsThenStartsService(t *testing.T) {
	rt, _, prefix, calls, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("code=%d: %s", code, output)
	}
	want := []string{"preflight", "stop", "bootstrap", "service-install"}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls=%v want=%v", *calls, want)
	}
	data, err := os.ReadFile(filepath.Join(prefix, "bin", "ze"))
	if err != nil || string(data) != "new-binary" {
		t.Fatalf("binary=%q: %v", data, err)
	}
}

// Design: docs/guide/ze-install.md -- a custom prefix starts under the service account.
// The service's ownership transfer covers the private config leaf, not its
// prefix ancestors. Those new ancestors must therefore permit traversal while
// the configuration directory and credential data remain private.
func TestNativeInstallCreatesTraversableConfigAncestors(t *testing.T) {
	rt, _, prefix, _, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("install=%d: %s", code, output)
	}
	for _, path := range []string{prefix, filepath.Join(prefix, "etc")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o001 == 0 {
			t.Errorf("new config ancestor %s has mode %04o: the service account cannot traverse it", path, info.Mode().Perm())
		}
	}
	configDir := filepath.Join(prefix, "etc", "ze")
	for path, want := range map[string]os.FileMode{
		configDir:                            0o700,
		filepath.Join(configDir, "database"): 0o700,
		filepath.Join(configDir, "database", "credentials"): 0o600,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("private state %s has mode %04o, want %04o", path, info.Mode().Perm(), want)
		}
	}
	data, err := os.ReadFile(filepath.Join(configDir, "database", "credentials"))
	if err != nil || string(data) != "original-secret" {
		t.Fatalf("credentials=%q: %v", data, err)
	}
}

// Existing directory policy belongs to its owner. Foreground installation
// must not widen it merely because it creates a private config leaf beneath it.
func TestNativeInstallPreservesExistingConfigParentPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o700, 0o750, 0o755} {
		t.Run(mode.String(), func(t *testing.T) {
			rt, _, prefix, _, output := nativeFixture(t)
			parent := filepath.Join(prefix, "etc")
			if err := os.MkdirAll(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, mode); err != nil {
				t.Fatal(err)
			}
			if code := rt.install([]string{"--prefix", prefix, "--no-service"}); code != 0 {
				t.Fatalf("foreground install=%d: %s", code, output)
			}
			info, err := os.Stat(parent)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Errorf("existing parent mode=%04o, want preserved %04o", info.Mode().Perm(), mode)
			}
			leaf, err := os.Stat(filepath.Join(parent, "ze"))
			if err != nil {
				t.Fatal(err)
			}
			if leaf.Mode().Perm() != 0o700 {
				t.Errorf("private config leaf mode=%04o, want 0700", leaf.Mode().Perm())
			}
		})
	}
}

func TestNativeReinstallPreservesExistingCredentials(t *testing.T) {
	rt, _, prefix, calls, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("first install=%d: %s", code, output)
	}
	*calls = nil
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("reinstall=%d: %s", code, output)
	}
	if !reflect.DeepEqual(*calls, []string{"preflight", "stop", "service-install"}) {
		t.Fatalf("reinstall calls=%v", *calls)
	}
	data, err := os.ReadFile(filepath.Join(prefix, "etc", "ze", "database", "credentials"))
	if err != nil || string(data) != "original-secret" {
		t.Fatalf("credentials=%q: %v", data, err)
	}
}

func TestNativeConflictRefusesBeforeReplacingBinary(t *testing.T) {
	rt, service, prefix, calls, _ := nativeFixture(t)
	destination := filepath.Join(prefix, "bin", "ze")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	service.preflightErr = errors.New("unit belongs to another binary")
	if code := rt.install([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("conflicting install succeeded")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "old-binary" {
		t.Fatalf("old binary=%q: %v", data, err)
	}
	if !reflect.DeepEqual(*calls, []string{"preflight"}) {
		t.Fatalf("mutating calls=%v", *calls)
	}
}

func TestNativeUninstallStopsBeforeRemovingBinaryAndPreservesStore(t *testing.T) {
	rt, _, prefix, calls, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("install=%d: %s", code, output)
	}
	*calls = nil
	if code := rt.uninstall([]string{"--prefix", prefix, "--yes"}); code != 0 {
		t.Fatalf("uninstall=%d: %s", code, output)
	}
	if !reflect.DeepEqual(*calls, []string{"preflight", "stop", "service-uninstall"}) {
		t.Fatalf("uninstall calls=%v", *calls)
	}
	if _, err := os.Stat(filepath.Join(prefix, "bin", "ze")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("binary remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "etc", "ze", "database", "credentials")); err != nil {
		t.Fatalf("store removed: %v", err)
	}
}

func TestNativeUninstallStopFailurePreservesBinary(t *testing.T) {
	rt, service, prefix, _, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("install=%d: %s", code, output)
	}
	service.stopErr = errors.New("stop failed")
	if code := rt.uninstall([]string{"--prefix", prefix, "--yes"}); code == 0 {
		t.Fatal("failed stop succeeded")
	}
	if _, err := os.Stat(filepath.Join(prefix, "bin", "ze")); err != nil {
		t.Fatalf("binary removed after failed stop: %v", err)
	}
}

func TestNativeInstallWithoutSystemdPrintsForegroundCommand(t *testing.T) {
	rt, service, prefix, calls, output := nativeFixture(t)
	service.available = false
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("install=%d: %s", code, output)
	}
	if !reflect.DeepEqual(*calls, []string{"bootstrap"}) {
		t.Fatalf("unexpected service calls=%v", *calls)
	}
	if !strings.Contains(output.String(), filepath.Join(prefix, "bin", "ze")+" start") {
		t.Fatalf("no foreground command: %s", output)
	}
}

func TestNativeDryRunLeavesTargetAbsent(t *testing.T) {
	rt, _, prefix, calls, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix, "--dry-run"}); code != 0 {
		t.Fatalf("dry-run=%d: %s", code, output)
	}
	if _, err := os.Stat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created target: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run ran lifecycle: %v", *calls)
	}
}

func TestNativePackageMarkerRefusesBeforeReplacingBinary(t *testing.T) {
	rt, _, prefix, calls, _ := nativeFixture(t)
	marker := filepath.Join(prefix, "share", "ze", "install-method")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("deb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := rt.install([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("package-owned install succeeded")
	}
	if _, err := os.Stat(filepath.Join(prefix, "bin", "ze")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package conflict mutated binary: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("package conflict invoked lifecycle: %v", *calls)
	}
}

func TestNativeSymlinkConfigRefusesBeforeReplacingBinary(t *testing.T) {
	rt, _, prefix, calls, _ := nativeFixture(t)
	if err := os.MkdirAll(filepath.Join(prefix, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(prefix, "etc", "ze")); err != nil {
		t.Fatal(err)
	}
	if code := rt.install([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("symlink config admitted")
	}
	if len(*calls) != 0 {
		t.Fatalf("symlink conflict invoked lifecycle: %v", *calls)
	}
}
