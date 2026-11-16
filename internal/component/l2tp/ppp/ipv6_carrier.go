// Design: docs/research/l2tpv2-ze-integration.md -- session-owned IPv6 control transport

package ppp

import (
	"context"
	"net/netip"
)

// IPv6Carrier carries the IPv6 control traffic of one admitted PPP session.
// The kernel implementation owns separate ICMPv6 and DHCPv6 sockets; a data
// plane with a shared PPP endpoint demultiplexes that endpoint's one reader.
// Neither implementation may start another reader of the PPP frame channel.
//
// ReadRS reports one received Router Solicitation. ReadDHCPv6 returns one UDP
// payload and its reply address. Each read observes its context without closing
// the write path: shutdown MUST join DHCPv6 and solicitation readers, send the
// final Router Advertisement, and only then call Close. The caller MUST NOT
// reuse a receive buffer until the read returns.
//
// Close is idempotent and releases only this carrier's resources, never a
// borrowed PPP frame endpoint. Reads and writes may run concurrently; there is
// at most one reader of each kind. A nil carrier is invalid.
type IPv6Carrier interface {
	ReadRS(context.Context) error
	WriteRA(payload []byte, hopLimit int) error
	ReadDHCPv6(context.Context, []byte) (int, netip.AddrPort, error)
	WriteDHCPv6(payload []byte, peer netip.AddrPort) (int, error)
	Close() error
}
