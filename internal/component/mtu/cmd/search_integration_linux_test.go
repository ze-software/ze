//go:build integration && linux

// Design: docs/architecture/diagnostics/path-mtu.md -- the search proven against a Linux router
//
// VALIDATES: AC-1 and AC-8 (a clamped path measures the clamp, confirmed on
// the wire and reported via ICMP), AC-9 (with the router's ICMP errors
// dropped, the ladder and the number line find the clamp and the method
// names a search), AC-11 (exhaustive bypasses a poisoned cache and disagrees
// with it). The Linux router in the middle namespace is the other
// implementation, as in probe-df-clamped-path.
// PREVENTS: a wire prober that matches another flow's reply or refusal, a
// search that reports the cache as a measurement, and a filtered path that
// reads as unmeasurable.
//
// Topology, three namespaces joined by two veth pairs:
//
//	sender ----sr0/rs0---- router ----rf0/fr0---- far
//	10.99.1.1     1500    10.99.1.2  1400      10.99.2.2
//
// The helpers below are copied from
// internal/core/probe/errqueue_integration_linux_test.go, where they are
// unexported in a test file and cannot be imported. Every test skips,
// never fails, when the namespaces or the raw socket are out of reach:
// the QEMU runner (./le qemu all-tests) is where they run for real.

package cmd

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/probe"
)

const (
	pathLinkMTU  = 1500
	pathClampMTU = 1400
	// pathProbeWait is the wait for one probe on a veth path, where an
	// answer arrives in microseconds: the filtered test loses 23 probes on
	// purpose, and 2 s each would put the package near the runner's limit.
	pathProbeWait = 300 * time.Millisecond
)

var (
	senderAddr4   = netip.MustParseAddr("10.99.1.1")
	routerNear4   = netip.MustParseAddr("10.99.1.2")
	routerFarAddr = netip.MustParseAddr("10.99.2.1")
	farAddr4      = netip.MustParseAddr("10.99.2.2")
	senderAddr6   = netip.MustParseAddr("fd99:1::1")
	routerNear6   = netip.MustParseAddr("fd99:1::2")
	routerFar6    = netip.MustParseAddr("fd99:2::1")
	farAddr6      = netip.MustParseAddr("fd99:2::2")
)

// clampedPath is the three-namespace topology and the handles to drive it.
type clampedPath struct {
	orig, sender, router, far             netns.NsHandle
	senderHandle, routerHandle, farHandle *netlink.Handle
}

// withClampedPath builds the topology, leaves the calling goroutine locked
// to its thread and inside the sender namespace, runs fn, and tears
// everything down. It skips, never fails, when a namespace or a link cannot
// be created.
func withClampedPath(t *testing.T, fn func(p *clampedPath)) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root: network namespaces, veth links and raw sockets")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	orig, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: current namespace: %v", err)
	}
	t.Cleanup(func() {
		if setErr := netns.Set(orig); setErr != nil {
			t.Errorf("restore namespace: %v", setErr)
		}
		orig.Close() //nolint:errcheck // best-effort cleanup
	})
	p := &clampedPath{orig: orig}
	base := nsBaseName(t.Name())
	p.sender = newNamespace(t, orig, base+"s")
	p.router = newNamespace(t, orig, base+"r")
	p.far = newNamespace(t, orig, base+"f")

	p.senderHandle = handleAt(t, p.sender)
	p.routerHandle = handleAt(t, p.router)
	p.farHandle = handleAt(t, p.far)

	addVeth(t, p.senderHandle, "sr0", "rs0", p.router)
	addVeth(t, p.routerHandle, "rf0", "fr0", p.far)

	configureLink(t, p.senderHandle, "sr0", pathLinkMTU, senderAddr4, senderAddr6)
	configureLink(t, p.routerHandle, "rs0", pathLinkMTU, routerNear4, routerNear6)
	configureLink(t, p.routerHandle, "rf0", pathClampMTU, routerFarAddr, routerFar6)
	configureLink(t, p.farHandle, "fr0", pathClampMTU, farAddr4, farAddr6)

	addRoute(t, p.senderHandle, "sr0", netip.MustParsePrefix("10.99.2.0/24"), routerNear4)
	addRoute(t, p.senderHandle, "sr0", netip.MustParsePrefix("fd99:2::/64"), routerNear6)
	addRoute(t, p.farHandle, "fr0", netip.MustParsePrefix("10.99.1.0/24"), routerFarAddr)
	addRoute(t, p.farHandle, "fr0", netip.MustParsePrefix("fd99:1::/64"), routerFar6)

	enableForwarding(t, p.router)

	if setErr := netns.Set(p.sender); setErr != nil {
		t.Fatalf("enter sender namespace: %v", setErr)
	}
	fn(p)
}

// nsBaseName derives a short, valid namespace name prefix from a test name.
func nsBaseName(testName string) string {
	name := strings.NewReplacer("/", "", "_", "", "Test", "", "Search", "").Replace(testName)
	name = strings.ToLower(name)
	if len(name) > 10 {
		name = name[:10]
	}
	return "zemtu" + name
}

