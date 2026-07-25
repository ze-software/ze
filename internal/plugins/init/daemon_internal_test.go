package init

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: daemonRunning positively identifies a ze daemon by its
// "SSH-2.0-ze" SSH banner and ignores any other TCP listener (host OpenSSH, a
// bare/silent listener, an oversized flood). AC-1 of
// spec-fixit-appliance-evidence-config.
// PREVENTS: `ze init --force` false-reporting a running daemon when the
// configured SSH port is answered by a non-ze listener (e.g. host sshd on
// 0.0.0.0:22), which silently aborts a fresh init and reuses a stale seed DB.

// seedSSHDefaultDB writes a minimal zefs database whose meta/ssh/default points
// daemonRunning's probe at host:port.
func seedSSHDefaultDB(t *testing.T, dbPath, host, port string) {
	t.Helper()
	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.WriteFile(zefs.KeySSHDefault.Pattern, []byte(host+"/"+port), 0); err != nil {
		t.Fatalf("write ssh default: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// startBannerListener starts a 127.0.0.1 TCP listener that, on each connection,
// writes banner (when non-empty) then closes. It returns the bound host and
// port and is closed on test cleanup.
func startBannerListener(t *testing.T, banner string) (host, port string) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			if banner != "" {
				conn.Write([]byte(banner)) //nolint:errcheck // best-effort test banner
			}
			conn.Close() //nolint:errcheck // test listener
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	return host, port
}

func TestDaemonRunningIgnoresNonZeListener(t *testing.T) {
	cases := []struct {
		name   string
		banner string
	}{
		{"host-openssh", "SSH-2.0-OpenSSH_9.6\r\n"},
		{"generic-go-ssh", "SSH-2.0-Go\r\n"},                                           // the old default; no longer ze
		{"empty-immediate-close", ""},                                                  // answers TCP, sends nothing
		{"non-ssh-noise", "HELLO not-an-ssh-server\n"},                                 // random protocol
		{"oversized-non-ze-no-newline", "SSH-2.0-OpenSSH_" + strings.Repeat("x", 300)}, // flood, no '\n'
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, port := startBannerListener(t, tc.banner)
			dbPath := filepath.Join(t.TempDir(), "database.zefs")
			seedSSHDefaultDB(t, dbPath, host, port)

			if daemonRunning(dbPath) {
				t.Fatalf("daemonRunning = true for non-ze listener (banner %q); want false", tc.banner)
			}
		})
	}
}

func TestDaemonRunningAcceptsZeBanner(t *testing.T) {
	host, port := startBannerListener(t, sshclient.ServerVersionBanner+"\r\n")
	dbPath := filepath.Join(t.TempDir(), "database.zefs")
	seedSSHDefaultDB(t, dbPath, host, port)

	if !daemonRunning(dbPath) {
		t.Fatalf("daemonRunning = false for a ze %q banner; want true", sshclient.ServerVersionBanner)
	}
}

func TestDaemonRunningFalseWhenPortUnreachable(t *testing.T) {
	// Bind then immediately release the port so nothing is listening.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	if closeErr := ln.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	dbPath := filepath.Join(t.TempDir(), "database.zefs")
	seedSSHDefaultDB(t, dbPath, host, port)

	if daemonRunning(dbPath) {
		t.Fatal("daemonRunning = true with nothing listening; want false")
	}
}

// TestDaemonRunningTimesOutOnSilentListener proves the bounded read: a listener
// that accepts and never speaks must not hang the probe, and yields false.
func TestDaemonRunningTimesOutOnSilentListener(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup

	held := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		held <- conn // hold open, never write
	}()
	t.Cleanup(func() {
		select {
		case c := <-held:
			c.Close() //nolint:errcheck // test cleanup
		default:
		}
	})

	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "database.zefs")
	seedSSHDefaultDB(t, dbPath, host, port)

	done := make(chan bool, 1)
	go func() { done <- daemonRunning(dbPath) }()
	select {
	case got := <-done:
		if got {
			t.Fatal("daemonRunning = true for a silent listener; want false")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("daemonRunning hung on a silent listener (bounded read not enforced)")
	}
}
