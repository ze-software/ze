// Design: ai/rules/plugins.md -- ze_fleet_ui absent (compile-out) validation
//
//go:build !ze_fleet_ui

package hub

// VALIDATES: AC-9 -- a build with ze_web and without ze_fleet_ui links the web
// package and nothing of the fleet package: its symbol table holds no fleet
// symbol.
// PREVENTS: the fleet package leaking into a build that dropped its gate,
// through an always-on import or an ungated registration.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTag_FleetUI_AbsentBinaryDropsFleetSymbols(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-core-web")
	build := exec.CommandContext(t.Context(), "go", "build", "-tags", "ze_core ze_web", "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ze_core ze_web failed: %v\n%s", err, out)
	}

	out, err := exec.CommandContext(t.Context(), "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}

	const (
		webSymbol   = "internal/component/web."
		fleetSymbol = "internal/component/web/fleetui"
	)

	// The web package must be linked, or the absence of its nested fleet
	// package says nothing about the gate.
	webLinked := false
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, fleetSymbol) {
			t.Fatalf("ze_core ze_web binary retained fleet symbol %q", line)
		}
		if strings.Contains(line, webSymbol) {
			webLinked = true
		}
	}
	if !webLinked {
		t.Fatal("ze_core ze_web binary links no web symbol, so the fleet check read nothing")
	}
}
