// Design: docs/contributing/spec-workflow.md -- a package-relative producer name
// VALIDATES: a spec name whose first segment is no top-level directory
// (reactor/x.go) resolves as a path suffix over tracked files: one live match
// must be read, a deleted or unmatched name is skipped, several matches refuse,
// and a root-anchored path keeps the root rule.
// PREVENTS: a spec naming reactor/filter_format.go being checked at the root,
// where git never held it, and so refusing every edit of the spec.
package hookruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	shorthandLive    = "internal/component/bgp/reactor/filter_live.go"
	shorthandGone    = "internal/component/bgp/reactor/filter_gone.go"
	shorthandDupOne  = "internal/component/bgp/reactor/dup.go"
	shorthandDupTwo  = "internal/component/bmp/reactor/dup.go"
	shorthandContent = "package reactor\n"
)

// shorthandFixture is a checkout whose tracked files hold shorthandLive and two
// reactor/dup.go files, whose history added and then deleted shorthandGone, and
// in which evidenceProbeB is read in this context.
func shorthandFixture(t *testing.T) string {
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
	for _, path := range []string{shorthandLive, shorthandGone, shorthandDupOne, shorthandDupTwo} {
		writeHookFixture(t, root, path, shorthandContent)
		git("add", "--", path)
	}
	git("commit", "-q", "-m", "add producers")
	git("rm", "-q", "--", shorthandGone)
	git("commit", "-q", "-m", "delete producer")
	writeHookFixture(t, root, evidenceProbeB, "package other\n")
	recordObservedRead(t, root, filepath.Join(root, evidenceProbeB), "package other\n", 1, 2, 2)
	return root
}

func TestDesignEvidenceRequiresReadOfPackageRelativeProducer(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := shorthandFixture(t)
			named := "## " + heading + "\n`reactor/filter_live.go`\n"
			message := observedDesignGate(t, root, named, 2)
			if !strings.Contains(message, "Missing current subject evidence: "+shorthandLive) {
				t.Fatalf("the resolved producer is not required: %s", message)
			}
			recordObservedRead(t, root, filepath.Join(root, shorthandLive), shorthandContent, 1, 2, 2)
			observedDesignGate(t, root, named, 0)
		})
	}
}

func TestDesignEvidenceSkipsPackageRelativeProducerDeleted(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := shorthandFixture(t)
			named := "## " + heading + "\n`reactor/filter_gone.go`\n"
			writeHookFixture(t, root, evidenceProbeSpec, named)
			observedDesignGate(t, root, named+"\nEdited.\n", 0)
		})
	}
}

func TestDesignEvidenceIgnoresPackageRelativeNameMatchingNothing(t *testing.T) {
	root := shorthandFixture(t)
	observedDesignGate(t, root, "## Current Behavior\n`reactor/filter_never.go`\n", 0)
}

func TestDesignEvidenceRefusesAmbiguousPackageRelativeProducer(t *testing.T) {
	root := shorthandFixture(t)
	message := observedDesignGate(t, root, "## Files to Modify\n`reactor/dup.go`\n", 2)
	for _, want := range []string{"ambiguous", shorthandDupOne, shorthandDupTwo} {
		if !strings.Contains(message, want) {
			t.Fatalf("ambiguous refusal does not name %q: %s", want, message)
		}
	}
}

// TestDesignEvidenceRootAnchoredPathKeepsRootRule checks that a name whose
// first segment is a top-level directory is never resolved as a suffix: once
// reactor/ exists at the root, reactor/filter_live.go is a root path git never
// held, and it refuses as before.
func TestDesignEvidenceRootAnchoredPathKeepsRootRule(t *testing.T) {
	root := shorthandFixture(t)
	if err := os.Mkdir(filepath.Join(root, "reactor"), 0o750); err != nil {
		t.Fatal(err)
	}
	message := observedDesignGate(t, root, "## Current Behavior\n`reactor/filter_live.go`\n", 2)
	if !strings.Contains(message, "reactor/filter_live.go: inspect producer") || !strings.Contains(message, "git history never deleted it") {
		t.Fatalf("a root-anchored path did not keep the root rule: %s", message)
	}
}
