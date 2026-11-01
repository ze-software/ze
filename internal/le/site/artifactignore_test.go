// Design: website/AI.md -- the artifact checkout keeps its own ignore list
package site

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestTheBuildLeavesTheArtifactItsOwnIgnoreList covers the two build steps
// that decide which .gitignore the published branch carries.
//
// Goal: a staged copy of website/.gitignore never ships, and the artifact's own
// list survives every build. Method: stage the source file at the artifact root,
// run the source-only trim, then the artifact ignore writer, as Build does.
func TestTheBuildLeavesTheArtifactItsOwnIgnoreList(t *testing.T) {
	output := t.TempDir()
	staged := filepath.Join(output, artifactIgnoreFile)
	if err := os.WriteFile(staged, []byte("__pycache__\n/assets/demos/\n"), 0o600); err != nil {
		t.Fatalf("stage the source ignore file: %v", err)
	}

	if err := removeSourceOnly(output); err != nil {
		t.Fatalf("remove source-only files: %v", err)
	}
	if err := writeArtifactIgnore(output); err != nil {
		t.Fatalf("write the artifact ignore list: %v", err)
	}

	if got := readArtifact(t, output, artifactIgnoreFile); got != artifactIgnoreContent {
		t.Errorf("the artifact ignore list reads %q, want %q", got, artifactIgnoreContent)
	}
	found, err := publishedSourceOnly(output)
	if err != nil {
		t.Fatalf("scan the artifact: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("the check reports %v as published source, and the artifact's own ignore list is not source", found)
	}
	if slices.Contains(checkNamedArtifacts(output), artifactIgnoreFile) {
		t.Errorf("the check reports %s absent after the build wrote it", artifactIgnoreFile)
	}
}

// TestSiteCheckRefusesTheSourceIgnoreListAtTheRoot covers the guard the
// exception above must not open: any root .gitignore whose content is not the
// artifact's own list is the source file leaking, and is reported.
func TestSiteCheckRefusesTheSourceIgnoreListAtTheRoot(t *testing.T) {
	output := t.TempDir()
	if err := os.WriteFile(filepath.Join(output, artifactIgnoreFile), []byte("__pycache__\n"), 0o600); err != nil {
		t.Fatalf("write a foreign ignore file: %v", err)
	}

	found, err := publishedSourceOnly(output)
	if err != nil {
		t.Fatalf("scan the artifact: %v", err)
	}
	if !slices.Equal(found, []string{artifactIgnoreFile}) {
		t.Errorf("the check reports %v, want only %s", found, artifactIgnoreFile)
	}
}

// TestSiteCheckReportsAMissingArtifactIgnoreList covers the named-artifact
// half: an artifact with no ignore list would let tmp/ reach the branch.
func TestSiteCheckReportsAMissingArtifactIgnoreList(t *testing.T) {
	if !slices.Contains(checkNamedArtifacts(t.TempDir()), artifactIgnoreFile) {
		t.Errorf("the check does not report %s absent from an empty artifact", artifactIgnoreFile)
	}
}
