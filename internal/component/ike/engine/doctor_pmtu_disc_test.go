package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// pmtuDiscTree builds a vpn ipsec section with three peers: v4copy (IPv4, df-bit left
// at its copy default), v4clear (IPv4, df-bit clear) and v6copy (IPv6, copy).
func pmtuDiscTree() *config.Tree {
	ipsecRoot := config.NewTree()
	ipsecRoot.Set("interface", "eth0")
	sts := ipsecRoot.GetOrCreateContainer("site-to-site")
	add := func(name, remote, df string) {
		peer := config.NewTree()
		peer.Set("connection-type", "initiate")
		peer.Set("remote-address", remote)
		if df != "" {
			peer.Set("df-bit", df)
		}
		sts.AddListEntry("peer", name, peer)
	}
	add("v4copy", "203.0.113.7", "")
	add("v4clear", "203.0.113.8", "clear")
	add("v6copy", "2001:db8::7", "copy")
	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)
	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// withNoPMTUDisc points procRoot at a tree whose net.ipv4.ip_no_pmtu_disc holds value,
// or at an empty tree when value is "".
func withNoPMTUDisc(t *testing.T, value string) {
	t.Helper()
	root := t.TempDir()
	if value != "" {
		path := filepath.Join(root, noPMTUDiscKey)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previous := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = previous })
}

// VALIDATES: with net.ipv4.ip_no_pmtu_disc set, the check names every IPv4 df-bit copy
// peer, and only those, with the sysctl value and the exact fix command.
// PREVENTS: a df-bit copy peer whose tunnel header the kernel writes with DF clear
// while nothing but `show vpn ipsec dataplane sa` says so.
func TestNoPMTUDiscDoctorNamesIPv4CopyPeers(t *testing.T) {
	withNoPMTUDisc(t, "1")
	diags := checkIPsecNoPMTUDisc(registry.DoctorCheckContext{Tree: pmtuDiscTree()})
	if len(diags) != 1 {
		t.Fatalf("ip_no_pmtu_disc 1 produced %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-ipsec-no-pmtu-disc" {
		t.Errorf("code is %q, want doctor-ipsec-no-pmtu-disc", diags[0].Code)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("severity is %q, want warning", diags[0].Severity)
	}
	msg := diags[0].Message
	for _, want := range []string{"is 1", "v4copy", "set sysctl setting net.ipv4.ip_no_pmtu_disc value 0"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
	for _, unwanted := range []string{"v4clear", "v6copy"} {
		if strings.Contains(msg, unwanted) {
			t.Errorf("message %q names %q, whose DF the sysctl does not change", msg, unwanted)
		}
	}
}

// VALIDATES: ip_no_pmtu_disc 0 leaves the configured DF treatment alone, so nothing is
// reported; an unreadable sysctl is reported as unknown, never as 0.
// PREVENTS: a warning on every healthy host, and a read failure taken as "not set".
func TestNoPMTUDiscDoctorZeroAndUnreadable(t *testing.T) {
	withNoPMTUDisc(t, "0")
	if diags := checkIPsecNoPMTUDisc(registry.DoctorCheckContext{Tree: pmtuDiscTree()}); len(diags) != 0 {
		t.Errorf("ip_no_pmtu_disc 0 produced %v, want nothing", diags)
	}
	withNoPMTUDisc(t, "")
	diags := checkIPsecNoPMTUDisc(registry.DoctorCheckContext{Tree: pmtuDiscTree()})
	if len(diags) != 1 {
		t.Fatalf("an unreadable sysctl produced %d diagnostics, want 1", len(diags))
	}
	if !strings.Contains(diags[0].Message, "cannot read net.ipv4.ip_no_pmtu_disc") {
		t.Errorf("message %q does not say the sysctl could not be read", diags[0].Message)
	}
}
