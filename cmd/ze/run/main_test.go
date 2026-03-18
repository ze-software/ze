package run

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

// TestRunCommandTree verifies the full command tree is built from RPC registrations.
//
// VALIDATES: commands are dynamically discovered from registered RPCs.
// PREVENTS: tree diverging from actual RPC registrations.
func TestRunCommandTree(t *testing.T) {
	tree := cli.BuildCommandTree(false)

	// These top-level commands must exist (they come from RPC registrations)
	topLevel := []string{"daemon", "peer", "rib", "system", "summary"}
	for _, cmd := range topLevel {
		if _, ok := tree.Children[cmd]; !ok {
			t.Errorf("missing top-level command in tree: %s", cmd)
		}
	}

	// Verify peer has expected subcommands including destructive ones
	peer := tree.Children["peer"]
	if peer == nil {
		t.Fatal("peer command missing from tree")
		return
	}
	for _, sub := range []string{"list", "detail", "teardown", "add", "remove"} {
		if _, ok := peer.Children[sub]; !ok {
			t.Errorf("peer missing subcommand: %s", sub)
		}
	}

	// Verify daemon has both status and shutdown
	daemon := tree.Children["daemon"]
	if daemon == nil {
		t.Fatal("daemon command missing from tree")
		return
	}
	for _, sub := range []string{"status", "shutdown", "reload"} {
		if _, ok := daemon.Children[sub]; !ok {
			t.Errorf("daemon missing subcommand: %s", sub)
		}
	}
}

// TestIsValidCommand verifies command validation against the tree.
//
// VALIDATES: valid commands pass, invalid commands fail.
// PREVENTS: typos or wrong commands reaching the daemon.
func TestIsValidCommand(t *testing.T) {
	fullTree := cli.BuildCommandTree(false)
	showTree := cli.BuildCommandTree(true)

	tests := []struct {
		name  string
		words []string
		tree  *cli.Command
		want  bool
	}{
		{"summary", []string{"summary"}, fullTree, true},
		{"peer_list", []string{"peer", "list"}, fullTree, true},
		{"peer_detail", []string{"peer", "detail"}, fullTree, true},
		{"peer_teardown_run", []string{"peer", "teardown"}, fullTree, true},
		{"peer_teardown_show", []string{"peer", "teardown"}, showTree, false},
		{"daemon_status", []string{"daemon", "status"}, fullTree, true},
		{"daemon_shutdown_run", []string{"daemon", "shutdown"}, fullTree, true},
		{"daemon_shutdown_show", []string{"daemon", "shutdown"}, showTree, false},
		{"peer_branch", []string{"peer"}, fullTree, true},
		{"unknown", []string{"nonexistent"}, fullTree, false},
		{"peer_bad", []string{"peer", "nonexistent"}, fullTree, false},
		{"empty", nil, fullTree, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmdutil.IsValidCommand(tt.words, tt.tree)
			if got != tt.want {
				t.Errorf("IsValidCommand(%v) = %v, want %v", tt.words, got, tt.want)
			}
		})
	}
}

// TestFindNodeGroupCommand verifies group commands are detected.
//
// VALIDATES: FindNode returns node with children but no Description for groups.
// PREVENTS: Group commands like "peer" being dispatched to daemon instead of showing help.
func TestFindNodeGroupCommand(t *testing.T) {
	tree := cli.BuildCommandTree(false)

	// "peer" is a group — has children, no Description
	node := cmdutil.FindNode([]string{"peer"}, tree)
	if node == nil {
		t.Fatal("FindNode(peer) returned nil")
		return
	}
	if node.Description != "" {
		t.Errorf("peer should be a group (no Description), got %q", node.Description)
	}
	if len(node.Children) == 0 {
		t.Error("peer should have children")
	}

	// "peer detail" is a leaf — has Description
	detailNode := cmdutil.FindNode([]string{"peer", "detail"}, tree)
	if detailNode == nil {
		t.Fatal("FindNode(peer detail) returned nil")
		return
	}
	if detailNode.Description == "" {
		t.Error("peer detail should have a Description")
	}

	// "nonexistent" should return nil
	if cmdutil.FindNode([]string{"nonexistent"}, tree) != nil {
		t.Error("FindNode(nonexistent) should return nil")
	}
}