func newNamespace(t *testing.T, orig netns.NsHandle, name string) netns.NsHandle {
	t.Helper()
	ns, err := netns.NewNamed(name)
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: create namespace %s: %v", name, err)
	}
	// NewNamed moves the thread into the new namespace; go back at once.
	if setErr := netns.Set(orig); setErr != nil {
		t.Fatalf("return to original namespace: %v", setErr)
	}
	t.Cleanup(func() {
		ns.Close()              //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(name) //nolint:errcheck // best-effort cleanup
	})
	return ns
}

func handleAt(t *testing.T, ns netns.NsHandle) *netlink.Handle {
	t.Helper()
	h, err := netlink.NewHandleAt(ns)
	if err != nil {
		t.Fatalf("netlink handle: %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

// addVeth creates a veth pair with name in h's namespace and peer moved
// into peerNS.
func addVeth(t *testing.T, h *netlink.Handle, name, peer string, peerNS netns.NsHandle) {
	t.Helper()
	veth := &netlink.Veth{PeerName: peer, PeerNamespace: netlink.NsFd(peerNS)}
	veth.Name = name
	veth.MTU = pathLinkMTU
	if err := h.LinkAdd(veth); err != nil {
		t.Skipf("add veth %s/%s (needs CAP_NET_ADMIN): %v", name, peer, err)
	}
}

// configureLink sets the MTU, adds one address of each family and brings
// the link up.
func configureLink(t *testing.T, h *netlink.Handle, name string, mtu int, addr4, addr6 netip.Addr) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	if err := h.LinkSetMTU(link, mtu); err != nil {
		t.Fatalf("set %s mtu %d: %v", name, mtu, err)
	}
	addAddr(t, h, name, addr4, 24)
	addAddr(t, h, name, addr6, 64)
	if err := h.LinkSetUp(link); err != nil {
		t.Fatalf("up %s: %v", name, err)
	}
}

// addAddr adds addr/bits to the named link. An IPv6 address skips DAD so
// it is usable at once.
func addAddr(t *testing.T, h *netlink.Handle, name string, addr netip.Addr, bits int) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	a := &netlink.Addr{IPNet: &net.IPNet{IP: addr.AsSlice(), Mask: net.CIDRMask(bits, addr.BitLen())}}
	if addr.Is6() {
		a.Flags = unix.IFA_F_NODAD
	}
	if err := h.AddrAdd(link, a); err != nil {
		t.Fatalf("add %s to %s: %v", a.IPNet, name, err)
	}
}

func addRoute(t *testing.T, h *netlink.Handle, name string, dst netip.Prefix, via netip.Addr) {
	t.Helper()
	link, err := h.LinkByName(name)
	if err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: dst.Addr().AsSlice(), Mask: net.CIDRMask(dst.Bits(), dst.Addr().BitLen())},
		Gw:        via.AsSlice(),
	}
	if err := h.RouteAdd(route); err != nil {
		t.Fatalf("add route %s via %s: %v", dst, via, err)
	}
}

