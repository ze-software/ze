// Design: docs/architecture/core-design.md -- native unit test child environment.
package testunit

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
)

// TestAllRaceChildOverridesLauncherCGO executes a helper child with the exact
// environment allRunner supplies, proving inherited CGO0 cannot disable race.
func TestAllRaceChildOverridesLauncherCGO(t *testing.T) {
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("ZE_REPO_ROOT", "/launcher-root")
	t.Setenv("ze.repo.root", "/launcher-root-canonical")
	t.Setenv("ZE_LE_BUILD_NAME", "le")
	t.Setenv("ze.le.build.name", "le")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	chain := fixtureToolchain()
	chain.Root = t.TempDir()
	raceChildren := 0
	run := func(name string, argv []string, root string, environment []string) (gaterun.ActionReport, int) {
		if !slices.Contains(argv, "-race") {
			return gaterun.ActionReport{Action: name, Command: argv}, 0
		}
		raceChildren++
		child := exec.CommandContext(context.Background(), binary,
			"-test.run=^TestUnitChildEnvironmentProbe$", "--", "unit-child-environment")
		child.Dir = root
		child.Env = environment
		if output, err := child.CombinedOutput(); err != nil {
			t.Fatalf("native race child's environment failed: %v\n%s", err, output)
		}
		return gaterun.ActionReport{Action: name, Command: argv}, 0
	}
	if _, code := allRunner(chain, run)(); code != 0 {
		t.Fatalf("all returned %d", code)
	}
	if raceChildren != 1 {
		t.Fatalf("executed %d race children, want the whole-checkout child", raceChildren)
	}
}

// TestUnitChildEnvironmentProbe reads the effective process environment in the
// child above. An ordinary suite invocation does not enter its helper branch.
func TestUnitChildEnvironmentProbe(t *testing.T) {
	if !slices.Contains(os.Args, "unit-child-environment") {
		return
	}
	if got := os.Getenv("CGO_ENABLED"); got != "1" {
		t.Fatalf("CGO_ENABLED=%q, want 1 for the race command", got)
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ReplaceAll(strings.ToLower(key), "_", ".")
		switch key {
		case "ze.repo.root", "ze.le.build.name":
			t.Fatalf("launcher-only setting survived into the test: %s", key)
		}
	}
}
