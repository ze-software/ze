package process

import (
	"os"
	"strings"
	"testing"
)

// VALIDATES: the engine's own directory is a FALLBACK on a spawned external
// plugin's PATH, and never shadows a command the inherited PATH already
// resolves.
//
// A cross-built checkout holds a host binary and a guest binary of the SAME
// name side by side. Under QEMU, /workspace/bin carries a darwin `ze-test`
// beside `ze-test-linux-arm64`, and the daemon runs as the latter, so
// execBinDir answers /workspace/bin. Putting that directory FIRST made a
// plugin's `run "ze-test fixture ..."` resolve to the darwin binary, which the
// guest shell reads as a script and reports as
// `/workspace/bin/ze-test: line 1: syntax error: unexpected "("`.
// The test runner already puts a shim directory holding the right binary at
// the head of the child's PATH (setupBinShims, internal/test/runner/runner.go);
// the prepend was the only reason that shim lost.
func TestPluginPathEnvKeepsTheEngineDirectoryAsAFallback(t *testing.T) {
	separator := string(os.PathListSeparator)
	inherited := strings.Join([]string{"/shim", "/usr/bin"}, separator)

	got := pluginPathEnv("/workspace/bin", inherited)

	want := "PATH=" + strings.Join([]string{"/shim", "/usr/bin", "/workspace/bin"}, separator)
	if got != want {
		t.Errorf("pluginPathEnv put the engine directory in the wrong place\n got: %s\nwant: %s", got, want)
	}
}

// VALIDATES: the two degenerate inputs. An engine directory that could not be
// determined adds no PATH entry at all, so the child keeps the environment it
// inherited; an empty inherited PATH leaves the engine directory as the only
// entry, which is the dev case the fallback exists for.
func TestPluginPathEnvDegenerateInputs(t *testing.T) {
	if got := pluginPathEnv("", "/usr/bin"); got != "" {
		t.Errorf("an unknown engine directory must add no PATH entry, got %q", got)
	}
	if got := pluginPathEnv("/workspace/bin", ""); got != "PATH=/workspace/bin" {
		t.Errorf("an empty inherited PATH must leave the engine directory alone, got %q", got)
	}
}
