//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: register_dns_source_router_follows_ra_rdnss_linux.go -- the registration of the driver below
// Related: dns_source_router_follows_lease_linux.go -- the IPCP sibling: the lookup and answer reused here
// RFC 8106 Sections 5.1 and 6.2 -- see rfc/short/rfc8106.md.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The poll bounds and the first advertisement's Lifetime, shared by every
// scenario that advertises an RDNSS server (rdnssScene).
const (
	// raRDNSSAttempts bounds each poll at 30 s.
	raRDNSSAttempts = 120
	raRDNSSDelay    = 250 * time.Millisecond
	// raRDNSSLifetime is the RDNSS Lifetime of the first advertisement: long
	// enough that only the Lifetime-0 advertisement can withdraw the server.
	raRDNSSLifetime = 600
)

// rdnssScene is one RA RDNSS setup: the daemon creates the veth pair from
// the .ci config and sets accept-ra 2 on receiver; the fixture advertises from
// advertiser and creates stubLink, which holds server. The server is global
// unicast (an RDNSS option carries IPv6 addresses only), held by a dummy link
// so the stub can bind it. Each scenario takes its own names and server, so
// scenarios of one suite never share a device or an address.
type rdnssScene struct {
	receiver   string
	advertiser string
	stubLink   string
	server     netip.Addr
}

// raRDNSSScene holds the devices this fixture shares with
// test/plugin/dns-source-router-follows-ra-rdnss.ci.
var raRDNSSScene = rdnssScene{
	receiver:   "zerdnss0",
	advertiser: "zerdnss1",
	stubLink:   "zerdnssdns0",
	server:     netip.MustParseAddr("2001:db8:53::53"),
}

// dnsSourceRouterFollowsRARDNSS proves spec-home-cpe-23 Phase 2a through the
// daemon (AC-21, AC-22): with no system name-server, Ze's own lookups use
// the server an accepted RA's RDNSS option named on raRDNSSScene.receiver, and stop
// using it when a later RA gives it Lifetime 0. Before any RA, the lookup of
// the stub's name must not return the stub's record.
func dnsSourceRouterFollowsRARDNSS(ctx context.Context, plugin *sdk.Plugin) error {
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	// The plugin suite runs in the guest's own namespace, so the dummy link
	// this fixture creates is removed on every return path.
	defer netlink.LinkDel(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: raRDNSSScene.stubLink}}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	stub, err := raRDNSSScene.startStub()
	if err != nil {
		return err
	}
	defer stub.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	var before map[string]any
	_, beforeErr := Dispatch(runCtx, plugin, leaseLookup, &before)
	if beforeErr == nil && dnsRecordsEqual(before, leaseStubRecord) {
		return fmt.Errorf("%s answered %s before any RA named the stub's server: the test cannot tell the RA's effect",
			leaseLookup, dnsAnswerLine(before))
	}

	fd, err := raRDNSSScene.sender(runCtx)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last send is done; a close error changes no verdict.

	// The daemon writes accept_ra on its end some time after it starts, and
	// the kernel drops an RA it is not yet accepting, so every poll sends one.
	var sendErr error
	followed := Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		sendErr = raRDNSSScene.send(fd, raRDNSSLifetime)
		if sendErr != nil {
			return false
		}
		var answer map[string]any
		_, err := Dispatch(runCtx, plugin, leaseLookup, &answer)
		return err == nil && dnsRecordsEqual(answer, leaseStubRecord)
	})
	if !followed {
		return errors.Join(fmt.Errorf("the daemon never resolved web.example.test through the RDNSS server %s advertised to %s",
			raRDNSSScene.server, raRDNSSScene.receiver), sendErr)
	}

	// RFC 8106 Section 5.1: "A value of zero means that the RDNSS address
	// MUST no longer be used."
	if err := raRDNSSScene.send(fd, 0); err != nil {
		return err
	}
	withdrawn := Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		var answer map[string]any
		_, err := Dispatch(runCtx, plugin, leaseLookup, &answer)
		return err != nil || !dnsRecordsEqual(answer, leaseStubRecord)
	})
	if !withdrawn {
		return fmt.Errorf("the daemon still resolved web.example.test through %s after an RA gave it Lifetime 0", raRDNSSScene.server)
	}

	var tb textbuf.Buffer
	tb.Str("OK: web.example.test type A -> ").Str(leaseStubRecord).
		Str(" through the RA RDNSS server, then withdrawn at Lifetime 0\n").StdErr()
	return nil
}

