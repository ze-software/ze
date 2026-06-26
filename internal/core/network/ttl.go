// Design: docs/architecture/core-design.md, network abstraction layer
// Overview: network.go, RealDialer / RealListenerFactory socket setup
//
// Package network provides injectable abstractions for network operations.
package network

import (
	"errors"
	"net"
)

var errIPTTLSocketOptionsUnsupported = errors.New("IP TTL socket options are not supported on this platform")

// SetIPTTL sets the outgoing IPv4 TTL or IPv6 Hop Limit on a socket fd.
func SetIPTTL(fd int, peerIP net.IP, ttl uint8) error {
	return setIPTTL(fd, peerIP, ttl)
}

// SetIPMinTTL sets the inbound minimum IPv4 TTL or IPv6 Hop Limit on a socket fd.
func SetIPMinTTL(fd int, peerIP net.IP, ttl uint8) error {
	return setIPMinTTL(fd, peerIP, ttl)
}

// SetListenIPTTL sets the outgoing IPv4 TTL and IPv6 Hop Limit on a listen
// socket fd. Unlike SetIPTTL it has no peer address to pick a family from (a
// shared listener may be dual-stack), so it sets both options best-effort: the
// SYN-ACK the kernel emits for an inbound connection must carry the configured
// TTL (255 for GTSM, RFC 5082) before the application accepts the socket and
// applies the per-peer value. Without this, GTSM peers that initiate the
// connection drop our SYN-ACK because it carries the default TTL.
func SetListenIPTTL(fd int, ttl uint8) error {
	return setListenIPTTL(fd, ttl)
}

// IsIPTTLUnsupported reports whether err means TTL socket options are unavailable.
func IsIPTTLUnsupported(err error) bool {
	return errors.Is(err, errIPTTLSocketOptionsUnsupported)
}
