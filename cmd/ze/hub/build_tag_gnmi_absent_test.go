// Design: ai/rules/plugins.md -- ze_gnmi absent (compile-out) validation
//
//go:build !ze_gnmi

package hub

// VALIDATES: without the ze_gnmi build tag (e.g. ze-stripped), the gNMI seam
// stays nil, gNMI config schema is absent, the show command is absent, and the
// binary contains no gNMI symbols.
// PREVENTS: a regression where gNMI leaks into a hardened build via an
// always-on import, ungated schema import, or ungated RPC registration.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_GNMI_Absent(t *testing.T) {
	if gnmiBuild != nil || gnmiReloadNotify != nil {
		t.Fatal("non-ze_gnmi build: gNMI seam unexpectedly installed (gNMI not compiled out)")
	}
}

func TestBuildTag_GNMI_AbsentRejectsGNMIConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
environment {
	gnmi {
		enabled true;
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_gnmi build unexpectedly accepted gnmi config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("gnmi config rejection = %v, want clean unknown-field rejection", err)
	}
}

func TestBuildTag_GNMI_AbsentBinaryDropsGNMISymbolsAndCommand(t *testing.T) {
	// -short guard only; this test still runs in full (make ze-standard-test
	// passes no -short). It builds and spawns the ze binary, so opt-in -short
	// runs skip it for speed. No coverage is lost in the verify/CI suite.
	if testing.Short() {
		t.Skip("builds and runs the ze binary (slow); skipped under -short")
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
		"internal/component/gnmi.",
		"internal/component/gnmi/",
		"gnmiBuildImpl",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_core binary retained gNMI symbol %q matching %q", line, needle)
			}
		}
	}

	cmd := exec.Command(bin, "show", "gnmi")
	cmdOut, cmdErr := cmd.CombinedOutput()
	if cmdErr == nil {
		t.Fatalf("ze_core show gnmi unexpectedly succeeded: %s", cmdOut)
	}
	text := string(cmdOut)
	if !strings.Contains(text, "unknown") || strings.Contains(strings.ToLower(text), "panic") {
		t.Fatalf("ze_core show gnmi output = %q, want clean unknown-command error", text)
	}
}
