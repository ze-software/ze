// Design: docs/contributing/testing.md -- scoped lint guards the compiler dependency closure without widening direct lint scope.
package golint

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// TestLintDependencyInputPopulation compiles real ordinary, internal-test and
// external-test consumers before querying the same scoped Go-list producer.
// Excluded imported source must refuse; an allowed dependency must not become
// a directly linted package merely because the compiler loads it.
func TestLintDependencyInputPopulation(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		testPackage string
		allowed     bool
	}{
		{name: "ordinary-import"},
		{name: "internal-test-import", testPackage: "live"},
		{name: "external-test-import", testPackage: "live_test"},
		{name: "allowed-dependency-is-not-direct-scope", allowed: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			writeLintFile(t, root, "go.mod", "module lintfixture\n\ngo 1.27.0\n")
			dependency := "docs/probe"
			if testCase.allowed {
				dependency = "shared"
			}
			writeLintFile(t, root, dependency+"/probe.go", "package probe\nconst Value = 9\n")
			if testCase.testPackage == "" {
				writeLintFile(t, root, "live/live.go", "package live\nimport \"lintfixture/"+dependency+"\"\nconst Value = probe.Value\n")
			} else {
				writeLintFile(t, root, "live/live.go", "package live\nconst Value = 7\n")
				writeLintFile(t, root, "live/live_test.go", "package "+testCase.testPackage+"\nimport (\"testing\"; \"lintfixture/"+dependency+"\")\nfunc TestImportedValue(t *testing.T) { if probe.Value != 9 { t.Fatal(probe.Value) } }\n")
			}
			runner := &Runner{
				ctx: t.Context(), root: root, toolchain: fixtureChain(root),
				ops: runnerOps{capture: captureCommand},
			}
			cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./live")
			cmd.Dir = root
			cmd.Env = runner.toolchain.Environment(gotoolchain.EnvOptions{Test: true})
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("real compiler did not resolve owned consumer dependency: %v\n%s", err, output)
			}
			packages, files, err := runner.goList(Flavor{Name: "host", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, []string{"./live"})
			if !testCase.allowed {
				if err == nil {
					t.Fatalf("scoped lint admitted excluded compiler dependency %s/probe.go (direct files=%v)", dependency, files)
				}
				if !strings.Contains(err.Error(), dependency+"/probe.go") {
					t.Fatalf("refusal does not identify excluded compiler dependency: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("allowed dependency stopped scoped lint: %v", err)
			}
			if !files["live/live.go"] {
				t.Fatalf("direct lint scope lost requested consumer: %v", files)
			}
			if files[dependency+"/probe.go"] {
				t.Fatalf("compiler dependency was unexpectedly added to direct lint scope: %v", files)
			}
			if len(packages) != 1 {
				t.Fatalf("direct lint packages expanded beyond requested consumer: %v", packages)
			}
		})
	}
}
