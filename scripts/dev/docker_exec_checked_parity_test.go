// The migration proof for docker_exec_checked.py compares its JSON document
// with the native analyzer. This file stays beside the producer and leaves with
// it at the final swap.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. Both implementations give
// the same structured report and exit code over fixtures and the live tree.
// PREVENTS: output parity over an empty population and a port whose wrapper
// fixpoint, exact site order, baseline, or failing exit code differs.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/functional"
)

const dockerExecParityScript = "docker_exec_checked.py"

func dockerExecParityWriteTree(t *testing.T, baseline int, extra string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"test/interop", "test/health"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o750); err != nil {
			t.Fatalf("creating fixture directory: %v", err)
		}
	}
	source := `
# A triple-quoted decoy contains syntax that is not code.
DECOY = """return docker_exec_quiet(container, ["not", "a", "call"])"""

class FRR:
    def route_table(self):
        return self._vtysh_quiet("show ip route")

    def _vtysh_quiet(self, command):
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

def docker_exec_quiet(container, cmd):
    return ""

def established(container):
    out = FRR()._vtysh_quiet("show bgp summary")
    return "Established" in out

def checked(container):
    out = docker_exec_quiet(container, ["show"])
    if not out.strip():
        raise RuntimeError("failed")
    return out

def warm(container):
    docker_exec_quiet(container, ["show", "version"])

def diagnostic(container):
    print(docker_exec_quiet(container, ["show"])[:100])  # fail-open-ok: diagnostic
` + extra
	if err := os.WriteFile(filepath.Join(root, "test", "interop", "interop.py"), []byte(source), 0o600); err != nil {
		t.Fatalf("writing Python fixture: %v", err)
	}
	body, err := json.Marshal(map[string]int{"unchecked": baseline})
	if err != nil {
		t.Fatalf("encoding baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "test", "health", "docker-exec-baseline.json"), body, 0o600); err != nil {
		t.Fatalf("writing baseline: %v", err)
	}
	return root
}

func dockerExecParityNative(t *testing.T, root string) devPyResult {
	t.Helper()
	report, err := functional.CheckDockerExec(root)
	if err != nil {
		return devPyResult{Stderr: "docker-exec-check: " + err.Error() + "\n", Code: 1}
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("encoding native report: %v", err)
	}
	return devPyResult{Stdout: string(body) + "\n", Code: report.Code()}
}

func dockerExecParityCompare(t *testing.T, root string) devPyResult {
	t.Helper()
	script := devPyRunScript(t, dockerExecParityScript, []string{"--root", root, "--json"}, devPyRoot(t))
	command := dockerExecParityNative(t, root)
	devPyAgree(t, "docker-exec JSON report", script, command, script.Stdout, command.Stdout)
	return command
}

func TestDockerExecBothHalvesAgreeAtAndAboveTheFloor(t *testing.T) {
	t.Run("at floor", func(t *testing.T) {
		if result := dockerExecParityCompare(t, dockerExecParityWriteTree(t, 1, "")); result.Code != 0 {
			t.Errorf("the floor-equal fixture exits %d, want 0", result.Code)
		}
	})
	t.Run("above floor", func(t *testing.T) {
		result := dockerExecParityCompare(t, dockerExecParityWriteTree(t, 1, `
def newly_added(container):
    out = docker_exec_quiet(container, ["show", "isis", "neighbor"])
    return "Up" in out
`))
		if result.Code != 1 {
			t.Errorf("the over-floor fixture exits %d, want 1", result.Code)
		}
	})
}

func TestDockerExecBothHalvesPrintTheSameSuccessVerdict(t *testing.T) {
	root := dockerExecParityWriteTree(t, 1, "")
	script := devPyRunScript(t, dockerExecParityScript, []string{"--root", root}, devPyRoot(t))
	report, err := functional.CheckDockerExec(root)
	if err != nil {
		t.Fatalf("native check: %v", err)
	}
	command := devPyResult{Stdout: report.Text(), Code: report.Code()}
	devPyAgree(t, "docker-exec success text", script, command, script.Stdout, command.Stdout)
}

func TestDockerExecBothHalvesAgreeOverTheLiveTree(t *testing.T) {
	root := devPyRoot(t)
	script := devPyRunScript(t, dockerExecParityScript, []string{"--root", root, "--json"}, root)
	command := dockerExecParityNative(t, root)
	devPyAgree(t, "docker-exec live report", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, `"files-scanned":`) {
		t.Errorf("the live comparison carries no file population: %s", command.Stdout)
	}
	if !strings.Contains(command.Stdout, `"docker_exec_quiet"`) {
		t.Errorf("the live comparison carries no seed: %s", command.Stdout)
	}
}

func TestDockerExecBothHalvesRunTheFixtureSelftest(t *testing.T) {
	script := devPyRunScript(t, dockerExecParityScript, []string{"--selftest"}, devPyRoot(t))
	report, err := functional.SelftestDockerExec()
	if err != nil {
		t.Fatalf("native selftest: %v", err)
	}
	command := devPyResult{Stdout: report.Text(), Code: 0}
	devPyAgree(t, "docker-exec selftest", script, command, script.Stdout, command.Stdout)
	if len(report.Verdicts) < 7 {
		t.Errorf("the selftest exercised only %d verdict sites", len(report.Verdicts))
	}
}

func TestDockerExecBothHalvesRefuseUnreadableInputs(t *testing.T) {
	t.Run("missing baseline", func(t *testing.T) {
		root := dockerExecParityWriteTree(t, 2, "")
		if err := os.Remove(filepath.Join(root, "test", "health", "docker-exec-baseline.json")); err != nil {
			t.Fatalf("removing baseline: %v", err)
		}
		script := devPyRunScript(t, dockerExecParityScript, []string{"--root", root}, devPyRoot(t))
		command := dockerExecParityNative(t, root)
		if script.Code != 1 || command.Code != 1 {
			t.Errorf("missing baseline exits script=%d command=%d", script.Code, command.Code)
		}
		for name, text := range map[string]string{"script": script.Stderr, "command": command.Stderr} {
			if !strings.Contains(text, "does not exist") {
				t.Errorf("%s does not name the missing baseline: %s", name, text)
			}
		}
	})
	t.Run("invalid Python", func(t *testing.T) {
		root := dockerExecParityWriteTree(t, 2, "")
		path := filepath.Join(root, "test", "interop", "broken.py")
		if err := os.WriteFile(path, []byte("def (:\n"), 0o600); err != nil {
			t.Fatalf("writing broken fixture: %v", err)
		}
		script := devPyRunScript(t, dockerExecParityScript, []string{"--root", root}, devPyRoot(t))
		command := dockerExecParityNative(t, root)
		if script.Code != 1 || command.Code != 1 {
			t.Errorf("invalid Python exits script=%d command=%d", script.Code, command.Code)
		}
		for name, text := range map[string]string{"script": script.Stderr, "command": command.Stderr} {
			if !strings.Contains(text, "broken.py") {
				t.Errorf("%s does not name the broken file: %s", name, text)
			}
		}
	})
}
