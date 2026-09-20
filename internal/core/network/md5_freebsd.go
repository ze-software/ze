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
// FreeBSD takes the key from the Security Association Database, which ze does not
// write, so the configured password reaches the kernel only when setkey(8) carries
// it there. The flag is still set, because the kernel signs with the SAD entry of an
// operator who did populate it. What a configured password alone does NOT do is say
// so, which is why tcpMD5Supported below answers false.
func setTCPMD5Sig(fd int, _ net.IP, _ string) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_TCP, tcpMD5SigFreeBSD, 1)
}

// tcpMD5Supported reports whether a password an operator configures on ze takes
// effect on this platform. It is false on FreeBSD: setTCPMD5Sig installs no key, so
// a peering whose SAD entry is missing carries unsigned segments while the config
// asks for MD5. The two surfaces that read this predicate -- parsePeer
// (internal/component/bgp/reactor/config.go) and doctorCheckBGPMD5
// (internal/component/bgp/config/doctor_checks.go) -- then warn the operator, which
// a predicate answering true made unreachable (plan/journal/silent-fall-through.md,
// 2026-09-01). Writing the SAD from ze is what would make this true; it needs a
// PF_KEY writer ze does not have.
func tcpMD5Supported() bool { return false }