// enableSysctlIn writes 1 to one /proc/sys switch from inside ns. /proc/sys
// is the calling thread's namespace, so the thread steps in and back out.
func enableSysctlIn(t *testing.T, ns netns.NsHandle, path string) {
	t.Helper()
	orig, err := netns.Get()
	if err != nil {
		t.Fatalf("current namespace: %v", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort cleanup
	if setErr := netns.Set(ns); setErr != nil {
		t.Fatalf("enter namespace: %v", setErr)
	}
	defer func() {
		if setErr := netns.Set(orig); setErr != nil {
			t.Fatalf("leave namespace: %v", setErr)
		}
	}()
	if err := os.WriteFile(path, []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// enableForwarding turns the router namespace into a router.
func enableForwarding(t *testing.T, router netns.NsHandle) {
	t.Helper()
	enableSysctlIn(t, router, "/proc/sys/net/ipv4/ip_forward")
	enableSysctlIn(t, router, "/proc/sys/net/ipv6/conf/all/forwarding")
}

// setFarLinkMTU moves the clamp: both ends of the far link take mtu.
func (p *clampedPath) setFarLinkMTU(t *testing.T, mtu int) {
	t.Helper()
	for _, end := range []struct {
		h    *netlink.Handle
		name string
	}{{p.routerHandle, "rf0"}, {p.farHandle, "fr0"}} {
		link, err := end.h.LinkByName(end.name)
		if err != nil {
			t.Fatalf("link %s: %v", end.name, err)
		}
		if err := end.h.LinkSetMTU(link, mtu); err != nil {
			t.Fatalf("set %s mtu %d: %v", end.name, mtu, err)
		}
	}
}

// dropRouterICMPErrors makes every ICMP error the router sends vanish
// before the sender's stack reads it: strict reverse-path filtering on the
// sender plus a blackhole route for the router's near address, so a
// datagram FROM that address fails the source check and is dropped in
// ip_route_input, before the error reaches the probe socket or the
// path-MTU cache. Echo replies come from the far address and pass; the
// gateway is still resolved on the link for sending. This is the simplest
// filter that needs no firewall binary in the namespace.
func (p *clampedPath) dropRouterICMPErrors(t *testing.T) {
	t.Helper()
	enableSysctlIn(t, p.sender, "/proc/sys/net/ipv4/conf/all/rp_filter")
	enableSysctlIn(t, p.sender, "/proc/sys/net/ipv4/conf/sr0/rp_filter")
	route := &netlink.Route{
		Dst:  &net.IPNet{IP: routerNear4.AsSlice(), Mask: net.CIDRMask(32, 32)},
		Type: unix.RTN_BLACKHOLE,
	}
	if err := p.senderHandle.RouteAdd(route); err != nil {
		t.Fatalf("add blackhole for %s: %v", routerNear4, err)
	}
}

// openPathProber opens the wire prober toward the far address in the
// sender namespace, skipping when the raw socket is refused.
func openPathProber(t *testing.T, df probe.DFMode) *wireProber {
	t.Helper()
	w, err := openWireProber(context.Background(), farAddr4, df)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skipf("requires CAP_NET_RAW: %v", err)
		}
		t.Fatalf("openWireProber(%v): %v", df, err)
	}
	w.wait = pathProbeWait
	t.Cleanup(func() { w.close() }) //nolint:errcheck // best-effort cleanup
	return w
}

// TestSearchClampedPathViaICMP (AC-1, AC-8): the router reports 1400, the
// search confirms it on the wire and reports it via ICMP.
func TestSearchClampedPathViaICMP(t *testing.T) {
	withClampedPath(t, func(_ *clampedPath) {
		w := openPathProber(t, probe.DFHonorCache)
		res, err := searchPathMTU(context.Background(), w, farAddr4, false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if res.outcome != searchMeasured {
			t.Fatalf("outcome %v, want measured", res.outcome)
		}
		if res.pathMTU != pathClampMTU {
			t.Errorf("path MTU %d, want the clamp %d", res.pathMTU, pathClampMTU)
		}
		if res.method != searchViaICMP {
			t.Errorf("method %q, want %q", res.method, searchViaICMP)
		}
	})
}

// TestSearchFilteredPathLadderThenBisect (AC-9): with the router's ICMP
// errors dropped, no figure is ever reported, the ladder and the number
// line find the clamp, and the method says the answer came from a search.
func TestSearchFilteredPathLadderThenBisect(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		p.dropRouterICMPErrors(t)
		w := openPathProber(t, probe.DFHonorCache)
		res, err := searchPathMTU(context.Background(), w, farAddr4, false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if res.outcome != searchMeasured {
			t.Fatalf("outcome %v, want measured", res.outcome)
		}
		if res.pathMTU != pathClampMTU {
			t.Errorf("path MTU %d, want the clamp %d", res.pathMTU, pathClampMTU)
		}
		if res.method != searchICMPFiltered {
			t.Errorf("method %q, want %q", res.method, searchICMPFiltered)
		}
		for _, r := range res.records {
			if r.answer.outcome == probeRefusedReported && !r.answer.local {
				t.Fatalf("a router report reached the socket through the filter: %+v", r)
			}
		}
	})
}

// TestSearchExhaustiveBypassesPoisonedCache (AC-11): honor-cache learns 1400,
// the clamp is lifted, the kernel still holds 1400, and an exhaustive search
// in bypass mode answers 1500 while the cache says 1400.
func TestSearchExhaustiveBypassesPoisonedCache(t *testing.T) {
	withClampedPath(t, func(p *clampedPath) {
		honor := openPathProber(t, probe.DFHonorCache)
		res, err := searchPathMTU(context.Background(), honor, farAddr4, false)
		if err != nil {
			t.Fatalf("poisoning search: %v", err)
		}
		if res.pathMTU != pathClampMTU {
			t.Fatalf("poisoning search measured %d, want %d", res.pathMTU, pathClampMTU)
		}

		p.setFarLinkMTU(t, pathLinkMTU)

		estimate, err := probe.KernelPathMTU(context.Background(), farAddr4)
		if err != nil {
			t.Fatalf("KernelPathMTU: %v", err)
		}
		if estimate != pathClampMTU {
			t.Fatalf("kernel estimate %d after the clamp was lifted, want the cached %d", estimate, pathClampMTU)
		}

		bypass := openPathProber(t, probe.DFBypassCache)
		exhaustive, err := searchPathMTU(context.Background(), bypass, farAddr4, true)
		if err != nil {
			t.Fatalf("exhaustive search: %v", err)
		}
		if exhaustive.outcome != searchMeasured {
			t.Fatalf("exhaustive outcome %v, want measured", exhaustive.outcome)
		}
		if exhaustive.pathMTU != pathLinkMTU {
			t.Errorf("exhaustive path MTU %d, want the lifted %d", exhaustive.pathMTU, pathLinkMTU)
		}
		if exhaustive.method != searchForcedFullSearch {
			t.Errorf("exhaustive method %q, want %q", exhaustive.method, searchForcedFullSearch)
		}
		if exhaustive.pathMTU == estimate {
			t.Errorf("the exhaustive answer %d equals the poisoned cache", exhaustive.pathMTU)
		}
	})
}
