//go:build integration && linux

// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: nduseropt_linux.go -- the RTNLGRP_ND_USEROPT subscription under test

package ifacenetlink

import (
	"encoding/json"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestMonitorEmitsRDNSSFromAcceptedRA validates A-7 on the running kernel:
// the RDNSS option of an RA the kernel accepts reaches the bus as
// nd-user-option on the receiving device, and an RA arriving where accept_ra
// is 0 produces nothing. Method: a veth pair in a fresh namespace, Ze's own
// RA builder sending from one end over a raw ICMPv6 socket, the monitor
// listening on the other.
//
// PREVENTS: an RA RDNSS producer that never hears an option, because the
// monitor did not join RTNLGRP_ND_USEROPT or misread nduseroptmsg.
func TestMonitorEmitsRDNSSFromAcceptedRA(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const host, router = "rdnss-host", "rdnss-rtr"
	b := &netlinkBackend{}
	if err := b.CreateVeth(host, router); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.DeleteInterface(host); err != nil {
			t.Error(err)
		}
	})
	writeSysctl(t, "net/ipv6/conf/all/accept_dad", "0")
	writeSysctl(t, "net/ipv6/conf/"+host+"/accept_dad", "0")
	writeSysctl(t, "net/ipv6/conf/"+router+"/accept_dad", "0")
	writeSysctl(t, "net/ipv6/conf/"+host+"/accept_ra", "0")
	for _, name := range []string{host, router} {
		if err := b.SetAdminUp(name); err != nil {
			t.Fatal(err)
		}
	}
	sock := raSender(t, router)

	m, bus := newTestMonitor()
	if err := m.start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.stop)

	server := netip.MustParseAddr("2001:db8::53")
	ra := buildTestRA(t, server)

	// accept_ra 0: the kernel processes no RA option for userspace.
	for range 5 {
		sendRA(t, sock, router, ra)
		time.Sleep(100 * time.Millisecond)
	}
	if rdnssEvent(t, bus, host) {
		t.Fatal("an RDNSS option reached userspace from an interface with accept_ra 0")
	}

	writeSysctl(t, "net/ipv6/conf/"+host+"/accept_ra", "1")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		sendRA(t, sock, router, ra)
		if rdnssEvent(t, bus, host) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no %s event for the RDNSS option on %s", ifaceevents.EventNDUserOption, host)
}

// buildTestRA builds an RA carrying one RDNSS option with Lifetime 600.
func buildTestRA(t *testing.T, server netip.Addr) []byte {
	t.Helper()
	cfg := ndp.RAConfig{RDNSS: []netip.Addr{server}, RDNSSLifetime: 600}
	buf := make([]byte, ndp.RALen(cfg))
	return buf[:ndp.BuildRA(buf, 0, cfg)]
}

// raSender opens a raw ICMPv6 socket bound to device, with the hop limit 255
// an RA needs (RFC 4861 Section 6.1.2). It waits for device's link-local
// address, which the RA's source must be.
func raSender(t *testing.T, device string) int {
	t.Helper()
	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !hasLinkLocal(t, link) {
		if time.Now().After(deadline) {
			t.Fatalf("%s has no link-local address", device)
		}
		time.Sleep(50 * time.Millisecond)
	}
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, unix.IPPROTO_ICMPV6)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	if err := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, device); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, ndp.MessageHopLimit); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_IF, link.Attrs().Index); err != nil {
		t.Fatal(err)
	}
	return fd
}

func hasLinkLocal(t *testing.T, link netlink.Link) bool {
	t.Helper()
	addrs, err := netlink.AddrList(link, unix.AF_INET6)
	if err != nil {
		t.Fatal(err)
	}
	// A tentative address cannot source a packet: DAD still runs when
	// net.ipv6.conf.all.accept_dad is on, whatever the device's own setting.
	for _, a := range addrs {
		if a.IP.IsLinkLocalUnicast() && a.Flags&unix.IFA_F_TENTATIVE == 0 {
			return true
		}
	}
	return false
}

// sendRA sends ra to ff02::1 out of device.
func sendRA(t *testing.T, fd int, device string, ra []byte) {
	t.Helper()
	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatal(err)
	}
	to := &unix.SockaddrInet6{ZoneId: uint32(link.Attrs().Index), Addr: [16]byte{0: 0xff, 1: 0x02, 15: 0x01}}
	if err := unix.Sendto(fd, ra, 0, to); err != nil {
		t.Fatal(err)
	}
}

// rdnssEvent reports whether the bus holds an nd-user-option event for an
// RDNSS option received on device.
func rdnssEvent(t *testing.T, bus *collectingEventBus, device string) bool {
	t.Helper()
	for _, ev := range bus.snapshot() {
		if ev.EventType != ifaceevents.EventNDUserOption {
			continue
		}
		data, ok := ev.Payload.(string)
		if !ok {
			t.Fatalf("nd-user-option payload is %T", ev.Payload)
		}
		var p iface.NDUserOptionPayload
		if err := json.Unmarshal([]byte(data), &p); err != nil {
			t.Fatal(err)
		}
		if p.Name != device || len(p.Option) == 0 || p.Option[0] != ndp.OptRDNSS {
			continue
		}
		parsed, err := ndp.ParseRDNSS(p.Option)
		if err != nil {
			t.Fatalf("RDNSS option from the kernel refused: %v", err)
		}
		if parsed.Lifetime != 600 || len(parsed.Servers) != 1 {
			t.Fatalf("RDNSS option %+v, want Lifetime 600 and one server", parsed)
		}
		return true
	}
	return false
}

// writeSysctl writes value to /proc/sys/<key> in the test's namespace.
func writeSysctl(t *testing.T, key, value string) {
	t.Helper()
	if err := os.WriteFile("/proc/sys/"+key, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
