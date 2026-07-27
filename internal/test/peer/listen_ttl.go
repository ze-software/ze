// Design: docs/architecture/testing/ci-format.md -- ze-peer GTSM (RFC 5082) listen TTL
// Related: peer.go -- Config.TTL and the listener it configures
// Related: peer_connmap.go -- the conn_map listener, which uses the same builder

package peer

import (
	"context"
	"net"
	"syscall"

	"github.com/ze-software/ze/internal/core/network"
)

// listenConfig returns the net.ListenConfig this peer listens with. With
// Config.TTL set it installs a Control hook that applies the outgoing TTL to the
// listen socket BEFORE bind, which is the only point that catches the kernel's
// own SYN-ACK -- that frame is emitted before Accept returns, so setting the
// option on the accepted connection would already be too late for a GTSM peer
// checking the TTL of every inbound packet.
//
// network.SetListenIPTTL sets both the IPv4 and IPv6 option best-effort because
// a listener may be dual-stack. A real setsockopt failure fails the bind rather
// than binding a socket that silently lacks the TTL the caller asked for.
//
// "This platform has no TTL socket options at all" is NOT such a failure, and
// must not fail the bind. GTSM is enforced by the same options on the DUT side:
// where they do not exist, ze's setIPMinTTL is the no-op stub
// (internal/core/network/ttl_other.go) and nothing checks the inbound TTL, so a
// peer that cannot set an outbound TTL is exactly as acceptable as it was before
// --ttl existed. Failing closed here instead broke test/plugin/bgp-gtsm.ci on
// macOS -- the listener never bound, so the test that this flag was added to FIX
// on Linux started failing on darwin.
func (p *Peer) listenConfig() net.ListenConfig {
	ttl := p.config.TTL
	if ttl == 0 {
		return net.ListenConfig{}
	}
	return net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var setErr error
			if err := c.Control(func(fd uintptr) {
				setErr = network.SetListenIPTTL(int(fd), ttl)
			}); err != nil {
				return err
			}
			if network.IsIPTTLUnsupported(setErr) {
				return nil
			}
			return setErr
		},
	}
}

// listen binds the peer's listener with the TTL policy above applied.
func (p *Peer) listen(ctx context.Context, addr string) (net.Listener, error) {
	lc := p.listenConfig()
	return lc.Listen(ctx, "tcp", addr)
}
