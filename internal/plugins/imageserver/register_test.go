package imageserver

import (
	"bytes"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firstIPv4Interface returns an interface name that has an IPv4 address and
// that address. Loopback (127.0.0.1) qualifies, so this is deterministic on
// normal hosts; it skips only when the host has no IPv4 at all.
func firstIPv4Interface(t *testing.T) (string, string) {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("net.Interfaces: %v", err)
	}
	for _, ni := range ifaces {
		addrs, addrErr := ni.Addrs()
		if addrErr != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ni.Name, ip4.String()
			}
		}
	}
	// environmental guard, not a coverage drop -- interfaceIPv4Direct
	// is a kernel-IPv4 lookup, so a host with zero IPv4 addresses cannot exercise
	// it. Loopback (127.0.0.1) satisfies this on every normal host.
	t.Skip("no interface with an IPv4 address on this host")
	return "", ""
}

// TestInterfaceIPv4Direct verifies the stdlib fallback resolves a real
// interface's IPv4 without any iface backend.
func TestInterfaceIPv4Direct(t *testing.T) {
	name, want := firstIPv4Interface(t)
	got, err := interfaceIPv4Direct(name)
	if err != nil {
		t.Fatalf("interfaceIPv4Direct(%q): %v", name, err)
	}
	if got != want {
		t.Errorf("interfaceIPv4Direct(%q) = %q, want %q", name, got, want)
	}
}

// TestResolveInterfaceIPv4FallsBackWithoutBackend pins the install/provision
// fix: with no iface backend loaded (the `ze-setup install remote` scenario),
// resolveInterfaceIPv4 must fall back to a direct kernel lookup instead of
// failing with "no backend loaded".
func TestResolveInterfaceIPv4FallsBackWithoutBackend(t *testing.T) {
	name, want := firstIPv4Interface(t)
	got, err := resolveInterfaceIPv4(name)
	if err != nil {
		t.Fatalf("resolveInterfaceIPv4(%q) fell through without backend: %v", name, err)
	}
	if got != want {
		t.Errorf("resolveInterfaceIPv4(%q) = %q, want %q", name, got, want)
	}
}

// TestInterfaceIPv4DirectUnknown reports an error (not a panic) for a name that
// has no kernel device.
func TestInterfaceIPv4DirectUnknown(t *testing.T) {
	if _, err := interfaceIPv4Direct("ze-nonexistent-iface0"); err == nil {
		t.Fatal("expected error for nonexistent interface, got nil")
	}
}

func captureServerLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestLogServedImageWithManifest(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"ze-20260101-000000.img", "ze-20260617-160000.img"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"appliance":"prod","timestamp":"20260617-160000","ze-version":"dev","arch":"amd64","image":"ze-20260617-160000.img","image-sha256":"deadbeef"}`
	if err := os.WriteFile(filepath.Join(dir, "build.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	log, buf := captureServerLog()
	logServedImage(log, dir)
	out := buf.String()

	for _, want := range []string{"ze-20260617-160000.img", "prod", "deadbeef", "image to install"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "different image") {
		t.Errorf("unexpected mismatch warning:\n%s", out)
	}
}

func TestLogServedImageManifestMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze-20260617-160000.img"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// manifest names an older image than the newest one actually on disk.
	manifest := `{"image":"ze-20260101-000000.img","timestamp":"20260101-000000"}`
	if err := os.WriteFile(filepath.Join(dir, "build.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	log, buf := captureServerLog()
	logServedImage(log, dir)
	if !strings.Contains(buf.String(), "different image") {
		t.Errorf("expected mismatch warning:\n%s", buf.String())
	}
}

func TestLogServedImageNoManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ze-1.img"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	log, buf := captureServerLog()
	logServedImage(log, dir)
	if !strings.Contains(buf.String(), "no build.json") {
		t.Errorf("expected missing-manifest warning:\n%s", buf.String())
	}
}

func TestLogServedImageNoImages(t *testing.T) {
	log, buf := captureServerLog()
	logServedImage(log, t.TempDir())
	if !strings.Contains(buf.String(), "no installable") {
		t.Errorf("expected no-image warning:\n%s", buf.String())
	}
}
