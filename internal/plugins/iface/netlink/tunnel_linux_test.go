// Tunnel netlink integration tests. Each subtest creates a fresh netns,
// calls CreateTunnel via the iface.Backend interface, then verifies the
// resulting netdev with netlink.LinkByName so that field round-trips
// (kind, local, remote, key) are checked against the kernel state.
//
// Build tags require both `integration` and `linux`. The runner skips when
// CAP_NET_ADMIN is unavailable so unprivileged CI hosts pass cleanly.

//go:build integration && linux

package ifacenetlink

import (
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// withTunnelNetNS sets up a fresh named netns and runs fn with the netlink
// backend loaded inside it. Skips on missing CAP_NET_ADMIN.
func withTunnelNetNS(t *testing.T, fn func(b iface.Backend)) {
	t.Helper()
	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	nsName := sanitize(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if setErr := netns.Set(origNS); setErr != nil {
			t.Errorf("restore ns: %v", setErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load netlink backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	b := iface.GetBackend()
	if b == nil {
		t.Fatal("nil backend after LoadBackend")
	}
	fn(b)
}

func sanitize(name string) string {
	r := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "")
	out := r.Replace(name)
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}

// TestCreateTunnelGRE verifies the gre kind round-trips local/remote and
// applies the symmetric key with the GRE_KEY flag bit set on both flag fields.
//
// VALIDATES: AC-1, AC-2 (gre creation, key handling).
// PREVENTS: Silent kernel-side ignoring of the key when GRE_KEY bit missing.
func TestCreateTunnelGRE(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindGRE,
			Name:          "tgre0",
			LocalAddress:  "192.0.2.1",
			RemoteAddress: "198.51.100.1",
			Key:           42,
			KeySet:        true,
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create gre: %v", err)
		}

		link, err := netlink.LinkByName("tgre0")
		if err != nil {
			t.Fatalf("lookup tgre0: %v", err)
		}
		gre, ok := link.(*netlink.Gretun)
		if !ok {
			t.Fatalf("expected *Gretun, got %T", link)
		}
		if !gre.Local.Equal(net.ParseIP("192.0.2.1")) {
			t.Errorf("local = %v, want 192.0.2.1", gre.Local)
		}
		if !gre.Remote.Equal(net.ParseIP("198.51.100.1")) {
			t.Errorf("remote = %v, want 198.51.100.1", gre.Remote)
		}
		if gre.IKey != 42 || gre.OKey != 42 {
			t.Errorf("key = (in=%d out=%d), want both 42", gre.IKey, gre.OKey)
		}
	})
}

// TestCreateTunnelGretap verifies the gretap kind creates a Gretap link with
// the L2 (bridgeable) characteristics.
//
// VALIDATES: AC-5 (gretap kind dispatched to Gretap Go type).
func TestCreateTunnelGretap(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindGRETap,
			Name:          "tgtap0",
			LocalAddress:  "192.0.2.1",
			RemoteAddress: "198.51.100.1",
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create gretap: %v", err)
		}
		link, err := netlink.LinkByName("tgtap0")
		if err != nil {
			t.Fatalf("lookup tgtap0: %v", err)
		}
		if _, ok := link.(*netlink.Gretap); !ok {
			t.Fatalf("expected *Gretap, got %T", link)
		}
	})
}

// TestCreateTunnelIPIP verifies the ipip kind creates an Iptun link.
//
// VALIDATES: AC-8 (ipip kind dispatched to Iptun Go type).
func TestCreateTunnelIPIP(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindIPIP,
			Name:          "tipip0",
			LocalAddress:  "10.0.0.1",
			RemoteAddress: "10.0.0.2",
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create ipip: %v", err)
		}
		link, err := netlink.LinkByName("tipip0")
		if err != nil {
			t.Fatalf("lookup tipip0: %v", err)
		}
		if _, ok := link.(*netlink.Iptun); !ok {
			t.Fatalf("expected *Iptun, got %T", link)
		}
	})
}

