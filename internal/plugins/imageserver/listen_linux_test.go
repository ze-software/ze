//go:build linux

// VALIDATES: each listener serves a boot script naming its own address, so a
// client that reaches the second interface is told to fetch the kernel from
// that interface.
// PREVENTS: regression to one mux built for the first interface, which would
// send every client to the first address whichever interface it reached.

package imageserver

import (
	"net/http"
	"strings"
	"testing"
)

// TestServeTargetAdvertisesItsOwnAddress binds two loopback addresses, which
// Linux carries without configuration for the whole 127.0.0.0/8 range, and
// reads the generated boot script from each.
func TestServeTargetAdvertisesItsOwnAddress(t *testing.T) {
	cfg := imageConfig{
		Enabled:        true,
		ImageDirectory: imageDirWithImage(t),
		BootDirectory:  t.TempDir(),
		ListenPort:     0,
	}
	targets := []listenTarget{{iface: "first", ip: "127.0.0.1"}, {iface: "second", ip: "127.0.0.2"}}

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
	for i, srv := range servers {
		status, body := getStatus(t, srv.Addr, "/install/boot/boot.ipxe")
		if status != http.StatusOK {
			t.Fatalf("GET boot.ipxe on %s: status %d, want 200", srv.Addr, status)
		}
		want := "ze.server=" + targets[i].ip
		if !strings.Contains(body, want) {
			t.Errorf("boot script from %s does not carry %q:\n%s", srv.Addr, want, body)
		}
	}
}
