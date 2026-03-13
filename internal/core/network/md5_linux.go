// Design: docs/architecture/core-design.md -- TCP MD5 authentication (RFC 2385)
// Overview: network.go -- network abstraction layer

//go:build linux

package network

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// setTCPMD5Sig sets the TCP_MD5SIG socket option on the given fd.
// RFC 2385: TCP MD5 authentication requires this to be set before connect/accept.
// The addr is the remote peer address to authenticate.
func setTCPMD5Sig(fd int, addr net.IP, key string) error {
	if len(key) > 80 {
		return fmt.Errorf("md5 key too long: %d bytes, max 80", len(key))
	}

	sig := unix.TCPMD5Sig{
		Keylen: uint16(len(key)),
	}
	copy(sig.Key[:], key)

	sa, err := ipToSockaddrStorage(addr)
	if err != nil {
		return fmt.Errorf("md5 sockaddr: %w", err)
	}
	sig.Addr = sa

	return unix.SetsockoptTCPMD5Sig(fd, unix.IPPROTO_TCP, unix.TCP_MD5SIG, &sig)
}

// ipToSockaddrStorage converts a net.IP to a unix.RawSockaddrAny-backed SockaddrStorage.
func ipToSockaddrStorage(ip net.IP) (unix.SockaddrStorage, error) {
	var ss unix.SockaddrStorage

	if ip4 := ip.To4(); ip4 != nil {
		// AF_INET
		ss.Family = unix.AF_INET
		// sockaddr_in layout after family(2 bytes): port(2 bytes), addr(4 bytes)
		copy(ss.Data[2:6], ip4)
		return ss, nil
	}

	if ip6 := ip.To16(); ip6 != nil {
		// AF_INET6
		ss.Family = unix.AF_INET6
		// sockaddr_in6 layout after family(2 bytes): port(2 bytes), flowinfo(4 bytes), addr(16 bytes)
		copy(ss.Data[6:22], ip6)
		return ss, nil
	}

	return ss, fmt.Errorf("invalid IP address: %v", ip)
}

// tcpMD5Supported reports whether TCP MD5 is supported on this platform.
func tcpMD5Supported() bool { return true }
