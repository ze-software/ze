// Design: docs/architecture/core-design.md -- TCP MD5 authentication (RFC 2385)
// Overview: network.go -- network abstraction layer

//go:build freebsd

package network

import (
	"net"

	"golang.org/x/sys/unix"
)

// tcpMD5SigFreeBSD is the TCP_MD5SIG socket option from FreeBSD's tcp.h.
// FreeBSD treats this as a boolean flag: 1 = enable, 0 = disable.
// The actual MD5 key must be configured in the Security Association Database
// via setkey(8) before starting ze. Example setkey.conf:
//
//	add <local-ip> <peer-ip> tcp 0x1000 -A tcp-md5 "password";
//	add <peer-ip> <local-ip> tcp 0x1000 -A tcp-md5 "password";
const tcpMD5SigFreeBSD = 0x10

// setTCPMD5Sig enables TCP_MD5SIG on the given fd for FreeBSD.
// FreeBSD requires the MD5 key to be pre-configured in the SAD via setkey(8).
// This function only enables the socket flag; the password parameter is not
// used directly but must match the SAD entry.
func setTCPMD5Sig(fd int, _ net.IP, _ string) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, tcpMD5SigFreeBSD, 1)
}

// tcpMD5Supported reports whether TCP MD5 is supported on this platform.
func tcpMD5Supported() bool { return true }
