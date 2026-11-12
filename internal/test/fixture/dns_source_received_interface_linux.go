//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: register_dns_source_router_merge_linux.go -- the registration of the driver below
// Related: dns_source_router_follows_ra_rdnss_linux.go -- the RA sender this generalizes to two links
// Related: dns_source_router_merge_linux.go -- the resolv.conf reader reused here
// RFC 8106 Section 5.1 -- see rfc/short/rfc8106.md.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The devices this fixture shares with
// test/draft/plugin/dns-source-received-interface.ci. The daemon creates both
// veth pairs and sets accept-ra 2 on their near ends. The .ci names
// rifKeptReceiver alone in received-interface. rifDroppedReceiver sorts first,
// so a filter that is ignored lists its server first and the lookup meets the
// dropped stub's record.
const (
	rifDroppedReceiver   = "zerifa0"
	rifDroppedAdvertiser = "zerifa1"
	rifKeptReceiver      = "zerifb0"
	rifKeptAdvertiser    = "zerifb1"
	rifStubLink          = "zerifdns0"
	// rifResolvConf is named relative to the runner's per-test working
	// directory, which this fixture shares with the daemon. The .ci names it
	// /proc/self/cwd/<this name>, because config validation refuses a
	// relative resolv-conf-path (ze:validate "resolv-conf-path").
	rifResolvConf = "dns-source-received-interface.resolv.conf"
	// rifDroppedRecord is what the dropped interface's stub answers for
	// web.example.test, so its answer can be told from the kept stub's
	// leaseStubRecord.
	rifDroppedRecord = "203.0.113.77"
)

var (
	rifDroppedServer = netip.MustParseAddr("2001:db8:53::a53")
	rifKeptServer    = netip.MustParseAddr("2001:db8:53::b53")
	// rifDroppedPrefix is an on-link prefix the dropped interface's RA carries
	// beside its RDNSS option. The kernel installs its route while processing
	// that RA, before it hands the RDNSS option up, so the route is the proof
	// that the RA was accepted.
	rifDroppedPrefix = netip.MustParsePrefix("2001:db8:a::/64")
)

// dnsSourceReceivedInterface proves spec-home-cpe-23 AC-24 through the daemon:
// with system dns-source source [ received ] received-interface [ zerifb0 ],
// resolvers learned on two interfaces, the router lists the one learned on
// zerifb0 alone, and the internal resolver asks it alone.
//
// Order is the barrier. The RA to the dropped interface goes first, and the
// fixture waits until the kernel installed that RA's prefix route, so the
// kernel has accepted it. Only then is the kept interface advertised, and the
// fixture waits until resolv.conf lists exactly the kept server. Ze's netlink
// monitor reads the RDNSS options in the order the kernel sent them, so when
// the kept record is in the file the dropped one has already been offered:
// a filter that is ignored lists both, and this wait fails. The lookup must
// then answer the kept stub's record, which an ignored filter, asking the
// dropped stub first, cannot produce.
func dnsSourceReceivedInterface(ctx context.Context, plugin *sdk.Plugin) error {
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	// The plugin suite runs in the guest's own namespace, so the dummy link
	// this fixture creates is removed on every return path.
	defer netlink.LinkDel(&netlink.Dummy{Name: rifStubLink}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	dropped, kept, err := rifStartStubs()
	if err != nil {
		return err
	}
	defer dropped.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.
	defer kept.Close()    //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	droppedFD, droppedIndex, err := rifSender(runCtx, rifDroppedAdvertiser)
	if err != nil {
		return err
	}
	defer unix.Close(droppedFD) //nolint:errcheck // The scenario's last send is done; a close error changes no verdict.
	keptFD, keptIndex, err := rifSender(runCtx, rifKeptAdvertiser)
	if err != nil {
		return err
	}
	defer unix.Close(keptFD) //nolint:errcheck // The scenario's last send is done; a close error changes no verdict.

	// The daemon writes accept_ra some time after it starts, and the kernel
	// drops an RA it is not yet accepting, so every poll sends one.
	droppedRA := ndp.RAConfig{
		RDNSS:         []netip.Addr{rifDroppedServer},
		RDNSSLifetime: raRDNSSLifetime,
		Prefixes: []ndp.PrefixInfo{{
			Prefix: rifDroppedPrefix, OnLink: true,
			ValidLifetime: raRDNSSLifetime, PreferredLifetime: raRDNSSLifetime,
		}},
	}
	var sendErr error
	if !Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		sendErr = rifSend(droppedFD, droppedIndex, droppedRA)
		return sendErr == nil && rifHasPrefixRoute(rifDroppedReceiver, rifDroppedPrefix)
	}) {
		return errors.Join(fmt.Errorf("the kernel never accepted the RA on %s: no route to %s", rifDroppedReceiver, rifDroppedPrefix), sendErr)
	}

	keptRA := ndp.RAConfig{RDNSS: []netip.Addr{rifKeptServer}, RDNSSLifetime: raRDNSSLifetime}
	var listed []netip.Addr
	var readErr error
	if !Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		sendErr = rifSend(keptFD, keptIndex, keptRA)
		if sendErr != nil {
			return false
		}
		listed, readErr = dnsSourceReadResolvConf(rifResolvConf)
		return readErr == nil && slices.Equal(listed, []netip.Addr{rifKeptServer})
	}) {
		return errors.Join(fmt.Errorf("%s never listed %s alone after RAs on %s and %s: it lists %v",
			rifResolvConf, rifKeptServer, rifDroppedReceiver, rifKeptReceiver, listed), sendErr, readErr)
	}

	data, err := fixture06PollObject(runCtx, plugin, leaseLookup, leaseLookupAttempts,
		func(answer map[string]any) bool { return dnsRecordsEqual(answer, leaseStubRecord) })
	if err != nil {
		return fmt.Errorf("the daemon never resolved web.example.test through %s, the server learned on %s: %w",
			rifKeptServer, rifKeptReceiver, err)
	}

	// The file is read once more: the lookup took time, and a dropped record
	// arriving late would show here.
	listed, readErr = dnsSourceReadResolvConf(rifResolvConf)
	if readErr != nil {
		return readErr
	}
	if !slices.Equal(listed, []netip.Addr{rifKeptServer}) {
		return fmt.Errorf("%s lists %v after the lookup, want %s alone", rifResolvConf, listed, rifKeptServer)
	}

	var tb textbuf.Buffer
	return tb.Str("OK: resolv.conf lists ").Str(rifKeptServer.String()).Str(" alone, learned on ").Str(rifKeptReceiver).
		Str(", and web.example.test type A -> ").Str(dnsAnswerLine(data)).
		Str(" through it, with a resolver learned on ").Str(rifDroppedReceiver).Str(" filtered out\n").StdErr()
}

