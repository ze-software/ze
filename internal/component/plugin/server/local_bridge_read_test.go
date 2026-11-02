// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// confinedReadFile writes content as router.conf in the configuration
// directory XDG_CONFIG_HOME points at, the only place the daemon route reads
// a caller-named file from (command.CallerPath), and answers its path.
func confinedReadFile(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	dir := filepath.Join(home, "ze")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "router.conf")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// TestDaemonReadOfCallerConfigLeavesDaemonEnvironment proves a daemon-served
// command that reads a caller's config file applies nothing from it to the
// answering daemon.
//
// Goal: a file whose `environment { log { level debug; } }` block the loader
// would publish (config.LoadConfig, applyParsedEnvironment) leaves the
// daemon's base log level (`ze.log`) exactly as it was, for `show plugin declarations
// config`, which resolves the plugin set through the loader, and for every
// config command that parses a caller-named file.
// Method: each command runs through the dispatcher the server loads, over one
// confined file; the env key is read after each. A read that applied the
// block sets the key, since the block writes it only while it is unset.
func TestDaemonReadOfCallerConfigLeavesDaemonEnvironment(t *testing.T) {
	const levelKey = "ze.log" // The base level `log { level ... }` writes (slogutil envKeyBaseLevel).
	if before := env.Get(levelKey); before != "" {
		t.Fatalf("%s is %q before the test: the environment block would not be written over it, so the test proves nothing", levelKey, before)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv(levelKey); err != nil {
			t.Errorf("unset %s: %v", levelKey, err)
		}
		env.ResetCache()
	})

	file := confinedReadFile(t, "environment {\n\tlog {\n\t\tlevel debug;\n\t}\n}\n")
	d, _ := localBridgeDispatcher(t)
	for _, input := range []string{
		"show plugin declarations config " + file,
		"show config dump " + file,
		"show config validation " + file,
		"show config graph " + file,
		"show config fmt " + file,
		"show config diff " + file + " " + file,
	} {
		resp, err := d.Dispatch(&CommandContext{Username: "operator"}, input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if resp.Status == plugin.StatusError {
			t.Fatalf("%s: refused: %q", input, resp.Error)
		}
		if after := env.Get(levelKey); after != "" {
			t.Fatalf("%s set the daemon's %s to %q from the caller's file", input, levelKey, after)
		}
	}
}

// TestShowConfigFmtDiffReachesDaemonCallerAsWarnings proves the diff that
// `show config fmt diff <file>` writes as its Diagnostic reaches a daemon
// caller, as the response Warnings.
//
// Goal: the diff the local route prints arrives on every other interface,
// one line per warning, under the warning status.
// Method: dispatch over a confined unformatted file; the warnings must carry
// the re-indented line the diff adds.
func TestShowConfigFmtDiffReachesDaemonCallerAsWarnings(t *testing.T) {
	file := confinedReadFile(t, "bgp {\nrouter-id 1.1.1.1;\n}\n")
	d, _ := localBridgeDispatcher(t)

	resp, err := d.Dispatch(&CommandContext{Username: "operator"}, "show config fmt diff "+file)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if resp.Status != plugin.StatusWarning {
		t.Fatalf("status %q error %q, want the warning status a code-0 answer with a diagnostic takes", resp.Status, resp.Error)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("no warnings: the diff never reached the daemon caller")
	}
	got := strings.Join(resp.Warnings, "\n")
	if !strings.Contains(got, "router-id 1.1.1.1;") {
		t.Errorf("warnings do not carry the diff of the changed line:\n%s", got)
	}
}
