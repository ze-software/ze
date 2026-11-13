// Design: docs/architecture/testing/qemu-integration.md -- selected guest preparation.
// Related: alltests.go -- executeOne reports preparation failure before tests run.

package testqemu

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestSelectedNamespaceFailureNamesRecovery drives the actual selected entry
// through its existing child seam. Preparation failure must stop the test and
// explain how to start it in a fresh guest; successful preparation still runs it.
func TestSelectedNamespaceFailureNamesRecovery(t *testing.T) {
	for _, preparationExit := range []int{42, 0} {
		name := "failure"
		if preparationExit == 0 {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			run := vmFixture(t)
			run.Test = "test/draft/plugin/namespace-error.ci"
			writeFile(t, run.Workspace, run.Test, "option=netns-link:name=zesrerror\ncmd=foreground:seq=1:exec=ze doctor\n")
			children := 0
			run.Run = func(_ []string, _ []string, tally io.Writer) int {
				children++
				if children == 1 {
					return preparationExit
				}
				if tally != nil {
					if _, err := io.WriteString(tally, "pass 1/1 100.0% 1s\n"); err != nil {
						t.Fatal(err)
					}
				}
				return 0
			}
			capture, err := os.CreateTemp(t.TempDir(), "stderr-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := capture.Close(); err != nil {
					t.Error(err)
				}
			})
			previous := os.Stderr
			os.Stderr = capture
			t.Cleanup(func() { os.Stderr = previous })
			report, code := run.executeOne()
			os.Stderr = previous
			data, err := os.ReadFile(capture.Name())
			if err != nil {
				t.Fatal(err)
			}
			message := strings.TrimSpace(string(data))
			if preparationExit == 0 {
				if code != 0 || len(report.Phases) != 1 {
					t.Fatalf("prepared selected test = code %d phases %+v", code, report.Phases)
				}
				if children != 3 {
					t.Errorf("successful child count %d, want namespace, conntrack and selected suite", children)
				}
				if message != "" {
					t.Errorf("successful preparation emitted a refusal: %q", message)
				}
				return
			}
			if code == 0 || len(report.Phases) != 0 || children != 1 {
				t.Fatalf("failed preparation launched work: code=%d phases=%+v children=%d", code, report.Phases, children)
			}
			if !strings.Contains(message, run.Test) {
				t.Errorf("refusal omitted selected test %q: %q", run.Test, message)
			}
			if !strings.Contains(message, ErrNamespacePreparation.Error()) || !strings.Contains(message, "exit 42") {
				t.Errorf("refusal lost preparation cause: %q", message)
			}
			if !strings.HasSuffix(message, "\nfix: ./le test qemu ci test "+run.Test) {
				t.Errorf("refusal has no final fresh-guest recovery command: %q", message)
			}
		})
	}
}
