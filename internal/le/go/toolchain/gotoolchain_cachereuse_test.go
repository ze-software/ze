// Related: gotoolchain.go -- Environment and TrimpathGoFlags, the GOFLAGS every le Go command runs under
//
// VALIDATES: a package tested in one checkout path is answered `(cached)` when
// the same source is tested again from a second path that shares the Go
// cache, under the environment the verify gate's unit stages build
// (internal/le/test/unit, groups.go: EnvOptions{Test: true, Procs: true}).
// PREVENTS: every verify worktree compiling and testing the module cold, as
// the run of 2026-10-18 did (0 of 657 `ok` lines cached), because Go hashes
// the package directory into each action ID unless -trimpath is set.

package gotoolchain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// warmModule is a module with one package and one test that reads no file,
// so the test result cache keys on the build and nothing else.
var warmModule = map[string]string{
	"go.mod":            "module example.com/warm\n\ngo 1.22\n",
	"warm/warm.go":      "// Package warm is a cache-reuse fixture.\npackage warm\n\n// Double answers twice n.\nfunc Double(n int) int { return n * 2 }\n",
	"warm/warm_test.go": "package warm\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n",
}

// TestASecondCheckoutPathReusesTheFirstOnesTestResult proves the mechanism
// AC-16 of spec-verify-gate-warm-timed-first-red depends on, deterministically.
// Two full verify runs cannot prove it: other sessions load the host, so a
// time comparison is noise, and the owner empties the build caches for disk
// space, so a second warm run cannot be relied on (owner, 2026-10-21).
//
// Method: write the same module to two checkout roots under t.TempDir, each
// with a cache link to one private shared directory, as a verify worktree
// links the checkout's cache (sharedCacheLink). Run `go test ./warm` in the
// first root, then the second, each under Toolchain.Environment for its own
// root, and require the second output to say `(cached)`. Without -trimpath
// in TrimpathGoFlags the second run compiles and tests again, and this fails.
func TestASecondCheckoutPathReusesTheFirstOnesTestResult(t *testing.T) {
	if _, err := exec.LookPath(goProgram); err != nil {
		t.Skipf("no %s executable on PATH (%v): this test runs the real go command", goProgram, err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	if err := os.Unsetenv("GOFLAGS"); err != nil {
		t.Fatal(err)
	}
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)

	shared := t.TempDir()
	base := t.TempDir()
	first := writeWarmCheckout(t, filepath.Join(base, "first"), shared)
	second := writeWarmCheckout(t, filepath.Join(base, "second"), shared)

	if output := runWarmTest(t, first); strings.Contains(output, "(cached)") {
		t.Fatalf("the first run, over an empty cache, answered (cached):\n%s", output)
	}
	output := runWarmTest(t, second)
	if !strings.Contains(output, "(cached)") {
		t.Fatalf("the second checkout path did not reuse the first one's test result; "+
			"its compile and test action IDs differ, so a fresh verify worktree runs cold:\n%s", output)
	}
}

// writeWarmCheckout writes warmModule under root and links root/cache to
// shared, so GoCache(root) resolves into the one shared directory.
func writeWarmCheckout(t *testing.T, root, shared string) string {
	t.Helper()
	for name, content := range warmModule {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(shared, filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	return root
}

// runWarmTest runs `go test ./warm` in root under the environment the unit
// stages use, and answers its combined output.
func runWarmTest(t *testing.T, root string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, goProgram, "test", "./warm")
	cmd.Dir = root
	cmd.Env = Toolchain{Root: root, Procs: 1}.Environment(EnvOptions{Test: true, Procs: true})
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./warm in %s: %v\n%s", root, err, output)
	}
	return string(output)
}
