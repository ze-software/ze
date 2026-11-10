// Design: docs/contributing/testing.md -- generated test-main source must not become an authored lint input.
package golint

import (
	"os/exec"
	"runtime"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// TestLintAllowedAuthoredTestScope compiles allowed internal and external tests,
// then requires the native dependency/test query to retain exactly the authored
// scope. Synthetic test-main GoFiles may be absolute; they must not be joined
// beneath the package directory or promoted into directly linted packages.
func TestLintAllowedAuthoredTestScope(t *testing.T) {
	for _, testPackage := range []string{"live", "live_test"} {
		t.Run(testPackage, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
			writeLintFile(t, root, "live/live.go", "package live\nconst Value = 7\n")
			writeLintFile(t, root, "shared/probe.go", "package probe\nconst Value = 11\n")
			imports := "import (\"testing\"; \"lintfixture/shared\")\n"
			value := "Value"
			if testPackage == "live_test" {
				imports = "import (\"testing\"; \"lintfixture/shared\"; \"lintfixture/live\")\n"
				value = "live.Value"
			}
			writeLintFile(t, root, "live/live_test.go", "package "+testPackage+"\n"+imports+"func TestAllowedValues(t *testing.T) { if "+value+" != 7 { t.Fatal(\"wrong live value\") }; if probe.Value != 11 { t.Fatal(\"wrong dependency value\") } }\n")
			runner := &Runner{
				ctx: t.Context(), root: root, toolchain: fixtureChain(root),
				ops: runnerOps{capture: captureCommand},
			}
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./live")
			cmd.Dir = root
			cmd.Env = runner.toolchain.Environment(gotoolchain.EnvOptions{Test: true})
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("real compiler did not execute allowed authored tests: %v\n%s", err, output)
			}
			packages, files, _, err := runner.goList(Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, []string{"./live"})
			if err != nil {
				t.Fatalf("allowed authored test scope refused: %v", err)
			}
			if len(packages) != 1 {
				t.Fatalf("direct lint scope includes dependency or generated test-main package: %v", packages)
			}
			if len(files) != 2 {
				t.Fatalf("direct lint inputs differ from two authored files: %v", files)
			}
			if !files["live/live.go"] {
				t.Fatalf("direct lint scope lost authored source: %v", files)
			}
			if !files["live/live_test.go"] {
				t.Fatalf("direct lint scope lost authored test source: %v", files)
			}
		})
	}
}