// rifStartStubs creates the stub link holding both servers, without Duplicate
// Address Detection so the addresses bind at once, and starts one stub on
// each, port 53 because an RDNSS option carries no port. The dropped stub
// answers rifDroppedRecord for web.example.test; the kept stub answers the
// zone's own leaseStubRecord.
func rifStartStubs() (dropped, kept *dnsmock.Server, err error) {
	link := &netlink.Dummy{Name: rifStubLink}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", rifStubLink, err)
	}
	for _, server := range []netip.Addr{rifDroppedServer, rifKeptServer} {
		address, err := netlink.ParseAddr(server.String() + "/128")
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", server, err)
		}
		address.Flags = unix.IFA_F_NODAD
		if err := netlink.AddrAdd(link, address); err != nil {
			return nil, nil, fmt.Errorf("add %s to %s: %w", server, rifStubLink, err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return nil, nil, fmt.Errorf("bring %s up: %w", rifStubLink, err)
	}
	dropped, err = dnsmock.Start(netip.AddrPortFrom(rifDroppedServer, 53).String())
	if err != nil {
		return nil, nil, err
	}
	dropped.Set("web.example.test", mdns.TypeA, dnsmock.Answer{Addresses: []netip.Addr{netip.MustParseAddr(rifDroppedRecord)}})
	kept, err = dnsmock.Start(netip.AddrPortFrom(rifKeptServer, 53).String())
	if err != nil {
		return nil, nil, errors.Join(err, dropped.Close())
	}
	return dropped, kept, nil
}

// rifSender waits for the daemon's veth end name, brings it up, waits for its
// link-local address (an RA's source, RFC 4861 Section 6.1.2), and opens a
// raw ICMPv6 socket bound to it with hop limit 255. It answers the socket and
// the link's index.
func rifSender(ctx context.Context, name string) (fd, index int, err error) {
	var link netlink.Link
	if !Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		found, err := netlink.LinkByName(name)
		link = found
		// refuse-by-default: guards Poll -- a failed read is "not yet", the caller polls
		return err == nil
	}) {
		return -1, 0, fmt.Errorf("%s never appeared: the daemon did not create the veth pair", name)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return -1, 0, fmt.Errorf("bring %s up: %w", name, err)
	}
	if !Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool { return raRDNSSHasLinkLocal(link) }) {
		return -1, 0, fmt.Errorf("%s has no usable link-local address to advertise from", name)
	}
	fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_ICMPV6)
	if err != nil {
		return -1, 0, fmt.Errorf("open ICMPv6 socket: %w", err)
	}
	index = link.Attrs().Index
	setup := errors.Join(
		unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, name),
		unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, ndp.MessageHopLimit),
		unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_IF, index),
	)
	if setup != nil {
		return -1, 0, errors.Join(fmt.Errorf("configure ICMPv6 socket on %s: %w", name, setup), unix.Close(fd))
	}
	return fd, index, nil
}

// rifSend sends the RA cfg describes to ff02::1 out of the link at index. The
// kernel fills the ICMPv6 checksum.
func rifSend(fd, index int, cfg ndp.RAConfig) error {
	ra := make([]byte, ndp.RALen(cfg))
	ra = ra[:ndp.BuildRA(ra, 0, cfg)]
	to := &unix.SockaddrInet6{ZoneId: uint32(index), Addr: [16]byte{0: 0xff, 1: 0x02, 15: 0x01}} //nolint:gosec // an interface index is never negative
	if err := unix.Sendto(fd, ra, 0, to); err != nil {
		return fmt.Errorf("send RA on link %d: %w", index, err)
	}
	return nil
}

// rifHasPrefixRoute reports whether the kernel holds a route to prefix on the
// link name. A failed read is "not yet": the caller polls.
func rifHasPrefixRoute(name string, prefix netip.Prefix) bool {
	link, err := netlink.LinkByName(name)
	// refuse-by-default: guards Poll -- a failed read is "not yet", the caller polls
	if err != nil {
		return false
	}
	routes, err := netlink.RouteList(link, unix.AF_INET6)
	// refuse-by-default: guards Poll -- a failed read is "not yet", the caller polls
	if err != nil {
		return false
	}
	for i := range routes {
		dst := routes[i].Dst
		if dst == nil {
			continue
		}
		ones, _ := dst.Mask.Size()
		addr, ok := netip.AddrFromSlice(dst.IP)
		if !ok {
			continue
		}
		if netip.PrefixFrom(addr, ones) == prefix {
			return true
		}
	}
	return false
}
