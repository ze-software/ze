package hub

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestHubNoBackendImports enforces the AAA registry pattern: the hub must
// not import any AAA backend (tacacs, radius, ldap, etc.) by name. Backends
// self-register via aaa.Default; the hub walks the registry.
//
// VALIDATES: spec-aaa-registry AC-2 -- cmd/ze/hub imports no backend directly.
// PREVENTS: regressions that couple the hub to a specific backend.
func TestHubNoBackendImports(t *testing.T) {
	// Anchor: fail loud if the test is not running from cmd/ze/hub/.
	// Without this, a future test-harness change that runs tests from a
	// different working directory would walk the wrong tree and pass vacuously.
	if _, err := os.Stat("infra_setup.go"); err != nil {
		t.Fatalf("test must run from cmd/ze/hub/ (infra_setup.go not found): %v", err)
	}

	// Forbid direct imports of backend packages. Keep the list in sync with
	// backends registered via internal/component/aaa/all/all.go.
	// Future backends (oidc, kerberos, saml, ...) must be appended here.
	forbidden := regexp.MustCompile(`"codeberg\.org/thomas-mangin/ze/internal/component/(tacacs|radius|ldap|oidc|kerberos|saml)`)
	err := filepath.Walk(".", func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if match := forbidden.FindString(string(data)); match != "" {
			t.Errorf("%s: hub imports backend %q -- must go through aaa.Default", path, match)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
