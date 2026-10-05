package aihooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWeakeningProbeTreesCommitOriginals proves the ownership comparison reads
// original tests from HEAD, rather than refusing an unreadable Git baseline.
func TestWeakeningProbeTreesCommitOriginals(t *testing.T) {
	for _, name := range []string{categoryRFCTestGuard, categoryRFCApproval, categoryWeakenedHatch, categoryDraftIncubator} {
		t.Run(name, func(t *testing.T) {
			probe := categoryProbes[name]
			root, err := probe.tree("")
			if root != "" {
				t.Cleanup(func() {
					if err := os.RemoveAll(root); err != nil {
						t.Error(err)
					}
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			path := probe.file
			if name == categoryDraftIncubator {
				path = "test/unit/probe_test.go"
			}
			ctx, cancel := context.WithTimeout(t.Context(), probeGitTimeout)
			defer cancel()
			command := exec.CommandContext(ctx, "git", "show", "HEAD:"+path)
			command.Dir = root
			committed, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("read original HEAD: %v: %s", err, committed)
			}
			original, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			if string(committed) != string(original) {
				t.Fatalf("HEAD differs from original test: %q != %q", committed, original)
			}
		})
	}
}

// TestWeakeningProbeRejectsUnreadableOriginal proves an infrastructure refusal
// cannot masquerade as the unauthorized-edit refusal a fixture is meant to test.
func TestWeakeningProbeRejectsUnreadableOriginal(t *testing.T) {
	probe := categoryProbes[categoryRFCTestGuard]
	probe.tree = func(string) (string, error) {
		return probeTree("unreadable-head", map[string]string{probeTestPath: probeTaggedTest})
	}
	allowed, err := probeVerdict(probe, probeTaggedEdit)
	if allowed || err == nil {
		t.Fatalf("unreadable HEAD = allowed %v, error %v; want fixture error", allowed, err)
	}
	if !strings.Contains(err.Error(), "before judging authorization") {
		t.Fatalf("wrong failure reason: %v", err)
	}
}
