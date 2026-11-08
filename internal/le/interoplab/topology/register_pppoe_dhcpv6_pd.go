// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_pppoe_dhcpv6_pd.go -- the checker this file registers.
// Related: check_pppoe_dhcpv6_pd_lan.go -- its LAN half (Phases 4 and 5).
// Related: register_pppoe_mss_pmtu.go -- the accel-ppp image and CPE node it shares.
// Related: register_home_lab.go -- the dhcpcd LAN host it reuses.
package topology

import "github.com/ze-software/ze/internal/le/interoplab"

// pppoeDHCPv6PDScenario is spec-home-cpe-8's scenario: the Ze PPPoE client
// on IPv6 against accel-ppp (IPv6CP, the WAN RA and SLAAC, DHCPv6 prefix
// delegation on pppoe0), and the delegated prefix's use on a LAN unit: a /64
// on lan0, advertised to a dhcpcd host, and the Router Lifetime withdrawn when
// accel-ppp ends the session.
const pppoeDHCPv6PDScenario = "pppoe-dhcpv6-pd-ze-client"

// Values accel-ppp.conf, ze-cpe.conf and the checker share. 2001:db8::/32 is
// the documentation range (RFC 3849).
const (
	// pdAccelLinkLocal is accel-ppp's link-local address on the session,
	// from its ipv6-intf-id 0:0:0:1: the RA's source and so the CPE's
	// default router.
	pdAccelLinkLocal = "fe80::1"
	// pdSLAACPool is accel-ppp's ipv6-pool, whose /64 its RA carries.
	pdSLAACPool = "2001:db8:198::/48"
	// pdDelegatePool and pdDelegatedLength are its delegate pool and the
	// length it delegates, which differs from the hint pdHintLength.
	pdDelegatePool    = "2001:db8:2000::/40"
	pdDelegatedLength = 60
	pdHintLength      = 56
	// pdValidLifetime and pdPreferredLifetime are accel-ppp's
	// [ipv6-dhcp] valid-lifetime and pref-lifetime, in seconds: what each
	// Reply grants the delegated prefix.
	pdValidLifetime     = 120
	pdPreferredLifetime = 60
	// pdAccelLog is the file accel-ppp logs to ([log] log-file).
	pdAccelLog = "/var/log/accel-ppp.log"
	// pdAccelChapLines is the chap-secrets line PAP reads: ze-cpe.conf's
	// credentials.
	pdAccelChapLines = "alice * s3cr3t *"
	// pdAccelUsername is the session accel-cmd terminates.
	pdAccelUsername = "alice"
	// pdAccelTarget is the address the LAN host pings across Ze: accel-ppp's
	// loopback, outside both pools, which accel-ppp reaches back through the
	// route it installs for the delegated prefix (route-via-gw).
	pdAccelTarget = "2001:db8:ffff::1"
)

// The LAN side of the scenario: ze-cpe.conf's lan0 unit and the dhcpcd host
// on it.
const (
	// pdLANLink is the link between the CPE's lan0 and the LAN host.
	pdLANLink = "delegated-lan"
	// pdLANDevice is the CPE's device on pdLANLink, the name ze-cpe.conf
	// configures.
	pdLANDevice = "lan0"
	// nodeSLAACHost is the LAN host dhcpcd numbers from the CPE's RAs.
	nodeSLAACHost = "slaac-host"
	// pdSubnetID is ze-cpe.conf's delegated-prefix subnet-id: the /64 lan0
	// takes is the delegated /60 with this value in bits 60 to 63.
	pdSubnetID = 1
	// pdRouterLifetime is ze-cpe.conf's router-advertisement
	// router-lifetime, in seconds.
	pdRouterLifetime = 1800
	// pdRAIntervalMax is its maximum-interval, in seconds: the longest gap
	// between two periodic RAs.
	pdRAIntervalMax = 4
	// pdLANCapture is the file the CPE's prestart records every RA it sends
	// on lan0 into.
	pdLANCapture = "/run/lan-ra.pcap"
	// pdDHCPCDLog is the file the LAN host's dhcpcd logs to (--logfile).
	pdDHCPCDLog = "/run/dhcpcd-lan.log"
)

// The node arguments both peers take: IPv6 on, which a lab container starts
// with off. The default is set too, so a link the harness moves into the
// namespace after start has IPv6 on.
var ipv6On = []string{dockerSysctl, "net.ipv6.conf.all.disable_ipv6=0", dockerSysctl, "net.ipv6.conf.default.disable_ipv6=0"}

// slaacHost is the dhcpcd LAN host of the scenario (dhcpClient, the Knot
// image's dhcpcd 10.5.2). The kernel's own Router Discovery and SLAAC are off
// on every device the namespace gains after start, so the address and the
// routes the host holds on the link are the ones dhcpcd installs: dhcpcd's
// if_setup_inet6 (src/if-linux.c) writes accept_ra and autoconf to 0 itself,
// and Docker's read-only /proc/sys refuses that write, which dhcpcd only logs.
func slaacHost() node {
	host := dhcpClient(nodeSLAACHost, 4, "")
	host.arguments = append(host.arguments, ipv6On...)
	host.arguments = append(host.arguments, dockerSysctl, "net.ipv6.conf.default.accept_ra=0",
		dockerSysctl, "net.ipv6.conf.default.autoconf=0")
	return host
}

// pdCPE is the Ze CPE: a run-time net sysctl writer (the PPPoE client writes
// pppoe0's sysctls on each session), recording every RA it sends on lan0 from
// before Ze starts, so the capture holds the first one.
func pdCPE() node {
	cpe := netSysctlWriter(zeNode(nodeCPE, 3, "ze-cpe.conf", append(interoplab.PPPDeviceArguments(), ipv6On...)...))
	cpe.prestart = captureStart(pdLANDevice, pdLANCapture, "icmp6 and ip6[40] == 134")
	return cpe
}

// slaac-host -- lan0 ze-cpe ==PPPoE (link access, VLAN 198, no prefix)==
// accel. The CPE starts after accel-ppp, so the concentrator answers its
// first PADI. The LAN link carries no prefix: the CPE numbers lan0 from the
// delegation, and the host from the CPE's RAs.
func init() {
	scenarios[pppoeDHCPv6PDScenario] = scenario{
		nodes: []node{
			{name: nodeAccel, image: accelImage, host: 2, config: "accel-ppp.conf", mountedAt: "/etc/accel-ppp.conf",
				capabilities: []string{capabilityNetAdmin},
				arguments:    append(interoplab.PPPDeviceArguments(), ipv6On...),
				prestart: "mkdir -p /etc/accel-ppp /var/log; printf '%s\\n' '" + pdAccelChapLines + "' > /etc/accel-ppp/chap-secrets\n" +
					"ip -6 address add " + pdAccelTarget + "/128 dev lo",
				logFiles: []string{pdAccelLog}},
			pdCPE(),
			slaacHost(),
		},
		links: []interoplab.Link{
			{Name: accessLink, VLAN: 198, Members: []string{nodeAccel, nodeCPE}},
			{Name: pdLANLink, VLAN: 199, Members: []string{nodeCPE, nodeSLAACHost},
				Devices: map[string]string{nodeCPE: pdLANDevice}},
		},
		checker: checkPPPoEDHCPv6PD,
	}
}
