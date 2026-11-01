package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZeDaemonConfigArgIndex(t *testing.T) {
	// VALIDATES: daemon config arguments are separated from verb operands,
	// so only daemon stdin blocks become restartable source files.
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
		{name: "doctor subcommand", args: []string{"doctor", "--json", "config", "empty.conf"}, want: -1},
		{name: "service subcommand", args: []string{"service", "install", "--dry-run"}, want: -1},
		// After spec-fixit-config-file-positional-grammar the runner launches a
		// config file as `ze start <config>`; the leading verb is skipped so the
		// path is still found for source routing.
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
	// The directory is the one the CHILD runs in (Record.WorkDir), which every
	// record has since 2026-08-28. It was Record.TmpfsTempDir until 2026-09-03,
	// and that field is set only when the .ci declares files, so the 1503 .ci
	// files that declare none armed nothing and their fixtures polled a
	// daemon.pid nobody wrote (plan/journal/guard-added-to-one-half-of-a-pair.md).
	tests := []struct {
		name    string
		mode    string
		binName string
		workDir string
		want    bool
	}{
		{name: "background ze with a work directory", mode: "background", binName: "ze", workDir: "/tmp/x", want: true},
		{name: "foreground ze with a work directory", mode: modeForeground, binName: "ze", workDir: "/tmp/x", want: true},
		{name: "background ze-peer", mode: "background", binName: binNamePeer, workDir: "/tmp/x", want: false},
		{name: "background native helper", mode: "background", binName: "ze-test", workDir: "/tmp/x", want: false},
		{name: "foreground native helper", mode: modeForeground, binName: "ze-test", workDir: "/tmp/x", want: false},
		// Defensive only. Every record carries a WorkDir, so an empty one means
		// the runner failed to make it; arming a path rooted at "" would write
		// daemon.ready into whatever directory the runner happens to sit in,
		// which is the repository root this whole change exists to keep clean.
		{name: "background ze with no work directory", mode: "background", binName: "ze", workDir: "", want: false},
		{name: "foreground ze with no work directory", mode: modeForeground, binName: "ze", workDir: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zeReadyFileEnabled(tt.mode, tt.binName, tt.workDir); got != tt.want {
				t.Fatalf("zeReadyFileEnabled(%q, %q, %q) = %v, want %v",
					tt.mode, tt.binName, tt.workDir, got, tt.want)
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

// TestRecordPortExpansionRefusesUnreservedPort pins the port placeholders a .ci
// test may write to the ports its lease reserves.
//
// VALIDATES: $PORT, $PORT2 and $PORT3 expand to the first, second and third port
// of the test's lease, the lease spans all three, and a .ci naming any other
// $PORT<n> fails discovery with a refusal naming the file, the placeholder and
// the placeholders it may use instead.
// PREVENTS: $PORT3 expanding as $PORT followed by "3" (port 1790 became 17903),
// a port no lease reserved, which parallel tests could bind at the same time.
func TestRecordPortExpansionRefusesUnreservedPort(t *testing.T) {
	if got, want := ExpandPorts("a $PORT b $PORT2 c $PORT3 d", 1790), "a 1790 b 1791 c 1792 d"; got != want {
		t.Errorf("ExpandPorts = %q, want %q", got, want)
	}
	if TestPortSpan != 3 {
		t.Errorf("TestPortSpan = %d, want 3: the lease must reserve every port a placeholder names", TestPortSpan)
	}

	dir := t.TempDir()
	ciFile := filepath.Join(dir, "port-four.ci")
	content := "cmd=foreground:seq=1:exec=le mock rtr --port $PORT4\n" +
		"expect=exit:code=0\n"
	if err := os.WriteFile(ciFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write .ci: %v", err)
	}
	_, err := NewEncodingTests(dir).parseAndAdd(ciFile)
	if err == nil {
		t.Fatal("parseAndAdd accepted $PORT4, a port no lease reserves")
	}
	for _, want := range []string{"port-four.ci", "$PORT4", "$PORT3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err.Error(), want)
		}
	}
}