// TestExtractSelectorFromCommand verifies selector extraction from command words.
//
// VALIDATES: IP selectors between group and subcommand are extracted correctly.
// PREVENTS: "peer 127.0.0.2 show" failing as "unknown command".
func TestExtractSelectorFromCommand(t *testing.T) {
	tree := cli.BuildCommandTree(false)

	tests := []struct {
		name         string
		words        []string
		wantWords    []string
		wantSelector string
	}{
		{"no_selector", []string{"peer", "list"}, []string{"peer", "list"}, ""},
		{"ip_selector", []string{"peer", "127.0.0.2", "show"}, []string{"peer", "show"}, "127.0.0.2"},
		{"ip_selector_list", []string{"peer", "10.0.0.1", "list"}, []string{"peer", "list"}, "10.0.0.1"},
		{"glob_selector", []string{"peer", "192.168.*.*", "show"}, []string{"peer", "show"}, "192.168.*.*"},
		{"summary_no_selector", []string{"summary"}, []string{"summary"}, ""},
		{"single_word", []string{"peer"}, []string{"peer"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWords, gotSelector := cmdutil.ExtractSelector(tt.words, tree)
			if gotSelector != tt.wantSelector {
				t.Errorf("selector = %q, want %q", gotSelector, tt.wantSelector)
			}
			if len(gotWords) != len(tt.wantWords) {
				t.Fatalf("words = %v, want %v", gotWords, tt.wantWords)
			}
			for i, w := range tt.wantWords {
				if gotWords[i] != w {
					t.Errorf("words[%d] = %q, want %q", i, gotWords[i], w)
				}
			}
		})
	}
}

// TestCommandList verifies dynamic command listing.
//
// VALIDATES: CommandList() returns entries from RPC registrations.
// PREVENTS: help output being empty or missing key commands.
func TestCommandList(t *testing.T) {
	fullTree := cli.BuildCommandTree(false)
	entries := cmdutil.CommandList(fullTree)
	if len(entries) == 0 {
		t.Fatal("CommandList() returned no entries")
	}

	found := make(map[string]bool)
	for _, e := range entries {
		found[e.Name] = true
	}

	for _, want := range []string{"peer", "summary", "daemon", "system"} {
		if !found[want] {
			t.Errorf("CommandList() missing entry: %s", want)
		}
	}
}

// TestRunHelp verifies all help flag variants work.
//
// VALIDATES: ze run help, ze run --help, ze run -h all show usage.
// PREVENTS: help flags being mishandled.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var code int
			output := captureStderr(t, func() {
				code = Run([]string{arg})
			})

			if code != 0 {
				t.Errorf("Run([%s]) returned %d, want 0", arg, code)
			}

			if !strings.Contains(output, "Usage:") {
				t.Errorf("Run([%s]) output missing 'Usage:'", arg)
			}

			if !strings.Contains(output, "ze run") {
				t.Errorf("Run([%s]) output missing 'ze run'", arg)
			}
		})
	}
}

// TestRunHelpListsCommands verifies help output includes dynamically discovered commands.
//
// VALIDATES: help text is populated from RPC registrations.
// PREVENTS: help showing hardcoded list that diverges from actual commands.
func TestRunHelpListsCommands(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"help"})
	})

	if code != 0 {
		t.Errorf("Run([help]) returned %d, want 0", code)
	}

	for _, want := range []string{"peer", "summary", "daemon"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing command: %s\noutput: %s", want, output)
		}
	}
}

// TestRunNoArgs verifies that no args prints usage and returns error.
//
// VALIDATES: ze run with no args shows usage, exit 1.
// PREVENTS: cryptic errors when no subcommand given.
func TestRunNoArgs(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run(nil)
	})

	if code != 1 {
		t.Errorf("Run(nil) returned %d, want 1", code)
	}

	if !strings.Contains(output, "Usage:") {
		t.Errorf("Run(nil) output missing 'Usage:'")
	}
}

// TestRunUnknown verifies that unknown subcommand returns error.
//
// VALIDATES: ze run unknown prints error, exit 1.
// PREVENTS: unknown subcommands being sent to daemon.
func TestRunUnknown(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"nonexistent"})
	})

	if code != 1 {
		t.Errorf("Run([nonexistent]) returned %d, want 1", code)
	}

	if !strings.Contains(output, "unknown command") {
		t.Errorf("Run([nonexistent]) output missing 'unknown command'")
	}
}

// TestRunConnectionFailure verifies error handling when daemon not running.
//
// VALIDATES: Graceful error when SSH connection fails.
// PREVENTS: Cryptic errors or panics on connection failure.
func TestRunConnectionFailure(t *testing.T) {
	var code int
	output := captureStderr(t, func() {
		code = Run([]string{"summary"})
	})

	if code != 1 {
		t.Errorf("Run() with no daemon returned %d, want 1", code)
	}

	if !strings.Contains(output, "cannot connect") && !strings.Contains(output, "error") {
		t.Errorf("Run() stderr = %q, want connection or error message", output)
	}
}
