// Design: plan/learned/972-ospf-af-unify.md -- Phase 1: the engine consumes transport
// through an address-family-neutral interface, not the concrete OSPFv2 type, so a
// second (IPv6/OSPFv3) instance can later supply ospfv3/transport. This phase is a
// pure extract-interface refactor: the concrete *transport.Transport satisfies the
// interface unchanged, so OSPFv2 behavior is bit-for-bit identical.

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
	"github.com/ze-software/ze/pkg/ze"
)

// Transport is the engine's view of the per-address-family raw transport. It is
// the union of what the engine calls directly and what it hands to the iface and
// neighbor sender interfaces (SendPacket / JoinAllDRouters / LeaveAllDRouters), so
// an engine instance can drive either the OSPFv2 (`ospf/transport`) or, once the
// receive type is made AF-neutral (a later phase), the OSPFv3 (`ospfv3/transport`)
// transport.
type Transport interface {
	SendPacket(name string, dst netip.Addr, payload []byte) error
	// SendPacketRouted sends a virtual-link packet ROUTED across a transit area (RFC 2328
	// section 8.1 / RFC 5340 section 2.9): IPv4 uses a TTL > 1 path distinct from the
	// TTL-1 link-local socket; IPv6 uses the global source src and a hop limit > 1 (src is
	// ignored by the IPv4 transport, which lets the kernel pick the source).
	SendPacketRouted(name string, dst, src netip.Addr, payload []byte) error
	Receive() <-chan transport.RawPacket
	EnableInterface(name string)
	DisableInterface(name string)
	HandleLinkUp(name string) error
	HandleLinkDown(name string) error
	JoinAllDRouters(name string) error
	LeaveAllDRouters(name string) error
	RecordDrop(name, reason string)
	InterfaceNameByIfIndex(ifindex int) (string, bool)
	InterfaceOpen(name string) bool
	OpenInterfaceCount() int
	SubscribeIfaceEvents(eb ze.EventBus) func()
	SetSigner(fn func(name string, payload []byte) []byte)
	SetMetrics(reg metrics.Registry)
	OnInterfaceUp(fn func(ifindex int, name string))
	OnInterfaceDown(fn func(ifindex int, name string))
	Close()
}

// Both raw transports satisfy the engine Transport interface, proving the transport seam
// is pluggable per address family: OSPFv2 (IPv4) unchanged, and OSPFv3 (IPv6) via its
// shared wire.RawPacket and its EnableInterface(name) default-Instance-ID wrapper.
var _ Transport = (*transport.Transport)(nil)
var _ Transport = (*ospfv3transport.Transport)(nil)
