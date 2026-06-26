//go:build linux

// Design: docs/architecture/core-design.md, network abstraction layer
// Overview: network.go, RealDialer socket setup

package network

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func setIPTTL(fd int, peerIP net.IP, ttl uint8) error {
	if ttl == 0 {
		return nil
	}
	if peerIP.To4() != nil {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL, int(ttl)); err != nil {
			return fmt.Errorf("setsockopt IP_TTL=%d: %w", ttl, err)
		}
		return nil
	}
	if peerIP.To16() != nil {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, int(ttl)); err != nil {
			return fmt.Errorf("setsockopt IPV6_UNICAST_HOPS=%d: %w", ttl, err)
		}
		return nil
	}
	return fmt.Errorf("invalid IP address: %v", peerIP)
}

func setListenIPTTL(fd int, ttl uint8) error {
	if ttl == 0 {
		return nil
	}
	// A shared listen socket may serve IPv4, IPv6, or both (dual-stack), and
	// the family is not known here. Set both options; the one that does not
	// apply to this socket's family returns an error we ignore. Succeed if at
	// least one option was accepted.
	v4Err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL, int(ttl))
	v6Err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, int(ttl))
	if v4Err != nil && v6Err != nil {
		return fmt.Errorf("setsockopt listen TTL=%d: IP_TTL=%v IPV6_UNICAST_HOPS=%v", ttl, v4Err, v6Err)
	}
	return nil
}

func setIPMinTTL(fd int, peerIP net.IP, ttl uint8) error {
	if ttl == 0 {
		return nil
	}
	if peerIP.To4() != nil {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MINTTL, int(ttl)); err != nil {
			return fmt.Errorf("setsockopt IP_MINTTL=%d: %w", ttl, err)
		}
		return nil
	}
	if peerIP.To16() != nil {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MINHOPCOUNT, int(ttl)); err != nil {
			return fmt.Errorf("setsockopt IPV6_MINHOPCOUNT=%d: %w", ttl, err)
		}
		return nil
	}
	return fmt.Errorf("invalid IP address: %v", peerIP)
}
