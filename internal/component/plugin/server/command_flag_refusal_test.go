// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package server

import (
	"strings"
	"testing"
)

// TestFlagRefusalNamesTheLocalCommand proves the dispatcher's refusal of a
// flag is actionable where it can be.
//
// Goal: a flag sent to a command that also has a local-data handler is
// refused with the local `ze` command to run; a flag sent to a daemon-only
// command keeps the plain refusal, since no local command takes it.
// Method: flagRefusal over `show plugin list`, which the plugin host
// registers as local data, and over a path no local registry holds.
func TestFlagRefusalNamesTheLocalCommand(t *testing.T) {
	err := flagRefusal("show plugin list", "--all", "show plugin list --all")
	want := `unexpected flag "--all": flags are interpreted by the client, not the daemon; ` +
		"run 'ze show plugin list --all' on the host that holds what it reads"
	if err.Error() != want {
		t.Errorf("local-data command:\n got %q\nwant %q", err, want)
	}

	err = flagRefusal("show no-such-local", "-u", "show no-such-local -u")
	if strings.Contains(err.Error(), "run 'ze") {
		t.Errorf("a daemon-only command named a local command: %q", err)
	}
	if !strings.HasPrefix(err.Error(), `unexpected flag "-u": flags are interpreted by the client`) {
		t.Errorf("daemon-only refusal changed: %q", err)
	}
}
