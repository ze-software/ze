// Design: ai/rules/plugins.md -- ze_rest absent (compile-out) validation
//
//go:build !ze_rest

package hub

// VALIDATES: without the ze_rest build tag, the REST build seam stays nil, the
// rest{} config schema is absent (rejected), and the binary contains no
// api/rest symbols. The base api-server schema and the parent api package stay
// linked (gRPC and gNMI may still use them).
// PREVENTS: a regression where the REST server leaks into a build via an
// always-on import or an ungated registration/schema import.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_REST_Absent(t *testing.T) {
	if restBuild != nil {
		t.Fatal("non-ze_rest build: REST seam unexpectedly installed (not compiled out)")
	}
}

func TestBuildTag_REST_AbsentRejectsRESTConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
environment {
	api-server {
		rest {
			enabled true;
		}
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_rest build unexpectedly accepted rest config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("rest config rejection = %v, want clean unknown-field rejection", err)
	}
}

func TestBuildTag_REST_AbsentBinaryDropsRESTSymbols(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-core")
	build := exec.Command("go", "build", "-tags", "ze_core", "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ze_core failed: %v\n%s", err, out)
	}

	out, err := exec.Command("go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	// The REST server subpackage + its YANG schema must vanish; the parent api
	// package (ConfigSessionManager) and base api-server schema stay, so they
	// are NOT checked here.
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "internal/component/api/rest") {
			t.Fatalf("ze_core binary retained REST symbol %q", line)
		}
	}
}
