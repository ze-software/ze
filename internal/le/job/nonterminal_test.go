package job

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNonterminalOwnershipPreservesTheActualExit distinguishes the command's
// outcome from cleanup's expected-stop policy, including a live-context SIGKILL.
func TestNonterminalOwnershipPreservesTheActualExit(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		code    int
	}{
		{name: "success", command: "exit 0", code: 0},
		{name: "failure", command: "exit 37", code: 37},
		{name: "unexpected-kill", command: "kill -KILL $$", code: 137},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			admission := &Admission{Root: t.TempDir(), OwnProcessGroup: true, Out: &output, Err: &output}
			code := admission.stream(t.Context(), []string{"sh", "-c", test.command}, admission.Root, nil, nil)
			if code != test.code {
				t.Fatalf("owned command returned %d, want %d; output: %s", code, test.code, output.String())
			}
		})
	}
}

// TestNonterminalOwnershipLeavesDefaultStdinAttached supplies actual inherited
// input in a subprocess, avoiding mutation of the package test process's stdin.
// The default path must retain it; only explicit compiler ownership gets EOF.
func TestNonterminalOwnershipLeavesDefaultStdinAttached(t *testing.T) {
	for _, mode := range []string{"terminal", "owned"} {
		t.Run(mode, func(t *testing.T) {
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), self, "-test.run=^TestNonterminalInputProcess$")
			command.Env = append(os.Environ(), "JOB_NONTERMINAL_INPUT_MODE="+mode)
			command.Stdin = strings.NewReader("retained-input\n")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("stdin control: %v\n%s", err, output)
			}
			want := "input:retained-input"
			if mode == "owned" {
				want = "input:EOF"
			}
			if string(output) != want {
				t.Fatalf("stdin control returned %q, want %q", output, want)
			}
		})
	}
}

func TestNonterminalInputProcess(t *testing.T) {
	mode := os.Getenv("JOB_NONTERMINAL_INPUT_MODE")
	if mode == "" {
		return
	}
	admission := &Admission{Root: t.TempDir(), OwnProcessGroup: mode == "owned", Out: os.Stdout, Err: os.Stderr}
	code := admission.stream(t.Context(), []string{"sh", "-c", `if IFS= read -r line; then printf 'input:%s' "$line"; else printf input:EOF; fi`}, admission.Root, nil, nil)
	os.Exit(code)
}
