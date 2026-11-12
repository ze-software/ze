// Design: docs/architecture/testing/interop.md -- ze's DHCPv4 server serving two subnets on one interface to dhcpcd.
// Related: register_dhcp_shared_segment.go -- the registration of the checker and the peer preparation.
// Related: check_dns_resolver.go -- the first dhcpcd scenario, whose image this one reuses.
package bgp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioDHCPSharedSegment = "dhcpv4-server-shared-segment-dhcpcd"

	// peerDHCPClient is the LAN host: the Knot image (test/interop/Dockerfile.knot),
	// which carries dhcpcd and iproute2, running only sleep. Each client is a
	// macvlan interface on its eth0 with a fixed MAC.
	peerDHCPClient          = "dhcpclient"
	dhcpClientHostNumber    = uint8(43)
	dhcpSharedTimeoutSecond = 40
	dhcpSharedReadyWait     = 30 * time.Second

	// dhcpDeviceSubnetAddress is ze's second address on eth0, in the device
	// subnet of ze.conf. The lab address is the household subnet's.
	dhcpDeviceSubnetAddress = "10.99.0.1"
	dhcpDeviceSubnetPrefix  = dhcpDeviceSubnetAddress + "/24"
)

// dhcpSharedClient is one dhcpcd client of the scenario: its interface, its
// MAC and the lease ze must bind it.
type dhcpSharedClient struct {
	iface string
	mac   string
}

var (
	dhcpLaptop  = dhcpSharedClient{iface: "mvlaptop", mac: "02:00:00:00:00:01"}
	dhcpPrinter = dhcpSharedClient{iface: "mvprinter", mac: "02:00:00:00:00:02"}
	dhcpGuest   = dhcpSharedClient{iface: "mvguest", mac: "02:00:00:00:00:03"}
	// dhcpStrayMAC is reserved nowhere: the printer first leases with it, from
	// the household pool, which is the wrong subnet once its own MAC returns.
	dhcpStrayMAC = "02:00:00:00:00:09"
)

// prepareDHCPSharedSegmentPeers adds the device subnet's address to ze's eth0
// before ze starts, and the dhcpcd host. dhcpcd needs NET_ADMIN to install a
// lease and to create its macvlan interfaces, and promote_secondaries is set
// to 1 for each new interface, as for dnsclient (prepare.go).
func prepareDHCPSharedSegmentPeers(peers []interoplab.PeerConfig, _ string) ([]interoplab.PeerConfig, error) {
	ze := -1
	for index := range peers {
		if peers[index].Name == "ze" {
			ze = index
		}
	}
	if ze < 0 {
		return peers, errors.New(scenarioDHCPSharedSegment + " has no Ze peer")
	}
	suffix, ok := strings.CutPrefix(peers[ze].Container, containerName("ze", ""))
	if !ok {
		return peers, errors.New(scenarioDHCPSharedSegment + ": Ze container " + peers[ze].Container + " does not carry the lab's name prefix")
	}
	peers[ze].Prestart += "ip address add " + dhcpDeviceSubnetPrefix + " dev " + containerInterface + "\n"
	peers = append(peers, interoplab.PeerConfig{Name: peerDHCPClient, Container: containerName(peerDHCPClient, suffix),
		Image: peerKnot, Host: dhcpClientHostNumber, Capabilities: []string{capabilityNetAdmin},
		Arguments: []string{dockerSysctlFlag, "net.ipv4.conf.default.promote_secondaries=1"},
		Command:   []string{cmdSleep, "infinity"},
		Ready:     &interoplab.ReadyProbe{Command: []string{"ip", ipObjectLink}, Timeout: dhcpSharedReadyWait, Interval: time.Second}})
	return peers, nil
}

