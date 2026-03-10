package show

import (
	"io"
	"os"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
)

// captureStderr captures stderr during a function call.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	old := os.Stderr
	os.Stderr = w

	fn()

	w.Close() //nolint:errcheck,gosec // test cleanup
	os.Stderr = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	return string(out)
}

// TestShowCommandTreeReadOnly verifies the show command tree only has read-only commands.
//
// VALIDATES: show tree built from read-only RPCs only.
// PREVENTS: destructive commands appearing under "ze show".
func TestShowCommandTreeReadOnly(t *testing.T) {
	tree := cli.BuildCommandTree(true)

	// Read-only commands must exist
	for _, cmd := range []string{"peer", "rib", "summary", "daemon", "system"} {
		if _, ok := tree.Children[cmd]; !ok {
			t.Errorf("show tree missing read-only command: %s", cmd)
		}
	}

	// Peer should have list/show but NOT teardown/add/remove
	peer := tree.Children["peer"]
	if peer == nil {
		t.Fatal("peer missing from show tree")
	}
	for _, sub := range []string{"list", "detail"} {
		if _, ok := peer.Children[sub]; !ok {
			t.Errorf("show peer missing read-only subcommand: %s", sub)
		}
	}
	for _, sub := range []string{"teardown", "add", "remove"} {
		if _, ok := peer.Children[sub]; ok {
			t.Errorf("show peer should NOT contain destructive subcommand: %s", sub)
		}
	}

	// RIB read-only commands must exist
	rib := tree.Children["rib"]
	if rib == nil {
		t.Fatal("rib missing from show tree")
	}
	for _, sub := range []string{"status", "routes", "best"} {
		if _, ok := rib.Children[sub]; !ok {
			t.Errorf("show rib missing read-only subcommand: %s", sub)
		}
	}
	// RIB clear is NOT read-only — must NOT appear in show tree
	if clear := rib.Children["clear"]; clear != nil {
		t.Errorf("show rib should NOT contain destructive subcommand: clear")
	}
}

// TestShowIsValidCommand verifies read-only validation.
//
// VALIDATES: read-only commands pass, destructive commands fail.
// PREVENTS: destructive commands being reachable via "ze show".
func TestShowIsValidCommand(t *testing.T) {
	showTree := cli.BuildCommandTree(true)

	tests := []struct {
		name  string
		words []string
		want  bool
	}{
		{"summary", []string{"summary"}, true},
		{"peer_list", []string{"peer", "list"}, true},
		{"peer_detail", []string{"peer", "detail"}, true},
		{"daemon_status", []string{"daemon", "status"}, true},
		{"peer_teardown_blocked", []string{"peer", "teardown"}, false},
		{"daemon_shutdown_blocked", []string{"daemon", "shutdown"}, false},
		{"unknown", []string{"nonexistent"}, false},
		{"empty", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmdutil.IsValidCommand(tt.words, showTree)
			if got != tt.want {
				t.Errorf("IsValidCommand(%v) = %v, want %v", tt.words, got, tt.want)
			}
		})
	}
}

// TestShowHelp verifies all help flag variants work.
//
// VALIDATES: ze show help, ze show --help, ze show -h all show usage.
// PREVENTS: help flags being mishandled.
func TestShowHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var code int
			output := captureStderr(t, func() {
				code = Run([]string{arg})
			})

			if code != 0 {
				t.Errorf("Run([%s]) returned %d, want 0", arg, code)
			}
			if !strings.Contains(output, "ze show") {
				t.Errorf("help output missing 'ze show'")
			}
		})
	}
}

// TestShowHelpListsCommands verifies help output includes dynamically discovered commands.
//
// VALIDATES: help text populated from read-only RPCs.
// PREVENTS: help showing wrong commands.
func TestShowHelpListsCommands(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"help"})
	})

	if code != 0 {
		t.Errorf("Run([help]) returned %d, want 0", code)
	}

	for _, want := range []string{"peer", "summary", "daemon"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing command: %s", want)
		}
	}
}

// TestShowNoArgs verifies that no args prints usage and returns error.
//
// VALIDATES: ze show with no args shows usage, exit 1.
// PREVENTS: cryptic errors when no subcommand given.
func TestShowNoArgs(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run(nil)
	})

	if code != 1 {
		t.Errorf("Run(nil) returned %d, want 1", code)
	}
	if !strings.Contains(output, "Usage:") {
		t.Errorf("output missing 'Usage:'")
	}
}

// TestShowUnknown verifies unknown subcommand returns error.
//
// VALIDATES: ze show unknown prints error, exit 1.
// PREVENTS: unknown subcommands reaching daemon.
func TestShowUnknown(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"nonexistent"})
	})

	if code != 1 {
		t.Errorf("Run([nonexistent]) returned %d, want 1", code)
	}
	if !strings.Contains(output, "unknown command") {
		t.Errorf("output missing 'unknown command'")
	}
}

// TestShowRejectsDestructive verifies destructive commands are rejected by show.
//
// VALIDATES: ze show peer teardown / daemon shutdown rejected.
// PREVENTS: destructive commands being executed via "ze show".
func TestShowRejectsDestructive(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"peer_teardown", []string{"peer", "teardown"}},
		{"daemon_shutdown", []string{"daemon", "shutdown"}},
		{"daemon_reload", []string{"daemon", "reload"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			output := captureStderr(t, func() {
				code = Run(tt.args)
			})

			if code != 1 {
				t.Errorf("Run(%v) returned %d, want 1", tt.args, code)
			}
			if !strings.Contains(output, "unknown command") {
				t.Errorf("Run(%v) output = %q, want 'unknown command'", tt.args, output)
			}
		})
	}
}

// TestShowConnectionFailure verifies error handling when daemon not running.
//
// VALIDATES: Graceful error when socket connection fails.
// PREVENTS: Cryptic errors or panics on connection failure.
func TestShowConnectionFailure(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"summary", "--socket", "/nonexistent/socket.sock"})
	})

	if code != 1 {
		t.Errorf("Run() with bad socket returned %d, want 1", code)
	}
	if !strings.Contains(output, "cannot connect") {
		t.Errorf("stderr = %q, want 'cannot connect'", output)
	}
}
