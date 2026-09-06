// VALIDATES: listen-interface is a leaf-list, so the image server binds one
// HTTP listener for every entry an operator names, not for the first alone.
// PREVENTS: regression of the silent drop where entry two and every later entry
// were parsed, ignored, and never logged, so an operator running the image
// server and the TFTP server for one PXE install got TFTP on every named
// interface and HTTP on one.

package imageserver

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// imageDirWithImage returns a directory holding one .img, which is what
// serveBootIPXE needs to build a boot script.
func imageDirWithImage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze-20260101-000000.img"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// getStatus fetches path from addr and returns the status and the body.
func getStatus(t *testing.T, addr, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET http://%s%s: %v", addr, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	return resp.StatusCode, string(body[:n])
}

// TestListenTargetsKeepsEveryEntry pins the leaf-list contract: every entry
// yields a target, and an entry that does not resolve is reported and skipped
// rather than stopping the entries after it.
func TestListenTargetsKeepsEveryEntry(t *testing.T) {
	name, want := firstIPv4Interface(t)

	log, buf := captureServerLog()
	targets := listenTargets([]string{name, "ze-no-such-iface0", name}, log)

	if len(targets) != 2 {
		t.Fatalf("listenTargets returned %d targets, want 2: %v", len(targets), targets)
	}
	for i, target := range targets {
		if target.iface != name {
			t.Errorf("target %d iface = %q, want %q", i, target.iface, name)
		}
		if target.ip != want {
			t.Errorf("target %d ip = %q, want %q", i, target.ip, want)
		}
	}
	if !strings.Contains(buf.String(), "ze-no-such-iface0") {
		t.Errorf("log does not name the unresolvable interface:\n%s", buf.String())
	}
}

// TestListenTargetsWithNoNameBindsEveryAddress pins the documented default: a
// configuration naming no interface binds the port on every address of the
// host, which is one target with an empty address.
func TestListenTargetsWithNoNameBindsEveryAddress(t *testing.T) {
	log, _ := captureServerLog()
	targets := listenTargets(nil, log)

	if len(targets) != 1 {
		t.Fatalf("listenTargets(nil) returned %d targets, want 1: %v", len(targets), targets)
	}
	if targets[0].ip != "" {
		t.Errorf("target ip = %q, want the empty wildcard address", targets[0].ip)
	}
}

// TestStartTargetsBindsEveryTarget proves the operator-visible half: two
// targets yield two listeners, and both serve. Port 0 lets each listener take
// its own ephemeral port, so the test needs no second address on the host.
func TestStartTargetsBindsEveryTarget(t *testing.T) {
	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDirWithImage(t),
		BootDirectory:  t.TempDir(),
		ListenPort:     0,
	}
	targets := []listenTarget{{iface: "first", ip: "127.0.0.1"}, {iface: "second", ip: "127.0.0.1"}}

	log, _ := captureServerLog()
	servers := startTargets(cfg, "", targets, log)
	t.Cleanup(func() {
		for _, srv := range servers {
			_ = srv.Close()
		}
	})

	if len(servers) != 2 {
		t.Fatalf("startTargets bound %d servers, want 2", len(servers))
	}
	if servers[0].Addr == servers[1].Addr {
		t.Fatalf("both servers report %q; each target owns its own listener", servers[0].Addr)
	}
	for _, srv := range servers {
		status, _ := getStatus(t, srv.Addr, "/install/boot/boot.ipxe")
		if status != http.StatusOK {
			t.Errorf("GET boot.ipxe on %s: status %d, want 200", srv.Addr, status)
		}
	}
}

// TestStartTargetsSkipsUnbindableTarget proves one unusable interface does not
// stop the interfaces after it from serving. 192.0.2.1 is TEST-NET-1, which no
// host carries, so the bind fails on every platform.
func TestStartTargetsSkipsUnbindableTarget(t *testing.T) {
	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDirWithImage(t),
		BootDirectory:  t.TempDir(),
		ListenPort:     0,
	}
	targets := []listenTarget{{iface: "unusable", ip: "192.0.2.1"}, {iface: "usable", ip: "127.0.0.1"}}

	log, buf := captureServerLog()
	servers := startTargets(cfg, "", targets, log)
	t.Cleanup(func() {
		for _, srv := range servers {
			_ = srv.Close()
		}
	})

	if len(servers) != 1 {
		t.Fatalf("startTargets bound %d servers, want 1", len(servers))
	}
	if !strings.HasPrefix(servers[0].Addr, "127.0.0.1:") {
		t.Errorf("bound server addr = %q, want the usable target", servers[0].Addr)
	}
	if !strings.Contains(buf.String(), "listen failed") {
		t.Errorf("log does not report the failed bind:\n%s", buf.String())
	}
	status, _ := getStatus(t, servers[0].Addr, "/install/boot/boot.ipxe")
	if status != http.StatusOK {
		t.Errorf("GET boot.ipxe on %s: status %d, want 200", servers[0].Addr, status)
	}
}
