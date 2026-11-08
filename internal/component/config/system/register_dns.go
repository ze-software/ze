// Design: docs/features/dns-resolver.md -- the router's resolvers and the system-dns event
// Related: resolv.go -- the registry that publishes on these handles
// Related: resolv_host.go -- the producer of the host origin's record

package system

import (
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/events"
)

// hostResolverOrigin is the origin of the host's own resolv.conf. Rank 500
// lists it after every protocol Ze learns resolvers from itself (DHCPv4 100,
// DHCPv6 200, PPP 300, RA 400), so a lease Ze holds always comes first.
var hostResolverOrigin = RegisterResolverOrigin("host", 500)

// dnsSnapshotEvent carries the registry's state to every consumer, in this
// process or a plugin process; dnsSnapshotRequest asks for it again.
var (
	dnsSnapshotEvent   = events.Register[*dnssource.Snapshot](dnssource.Namespace, dnssource.SnapshotEventType)
	dnsSnapshotRequest = events.RegisterSignal(dnssource.Namespace, dnssource.SnapshotRequestEventType)
)
