// Design: docs/contributing/spec-workflow.md -- a producer the spec's work deleted
// VALIDATES: a Current Behavior or Files to Modify path that git history shows
// was deleted is skipped, while a path git never held still refuses.
// PREVENTS: a spec whose own phase deleted a producer it names becoming
// uneditable ("inspect producer: no such file").
package hookruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// deletedHistoryFixture is a checkout whose history added evidenceProbeA and
// then deleted it, with evidenceProbeB present and read in this context.
func deletedHistoryFixture(t *testing.T) string {
	t.Helper()
	root := gitFixture(t)
	git := func(args ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root, "-c", "user.name=probe", "-c", "user.email=probe@example.invalid"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	writeHookFixture(t, root, evidenceProbeA, "package le\n")
	git("add", "--", evidenceProbeA)
	git("commit", "-q", "-m", "add producer")
	git("rm", "-q", "--", evidenceProbeA)
	git("commit", "-q", "-m", "delete producer")
	writeHookFixture(t, root, evidenceProbeB, "package other\n")
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
	return root
}

func TestDesignEvidenceSkipsProducerDeletedInHistory(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := deletedHistoryFixture(t)
			named := "## " + heading + "\n`" + evidenceProbeA + "`\n"
			writeHookFixture(t, root, evidenceProbeSpec, named)
			observedDesignGate(t, root, named+"\nEdited.\n", 0)
		})
	}
}

func TestDesignEvidenceRefusesProducerNeverInHistory(t *testing.T) {
	const never = "internal/le/producer_never.go"
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := deletedHistoryFixture(t)
			message := observedDesignGate(t, root, "## "+heading+"\n`"+never+"`\n", 2)
			if !strings.Contains(message, never) {
				t.Fatalf("a producer git never held is not named: %s", message)
			}
		})
	}
}
