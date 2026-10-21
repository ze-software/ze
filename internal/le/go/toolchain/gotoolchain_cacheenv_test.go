// Related: gotoolchain.go -- Environment, the environment the verify gate's unit stages run under
//
// VALIDATES: which environment reads the Go test result cache keys on, under
// the gate's own Toolchain.Environment (spec-verify-gate-warm-timed-first-red,
// assumption A-7). cmd/go hashes only the variables a test logged through
// internal/testlog (computeTestInputsID, cmd/go/internal/test/test.go, case
// "getenv"), and os.Getenv and os.LookupEnv log (testlog.Getenv in os/env.go)
// while os.Environ does not.
// PREVENTS: a claim that a cached pass is safe when the variable a test reads
// changed between two verify runs.

package gotoolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cacheEnvProbe is the variable the probe tests read, changed between runs.
const cacheEnvProbe = "ZE_TESTCACHE_ENV_PROBE"

// cacheEnvModule holds two packages that fail when cacheEnvProbe is "red":
// one reads it through os.Getenv, one finds it by scanning os.Environ.
var cacheEnvModule = map[string]string{
	"go.mod": "module example.com/cacheenv\n\ngo 1.22\n",
	"getenv/getenv_test.go": "package getenv\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
		"func TestProbe(t *testing.T) {\n\tif os.Getenv(\"" + cacheEnvProbe + "\") == \"red\" {\n\t\tt.Fatal(\"probe is red\")\n\t}\n}\n",
	"environ/environ_test.go": "package environ\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
		"func TestProbe(t *testing.T) {\n\tfor _, entry := range os.Environ() {\n" +
		"\t\tif entry == \"" + cacheEnvProbe + "=red\" {\n\t\t\tt.Fatal(\"probe is red\")\n\t\t}\n\t}\n}\n",
}

// TestTheTestCacheSeesGetenvButNotEnviron runs each probe package twice under
// Toolchain.Environment with a private cache, the probe "green" then "red".
// The os.Getenv package is rerun and fails, because its read is a cache input.
// The os.Environ package answers its first pass `(cached)`, because nothing
// told cmd/go it read the variable: a cached pass can hide a red that only the
// second environment shows.
func TestTheTestCacheSeesGetenvButNotEnviron(t *testing.T) {
	if _, err := exec.LookPath(goProgram); err != nil {
		t.Skipf("no %s executable on PATH (%v): this test runs the real go command", goProgram, err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)

	root := t.TempDir()
	for name, content := range cacheEnvModule {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, probe := range []struct {
		pkg        string
		secondRuns bool
	}{
		{"./getenv", true},
		{"./environ", false},
	} {
		t.Setenv(cacheEnvProbe, "green")
		if output, failed := runCacheEnvTest(t, root, probe.pkg); failed || strings.Contains(output, "(cached)") {
			t.Fatalf("%s first run, probe green, over an empty cache: failed=%v\n%s", probe.pkg, failed, output)
		}
		t.Setenv(cacheEnvProbe, "red")
		output, failed := runCacheEnvTest(t, root, probe.pkg)
		cached := strings.Contains(output, "(cached)")
		if probe.secondRuns && (cached || !failed) {
			t.Errorf("%s: an os.Getenv read is a cache input, so the red probe must rerun and fail; failed=%v\n%s",
				probe.pkg, failed, output)
		}
		if !probe.secondRuns && (!cached || failed) {
			t.Errorf("%s: an os.Environ read is not a cache input, so the first pass is answered (cached); failed=%v\n%s",
				probe.pkg, failed, output)
		}
	}
}

// runCacheEnvTest runs `go test pkg` in root under the unit stages'
// environment and answers its combined output and whether it failed.
func runCacheEnvTest(t *testing.T, root, pkg string) (string, bool) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), goProgram, "test", pkg)
	cmd.Dir = root
	cmd.Env = Toolchain{Root: root, Procs: 1}.Environment(EnvOptions{Test: true, Procs: true})
	output, err := cmd.CombinedOutput()
	return string(output), err != nil
}
