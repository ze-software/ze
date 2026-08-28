package interop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
)

// TestInteropRunnerFailsClosedWithoutDocker validates the native runner's
// prerequisite polarity without launching an interpreter or a probe program.
func TestInteropRunnerFailsClosedWithoutDocker(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	report := interopbgp.RunAt(t.Context(), root, interopbgp.Options{Scenario: "bgp-ebgp-gobgp"})
	if report.Code == 0 {
		t.Fatalf("native interop runner passed with Docker unavailable: %+v", report)
	}
	text := strings.ToLower(report.SetupError)
	if !strings.Contains(text, "docker") {
		t.Errorf("missing prerequisite must name Docker, got %+v", report)
	}
	if strings.Contains(text, "skip") {
		t.Errorf("missing Docker must be a failure, not a skip: %+v", report)
	}
}

// TestInteropTreeUsesOnlyCompiledHelpers makes the cutover executable: a new
// scenario cannot reintroduce an interpreter path, shell helper, or source probe.
func TestInteropTreeUsesOnlyCompiledHelpers(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	interopRoot := filepath.Join(root, "test", "interop")
	err = filepath.WalkDir(interopRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(interopRoot, path)
		if err != nil {
			return err
		}
		lowerName := strings.ToLower(entry.Name())
		for _, suffix := range []string{"." + "py", "." + "pyc", "." + "sh", "." + "bash", "." + "pl"} {
			if strings.HasSuffix(lowerName, suffix) {
				t.Errorf("interpreted-language artifact remains: %s", relative)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ToLower(string(data))
		for _, token := range []string{
			"python" + "3",
			"entrypoint\", \"" + "python",
			"#!/" + "bin/sh",
			"#!/usr/bin/env ba" + "sh",
			"." + "sh",
			"." + "bash",
			"." + "pl",
		} {
			if strings.Contains(text, token) {
				t.Errorf("interpreter launch remains in %s", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
