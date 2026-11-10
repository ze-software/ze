// Design: docs/contributing/testing.md -- lint admission measures the packages lint actually loads.
package golint

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// TestLintInputPopulation uses the producer's real go-list entry on an owned
// module. An ordinary Go package in an excluded tree must refuse admission;
// a nested-module backup with deliberately invalid Go must remain outside it.
func TestLintInputPopulation(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "excluded-real-input"
		if nested {
			name = "preserved-noninput-backup"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
			writeLintFile(t, root, "live/live.go", "package live\nconst Value = 7\n")
			relative := "docs/probe/probe.go"
			body := "package probe\nconst Value = 9\n"
			if nested {
				relative = "backups/probe.go"
				writeLintFile(t, root, "backups/go.mod", "module preservedbackup\n\ngo 1.27.0\n")
				body = "preserved evidence, deliberately not a Go package\n"
			}
			writeLintFile(t, root, relative, body)
			runner := &Runner{
				ctx: t.Context(), root: root, toolchain: fixtureChain(root),
				ops: runnerOps{capture: captureCommand},
			}
			_, files, _, err := runner.goList(Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, []string{"./..."})
			if !nested {
				if err == nil {
					t.Fatalf("lint admitted excluded real input %s (selected=%v)", relative, files)
				}
				if !strings.Contains(err.Error(), relative) {
					t.Fatalf("refusal does not name excluded real input %s: %v", relative, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("preserved noninput backup stopped lint: %v", err)
			}
			if !files["live/live.go"] {
				t.Fatalf("native population omitted live input: %v", files)
			}
			if files[relative] {
				t.Fatalf("native population compiled preserved backup %s", relative)
			}
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./...")
			cmd.Dir = root
			cmd.Env = runner.toolchain.Environment(gotoolchain.EnvOptions{Test: true})
			if output, runErr := cmd.CombinedOutput(); runErr != nil {
				t.Fatalf("real Go compilation included noninput backup %s: %v\n%s", filepath.Join(root, relative), runErr, output)
			}
		})
	}
}
