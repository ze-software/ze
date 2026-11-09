// Design: docs/features/dns-resolver.md -- the router's resolvers and the system-dns event
// Related: resolv.go -- the registry that publishes on dnssource.SnapshotEvent
// Related: resolv_host.go -- the producer of the host origin's record

package system

// hostResolverOrigin is the origin of the host's own resolv.conf. Rank 500
// lists it after every protocol Ze learns resolvers from itself (DHCPv4 100,
// DHCPv6 200, PPP 300, RA 400). It is a fallback: its record is used only
// while no lease of another origin lists a server, so a container's or the
// installer's resolver never joins the servers a lease supplied.
var hostResolverOrigin = RegisterFallbackResolverOrigin("host", 500)
