//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: dns_resolver_forward_learned_linux.go -- forwardAnswer, forwardAwaitAnswer, forwardAwaitLearned, forwardStartConfiguredStub and the records
// Related: register_dns_resolver_forward_learned_linux.go -- the registration of the drivers below
// Related: reload_fixture_live_stages.go -- liveStagesReload, the reload both drivers commit through

package fixture

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// dhcpv4LeaseScene is the link both DHCPv4-lease scenarios run on. The
// daemon creates the veth pair: its DHCP client runs on client, and its own
// DHCP server listens on server, the peer end, which ze leaves without an
// address or a carrier. The fixture gives server its subnet address and
// brings it up, so the lease the client takes carries the subnet's
// dns-server, leaseServer, where the learned stub answers on stubLink.
type dhcpv4LeaseScene struct {
	client, server, stubLink string
	serverAddress            netip.Prefix
	leaseServer              netip.Addr
}

// The devices and addresses of test/plugin/dns-resolver-forward-dhcpv4-default.ci
// (AC-20) and test/plugin/dns-resolver-forward-dhcpv4-replay.ci (AC-13).
var (
	dhcpv4DefaultScene = dhcpv4LeaseScene{
		client: "zerd40", server: "zerd41", stubLink: "zerd4dns0",
		serverAddress: netip.MustParsePrefix("192.0.2.1/24"),
		leaseServer:   netip.MustParseAddr("192.0.2.53"),
	}
	dhcpv4DefaultListener   = netip.MustParseAddrPort("127.0.23.83:53")
	dhcpv4DefaultConfigured = netip.MustParseAddrPort("127.0.23.84:53")

	dhcpv4ReplayScene = dhcpv4LeaseScene{
		client: "zerd4r0", server: "zerd4r1", stubLink: "zerd4rdns0",
		serverAddress: netip.MustParsePrefix("198.51.100.1/24"),
		leaseServer:   netip.MustParseAddr("198.51.100.53"),
	}
	dhcpv4ReplayListener = netip.MustParseAddrPort("127.0.23.93:53")
)

// dnsResolverForwardDHCPv4Default proves spec-home-cpe-23b AC-20 through the
// daemon: a resolver with no forward server and no forward dns-source
// forwards to the server a WAN DHCPv4 lease carries (the built-in default
// list configured, received, first-usable), and once reloadSource commits a
// forward server it forwards to that server alone, from the same listener
// socket: the reload does not rebind it (D-5).
func dnsResolverForwardDHCPv4Default(ctx context.Context, plugin *sdk.Plugin, reloadSource string) error {
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	scene := dhcpv4DefaultScene
	defer netlink.LinkDel(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: scene.stubLink}}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	learned, err := scene.startStub()
	if err != nil {
		return err
	}
	defer learned.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	configured, err := forwardStartConfiguredStub(dhcpv4DefaultConfigured)
	if err != nil {
		return err
	}
	defer configured.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	if err := scene.serve(runCtx); err != nil {
		return err
	}
	if err := scene.awaitLease(runCtx); err != nil {
		return err
	}
	if err := forwardAwaitLearned(runCtx, dhcpv4DefaultListener, learned, nil); err != nil {
		return fmt.Errorf("under the default list the LAN resolver never forwarded to %s, the DNS server of the DHCPv4 lease on %s: %w",
			scene.leaseServer, scene.client, err)
	}
	before, err := udpListenerInode(dhcpv4DefaultListener)
	if err != nil {
		return err
	}

	if err := liveStagesReload(runCtx, plugin, reloadSource); err != nil {
		return fmt.Errorf("reload %s: %w", reloadSource, err)
	}
	if !forwardAwaitAnswer(runCtx, dhcpv4DefaultListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("after forward server %s was committed the LAN resolver never forwarded to it alone",
			dhcpv4DefaultConfigured)
	}
	after, err := udpListenerInode(dhcpv4DefaultListener)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("committing a forward server rebound the listener on %s (socket inode %d, then %d)",
			dhcpv4DefaultListener, before, after)
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns-resolver default forward -> the DHCPv4 lease's server ").Str(scene.leaseServer.String()).
		Str(", then the committed forward server alone, listener not rebound\n").StdErr()
	return nil
}

