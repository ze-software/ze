package main

import (
	"bytes"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const featureTagsTimeout = 60 * time.Second

// runFeatureTagsCheck runs `go run scripts/codegen/feature_tags.go --check` from the
// repo root and returns combined output, failing the test on a non-zero exit.
// Black-box like runPluginImportsCheck: the tool is //go:build ignore, so it cannot
// be called in-process; drive it as a subprocess and assert on output.
func runFeatureTagsCheck(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), featureTagsTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", "run", filepath.Join(repoRoot(t), "scripts", "codegen", "feature_tags.go"), "--check")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("feature_tags.go --check failed (are .golangci.yml / gokrazy config stale? run make generate): %v\n%s", err, out)
	}
	return string(out)
}

// VALIDATES: scripts/codegen/feature_tags.go --check reports the generated tag lists
// (.golangci.yml build-tags, gokrazy GoBuildTags, quickstart go-install command) are
// current -- i.e. they match feature-gates.txt. The drift gate that replaces hand-maintenance.
// PREVENTS: a gate added to feature-gates.txt without regenerating the two static
// consumers (the exact miss that let ze_vrrp reach neither at first).
func TestFeatureTagsCheckRuns(t *testing.T) {
	out := runFeatureTagsCheck(t)
	if !strings.Contains(out, "current") {
		t.Fatalf("feature_tags.go --check did not report current tag lists:\n%s", out)
	}
}

// VALIDATES: --check does not mutate the files it validates.
// PREVENTS: a verification gate rewriting .golangci.yml / gokrazy config while merely
// checking currency.
func TestFeatureTagsCheckIsReadOnly(t *testing.T) {
	root := repoRoot(t)
	targets := []string{
		filepath.Join(root, ".golangci.yml"),
		filepath.Join(root, "gokrazy", "ze", "config.json"),
		filepath.Join(root, "docs", "guide", "quickstart.md"),
	}
	before := make([][]byte, len(targets))
	for i, p := range targets {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s before check: %v", p, err)
		}
		before[i] = b
	}
	_ = runFeatureTagsCheck(t)
	for i, p := range targets {
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s after check: %v", p, err)
		}
		if !bytes.Equal(before[i], after) {
			t.Fatalf("feature_tags.go --check mutated %s", p)
		}
	}
}

// VALIDATES: the generated tag lists actually contain every feature-gates.txt tag
// (ze_vrrp included) plus the required base tags. Proves the generator's output is
// correct, not merely self-consistent.
func TestFeatureTagsCoverManifest(t *testing.T) {
	root := repoRoot(t)
	manifest, err := os.ReadFile(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	var tags []string
	seen := map[string]bool{}
	for line := range strings.SplitSeq(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag := strings.Fields(line)[0]
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		t.Fatal("no tags parsed from feature-gates.txt")
	}

	golangci, err := os.ReadFile(filepath.Join(root, ".golangci.yml"))
	if err != nil {
		t.Fatalf("read .golangci.yml: %v", err)
	}
	gokrazy, err := os.ReadFile(filepath.Join(root, "gokrazy", "ze", "config.json"))
	if err != nil {
		t.Fatalf("read gokrazy config: %v", err)
	}
	quickstart, err := os.ReadFile(filepath.Join(root, "docs", "guide", "quickstart.md"))
	if err != nil {
		t.Fatalf("read quickstart.md: %v", err)
	}
	for _, tag := range tags {
		if !strings.Contains(string(golangci), tag) {
			t.Errorf(".golangci.yml build-tags missing %q", tag)
		}
		if !strings.Contains(string(gokrazy), tag) {
			t.Errorf("gokrazy GoBuildTags missing %q", tag)
		}
		if !strings.Contains(string(quickstart), tag) {
			t.Errorf("docs/guide/quickstart.md go-install command missing %q", tag)
		}
	}
	if !strings.Contains(string(gokrazy), "ze_appliance") {
		t.Error("gokrazy GoBuildTags missing base tag ze_appliance")
	}
}
