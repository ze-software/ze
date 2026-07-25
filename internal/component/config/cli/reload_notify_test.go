// Design: docs/architecture/config/syntax.md — config set/deactivate --reload opt-in

package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

// TestReloadOptInGate validates the --reload option's actual behavior: whether
// each command attempts to contact the daemon. It swaps the SSH seams
// (loadReloadCredentials / execReloadCommand) so it counts real notify attempts
// without opening a connection.
//
// This guards the gate INVERSION, not just the flag surface: a regression that
// kept the flag but left the gate as `if !*reload` would flip these counts and
// fail here. The prior functional test (cli-config-reload-flag.ci) could only
// prove the flag parses/rejects, since the .ci harness cannot observe the
// notify side effect.
//
// VALIDATES: AC-1 (default no notify), AC-2 (--reload opts in), AC-4 (same for
// deactivate), and the stdin skip.
func TestReloadOptInGate(t *testing.T) {
	oldLoad, oldExec := loadReloadCredentials, execReloadCommand
	t.Cleanup(func() { loadReloadCredentials, execReloadCommand = oldLoad, oldExec })

	var reloadCalls int
	loadReloadCredentials = func(string) (sshclient.Credentials, error) { return sshclient.Credentials{}, nil }
	execReloadCommand = func(sshclient.Credentials) error { reloadCalls++; return nil }

	const cfg = "bgp {\n\tsession {\n\t\tasn {\n\t\t\tlocal 65533\n\t\t}\n\t}\n}\n"

	// AC-1: default `set` (no --reload) must NOT contact the daemon.
	reloadCalls = 0
	if code := cmdSet([]string{writeTestConfig(t, cfg), "bgp", "session", "asn", "local", "65000"}); code != exitOK {
		t.Fatalf("default set exit = %d, want %d", code, exitOK)
	}
	if reloadCalls != 0 {
		t.Fatalf("default `set` must not notify the daemon; got %d reload call(s)", reloadCalls)
	}

	// AC-2: `set --reload` must notify the daemon exactly once.
	reloadCalls = 0
	if code := cmdSet([]string{"--reload", writeTestConfig(t, cfg), "bgp", "session", "asn", "local", "65001"}); code != exitOK {
		t.Fatalf("set --reload exit = %d, want %d", code, exitOK)
	}
	if reloadCalls != 1 {
		t.Fatalf("`set --reload` must notify the daemon once; got %d", reloadCalls)
	}

	// AC-2 (stdin): `set --reload -` has no on-disk file, so it must NOT notify.
	reloadCalls = 0
	var out strings.Builder
	restore := cliio.SwapStreams(strings.NewReader(cfg), &out)
	code := cmdSetImpl(storage.NewFilesystem(), []string{"--reload", "-", "bgp", "session", "asn", "local", "65002"})
	restore()
	if code != exitOK {
		t.Fatalf("set --reload - exit = %d, want %d", code, exitOK)
	}
	if reloadCalls != 0 {
		t.Fatalf("`set --reload -` (stdin) must not notify the daemon; got %d", reloadCalls)
	}

	// AC-4: default `deactivate` must NOT notify; `deactivate --reload` notifies once.
	reloadCalls = 0
	if rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{writeTestConfig(t, deactivateTestConfig), "bgp", "router-id"}); rc != exitOK {
		t.Fatalf("default deactivate rc = %d", rc)
	}
	if reloadCalls != 0 {
		t.Fatalf("default `deactivate` must not notify the daemon; got %d", reloadCalls)
	}

	reloadCalls = 0
	if rc := cmdDeactivateImpl(storage.NewFilesystem(), []string{"--reload", writeTestConfig(t, deactivateTestConfig), "bgp", "router-id"}); rc != exitOK {
		t.Fatalf("deactivate --reload rc = %d", rc)
	}
	if reloadCalls != 1 {
		t.Fatalf("`deactivate --reload` must notify the daemon once; got %d", reloadCalls)
	}
}
