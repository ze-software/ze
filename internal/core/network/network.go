// Package network provides injectable abstractions for network operations.
//
// Production code uses RealDialer and RealListenerFactory which delegate
// directly to the standard library with zero overhead beyond interface dispatch.
// Simulation and testing code can inject mock networks for deterministic execution.
//
// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure
// Detail: md5_linux.go — TCP MD5 setsockopt for Linux
// Detail: md5_freebsd.go — TCP MD5 setsockopt for FreeBSD
// Detail: md5_darwin.go — TCP MD5 unsupported on macOS
// Detail: md5_other.go — TCP MD5 fallback for other platforms
package network

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// Dialer abstracts outbound TCP connection creation.
//
// Production code uses RealDialer. Simulation code provides mock dialers
// that return mock connections (e.g., net.Pipe-based) without real TCP.
type Dialer interface {
	// DialContext connects to the address on the named network using the
	// provided context. Same semantics as net.Dialer.DialContext.
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// ListenerFactory abstracts inbound TCP listener creation.
//
// Production code uses RealListenerFactory. Simulation code provides mock
// factories that return mock listeners accepting mock connections.
type ListenerFactory interface {
	// Listen announces on the local network address.
	Listen(ctx context.Context, network, address string) (net.Listener, error)
}

// MD5Peer holds the MD5 key for a specific peer address.
// Used by RealListenerFactory to apply TCP_MD5SIG per peer on the listener socket.
type MD5Peer struct {
	Addr net.IP
	Key  string
}

// TCPMD5Supported reports whether TCP MD5 authentication (RFC 2385) is
// supported on this platform. Returns false on macOS and unsupported OSes.
func TCPMD5Supported() bool { return tcpMD5Supported() }

// RealDialer implements Dialer using net.Dialer.
// Supports optional local address binding for source IP selection
// and TCP MD5 authentication (RFC 2385).
type RealDialer struct {
	// LocalAddr is the local address to bind to for outgoing connections.
	// If nil, the OS chooses the local address.
	LocalAddr *net.TCPAddr

	// PeerAddr is the remote peer IP for TCP MD5 socket option.
	// Required when MD5Key is set.
	PeerAddr net.IP

	// MD5Key is the TCP MD5 authentication password (RFC 2385).
	// When non-empty, TCP_MD5SIG is set on the socket before connect.
	MD5Key string
}

// DialContext creates a real TCP connection using net.Dialer.
// If MD5Key is set, applies TCP_MD5SIG via the Control callback
// before the TCP handshake begins.
func (d *RealDialer) DialContext(ctx context.Context, nw, address string) (net.Conn, error) {
	nd := net.Dialer{}
	if d.LocalAddr != nil {
		nd.LocalAddr = d.LocalAddr
	}
	if d.MD5Key != "" {
		peerIP := d.PeerAddr
		password := d.MD5Key
		nd.Control = func(_, _ string, c syscall.RawConn) error {
			var sysErr error
			if err := c.Control(func(fd uintptr) {
				sysErr = setTCPMD5Sig(int(fd), peerIP, password)
			}); err != nil {
				return fmt.Errorf("md5 raw conn control: %w", err)
			}
			return sysErr
		}
	}
	return nd.DialContext(ctx, nw, address)
}

// RealListenerFactory implements ListenerFactory using net.ListenConfig.
// Supports TCP MD5 authentication (RFC 2385) for configured peers.
type RealListenerFactory struct {
	// MD5Peers maps peer addresses that require TCP MD5 authentication.
	// When non-empty, TCP_MD5SIG is set on the listener socket for each peer
	// before bind, so the kernel validates MD5 on incoming SYN packets.
	MD5Peers []MD5Peer
}

// Listen creates a real TCP listener using net.ListenConfig.
// If MD5Peers is configured, applies TCP_MD5SIG for each peer via the
// Control callback before the socket is bound.
func (f RealListenerFactory) Listen(ctx context.Context, nw, address string) (net.Listener, error) {
	lc := net.ListenConfig{}
	if len(f.MD5Peers) > 0 {
		peers := f.MD5Peers
		lc.Control = func(_, _ string, c syscall.RawConn) error {
			var sysErr error
			if err := c.Control(func(fd uintptr) {
				for _, p := range peers {
					if err := setTCPMD5Sig(int(fd), p.Addr, p.Key); err != nil {
						sysErr = fmt.Errorf("md5 for peer %s: %w", p.Addr, err)
						return
					}
				}
			}); err != nil {
				return fmt.Errorf("md5 raw conn control: %w", err)
			}
			return sysErr
		}
	}
	return lc.Listen(ctx, nw, address)
}
