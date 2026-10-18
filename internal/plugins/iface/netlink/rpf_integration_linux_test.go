//go:build integration && linux

// Design: docs/features/interfaces.md -- IPv6 reverse-path check. These tests
// drive ReplaceIPv6RPF against a real kernel and send real datagrams from a
// second network namespace, so the verdict is the kernel's, not the rule
// builder's.

package ifacenetlink

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// rpfLab is a router namespace holding r0 (2001:db8:1::1/64) and a dummy
// "other" (2001:db8:2::1/64), and a peer namespace holding p0, the other end
// of r0. The peer owns three sources:
//
//	2001:db8:1::2  valid: the route back leaves through r0
//	2001:db8:2::5  routed, but through "other": strict drops, loose passes
//	2001:db8:9::5  no route at all: both modes drop
type rpfLab struct {
	t        *testing.T
	router   netns.NsHandle
	peer     netns.NsHandle
	listener *net.UDPConn
}

const (
	rpfLabValid     = "2001:db8:1::2"
	rpfLabElsewhere = "2001:db8:2::5"
	rpfLabUnrouted  = "2001:db8:9::5"
	rpfLabRouter    = "2001:db8:1::1"
	rpfLabPort      = 40179
)

// newRPFLab builds the lab and leaves the calling thread in the router
// namespace. Skips without CAP_NET_ADMIN (ai/rules/platform-linux.md).
func newRPFLab(t *testing.T) *rpfLab {
	t.Helper()
	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}
	router, err := netns.New()
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}
	peer, err := netns.New()
	if err != nil {
		t.Fatalf("create peer namespace: %v", err)
	}
	lab := &rpfLab{t: t, router: router, peer: peer}
	t.Cleanup(func() {
		if lab.listener != nil {
			lab.listener.Close() //nolint:errcheck // best-effort cleanup
		}
		if err := netns.Set(origNS); err != nil {
			t.Errorf("restore namespace: %v", err)
		}
		origNS.Close() //nolint:errcheck // best-effort cleanup
		router.Close() //nolint:errcheck // best-effort cleanup
		peer.Close()   //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	lab.enter(router)
	veth := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "r0"}, PeerName: "p0", PeerNamespace: netlink.NsFd(int(peer))}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add veth: %v", err)
	}
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "other"}}); err != nil {
		t.Fatalf("add dummy: %v", err)
	}
	lab.up("lo")
	lab.addr("r0", rpfLabRouter+"/64")
	lab.addr("other", "2001:db8:2::1/64")
	lab.up("r0")
	lab.up("other")

	lab.enter(peer)
	lab.up("lo")
	lab.addr("p0", rpfLabValid+"/64")
	lab.addr("p0", rpfLabElsewhere+"/128")
	lab.addr("p0", rpfLabUnrouted+"/128")
	lab.up("p0")
	routerLink := lab.link("p0")
	// The router never answers, so the peer resolves it once, statically.
	lab.enter(router)
	routerMAC := lab.link("r0").Attrs().HardwareAddr
	lab.enter(peer)
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: routerLink.Attrs().Index, Family: unix.AF_INET6, State: netlink.NUD_PERMANENT,
		IP: net.ParseIP(rpfLabRouter), HardwareAddr: routerMAC,
	}); err != nil {
		t.Fatalf("add neighbor: %v", err)
	}

	lab.enter(router)
	listener, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP(rpfLabRouter), Port: rpfLabPort})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	lab.listener = listener
	return lab
}

func (l *rpfLab) enter(ns netns.NsHandle) {
	l.t.Helper()
	if err := netns.Set(ns); err != nil {
		l.t.Fatalf("enter namespace: %v", err)
	}
}

func (l *rpfLab) link(name string) netlink.Link {
	l.t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		l.t.Fatalf("link %s: %v", name, err)
	}
	return link
}

func (l *rpfLab) up(name string) {
	l.t.Helper()
	if err := netlink.LinkSetUp(l.link(name)); err != nil {
		l.t.Fatalf("up %s: %v", name, err)
	}
}