// checkDHCPSharedSegment proves the spec's user stories against dhcpcd
// 10.5.2: a client reserved in either subnet binds its reservation, an
// unreserved client binds from the household subnet, which eth0's primary
// address is in, every lease names as server identifier ze's own address in
// the leased subnet, and a client holding a lease from the wrong subnet is
// refused with a DHCPNAK when it reboots, then binds its reservation.
//
// dhcpcd's dhcp_start1 (src/dhcp.c) reboots, sending an INIT-REBOOT
// DHCPREQUEST for its saved lease, when read_lease found one, and sends a
// DHCPDISCOVER otherwise; its handle_dhcp logs "NAK:" on a DHCPNAK and
// restarts with a DHCPDISCOVER. read_lease reads the lease file of the
// interface name and does not compare its chaddr, so a lease bound under
// one MAC is rebooted under another.
func checkDHCPSharedSegment(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDHCPSharedSegment, assertion, cause)
	}
	household := labIPv4(check.Network, zeHostNumber)
	reservedHousehold := labIPv4(check.Network, 20)
	// Assertion 1. The three clients' interfaces exist, the printer's under
	// the stray MAC first.
	for _, c := range []dhcpSharedClient{dhcpLaptop, {iface: dhcpPrinter.iface, mac: dhcpStrayMAC}, dhcpGuest} {
		if err := dhcpMacvlan(ctx, check.Lab, c); err != nil {
			return fail(1, err)
		}
	}
	// Assertion 2. The laptop binds its household reservation, from ze's
	// household address.
	if err := dhcpSharedBind(ctx, check.Lab, dhcpLaptop.iface, reservedHousehold, household); err != nil {
		return fail(2, err)
	}
	// Assertion 3. The unreserved guest binds from the household pool.
	lease, err := dhcpSharedLease(ctx, check.Lab, dhcpGuest.iface)
	if err != nil {
		return fail(3, err)
	}
	if err := dhcpSharedVerdict(lease, "", household); err != nil {
		return fail(3, err)
	}
	if !strings.HasPrefix(lease.address, strings.TrimSuffix(household, strconv.Itoa(int(zeHostNumber)))) {
		return fail(3, errors.New("the unreserved client leased "+lease.address+", want an address of the household subnet"))
	}
	// Assertion 4. The printer, under the stray MAC, leases from the
	// household pool: the wrong subnet for its own MAC.
	stray, err := dhcpSharedLease(ctx, check.Lab, dhcpPrinter.iface)
	if err != nil {
		return fail(4, err)
	}
	if strings.HasPrefix(stray.address, "10.99.0.") {
		return fail(4, errors.New("the stray MAC leased "+stray.address+", want a household pool address"))
	}
	// Assertion 5. The printer stops, takes its own MAC back and reboots with
	// the household lease: ze answers its INIT-REBOOT with a DHCPNAK, and its
	// DHCPDISCOVER binds the reservation, 10.99.0.20, from ze's address in the
	// device subnet.
	for _, argv := range [][]string{
		{cmdDHCPCD, "-4", "-x", dhcpPrinter.iface},
		{"ip", ipObjectLink, ipActionSet, dhcpPrinter.iface, linkDown},
		{"ip", ipObjectLink, ipActionSet, dhcpPrinter.iface, ipLinkAddress, dhcpPrinter.mac},
		{"ip", ipObjectLink, ipActionSet, dhcpPrinter.iface, "up"},
	} {
		if _, err := check.Lab.Exec(ctx, peerDHCPClient, argv, nil); err != nil {
			return fail(5, err)
		}
	}
	rebind, err := check.Lab.Exec(ctx, peerDHCPClient, dhcpcdBind(dhcpPrinter.iface), nil)
	if err != nil {
		return fail(5, err)
	}
	if !strings.Contains(rebind.Stderr, dhcpPrinter.iface+": NAK:") {
		return fail(5, errors.New("dhcpcd logged no DHCPNAK for the wrong-subnet lease: "+strings.TrimSpace(rebind.Stderr)))
	}
	if err := dhcpSharedCheckLease(ctx, check.Lab, dhcpPrinter.iface, "10.99.0.20", dhcpDeviceSubnetAddress); err != nil {
		return fail(5, err)
	}
	return nil
}

// dhcpMacvlan creates c's macvlan interface on eth0 with c's MAC and brings it up.
func dhcpMacvlan(ctx context.Context, lab interoplab.CheckerLab, c dhcpSharedClient) error {
	for _, argv := range [][]string{
		{"ip", ipObjectLink, ipActionAdd, c.iface, "link", containerInterface, ipLinkAddress, c.mac, "type", "macvlan", "mode", "bridge"},
		{"ip", ipObjectLink, ipActionSet, c.iface, "up"},
	} {
		if _, err := lab.Exec(ctx, peerDHCPClient, argv, nil); err != nil {
			return err
		}
	}
	return nil
}

