// Smoke test for the //go:build ignore inventory tools in this directory.
//
// commands.go and inventory.go use //go:build ignore so they are excluded from
// normal compilation and from golangci-lint's type-checking pipeline. This test
// file does NOT have the ignore tag, so it is the only buildable file in the
// package and gives the linter and verify-changed a real target. It runs the
// command inventory tool as a subprocess and asserts it emits an inventory.

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandInventoryRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/inventory/commands.go")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command inventory failed:\n%s", out)
	}

	text := string(out)
	for _, want := range []string{"# Command Inventory", "Total:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("inventory output missing %q:\n%s", want, text)
		}
	}
}
