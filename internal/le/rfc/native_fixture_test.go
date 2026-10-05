// Design: docs/contributing/rfc-conformance-gates.md -- native observation isolation.

package rfc

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestNativeImplementationFixture runs a real child with the observation
// environment and poisoned launcher identities. Root must discover the child's
// checkout, while the toolchain's cache and process limit must still reach it.
func TestNativeImplementationFixture(t *testing.T) {
	const marker = "RFC_OBSERVATION_CHILD_ROOT"
	identities := []string{
		"ZE_REPO_ROOT", "ze.repo.root", "Ze.RePo_Root",
		"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name",
	}
	if root := os.Getenv(marker); root != "" {
		env.ResetCache()
		got, err := lepath.Root()
		if err != nil {
			t.Fatal(err)
		}
		if got != root {
			t.Fatalf("child checkout = %q, want %q", got, root)
		}
		for _, key := range identities {
			if value, held := os.LookupEnv(key); held {
				t.Errorf("child inherited launcher identity %s=%q", key, value)
			}
		}
		if got := os.Getenv("GOCACHE"); got != gotoolchain.GoCache(root) {
			t.Errorf("child cache = %q, want %q", got, gotoolchain.GoCache(root))
		}
		if got := os.Getenv("GOMAXPROCS"); got != "2" {
			t.Errorf("child process limit = %q, want 2", got)
		}
		return
	}

	root := checkoutRoot(t)
	for _, key := range identities {
		t.Setenv(key, filepath.Join(t.TempDir(), "launcher"))
	}
	t.Cleanup(env.ResetCache)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "wrong-cache"))
	t.Setenv("GOMAXPROCS", "7")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := observationRunner{
		toolchain: gotoolchain.Toolchain{Root: root, Procs: 2},
		carrier:   Carrier{Kind: kindUnit},
	}
	cmd := exec.CommandContext(t.Context(), binary, "-test.run=^TestNativeImplementationFixture$")
	cmd.Dir = root
	cmd.Env = append(runner.environment(""), marker+"="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native observation child: %v\n%s", err, output)
	}
}

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve checkout root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("checkout root %s has no go.mod: %v", root, err)
	}
	return root
}
