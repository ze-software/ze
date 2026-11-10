package iface

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrNoBackendLoadedNamesCauseAndFix pins what an operator reads when a
// by-name interface read finds no backend.
//
// VALIDATES: the journal row of 2026-11-07 in
// plan/journal/error-names-no-fix-command.md: the refusal names the cause (no
// interface section, so no backend is loaded) and the command that fixes it,
// on its own line, and it stays the sentinel callers match with errors.Is.
// PREVENTS: the bare `iface: no backend loaded`, which names neither, and a
// second copy of the fix text in a check that reports the same problem.
func TestErrNoBackendLoadedNamesCauseAndFix(t *testing.T) {
	const want = "iface: no backend loaded\n" +
		"Ze loads an interface backend only when the config has an interface section, so it cannot read or change an interface\n" +
		"fix: set interface backend netlink"
	if got := ErrNoBackendLoaded.Error(); got != want {
		t.Errorf("ErrNoBackendLoaded reads\n%s\nwant\n%s", got, want)
	}
	if got := ErrNoBackendLoaded.Fix(); got != "set interface backend netlink" {
		t.Errorf("Fix() = %q", got)
	}

	_, err := backendOrErr()
	if GetBackend() == nil && !errors.Is(err, ErrNoBackendLoaded) {
		t.Errorf("backendOrErr with no backend: got %v, want ErrNoBackendLoaded", err)
	}
	wrapped := fmt.Errorf("iface: get: %w", ErrNoBackendLoaded)
	if !errors.Is(wrapped, ErrNoBackendLoaded) {
		t.Error("a wrapped ErrNoBackendLoaded no longer matches errors.Is")
	}
}