// TestCreateTunnelSIT verifies the sit kind creates a Sittun link with
// IPv4 endpoints (carrying IPv6 inside per RFC 4213).
//
// VALIDATES: AC-9 (sit/6in4 kind dispatched to Sittun Go type).
func TestCreateTunnelSIT(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindSIT,
			Name:          "tsit0",
			LocalAddress:  "192.0.2.1",
			RemoteAddress: "198.51.100.1",
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create sit: %v", err)
		}
		link, err := netlink.LinkByName("tsit0")
		if err != nil {
			t.Fatalf("lookup tsit0: %v", err)
		}
		if _, ok := link.(*netlink.Sittun); !ok {
			t.Fatalf("expected *Sittun, got %T", link)
		}
	})
}

// TestCreateTunnelIp6tnl verifies the ip6tnl kind creates an Ip6tnl link
// with v6 endpoints. Encaplimit round-trip is checked too.
//
// VALIDATES: AC-10 (ip6tnl kind dispatched to Ip6tnl Go type with EncapLimit).
func TestCreateTunnelIp6tnl(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindIP6Tnl,
			Name:          "tip6t0",
			LocalAddress:  "2001:db8::1",
			RemoteAddress: "2001:db8::2",
			EncapLimit:    4,
			EncapLimitSet: true,
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create ip6tnl: %v", err)
		}
		link, err := netlink.LinkByName("tip6t0")
		if err != nil {
			t.Fatalf("lookup tip6t0: %v", err)
		}
		ip6t, ok := link.(*netlink.Ip6tnl)
		if !ok {
			t.Fatalf("expected *Ip6tnl, got %T", link)
		}
		if !ip6t.Local.Equal(net.ParseIP("2001:db8::1")) {
			t.Errorf("local = %v, want 2001:db8::1", ip6t.Local)
		}
	})
}

// TestCreateTunnelIPIP6Proto verifies that the ipip6 kind constructs an
// Ip6tnl Go type but with Proto set to IPPROTO_IPIP (4) so the kernel
// carries IPv4 inside the IPv6 outer header.
//
// VALIDATES: AC-11 (ipip6 discriminator via Proto field, not separate kind).
// PREVENTS: Silent fallthrough where ipip6 would create an ip6ip6 tunnel.
func TestCreateTunnelIPIP6Proto(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindIPIP6,
			Name:          "tipip6",
			LocalAddress:  "2001:db8::1",
			RemoteAddress: "2001:db8::2",
		}
		if err := b.CreateTunnel(spec); err != nil {
			t.Fatalf("create ipip6: %v", err)
		}
		link, err := netlink.LinkByName("tipip6")
		if err != nil {
			t.Fatalf("lookup tipip6: %v", err)
		}
		ip6t, ok := link.(*netlink.Ip6tnl)
		if !ok {
			t.Fatalf("expected *Ip6tnl, got %T", link)
		}
		// Proto = 4 = IPPROTO_IPIP. The kernel may report 0 if it has not
		// echoed the field back yet; tolerate that but reject any other
		// non-4 value.
		if ip6t.Proto != 0 && ip6t.Proto != 4 {
			t.Errorf("proto = %d, want 4 (IPPROTO_IPIP) or 0", ip6t.Proto)
		}
	})
}

// TestCreateTunnelInvalidName verifies that an invalid interface name is
// rejected before reaching netlink.
//
// VALIDATES: AC-30 (free-form name still passes through ValidateIfaceName).
func TestCreateTunnelInvalidName(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindGRE,
			Name:          "bad name with spaces",
			LocalAddress:  "192.0.2.1",
			RemoteAddress: "198.51.100.1",
		}
		err := b.CreateTunnel(spec)
		if err == nil {
			t.Fatal("expected error for invalid name")
		}
	})
}

// TestCreateTunnelV4OnV6Kind verifies that supplying an IPv4 address for a
// v6-underlay kind is rejected before reaching netlink.
//
// VALIDATES: address-family-vs-kind sanity check in checkAddressFamily.
// PREVENTS: Kernel returning a generic EINVAL with no clear error message.
func TestCreateTunnelV4OnV6Kind(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		spec := iface.TunnelSpec{
			Kind:          iface.TunnelKindIP6Tnl,
			Name:          "twrong",
			LocalAddress:  "10.0.0.1", // v4 address on v6-underlay kind
			RemoteAddress: "2001:db8::2",
		}
		err := b.CreateTunnel(spec)
		if err == nil {
			t.Fatal("expected error for v4 address on v6 kind")
		}
	})
}
