package runner

import "testing"

func TestZeDaemonConfigArgIndex(t *testing.T) {
	// VALIDATES: runner only disables blob storage for ze daemon config
	// invocations, not for ze subcommands that consume config paths themselves.
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "stdin config", args: []string{"-"}, want: 0},
		{name: "plugin flags", args: []string{"--plugin", "ze.bgp-rib", "-"}, want: 2},
		{name: "web flags", args: []string{"--web", "3443", "--insecure-web", "test.conf"}, want: 3},
		{name: "mcp flags", args: []string{"--mcp", "8080", "--mcp-token", "secret", "ze.conf"}, want: 4},
		{name: "config subcommand", args: []string{"config", "validate", "-"}, want: -1},
		{name: "doctor subcommand", args: []string{"doctor", "--json", "empty.conf"}, want: -1},
		{name: "service subcommand", args: []string{"service", "install", "--dry-run"}, want: -1},
		// After spec-fixit-config-file-positional-grammar the runner launches a
		// config file as `ze start <config>`; the leading verb is skipped so the
		// path is still found (and blob storage is still forced for it).
		{name: "start verb with config", args: []string{"start", "x.conf"}, want: 1},
		{name: "start verb no path", args: []string{"start"}, want: -1},
		{name: "start verb web flags", args: []string{"start", "--web", "3443", "--insecure-web", "test.conf"}, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeDaemonConfigArgIndex(tt.args); got != tt.want {
				t.Fatalf("zeDaemonConfigArgIndex(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestBackgroundZeGetsReadinessEnv(t *testing.T) {
	// VALIDATES: Fix A (AC-1) -- the runner arms the ze.ready.file handshake
	// (ZE_READY_FILE env + daemon.pid tracking) for BACKGROUND ze daemons, not
	// only foreground ones. Native fixture drivers launch `ze` as a background
	// command and poll daemon.pid/daemon.ready; before the fix, background ze
	// got neither and every such test timed out.
	// Foreground behavior is unchanged (still armed).
	tests := []struct {
		name         string
		mode         string
		binName      string
		tmpfsTempDir string
		want         bool
	}{
		{name: "background ze with tmpfs", mode: "background", binName: "ze", tmpfsTempDir: "/tmp/x", want: true},
		{name: "foreground ze with tmpfs", mode: modeForeground, binName: "ze", tmpfsTempDir: "/tmp/x", want: true},
		{name: "background ze without tmpfs", mode: "background", binName: "ze", tmpfsTempDir: "", want: false},
		{name: "foreground ze without tmpfs", mode: modeForeground, binName: "ze", tmpfsTempDir: "", want: false},
		{name: "background ze-peer", mode: "background", binName: binNameZePeer, tmpfsTempDir: "/tmp/x", want: false},
		{name: "background native helper", mode: "background", binName: "ze-test", tmpfsTempDir: "/tmp/x", want: false},
		{name: "foreground native helper", mode: modeForeground, binName: "ze-test", tmpfsTempDir: "/tmp/x", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeReadyFileEnabled(tt.mode, tt.binName, tt.tmpfsTempDir); got != tt.want {
				t.Fatalf("zeReadyFileEnabled(%q, %q, %q) = %v, want %v",
					tt.mode, tt.binName, tt.tmpfsTempDir, got, tt.want)
			}
		})
	}
}

func TestNetnsChildIDs(t *testing.T) {
	// VALIDATES: Fix B (A-4) -- the netns launch mode drops ze to a NON-root
	// uid parsed from ZE_TEST_UID. A root/invalid/absent uid yields ok=false so
	// the caller fails loudly (errNetnsNeedsUID) rather than silently running ze
	// as root; GID defaults to the uid when ZE_TEST_GID is absent or invalid.
	tests := []struct {
		name    string
		uidEnv  string
		gidEnv  string
		wantUID int
		wantGID int
		wantOK  bool
	}{
		{name: "unset uid rejected", uidEnv: "", gidEnv: "", wantOK: false},
		{name: "root uid rejected", uidEnv: "0", gidEnv: "", wantOK: false},
		{name: "negative uid rejected", uidEnv: "-1", gidEnv: "", wantOK: false},
		{name: "non-numeric uid rejected", uidEnv: "abc", gidEnv: "", wantOK: false},
		{name: "valid uid, gid defaults to uid", uidEnv: "1000", gidEnv: "", wantUID: 1000, wantGID: 1000, wantOK: true},
		{name: "valid uid and gid", uidEnv: "1000", gidEnv: "2000", wantUID: 1000, wantGID: 2000, wantOK: true},
		{name: "invalid gid falls back to uid", uidEnv: "1000", gidEnv: "bad", wantUID: 1000, wantGID: 1000, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ZE_TEST_UID", tt.uidEnv)
			t.Setenv("ZE_TEST_GID", tt.gidEnv)
			uid, gid, ok := netnsChildIDs()
			if ok != tt.wantOK {
				t.Fatalf("netnsChildIDs() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (uid != tt.wantUID || gid != tt.wantGID) {
				t.Fatalf("netnsChildIDs() = (%d, %d), want (%d, %d)", uid, gid, tt.wantUID, tt.wantGID)
			}
		})
	}
}

func TestZeDaemonShouldForceFileStorage(t *testing.T) {
	// VALIDATES: web functional tests keep blob storage enabled because the web
	// server requires it, while plain daemon tests still avoid shared zefs state.
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "plain config", args: []string{"test.conf"}, want: true},
		{name: "web config", args: []string{"--web", "3443", "--insecure-web", "test.conf"}, want: false},
		{name: "web equals", args: []string{"--web=3443", "test.conf"}, want: false},
		{name: "subcommand", args: []string{"config", "validate", "test.conf"}, want: false},
		// `ze start <config>` (the post-migration daemon launch) still forces file
		// storage; `ze start --web ... <config>` keeps blob for the web server.
		{name: "start verb config", args: []string{"start", "test.conf"}, want: true},
		{name: "start verb web config", args: []string{"start", "--web", "3443", "test.conf"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeDaemonShouldForceFileStorage(tt.args); got != tt.want {
				t.Fatalf("zeDaemonShouldForceFileStorage(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
