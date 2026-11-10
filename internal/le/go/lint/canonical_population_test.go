// Design: docs/contributing/testing.md -- a selected source symlink does not hide an excluded compiler input.
package golint

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// TestLintCanonicalInputPopulation compiles a real selected Go symlink whose
// target is preserved behind a nested-module boundary. Both an excluded target
// and a later target-resolution failure must refuse rather than certify only
// the safe-looking lexical source path.
func TestLintCanonicalInputPopulation(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		name := "excluded-canonical-target"
		if dangling {
			name = "canonical-resolution-error"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
			writeLintFile(t, root, "live/live.go", "package live\nconst Value = HiddenValue\n")
			writeLintFile(t, root, "backups/m/go.mod", "module preservedbackup\n\ngo 1.27.0\n")
			const target = "backups/m/source.go"
			writeLintFile(t, root, target, "package live\nconst HiddenValue = 42\n")
			linked := filepath.Join(root, "live", "linked.go")
			if err := os.Symlink(filepath.Join(root, filepath.FromSlash(target)), linked); err != nil {
				t.Fatal(err)
			}
			runner := &Runner{
				ctx: t.Context(), root: root, toolchain: fixtureChain(root),
				ops: runnerOps{capture: captureCommand},
			}
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./live")
			cmd.Dir = root
			cmd.Env = runner.toolchain.Environment(gotoolchain.EnvOptions{Test: true})
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("real compiler did not read owned symlink target: %v\n%s", err, output)
			}
			if dangling {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(target))); err != nil {
					t.Fatal(err)
				}
			}
			_, files, _, err := runner.goList(Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, []string{"./live"})
			if err == nil {
				t.Fatalf("lint admitted unresolved or excluded canonical compiler input %s (lexical files=%v)", target, files)
			}
			if dangling {
				if !strings.Contains(err.Error(), "linked.go") {
					t.Fatalf("resolution refusal does not identify selected symlink: %v", err)
				}
				if !strings.Contains(err.Error(), "no such file") {
					t.Fatalf("resolution refusal does not preserve filesystem cause: %v", err)
				}
				return
			}
			if !strings.Contains(err.Error(), target) {
				t.Fatalf("refusal does not identify excluded canonical target: %v", err)
			}
		})
	}
}