// startStub creates the dummy link stubLink holding server,
// without Duplicate Address Detection so the address binds at once, and
// starts the DNS stub on it, port 53 because an RDNSS option carries no port.
func (s rdnssScene) startStub() (*dnsmock.Server, error) {
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: s.stubLink}}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("create %s: %w", s.stubLink, err)
	}
	address, err := netlink.ParseAddr(s.server.String() + "/128")
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.server, err)
	}
	address.Flags = unix.IFA_F_NODAD
	if err := netlink.AddrAdd(link, address); err != nil {
		return nil, fmt.Errorf("add %s to %s: %w", s.server, s.stubLink, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("bring %s up: %w", s.stubLink, err)
	}
	return dnsmock.Start(netip.AddrPortFrom(s.server, 53).String())
}

// sender waits for the daemon's veth, brings the advertising end up,
// waits for its link-local address (an RA's source, RFC 4861 Section 6.1.2),
// and opens a raw ICMPv6 socket bound to it with hop limit 255.
func (s rdnssScene) sender(ctx context.Context) (int, error) {
	var link netlink.Link
	if !Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		found, err := netlink.LinkByName(s.advertiser)
		link = found
		return err == nil
	}) {
		return -1, fmt.Errorf("%s never appeared: the daemon did not create the veth pair", s.advertiser)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return -1, fmt.Errorf("bring %s up: %w", s.advertiser, err)
	}
	if !Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool { return raRDNSSHasLinkLocal(link) }) {
		return -1, fmt.Errorf("%s has no usable link-local address to advertise from", s.advertiser)
	}
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_ICMPV6)
	if err != nil {
		return -1, fmt.Errorf("open ICMPv6 socket: %w", err)
	}
	setup := errors.Join(
		unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, s.advertiser),
		unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, ndp.MessageHopLimit),
		unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_IF, link.Attrs().Index),
	)
	if setup != nil {
		return -1, errors.Join(fmt.Errorf("configure ICMPv6 socket on %s: %w", s.advertiser, setup), unix.Close(fd))
	}
	return fd, nil
}

// raRDNSSHasLinkLocal reports whether link holds a link-local address that is
// no longer tentative. A failed address read is "not yet": the caller polls.
func raRDNSSHasLinkLocal(link netlink.Link) bool {
	addrs, err := netlink.AddrList(link, unix.AF_INET6)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if !a.IP.IsLinkLocalUnicast() {
			continue
		}
		if a.Flags&unix.IFA_F_TENTATIVE != 0 {
			continue
		}
		return true
	}
	return false
}

// send sends an RA naming server with lifetime to ff02::1 out
// of the advertising end. The kernel fills the ICMPv6 checksum.
func (s rdnssScene) send(fd int, lifetime uint32) error {
	cfg := ndp.RAConfig{RDNSS: []netip.Addr{s.server}, RDNSSLifetime: lifetime}
	ra := make([]byte, ndp.RALen(cfg))
	ra = ra[:ndp.BuildRA(ra, 0, cfg)]
	link, err := netlink.LinkByName(s.advertiser)
	if err != nil {
		return fmt.Errorf("find %s: %w", s.advertiser, err)
	}
	to := &unix.SockaddrInet6{ZoneId: uint32(link.Attrs().Index), Addr: [16]byte{0: 0xff, 1: 0x02, 15: 0x01}} //nolint:gosec // an interface index is never negative
	if err := unix.Sendto(fd, ra, 0, to); err != nil {
		return fmt.Errorf("send RA on %s: %w", s.advertiser, err)
	}
	return nil
}
