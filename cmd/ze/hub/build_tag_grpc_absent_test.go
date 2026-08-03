// Design: ai/rules/plugins.md -- ze_grpc absent (compile-out) validation
//
//go:build !ze_grpc

package hub

// VALIDATES: without the ze_grpc build tag, the gRPC build seam stays nil, the
// grpc{} config schema is absent (rejected), and the binary contains no
// api/grpc symbols. The base api-server schema and the parent api package stay
// linked (REST and gNMI may still use them).
// PREVENTS: a regression where the gRPC server leaks into a build via an
// always-on import or an ungated registration/schema import.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_GRPC_Absent(t *testing.T) {
	if grpcBuild != nil {
		t.Fatal("non-ze_grpc build: gRPC seam unexpectedly installed (not compiled out)")
	}
}

func TestBuildTag_GRPC_AbsentRejectsGRPCConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
environment {
	api-server {
		grpc {
			enabled true;
		}
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_grpc build unexpectedly accepted grpc config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("grpc config rejection = %v, want clean unknown-field rejection", err)
	}
}

func TestBuildTag_GRPC_AbsentBinaryDropsGRPCSymbols(t *testing.T) {
	// test-relax: -short guard only; this test still runs in full (make ze-test
	// passes no -short). It builds the ze binary, so opt-in -short runs skip it.
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
	// The gRPC server subpackage + its YANG schema must vanish; the parent api
	// package and base api-server schema stay, so they are NOT checked here.
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "internal/component/api/grpc") {
			t.Fatalf("ze_core binary retained gRPC symbol %q", line)
		}
	}
}
