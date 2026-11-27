// VALIDATES: ensureNetwork keys off install-server reachability, not the mere
// presence of a default route.
// PREVENTS: regression of the dual-homed-target stall where the kernel's
// ip=dhcp configured a corporate NIC, leaving a default route that does not
// reach the install server, and the installer never brought up the install NIC.

package disk

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func swapNetProbes(probe func(string) bool, bringUp func() error) func() {
	origProbe, origBringUp := probeServer, bringUpAllNICs
	probeServer, bringUpAllNICs = probe, bringUp
	return func() { probeServer, bringUpAllNICs = origProbe, origBringUp }
}

// When the install server is already reachable (kernel configured the right
// NIC), ensureNetwork must not touch the NICs.
func TestEnsureNetworkSkipsDHCPWhenServerReachable(t *testing.T) {
	restore := swapNetProbes(
		func(string) bool { return true },
		func() error { t.Fatal("bringUpAllNICs called although server was reachable"); return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", "", 5); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
}

// Pins the dual-homed-target regression: a default route exists (kernel grabbed
// a corporate NIC) but the install server is unreachable. ensureNetwork must
// bring up all NICs and re-probe rather than trusting the stray default route.
func TestEnsureNetworkBringsUpNICsWhenServerUnreachable(t *testing.T) {
	probes := 0
	broughtUp := false
	restore := swapNetProbes(
		func(string) bool { probes++; return probes > 1 }, // unreachable first, reachable after bring-up
		func() error { broughtUp = true; return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", "", 5); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
	if !broughtUp {
		t.Fatal("bringUpAllNICs not called although the install server was unreachable")
	}
}

// ze.wait=0 is an explicit opt-out: no probe, no NIC changes.
func TestEnsureNetworkWaitZeroSkipsEverything(t *testing.T) {
	restore := swapNetProbes(
		func(string) bool { t.Fatal("probeServer called with ze.wait=0"); return false },
		func() error { t.Fatal("bringUpAllNICs called with ze.wait=0"); return nil },
	)
	defer restore()

	if err := ensureNetwork("198.19.255.1", "80", "", 0); err != nil {
		t.Fatalf("ensureNetwork: %v", err)
	}
}

type capturedLogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

type capturingSlogHandler struct {
	mu      sync.Mutex
	records []capturedLogRecord
}

func (h *capturingSlogHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (h *capturingSlogHandler) Handle(_ context.Context, r slog.Record) error {
	h.record(&r)
	return nil
}

func (h *capturingSlogHandler) record(r *slog.Record) {
	attrs := make(map[string]string)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	h.records = append(h.records, capturedLogRecord{
		level: r.Level,
		msg:   r.Message,
		attrs: attrs,
	})
	h.mu.Unlock()
}

func (h *capturingSlogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *capturingSlogHandler) WithGroup(string) slog.Handler { return h }

func (h *capturingSlogHandler) warnRecords() []capturedLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []capturedLogRecord
	for _, rec := range h.records {
		if rec.level == slog.LevelWarn {
			out = append(out, rec)
		}
	}
	return out
}

// VALIDATES: AC-4 -- fallbackDHCPIn emits a Warn log on the initrd console
// naming a NIC whose linkUp failed, why it matters, and the ze.mac= fix, while
// continuing to bring up and DHCP-lease a remaining healthy NIC without exiting
// PID 1.
// PREVENTS: a NIC bring-up failure in DHCP fallback hiding at Debug behind a
// generic "no NIC carrier detected" error on the serial console.
func TestFallbackDHCPWarnsOnLinkUpFailure(t *testing.T) {
	const (
		badNIC  = "eth0"
		goodNIC = "eth1"
	)

	sysnetDir := t.TempDir()
	for _, nic := range []struct {
		name    string
		carrier string
	}{
		{name: "lo", carrier: "1\n"},
		{name: badNIC, carrier: "0\n"},
		{name: goodNIC, carrier: "1\n"},
	} {
		nicDir := filepath.Join(sysnetDir, nic.name)
		if err := os.MkdirAll(nicDir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", nicDir, err)
		}
		if err := os.WriteFile(filepath.Join(nicDir, "carrier"), []byte(nic.carrier), 0o600); err != nil {
			t.Fatalf("write carrier for %s: %v", nic.name, err)
		}
	}

	origLinkUp := linkUp
	origDHCP := dhcpAcquireApply
	origLogger := slog.Default()
	t.Cleanup(func() {
		linkUp = origLinkUp
		dhcpAcquireApply = origDHCP
		slog.SetDefault(origLogger)
	})

	var goodBroughtUp, goodLeased bool
	linkUp = func(ifName string) error {
		switch ifName {
		case "lo":
			t.Errorf("linkUp called on loopback interface lo")
			return nil
		case badNIC:
			return errors.New("operation not supported")
		case goodNIC:
			goodBroughtUp = true
			return nil
		default:
			t.Errorf("unexpected interface %q in linkUp", ifName)
			return nil
		}
	}
	dhcpAcquireApply = func(ifName string) error {
		if ifName == goodNIC {
			goodLeased = true
			return nil
		}
		return errors.New("no carrier")
	}

	handler := &capturingSlogHandler{}
	slog.SetDefault(slog.New(handler))

	if err := fallbackDHCPIn(sysnetDir); err != nil {
		t.Fatalf("fallbackDHCPIn returned error despite healthy NIC %s: %v", goodNIC, err)
	}
	if !goodBroughtUp {
		t.Errorf("fallbackDHCPIn did not bring up healthy NIC %s after %s failed", goodNIC, badNIC)
	}
	if !goodLeased {
		t.Errorf("fallbackDHCPIn did not acquire DHCP lease on healthy NIC %s", goodNIC)
	}

	warns := handler.warnRecords()
	if len(warns) != 1 {
		t.Fatalf("Warn record count = %d, want 1 (records=%+v)", len(warns), warns)
	}
	warn := warns[0]
	if warn.attrs["iface"] != badNIC && !strings.Contains(warn.msg, badNIC) {
		t.Errorf("Warn does not name failing interface %q: msg=%q attrs=%v", badNIC, warn.msg, warn.attrs)
	}
	if !strings.Contains(warn.attrs["error"], "operation not supported") && !strings.Contains(warn.msg, "operation not supported") {
		t.Errorf("Warn does not carry linkUp error: msg=%q attrs=%v", warn.msg, warn.attrs)
	}
	msgLower := strings.ToLower(warn.msg)
	if !strings.Contains(msgLower, "link") {
		t.Errorf("Warn message does not state what failed (link bring-up): %q", warn.msg)
	}
	if !strings.Contains(msgLower, "carrier") && !strings.Contains(msgLower, "lease") {
		t.Errorf("Warn message does not state why it matters (carrier/lease): %q", warn.msg)
	}
	combined := warn.msg + " " + warn.attrs["fix"]
	if !strings.Contains(combined, "ze.mac=") {
		t.Errorf("Warn does not name the ze.mac= fix: msg=%q attrs=%v", warn.msg, warn.attrs)
	}
}
