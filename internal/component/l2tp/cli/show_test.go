package cli

import (
	"testing"
)

// stubForward swaps the daemon-forwarding seam for the duration of a test and
// captures what the verb would have sent. It restores the real forwarder on
// cleanup so tests cannot leak into each other.
func stubForward(t *testing.T) *struct {
	cmd    string
	user   string
	called bool
} {
	t.Helper()
	got := &struct {
		cmd    string
		user   string
		called bool
	}{}
	prev := forward
	forward = func(command, user string) int {
		got.cmd = command
		got.user = user
		got.called = true
		return 0
	}
	t.Cleanup(func() { forward = prev })
	return got
}

// TestCmdShowUserFlag drives the `--user`/`-u` flag from the command entry
// point (cmdShow), not the flag helper alone.
//
// VALIDATES: `ze l2tp show --user alice tunnels` resolves credentials as alice
// while still forwarding `show l2tp tunnels` to the daemon, so an operator who
// cannot read the zefs store can name themselves like they can on every other
// client CLI (`ze config set`, `ze signal`, ...).
// PREVENTS: the hardcoded LoadCredentialsWithFlags("") in forwardToDaemon,
// which left `ze l2tp show` as the only client CLI with no way to name a user.
func TestCmdShowUserFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCmd  string
		wantUser string
	}{
		{name: "no flag, subcommand only", args: []string{"tunnels"}, wantCmd: "show l2tp tunnels", wantUser: ""},
		{name: "no args yields summary", args: []string{}, wantCmd: "show l2tp", wantUser: ""},
		{name: "long flag before subcommand", args: []string{"--user", "alice", "tunnels"}, wantCmd: "show l2tp tunnels", wantUser: "alice"},
		{name: "short alias before subcommand", args: []string{"-u", "alice", "tunnels"}, wantCmd: "show l2tp tunnels", wantUser: "alice"},
		{name: "single-dash long form", args: []string{"-user", "bob", "sessions"}, wantCmd: "show l2tp sessions", wantUser: "bob"},
		{name: "flag does not eat positional selectors", args: []string{"-u", "alice", "tunnel", "id", "5"}, wantCmd: "show l2tp tunnel id 5", wantUser: "alice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stubForward(t)
			if code := cmdShow(tt.args); code != 0 {
				t.Fatalf("cmdShow(%q): got exit %d, want 0", tt.args, code)
			}
			if !got.called {
				t.Fatal("forward was never called")
			}
			if got.cmd != tt.wantCmd {
				t.Errorf("command: got %q, want %q", got.cmd, tt.wantCmd)
			}
			if got.user != tt.wantUser {
				t.Errorf("user: got %q, want %q", got.user, tt.wantUser)
			}
		})
	}
}

// TestClientFlagsRejectsTrailingFlag is the fail-closed guard.
//
// VALIDATES: a flag placed AFTER the subcommand is rejected with a non-zero
// exit and nothing is sent to the daemon.
// PREVENTS: silently authenticating as the WRONG user with exit 0. Go's flag
// package stops parsing at the first non-flag token, so `show tunnels --user
// alice` leaves `--user alice` in the positional tail. Forwarding that would
// hand it to Dispatcher.matchCommandTokens
// (internal/component/plugin/server/command.go:428), which returns unmatched
// trailing tokens as args and reports SUCCESS; the l2tp containers carry no
// leaves, so extractArgDefs yields no ArgDefs, the validator guarded at :584 is
// skipped entirely, and the handler's `_ []string` discards them. The operator
// would get a clean summary for the default user and never learn --user was
// ignored.
func TestClientFlagsRejectsTrailingFlag(t *testing.T) {
	tests := []struct {
		name string
		verb string
		run  func([]string) int
		args []string
	}{
		{name: "show", run: cmdShow, args: []string{"tunnels", "--user", "alice"}},
		{name: "show short alias", run: cmdShow, args: []string{"tunnels", "-u", "alice"}},
		{name: "tunnel teardown", run: cmdTunnelTeardown, args: []string{"all", "--user", "alice"}},
		{name: "session teardown", run: cmdSessionTeardown, args: []string{"all", "--user", "alice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stubForward(t)
			if code := tt.run(tt.args); code == 0 {
				t.Errorf("%v: got exit 0, want non-zero rejection", tt.args)
			}
			if got.called {
				t.Errorf("%v: forwarded %q to the daemon; want nothing sent", tt.args, got.cmd)
			}
		})
	}
}

// TestTeardownVerbsUserFlag covers the two destructive verbs, which need the
// flag for the same reason `show` does.
//
// VALIDATES: `ze l2tp tunnel --user alice id 5` tears down as alice.
// PREVENTS: fixing only `show` and leaving `tunnel`/`session` unreachable for
// operators who cannot read the zefs store.
func TestTeardownVerbsUserFlag(t *testing.T) {
	tests := []struct {
		name    string
		run     func([]string) int
		args    []string
		wantCmd string
	}{
		{name: "tunnel by id", run: cmdTunnelTeardown, args: []string{"--user", "alice", "id", "5"}, wantCmd: "clear l2tp tunnel id 5"},
		{name: "tunnel all", run: cmdTunnelTeardown, args: []string{"-u", "alice", "all"}, wantCmd: "clear l2tp tunnel all"},
		{name: "session by id", run: cmdSessionTeardown, args: []string{"--user", "alice", "id", "7"}, wantCmd: "clear l2tp session id 7"},
		{name: "session all", run: cmdSessionTeardown, args: []string{"-u", "alice", "all"}, wantCmd: "clear l2tp session all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stubForward(t)
			if code := tt.run(tt.args); code != 0 {
				t.Fatalf("%v: got exit %d, want 0", tt.args, code)
			}
			if got.cmd != tt.wantCmd {
				t.Errorf("command: got %q, want %q", got.cmd, tt.wantCmd)
			}
			if got.user != "alice" {
				t.Errorf("user: got %q, want %q", got.user, "alice")
			}
		})
	}
}

// TestTeardownVerbsRequireSelector pins the pre-existing contract that the
// destructive verbs refuse to run with no selector, now that a FlagSet sits in
// front of them.
//
// VALIDATES: `ze l2tp tunnel` alone still errors instead of forwarding.
// PREVENTS: flag parsing swallowing the empty-args check, so that a bare
// `ze l2tp tunnel --user alice` sends `clear l2tp tunnel` and tears down
// something the operator never selected.
func TestTeardownVerbsRequireSelector(t *testing.T) {
	tests := []struct {
		name string
		run  func([]string) int
		args []string
	}{
		{name: "tunnel no args", run: cmdTunnelTeardown, args: []string{}},
		{name: "session no args", run: cmdSessionTeardown, args: []string{}},
		{name: "tunnel flag but no selector", run: cmdTunnelTeardown, args: []string{"--user", "alice"}},
		{name: "session flag but no selector", run: cmdSessionTeardown, args: []string{"-u", "alice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stubForward(t)
			if code := tt.run(tt.args); code == 0 {
				t.Errorf("%v: got exit 0, want non-zero", tt.args)
			}
			if got.called {
				t.Errorf("%v: forwarded %q; want nothing sent", tt.args, got.cmd)
			}
		})
	}
}
