// Design: ai/rules/plugins.md -- ze_web absent (compile-out) validation
//
//go:build !ze_web

package hub

// VALIDATES: without the ze_web build tag (e.g. ze-stripped), the web factory
// and web-only daemon hook are absent, so the web package can be dropped from
// the binary.
// PREVENTS: a regression where web leaks into a hardened build via an always-on
// import or ungated registration.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestBuildTag_Web_Absent(t *testing.T) {
	if registeredServiceName("web") {
		t.Fatal("non-ze_web build: web factory unexpectedly registered (not compiled out)")
	}
	if webBuildStandalone != nil {
		t.Fatal("non-ze_web build: web-only daemon hook unexpectedly installed")
	}
	if code := RunWebOnly(nil, "", false); code != 1 {
		t.Fatalf("RunWebOnly without ze_web exit = %d, want 1", code)
	}
}

func TestBuildTag_Web_AbsentRejectsWebConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
environment {
	web {
		enabled true;
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_web build unexpectedly accepted web config")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("web config rejection = %v, want unknown field", err)
	}
}

func TestBuildTag_Web_AbsentBinaryDropsWebSymbols(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-core")
	build := exec.CommandContext(t.Context(), "go", "build", "-tags", "ze_core", "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ze_core failed: %v\n%s", err, out)
	}

	out, err := exec.CommandContext(t.Context(), "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	needles := []string{
		"internal/component/web.",
		"internal/component/web/",
		"buildWebService",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_core binary retained web symbol %q matching %q", line, needle)
			}
		}
	}
}
