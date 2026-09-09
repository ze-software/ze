package process

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// The function under test lives in the parent package (../childenv.go),
// because the declaration query starts the same run string and owes the same
// PATH. The test stays here, with the caller whose ordering it is about.
//
// VALIDATES: the engine's own directory is a FALLBACK on a spawned external
// plugin's PATH, and never shadows a command the inherited PATH already
// resolves.
//
// A cross-built checkout holds a host binary and a guest binary of the SAME
// name side by side. Under QEMU, /workspace/bin carries a darwin `ze-test`
// beside `ze-test-linux-arm64`, and the daemon runs as the latter, so
// plugin.EngineBinDir answers /workspace/bin. Putting that directory FIRST made a
// plugin's `run "ze-test fixture ..."` resolve to the darwin binary, which the
// guest shell reads as a script and reports as
// `/workspace/bin/ze-test: line 1: syntax error: unexpected "("`.
// The test runner already puts a shim directory holding the right binary at
// the head of the child's PATH (setupBinShims, internal/test/runner/runner.go);
// the prepend was the only reason that shim lost.
func TestPluginPathEnvKeepsTheEngineDirectoryAsAFallback(t *testing.T) {
	separator := string(os.PathListSeparator)
	inherited := "/shim" + separator + "/usr/bin"

	got := plugin.ChildPathEnv("/workspace/bin", inherited)

	want := "PATH=" + strings.Join([]string{"/shim", "/usr/bin", "/workspace/bin"}, separator)
	if got != want {
		t.Errorf("plugin.ChildPathEnv put the engine directory in the wrong place\n got: %s\nwant: %s", got, want)
	}
}

// VALIDATES: the two degenerate inputs. An engine directory that could not be
// determined adds no PATH entry at all, so the child keeps the environment it
// inherited; an empty inherited PATH leaves the engine directory as the only
// entry, which is the dev case the fallback exists for.
func TestPluginPathEnvDegenerateInputs(t *testing.T) {
	if got := plugin.ChildPathEnv("", "/usr/bin"); got != "" {
		t.Errorf("an unknown engine directory must add no PATH entry, got %q", got)
	}
	if got := plugin.ChildPathEnv("/workspace/bin", ""); got != "PATH=/workspace/bin" {
		t.Errorf("an empty inherited PATH must leave the engine directory alone, got %q", got)
	}
}
