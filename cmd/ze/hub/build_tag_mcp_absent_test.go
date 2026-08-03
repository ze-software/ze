// Design: ai/rules/plugins.md -- ze_mcp absent (compile-out) validation
//
//go:build !ze_mcp

package hub

// VALIDATES: without the ze_mcp build tag (e.g. ze-stripped or a hardened bare
// ze_core build), the MCP service factory is NOT registered, the mcp config
// schema is absent, and the binary contains no mcp symbols.
// PREVENTS: a regression where mcp leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_MCP_Absent(t *testing.T) {
	if registeredServiceName("mcp") {
		t.Fatal("non-ze_mcp build: mcp factory unexpectedly registered (not compiled out)")
	}
}

func TestBuildTag_MCP_AbsentRejectsMCPConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
environment {
	mcp {
		enabled true;
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_mcp build unexpectedly accepted mcp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("mcp config rejection = %v, want clean unknown-field rejection", err)
	}
}

func TestBuildTag_MCP_AbsentBinaryDropsMCPSymbols(t *testing.T) {
	// test-relax: -short guard only; this test still runs in full (make ze-test
	// passes no -short). It builds and spawns the ze binary, so opt-in -short
	// runs skip it for speed. No coverage is lost in the verify/CI suite.
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
	needles := []string{
		"internal/component/mcp.",
		"internal/component/mcp/",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_core binary retained mcp symbol %q matching %q", line, needle)
			}
		}
	}
}
