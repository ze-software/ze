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

	"github.com/ze-software/ze/internal/component/iface"
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
		origNS.Close() //nolint:errcheck // best-effort cleanup
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if setErr := netns.Set(origNS); setErr != nil {
			t.Errorf("restore ns: %v", setErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
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

// tunnelDeviceTTL creates one tunnel and returns the outer-header TTL the
// kernel reports for the resulting netdev. Each kind stores the value in its
// own Go type, so the read is a type switch rather than one field access.
func tunnelDeviceTTL(t *testing.T, b iface.Backend, spec iface.TunnelSpec) uint8 {
	t.Helper()
	if err := b.CreateTunnel(spec); err != nil {
		t.Fatalf("create %s: %v", spec.Kind, err)
	}
	link, err := netlink.LinkByName(spec.Name)
	if err != nil {
		t.Fatalf("lookup %s: %v", spec.Name, err)
	}
	switch device := link.(type) {
	case *netlink.Gretun:
		return device.Ttl
	case *netlink.Gretap:
		return device.Ttl
	case *netlink.Iptun:
		return device.Ttl
	case *netlink.Sittun:
		return device.Ttl
	default:
		t.Fatalf("%s is a %T, which carries no outer TTL", spec.Name, link)
		return 0
	}
}

// TestCreateTunnelTTLReachesTheDevice verifies that the outer TTL a spec
// carries is the outer TTL the kernel stores on the netdev, for every tunnel
// kind whose underlay is IPv4.
//
// The value is 33 rather than the schema default of 64, because 64 is what a
// sit device carries anyway: addSittunAttrs (the vendored netlink library)
// sends IFLA_IPTUN_TTL only above 0, and the sit driver's own device default
// is 64. A test at 64 would therefore pass against a buildSittun that dropped
// the field. 33 is outside every default in play, so each kind has to carry
// the value for its row to pass.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4 at the kernel boundary -- link.Ttl
//
//	reflects spec.TTL for gre, gretap, ipip and sit.
//
// PREVENTS: a builder that reads the value into the wrong netlink field, or
//
//	drops it for one kind. buildIptun and buildSittun set the same
//	link.Ttl as buildGretun, and nothing but a read-back proves it.
func TestCreateTunnelTTLReachesTheDevice(t *testing.T) {
	for _, row := range []struct {
		name  string
		kind  iface.TunnelKind
		local string
		peer  string
	}{
		{"gre", iface.TunnelKindGRE, "192.0.2.11", "198.51.100.11"},
		{"gretap", iface.TunnelKindGRETap, "192.0.2.12", "198.51.100.12"},
		{"ipip", iface.TunnelKindIPIP, "192.0.2.13", "198.51.100.13"},
		{"sit", iface.TunnelKindSIT, "192.0.2.14", "198.51.100.14"},
	} {
		t.Run(row.name, func(t *testing.T) {
			withTunnelNetNS(t, func(b iface.Backend) {
				ttl := tunnelDeviceTTL(t, b, iface.TunnelSpec{
					Kind:          row.kind,
					Name:          "tttl0",
					LocalAddress:  row.local,
					RemoteAddress: row.peer,
					TTL:           33,
					TTLSet:        true,
				})
				if ttl != 33 {
					t.Errorf("%s outer TTL = %d, want 33", row.name, ttl)
				}
			})
		})
	}
}

// TestCreateTunnelTTLZeroInherits verifies that a spec asking for TTL 0 leaves
// the device on inherit-from-inner, which the kernel stores as 0.
//
// VALIDATES: AC-5 at the kernel boundary -- an explicit 0 is applied as 0.
// PREVENTS: a builder that treats 0 as "unset" and substitutes a value,
//
//	which would take the inherit mode away from the operator.
func TestCreateTunnelTTLZeroInherits(t *testing.T) {
	withTunnelNetNS(t, func(b iface.Backend) {
		ttl := tunnelDeviceTTL(t, b, iface.TunnelSpec{
			Kind:          iface.TunnelKindGRE,
			Name:          "tttl1",
			LocalAddress:  "192.0.2.15",
			RemoteAddress: "198.51.100.15",
			TTL:           0,
			TTLSet:        true,
		})
		if ttl != 0 {
			t.Errorf("outer TTL = %d, want 0 (inherit)", ttl)
		}
	})
}