func (l *rpfLab) addr(name, cidr string) {
	l.t.Helper()
	a, err := netlink.ParseAddr(cidr)
	if err != nil {
		l.t.Fatalf("parse %s: %v", cidr, err)
	}
	a.Flags = unix.IFA_F_NODAD
	if err := netlink.AddrAdd(l.link(name), a); err != nil {
		l.t.Fatalf("addr %s on %s: %v", cidr, name, err)
	}
}

// arrives sends one datagram from source in the peer namespace and reports
// whether the router's socket received it. The thread is back in the router
// namespace on return.
func (l *rpfLab) arrives(source string) bool {
	l.t.Helper()
	l.enter(l.peer)
	conn, err := net.DialUDP("udp6",
		&net.UDPAddr{IP: net.ParseIP(source)},
		&net.UDPAddr{IP: net.ParseIP(rpfLabRouter), Port: rpfLabPort})
	l.enter(l.router)
	if err != nil {
		l.t.Fatalf("dial from %s: %v", source, err)
	}
	defer conn.Close() //nolint:errcheck // best-effort cleanup
	if _, err := conn.Write([]byte(source)); err != nil {
		l.t.Fatalf("send from %s: %v", source, err)
	}

	buf := make([]byte, 64)
	for {
		if err := l.listener.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			l.t.Fatalf("deadline: %v", err)
		}
		n, from, err := l.listener.ReadFromUDPAddrPort(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return false
		}
		if err != nil {
			l.t.Fatalf("read: %v", err)
		}
		if from.Addr() == netip.MustParseAddr(source) && string(buf[:n]) == source {
			return true
		}
	}
}

func (l *rpfLab) expect(label string, want map[string]bool) {
	l.t.Helper()
	for _, source := range []string{rpfLabValid, rpfLabElsewhere, rpfLabUnrouted} {
		if got := l.arrives(source); got != want[source] {
			l.t.Errorf("%s: datagram from %s arrived = %v, want %v", label, source, got, want[source])
		}
	}
}

// VALIDATES: strict drops a source whose route leaves another interface and a
// source with no route, loose drops only the unrouted one, a valid source
// passes both, and replacing the set with nothing removes the check.
// METHOD: one lab, four sets in turn; the first run with no check proves every
// source reaches the router, so a drop afterwards is the check's.
func TestIntegrationIPv6RPF(t *testing.T) {
	lab := newRPFLab(t)
	b := &netlinkBackend{}
	all := map[string]bool{rpfLabValid: true, rpfLabElsewhere: true, rpfLabUnrouted: true}

	lab.expect("no check", all)

	if err := b.ReplaceIPv6RPF([]iface.IPv6RPFCheck{{Interface: "r0", Mode: iface.IPv6RPFStrict}}); err != nil {
		t.Fatalf("strict: %v", err)
	}
	lab.expect("strict", map[string]bool{rpfLabValid: true})

	if err := b.ReplaceIPv6RPF([]iface.IPv6RPFCheck{{Interface: "r0", Mode: iface.IPv6RPFLoose}}); err != nil {
		t.Fatalf("loose: %v", err)
	}
	lab.expect("loose", map[string]bool{rpfLabValid: true, rpfLabElsewhere: true})

	if err := b.ReplaceIPv6RPF([]iface.IPv6RPFCheck{{Interface: "other", Mode: iface.IPv6RPFStrict}}); err != nil {
		t.Fatalf("other interface: %v", err)
	}
	lab.expect("check on another interface only", all)

	if err := b.ReplaceIPv6RPF(nil); err != nil {
		t.Fatalf("remove: %v", err)
	}
	lab.expect("removed", all)
	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("open nftables: %v", err)
	}
	tables, err := conn.ListTablesOfFamily(nftables.TableFamilyIPv6)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for _, table := range tables {
		if table.Name == rpfTableName {
			t.Errorf("table %s still present after the empty set", rpfTableName)
		}
	}
}