// dnsResolverForwardDHCPv4Replay proves spec-home-cpe-23b AC-13 through the
// daemon: the boot config has no `service dns-resolver`, so the resolver
// plugin first starts, from reloadSource, after the DHCPv4 lease exists. Its
// learned list then comes only from the replay it requests once started, and
// its first answer MUST already use it: any answer other than the lease
// server's record fails the run at once, where a timeout or a refused
// connection only means the listener is not bound yet.
func dnsResolverForwardDHCPv4Replay(ctx context.Context, plugin *sdk.Plugin, reloadSource string) error {
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	scene := dhcpv4ReplayScene
	defer netlink.LinkDel(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: scene.stubLink}}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	learned, err := scene.startStub()
	if err != nil {
		return err
	}
	defer learned.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	if err := scene.serve(runCtx); err != nil {
		return err
	}
	if err := scene.awaitLease(runCtx); err != nil {
		return err
	}

	if err := liveStagesReload(runCtx, plugin, reloadSource); err != nil {
		return fmt.Errorf("reload %s: %w", reloadSource, err)
	}
	var first error
	answered := Poll(runCtx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		got, err := forwardAnswer(runCtx, dhcpv4ReplayListener)
		if err == nil {
			if got != forwardLearnedRecord {
				first = fmt.Errorf("the resolver's first answer was %s, not %s from the lease's server %s",
					got, forwardLearnedRecord, scene.leaseServer)
			}
			return true
		}
		// forwardAnswer names a DNS reply that is not NOERROR this way; any
		// other error is a socket that does not answer yet.
		if strings.Contains(err.Error(), "answered rcode") {
			first = fmt.Errorf("the resolver started after the lease answered before it applied the replay: %w", err)
			return true
		}
		return false
	})
	if !answered {
		return fmt.Errorf("the resolver started by %s never answered on %s", reloadSource, dhcpv4ReplayListener)
	}
	if first != nil {
		return first
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns-resolver started after the DHCPv4 lease -> first answer from the replayed lease server ").
		Str(scene.leaseServer.String()).Str("\n").StdErr()
	return nil
}

// startStub creates the dummy link that holds leaseServer and starts the
// learned stub there, answering the mock's default zone.
func (s dhcpv4LeaseScene) startStub() (*dnsmock.Server, error) {
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: s.stubLink}}
	if err := netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("create %s: %w", s.stubLink, err)
	}
	address, err := netlink.ParseAddr(s.leaseServer.String() + "/32")
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.leaseServer, err)
	}
	if err := netlink.AddrAdd(link, address); err != nil {
		return nil, fmt.Errorf("add %s to %s: %w", s.leaseServer, s.stubLink, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("bring %s up: %w", s.stubLink, err)
	}
	return dnsmock.Start(netip.AddrPortFrom(s.leaseServer, 53).String())
}

// serve waits for the daemon's server end, gives it the subnet address the
// DHCP server answers from and brings it up.
func (s dhcpv4LeaseScene) serve(ctx context.Context) error {
	var link netlink.Link
	if !Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		found, err := netlink.LinkByName(s.server)
		if err != nil {
			return false
		}
		link = found
		return true
	}) {
		return fmt.Errorf("the daemon never created %s", s.server)
	}
	address, err := netlink.ParseAddr(s.serverAddress.String())
	if err != nil {
		return fmt.Errorf("parse %s: %w", s.serverAddress, err)
	}
	if err := netlink.AddrReplace(link, address); err != nil {
		return fmt.Errorf("add %s to %s: %w", s.serverAddress, s.server, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring %s up: %w", s.server, err)
	}
	return nil
}

// awaitLease waits until the daemon's DHCP client holds an IPv4 address
// inside the server's subnet on the client end.
func (s dhcpv4LeaseScene) awaitLease(ctx context.Context) error {
	subnet := s.serverAddress.Masked()
	leased := Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		link, err := netlink.LinkByName(s.client)
		if err != nil {
			return false
		}
		addrs, err := netlink.AddrList(link, unix.AF_INET)
		if err != nil {
			return false
		}
		for i := range addrs {
			addr, ok := netip.AddrFromSlice(addrs[i].IP.To4())
			if ok && subnet.Contains(addr) {
				return true
			}
		}
		return false
	})
	if !leased {
		return fmt.Errorf("the DHCP client on %s never leased an address in %s", s.client, subnet)
	}
	return nil
}

// udpListenerInode answers the socket inode of the IPv4 UDP socket bound to
// listener, read from /proc/net/udp. A rebound listener is a new socket, so
// a changed inode is the rebind. A listener with no socket is an error.
func udpListenerInode(listener netip.AddrPort) (uint64, error) {
	if !listener.Addr().Is4() {
		return 0, errors.New("udpListenerInode reads IPv4 sockets only, not " + listener.String())
	}
	// The kernel prints the local address as the network-order word read
	// in host order (little-endian on every architecture Ze builds for),
	// then the port in host order: 127.0.23.83:53 is "5317007F:0035".
	ip := listener.Addr().As4()
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], listener.Port())
	want := strings.ToUpper(hex.EncodeToString([]byte{ip[3], ip[2], ip[1], ip[0]}) + ":" + hex.EncodeToString(port[:]))

	file, err := os.Open("/proc/net/udp")
	if err != nil {
		return 0, fmt.Errorf("read the UDP socket table: %w", err)
	}
	defer file.Close() //nolint:errcheck // Read-only; a close error changes no answer.

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		if fields[1] != want {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("socket inode %q of %s: %w", fields[9], listener, err)
		}
		return inode, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read the UDP socket table: %w", err)
	}
	return 0, errors.New("no UDP socket is bound to " + listener.String())
}