// dhcpcdBind is dhcpcd binding one interface: -w returns once bound and
// leaves dhcpcd running, which --dumplease needs.
func dhcpcdBind(iface string) []string {
	return []string{cmdDHCPCD, "-4", "-w", "-t", strconv.Itoa(dhcpSharedTimeoutSecond), iface}
}

// dhcpSharedBind binds iface and checks its lease is address from server.
func dhcpSharedBind(ctx context.Context, lab interoplab.CheckerLab, iface, address, server string) error {
	if _, err := lab.Exec(ctx, peerDHCPClient, dhcpcdBind(iface), nil); err != nil {
		return err
	}
	return dhcpSharedCheckLease(ctx, lab, iface, address, server)
}

// dhcpSharedCheckLease reads iface's lease and checks it is address from server.
func dhcpSharedCheckLease(ctx context.Context, lab interoplab.CheckerLab, iface, address, server string) error {
	lease, err := dhcpSharedRead(ctx, lab, iface)
	if err != nil {
		return err
	}
	return dhcpSharedVerdict(lease, address, server)
}

// dhcpSharedLease binds iface and answers its lease.
func dhcpSharedLease(ctx context.Context, lab interoplab.CheckerLab, iface string) (dhcpSharedLeaseDump, error) {
	if _, err := lab.Exec(ctx, peerDHCPClient, dhcpcdBind(iface), nil); err != nil {
		return dhcpSharedLeaseDump{}, err
	}
	return dhcpSharedRead(ctx, lab, iface)
}

// dhcpSharedLeaseDump is the part of dhcpcd's --dumplease this scenario reads.
type dhcpSharedLeaseDump struct {
	iface    string
	address  string
	serverID string
}

// dhcpSharedRead reads iface's lease from the running dhcpcd.
func dhcpSharedRead(ctx context.Context, lab interoplab.CheckerLab, iface string) (dhcpSharedLeaseDump, error) {
	dump, err := lab.Query(ctx, peerDHCPClient, []string{cmdDHCPCD, "-4", "--dumplease", iface}, nil)
	if err != nil {
		return dhcpSharedLeaseDump{}, err
	}
	return parseDhcpcdSharedLease(iface, dump)
}

// parseDhcpcdSharedLease reads the address and the server identifier
// (dhcp_server_identifier, option 54) from dhcpcd's lease dump, one
// name=value line per option, values possibly single-quoted. It refuses a dump
// that holds no address or no server identifier.
func parseDhcpcdSharedLease(iface, output string) (dhcpSharedLeaseDump, error) {
	lease := dhcpSharedLeaseDump{iface: iface}
	for line := range strings.SplitSeq(output, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "'\"")
		switch name {
		case "ip_address":
			lease.address = value
		case "dhcp_server_identifier":
			lease.serverID = value
		}
	}
	var tb textbuf.Buffer
	if lease.address == "" {
		return lease, errors.New(tb.Str(iface).Str(": dhcpcd's lease dump holds no ip_address").String())
	}
	if lease.serverID == "" {
		return lease, errors.New(tb.Str(iface).Str(": dhcpcd's lease dump holds no dhcp_server_identifier").String())
	}
	return lease, nil
}

// dhcpSharedVerdict checks lease is address (any address when empty) and
// names server as its server identifier.
func dhcpSharedVerdict(lease dhcpSharedLeaseDump, address, server string) error {
	var tb textbuf.Buffer
	if address != "" && lease.address != address {
		return errors.New(tb.Str(lease.iface).Str(" leased ").Str(lease.address).Str(", want ").Str(address).String())
	}
	if lease.serverID != server {
		return errors.New(tb.Str(lease.iface).Str(" lease names server identifier ").Str(lease.serverID).
			Str(", want ze's address ").Str(server).Str(" in the leased subnet").String())
	}
	return nil
}
