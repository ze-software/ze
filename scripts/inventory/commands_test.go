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
	"os"
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

// VALIDATES: inventory.go stops when it cannot read a file in full, rather than
// publishing the counts it reached.
// PREVENTS: an RPC or line count that is short by however much of a file the
// scan missed, printed under a header claiming the output is always accurate.
func TestInventoryStopsOnUnreadableFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// The read binds the test cache to the tool. A build-ignored file is not an
	// input to this test package's build, and a subprocess read is not an input
	// to the cache either, so an edit to the tool would otherwise come back as
	// a cached pass.
	src, err := os.ReadFile("inventory.go")
	if err != nil {
		t.Fatalf("read the tool under test: %v", err)
	}
	if !strings.Contains(string(src), "func scanAll(") {
		t.Fatalf("inventory.go no longer holds scanAll; this test drives the wrong tool")
	}

	// One YANG line above bufio.MaxScanTokenSize (64 KiB) stops the scan, so
	// the rpc below it is never counted.
	fixture := t.TempDir()
	yang := filepath.Join(fixture, "yang")
	if err := os.MkdirAll(yang, 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	body := "module fixture {\n  // " + strings.Repeat("x", 70*1024) + "\n  rpc do-a-thing { }\n}\n"
	if err := os.WriteFile(filepath.Join(yang, "fixture.yang"), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/inventory/inventory.go", "--root", fixture)
	cmd.Dir = root
	out, runErr := cmd.CombinedOutput()

	if runErr == nil {
		t.Fatalf("inventory published counts from a file it could not read:\n%s", out)
	}
	if !strings.Contains(string(out), "fixture.yang") {
		t.Fatalf("inventory did not name the unreadable file:\n%s", out)
	}
}
