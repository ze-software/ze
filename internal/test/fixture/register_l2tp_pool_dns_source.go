// Design: docs/features/dns-resolver.md -- the subscriber pool's dns-source list, show system dns and its doctor checks
// Related: l2tp_pool_dns_source.go -- the subscriber and the RADIUS answers registered below
// Related: dns_source_operator.go -- the show system dns and show doctor observers registered below

package fixture

import "net/netip"

// The resolvers the MS DNS mock names for the subscriber (AC-9). Both sit in
// TEST-NET-2 (RFC 5737), so no real resolver answers them.
var (
	poolDNSRadiusPrimary   = netip.MustParseAddr("198.51.100.53")
	poolDNSRadiusSecondary = netip.MustParseAddr("198.51.100.54")
)

func init() {
	// spec-home-cpe-23b AC-9: an Access-Accept naming MS-Primary-DNS-Server
	// and MS-Secondary-DNS-Server. Argument: the port, then the shared key.
	Register("l2tp/pool-dns-source/radius-ms-dns",
		tunnelRadiusDriver(0, 2, poolDNSRadiusAttrs(poolDNSRadiusPrimary, poolDNSRadiusSecondary)))
	// AC-10: an Access-Accept naming no resolver.
	Register("l2tp/pool-dns-source/radius-plain", tunnelRadiusDriver(0, 2, nil))
	// The subscriber: the LNS port, then the expected primary and secondary.
	Register("l2tp/pool-dns-source", poolDNSSubscriber)
	// AC-12: show system dns through the daemon.
	Register("plugin/show-system-dns", observer02("show-system-dns-test", showSystemDNSConsumers))
	// The three dns-source doctor checks through show doctor, the daemon
	// form of ze doctor.
	Register("plugin/doctor-dns-source", observer02("doctor-dns-source-test", doctorDNSSourceFindings))
}
