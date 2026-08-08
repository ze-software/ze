package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestUserFlagIsRegisteredForCompletion drives the shell-completion surface of
// --user from the same registry the shells read.
//
// VALIDATES: `ze completion flags l2tp show` (and tunnel/session) emits --user,
// so bash, zsh and fish complete the flag. All three generators call
// `ze completion flags <path>`, which returns registry.CommandFlags(path)
// (internal/plugins/completion/flags.go, writeFlags).
// PREVENTS: a flag that works when typed in full but no shell can discover.
// The verbs parse --user in show.go, and the registry knew nothing about it, so
// `ze l2tp show --<TAB>` offered nothing.
func TestUserFlagIsRegisteredForCompletion(t *testing.T) {
	for _, path := range []string{"l2tp show", "l2tp tunnel", "l2tp session"} {
		t.Run(path, func(t *testing.T) {
			flags := registry.CommandFlags(path)
			if len(flags) == 0 {
				t.Fatalf("no flags registered for %q; shells complete nothing", path)
			}
			var found bool
			for _, f := range flags {
				if f.Name != "--user" {
					continue
				}
				found = true
				if f.Description == "" {
					t.Error("--user has no description; completion shows a bare flag")
				}
			}
			if !found {
				t.Errorf("--user missing from %q flags: %+v", path, flags)
			}
		})
	}
}

// TestRootMetaAdvertisesSubcommands checks the agent-facing help hint.
//
// VALIDATES: `ze help ai` prints the l2tp sub-paths and names --user
// (cmd/ze/help_ai.go reads Meta.Subs and skips the line when it is empty).
// PREVENTS: the empty Subs this command shipped with, which told an agent the
// root exists and nothing about how to call it.
func TestRootMetaAdvertisesSubcommands(t *testing.T) {
	var subs string
	for _, cmd := range registry.ListRoot() {
		if cmd.Name == "l2tp" {
			subs = cmd.Meta.ResolveSubs()
		}
	}
	if subs == "" {
		t.Fatal("l2tp root command advertises no subcommands; ze help ai prints nothing")
	}
	for _, want := range []string{"decode", "show", "tunnel", "session", "--user"} {
		if !strings.Contains(subs, want) {
			t.Errorf("Subs %q does not name %q", subs, want)
		}
	}
}
