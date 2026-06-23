// VALIDATES: the image server binds and serves on the interface address even
// with no iface backend loaded (the install/provision path).
// PREVENTS: regression of the PXE install failure where the image server never
// bound, so the target could not chainload and looped on DHCP DISCOVER.

package imageserver

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestImageServerServesViaResolveFallback proves the install-path fix end to
// end: with no iface backend loaded (the `ze-setup install remote` scenario),
// resolveInterfaceIPv4 yields a bindable address and the HTTP mux actually
// serves on it. This is the runtime-bind coverage the config-validation `.ci`
// tests cannot provide; it is the regression guard for the bug where the image
// server never bound and the PXE target looped forever on DHCP DISCOVER.
func TestImageServerServesViaResolveFallback(t *testing.T) {
	name, _ := firstIPv4Interface(t) // loopback on a normal host; no iface backend loaded

	ip, err := resolveInterfaceIPv4(name)
	if err != nil {
		t.Fatalf("resolveInterfaceIPv4(%q) without backend: %v", name, err)
	}

	imageDir := t.TempDir()
	if werr := os.WriteFile(filepath.Join(imageDir, "ze-20260101-000000.img"), []byte("x"), 0o600); werr != nil {
		t.Fatal(werr)
	}
	cfg := imageConfig{Enabled: true, ImageDirectory: imageDir, BootDirectory: t.TempDir()}
	mux := newMux(cfg, "", ip)

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Fatalf("listen on resolved %q: %v", ip, err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	url := "http://" + ln.Addr().String() + "/install/boot/boot.ipxe"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s (server bound to no-backend-resolved %q): %v", url, ip, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", url, resp.StatusCode)
	}
}
