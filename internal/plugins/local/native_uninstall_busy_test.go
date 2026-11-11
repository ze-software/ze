//go:build linux

package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

type policyNativeService struct {
	*recoveringNativeService
	unitPresent bool
	enabled     bool
}

func (service *policyNativeService) Uninstall(binary, dir string) int {
	service.unitPresent = false
	service.enabled = false
	return service.fakeNativeService.Uninstall(binary, dir)
}

func TestNativeUninstallBusyStorePreservesServicePolicy(t *testing.T) {
	for _, wasRunning := range []bool{false, true} {
		rt, fake, prefix, _, output := nativeFixture(t)
		binary := filepath.Join(prefix, "bin", "ze")
		configDir := filepath.Join(prefix, "etc", "ze")
		if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binary, []byte("prior-binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			t.Fatal(err)
		}
		owner, err := storage.Create(configDir)
		if err != nil {
			t.Fatal(err)
		}
		service := &policyNativeService{recoveringNativeService: &recoveringNativeService{fakeNativeService: fake, running: wasRunning}, unitPresent: true, enabled: true}
		rt.service = service
		if code := rt.uninstall([]string{"--prefix", prefix, "--yes"}); code == 0 {
			t.Fatalf("busy removal succeeded: %s", output)
		}
		if !service.unitPresent || !service.enabled {
			t.Fatalf("busy removal changed service policy (prior active=%v)", wasRunning)
		}
		if service.running != wasRunning {
			t.Fatalf("busy refusal changed active state: %v to %v", wasRunning, service.running)
		}
		data, err := os.ReadFile(binary)
		if err != nil || string(data) != "prior-binary" {
			t.Fatalf("busy refusal changed binary: %q %v", data, err)
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
