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
	"time"
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
// Supports optional local address binding, connection timeout,
// TCP MD5 authentication (RFC 2385), and outgoing TTL / Hop Limit
// control for GTSM (RFC 5082).
type RealDialer struct {
	// LocalAddr is the local address to bind to for outgoing connections.
	// If nil, the OS chooses the local address.
	LocalAddr *net.TCPAddr

	// Timeout is the maximum duration for the TCP connect.
	// Zero means no timeout (context deadline controls).
	Timeout time.Duration

	// PeerAddr is the remote peer IP for TCP MD5 socket option.
	// Required when MD5Key is set.
	PeerAddr net.IP

	// MD5Key is the TCP MD5 authentication password (RFC 2385).
	// When non-empty, TCP_MD5SIG is set on the socket before connect.
	MD5Key string

	// OutTTL is the outgoing IPv4 TTL or IPv6 Hop Limit.
	// Zero leaves the OS default unchanged.
	OutTTL uint8
}

// DialContext creates a real TCP connection using net.Dialer.
// If MD5Key or OutTTL is set, applies socket options via the Control callback
// before the TCP handshake begins.
func (d *RealDialer) DialContext(ctx context.Context, nw, address string) (net.Conn, error) {
	nd := net.Dialer{Timeout: d.Timeout}
	if d.LocalAddr != nil {
		nd.LocalAddr = d.LocalAddr
	}
	if d.MD5Key != "" || d.OutTTL != 0 {
		peerIP := d.PeerAddr
		password := d.MD5Key
		outTTL := d.OutTTL
		nd.Control = func(_, controlAddress string, c syscall.RawConn) error {
			var sysErr error
			if err := c.Control(func(fd uintptr) {
				if password != "" {
					if err := setTCPMD5Sig(int(fd), peerIP, password); err != nil {
						sysErr = err
						return
					}
				}
				if outTTL != 0 {
					ttlIP := controlAddressIP(controlAddress, peerIP)
					if ttlIP == nil {
						sysErr = fmt.Errorf("ttl peer address %q is not an IP address", controlAddress)
						return
					}
					if err := setIPTTL(int(fd), ttlIP, outTTL); err != nil && !IsIPTTLUnsupported(err) {
						sysErr = err
						return
					}
				}
			}); err != nil {
				return fmt.Errorf("raw conn control: %w", err)
			}
			return sysErr
		}
	}
	return nd.DialContext(ctx, nw, address)
}

// SetSourceAddress binds outgoing connections to the given local source IP.
// An empty string is a no-op (OS selects the source). A non-empty but
// unparseable address returns an error rather than leaving LocalAddr with a nil
// IP, which net.Dialer would bind to the wildcard address -- silently ignoring
// the operator-configured source. Every service configuring an outbound
// source-address routes it through here for uniform validation.
func (d *RealDialer) SetSourceAddress(source string) error {
	if source == "" {
		return nil
	}
	ip := net.ParseIP(source)
	if ip == nil {
		return fmt.Errorf("invalid source-address %q", source)
	}
	d.LocalAddr = &net.TCPAddr{IP: ip}
	return nil
}

func controlAddressIP(address string, fallback net.IP) net.IP {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip
		}
	}
	return fallback
}

// RealListenerFactory implements ListenerFactory using net.ListenConfig.
// Supports TCP MD5 authentication (RFC 2385) for configured peers.
type RealListenerFactory struct {
	// MD5Peers maps peer addresses that require TCP MD5 authentication.
	// When non-empty, TCP_MD5SIG is set on the listener socket for each peer
	// before bind, so the kernel validates MD5 on incoming SYN packets.
	MD5Peers []MD5Peer

	// ListenTTL is the outgoing IP TTL / IPv6 Hop Limit applied to the listen
	// socket (GTSM, RFC 5082). Zero leaves the OS default. When any peer on
	// this listener uses GTSM it is set to 255 so the kernel's SYN-ACK to an
	// inbound (peer-initiated) connection carries TTL 255 and survives the
	// peer's receive-side TTL gate. The accepted socket's per-peer TTL is then
	// applied in connectionEstablished.
	ListenTTL uint8
}

// Listen creates a real TCP listener using net.ListenConfig.
// If MD5Peers or ListenTTL is configured, applies the matching socket options
// via the Control callback before the socket is bound.
func (f RealListenerFactory) Listen(ctx context.Context, nw, address string) (net.Listener, error) {
	lc := net.ListenConfig{}
	if len(f.MD5Peers) > 0 || f.ListenTTL != 0 {
		peers := f.MD5Peers
		listenTTL := f.ListenTTL
		lc.Control = func(_, _ string, c syscall.RawConn) error {
			var sysErr error
			if err := c.Control(func(fd uintptr) {
				for _, p := range peers {
					if err := setTCPMD5Sig(int(fd), p.Addr, p.Key); err != nil {
						sysErr = fmt.Errorf("md5 for peer %s: %w", p.Addr, err)
						return
					}
				}
				if listenTTL != 0 {
					if err := setListenIPTTL(int(fd), listenTTL); err != nil && !IsIPTTLUnsupported(err) {
						sysErr = err
						return
					}
				}
			}); err != nil {
				return fmt.Errorf("listener raw conn control: %w", err)
			}
			return sysErr
		}
	}
	return lc.Listen(ctx, nw, address)
}
