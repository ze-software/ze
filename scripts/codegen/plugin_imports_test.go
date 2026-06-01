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

const pluginImportsTimeout = 60 * time.Second

// VALIDATES: scripts/codegen/plugin_imports.go --check verifies generated imports.
// PREVENTS: changed-file verification failing because build-ignore codegen scripts lack a test package.
func TestPluginImportsCheckRuns(t *testing.T) {
	out := runPluginImportsCheck(t)
	if !strings.Contains(out, "is current") {
		t.Fatalf("plugin_imports.go --check did not report current generated imports:\n%s", out)
	}
}

// VALIDATES: scripts/codegen/plugin_imports.go --check does not mutate internal/component/plugin/all/all.go.
// PREVENTS: verification gates modifying generated files while checking inventory consistency.
func TestPluginImportsCheckModeIsReadOnly(t *testing.T) {
	root := repoRoot(t)
	allGo := filepath.Join(root, "internal", "component", "plugin", "all", "all.go")
	before, err := os.ReadFile(allGo)
	if err != nil {
		t.Fatalf("read all.go before check: %v", err)
	}
	_ = runPluginImportsCheck(t)
	after, err := os.ReadFile(allGo)
	if err != nil {
		t.Fatalf("read all.go after check: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("plugin_imports.go --check mutated all.go")
	}
}

func runPluginImportsCheck(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pluginImportsTimeout)
	defer cancel()

	cmd := osexec.CommandContext(ctx, "go", "run", "scripts/codegen/plugin_imports.go", "--check")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin_imports.go --check failed: %v\n%s", err, out)
	}
	return string(out)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}
