//go:build linux

package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/cmd/ze/install"
)

type recoveringNativeService struct {
	*fakeNativeService
	running   bool
	failStart bool
}

func (f *recoveringNativeService) Stop(binary, dir string) (install.StoppedService, error) {
	previous := f.running
	f.running = false
	if _, err := f.fakeNativeService.Stop(binary, dir); err != nil {
		return nil, err
	}
	return recoveringStoppedService{service: f, previous: previous}, nil
}

type recoveringStoppedService struct {
	service  *recoveringNativeService
	previous bool
}

func (stopped recoveringStoppedService) Install() int { return stopped.service.Install("", "") }
func (stopped recoveringStoppedService) Restore() error {
	stopped.service.running = stopped.previous
	return nil
}

func (f *recoveringNativeService) Install(string, string) int {
	if f.failStart {
		return 1
	}
	f.running = true
	return 0
}

func TestNativeCopyFailureRestoresPreviouslyRunningService(t *testing.T) {
	rt, fake, prefix, _, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("initial install=%d: %s", code, output)
	}
	service := &recoveringNativeService{fakeNativeService: fake, running: true}
	rt.service = service
	rt.source = filepath.Join(t.TempDir(), "missing-source")
	if code := rt.install([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("failed replacement succeeded")
	}
	if !service.running {
		t.Fatal("failed replacement left the prior daemon stopped")
	}
}

func TestNativeStartFailureRestoresPreviousExecutable(t *testing.T) {
	rt, fake, prefix, _, output := nativeFixture(t)
	if code := rt.install([]string{"--prefix", prefix}); code != 0 {
		t.Fatalf("initial install=%d: %s", code, output)
	}
	binary := filepath.Join(prefix, "bin", "ze")
	if err := os.WriteFile(binary, []byte("known-working-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt.service = &recoveringNativeService{fakeNativeService: fake, running: true, failStart: true}
	if code := rt.install([]string{"--prefix", prefix}); code == 0 {
		t.Fatal("failed service start succeeded")
	}
	data, err := os.ReadFile(binary)
	if err != nil || string(data) != "known-working-binary" {
		t.Fatalf("previous binary not restored: %q %v", data, err)
	}
}
