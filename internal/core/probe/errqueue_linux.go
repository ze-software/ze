//go:build linux

// RFC: rfc/short/rfc792.md -- the ICMP errors the kernel queues for a DF probe
// Design: docs/architecture/diagnostics/active-probes.md -- the error queue and the kernel estimate
// Related: errqueue.go -- the outcome types this file fills
// Related: errqueue_other.go -- the non-Linux stubs with the same signatures
// Related: socket_linux.go -- applyDFOptions, which sets IP_RECVERR
//
// Linux hands a refusal to a socket with IP_RECVERR set in two ways at once.
// It queues a SockExtendedErr on the socket's error queue, readable with
// recvmsg(MSG_ERRQUEUE), and for a refusal that came from the network it
// also sets sk_err, so the next ordinary read on the socket returns that
// errno once (net/ipv4/raw.c raw_err, net/ipv6/raw.c rawv6_err). A refusal
// raised by this host's own send sets no sk_err: sendmsg itself returns
// EMSGSIZE, and the queued entry (net/ipv4/ip_sockglue.c ip_local_error)
// carries the cached estimate the send was measured against and quotes
// nothing. So a prober reads the queue when its ordinary read fails and
// when its send fails, and both reads go through drainErrorQueue.
//
// The queued data for a raw socket without IP_HDRINCL starts at the ICMP
// header of the datagram the error quotes (raw_err hands ip_icmp_error the
// payload past the quoted IP header), so the probe's identifier and sequence
// sit at the same offsets they hold in the echo request Ze built. The
// unprivileged datagram socket queues the same layout (net/ipv4/ping.c
// ping_err passes the quoted ICMP header too), with the identifier the
// kernel wrote at send, which is Socket.Identifier.
//
// Wire layout of the IP_RECVERR control message data, as the kernel writes
// it (ip_recv_error, ipv6_recv_error):
//
//	offset  size  field
//	0       4     ee_errno   host order; EMSGSIZE for a size refusal
//	4       1     ee_origin  SO_EE_ORIGIN_LOCAL, _ICMP or _ICMP6
//	5       1     ee_type    ICMP type of the message, 0 for a local error
//	6       1     ee_code    ICMP code, 0 for a local error
//	7       1     ee_pad
//	8       4     ee_info    host order; the reported next-hop MTU
//	12      4     ee_data
//	16      var   sockaddr of the offender: sockaddr_in for an ICMP origin,
//	              sockaddr_in6 for ICMP6, family AF_UNSPEC for a local error
//
// The IPv6 twin is the same layout under IPPROTO_IPV6 / IPV6_RECVERR.

package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/sys/unix"
)

// Family floors on a reported next-hop MTU. The two RFCs give the two
// families opposite rules, so neither constant is applied to the other.
const (
	// RFC 791: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation." 68 is the IPv4 number with
	// normative force; RFC 1191's 576 is a reassembly default it disclaims.
	ipv4MTUMin = 68
	// RFC 8200 Section 5: "IPv6 requires that every link in the Internet have
	// an MTU of 1280 octets or greater. This is known as the IPv6 minimum link
	// MTU." That sentence, not RFC 8201, is the authority for the constant.
	ipv6MTUMin = 1280
)

// errNoErrQueueAccess is what the reader answers for a conn that exposes no
// raw descriptor. Every conn OpenICMP returns exposes one, so this is a Ze
// defect at the call site rather than an operating condition.
var errNoErrQueueAccess = errors.New("probe: BUG: the conn exposes no raw descriptor to read the error queue from")

// sockExtendedErrLen is the byte length of struct sock_extended_err, the
// fixed head of the IP_RECVERR control message.
const sockExtendedErrLen = 16

// errQueueDatagramMax is the read buffer for one queued entry. The kernel
// quotes at most the original datagram, and a probe payload is capped well
// under this by its callers.
const errQueueDatagramMax = 1500

// drainErrorQueue reads the queued errors off conn, up to ErrQueueDrainMax
// of them, and hands each one to visit. It never blocks: the queue is read
// with MSG_DONTWAIT and an empty queue ends the drain. It answers
// ErrErrQueueUnsupported off Linux, and errNoErrQueueAccess for a conn that
// exposes no raw descriptor. family selects the control message level and
// the floor rule the reported value is held to.
func drainErrorQueue(conn net.PacketConn, family Family, visit func(QueuedError)) error {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return errNoErrQueueAccess
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return fmt.Errorf("probe: raw descriptor: %w", err)
	}
	data := make([]byte, errQueueDatagramMax)
	oob := make([]byte, unix.CmsgSpace(sockExtendedErrLen+unix.SizeofSockaddrInet6))
	var readErr error
	controlErr := raw.Control(func(fd uintptr) {
		for range ErrQueueDrainMax {
			n, oobn, _, _, recvErr := unix.Recvmsg(int(fd), data, oob, unix.MSG_ERRQUEUE|unix.MSG_DONTWAIT)
			if errors.Is(recvErr, unix.EAGAIN) {
				return
			}
			if recvErr != nil {
				readErr = fmt.Errorf("probe: read error queue: %w", recvErr)
				return
			}
			entry, parseErr := parseQueuedError(family, data[:n], oob[:oobn])
			if parseErr != nil {
				readErr = parseErr
				return
			}
			visit(entry)
		}
	})
	if controlErr != nil {
		return fmt.Errorf("probe: rawconn Control: %w", controlErr)
	}
	return readErr
}

// parseQueuedError turns one MSG_ERRQUEUE read into a QueuedError. data is
// the quoted datagram, oob the control messages. A read with no IP_RECVERR
// control message is refused rather than answered with a zero entry.
func parseQueuedError(family Family, data, oob []byte) (QueuedError, error) {
	level, kind := recvErrCmsg(family)
	cmsgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return QueuedError{}, fmt.Errorf("probe: parse error-queue control message: %w", err)
	}
	for _, cmsg := range cmsgs {
		if cmsg.Header.Level != level {
			continue
		}
		if cmsg.Header.Type != kind {
			continue
		}
		return parseExtendedErr(family, cmsg.Data, data)
	}
	return QueuedError{}, errors.New("probe: error-queue read carried no IP_RECVERR control message")
}

// recvErrCmsg is the control message level and type the kernel writes the
// extended error under, for family.
func recvErrCmsg(family Family) (level, kind int32) {
	if family == FamilyIPv6 {
		return unix.IPPROTO_IPV6, unix.IPV6_RECVERR
	}
	return unix.IPPROTO_IP, unix.IP_RECVERR
}

// parseExtendedErr reads the sock_extended_err at the front of cmsg, the
// offender address behind it, and the quoted echo header at the front of
// data. The layout is the table in the file header.
func parseExtendedErr(family Family, cmsg, data []byte) (QueuedError, error) {
	if len(cmsg) < sockExtendedErrLen {
		return QueuedError{}, fmt.Errorf("probe: IP_RECVERR control message is %d bytes, want at least %d", len(cmsg), sockExtendedErrLen)
	}
	entry := QueuedError{
		Errno: syscall.Errno(binary.NativeEndian.Uint32(cmsg[0:4])),
		Local: cmsg[4] == unix.SO_EE_ORIGIN_LOCAL,
	}
	info := binary.NativeEndian.Uint32(cmsg[8:12])
	entry.Outcome, entry.MTU = classifyReportedMTU(family, entry.Errno, info)
	entry.Offender = offenderAddr(cmsg[sockExtendedErrLen:])
	entry.Echo = quotedEcho(family, data)
	return entry, nil
}

// classifyReportedMTU holds the reported value to the family's rule and
// names the outcome. It is the enforcing code for the two RFC floors and
// for the meaning of a zero.
func classifyReportedMTU(family Family, errno syscall.Errno, info uint32) (ErrQueueOutcome, uint32) {
	// An error that is not a size refusal carries no next-hop MTU at all,
	// whatever ee_info holds for it.
	if errno != unix.EMSGSIZE {
		return ErrQueueMTUUnreported, 0
	}
	// RFC 1191 Section 3: "A Datagram Too Big message from an unmodified
	// router can be recognized by the presence of a zero in the
	// (newly-defined) Next-Hop MTU field." It is the signal that a search
	// MUST begin, and it is never returned as an MTU.
	if info == 0 {
		return ErrQueueMTUUnreported, 0
	}
	if family == FamilyIPv6 {
		// RFC 8201 Section 4: "If a node receives a Packet Too Big message
		// reporting a next-hop MTU that is less than the IPv6 minimum link
		// MTU, it must discard it." The document's Section 1.1 convention
		// writes its RFC 2119 words in lowercase. The message is discarded,
		// so its value is not reported; 1280 is RFC 8200 Section 5.
		if info < ipv6MTUMin {
			return ErrQueueMTUUnreported, 0
		}
		return ErrQueueMTUReported, info
	}
	// RFC 1191 Section 3: "A host MUST never reduce its estimate of the Path
	// MTU below 68 octets." The rule clamps the estimate and discards no
	// message, so the report stands and the value is raised to the floor the
	// RFC sets, which is RFC 791's 68.
	if info < ipv4MTUMin {
		return ErrQueueMTUReported, ipv4MTUMin
	}
	return ErrQueueMTUReported, info
}

// offenderAddr reads the sockaddr the kernel writes behind the extended
// error: the router that answered, for an ICMP origin. A local error carries
// AF_UNSPEC there and answers the zero Addr.
func offenderAddr(sa []byte) netip.Addr {
	if len(sa) < 2 {
		return netip.Addr{}
	}
	switch binary.NativeEndian.Uint16(sa[0:2]) {
	case unix.AF_INET:
		if len(sa) < unix.SizeofSockaddrInet4 {
			return netip.Addr{}
		}
		return netip.AddrFrom4([4]byte(sa[4:8]))
	case unix.AF_INET6:
		if len(sa) < unix.SizeofSockaddrInet6 {
			return netip.Addr{}
		}
		return netip.AddrFrom16([16]byte(sa[8:24]))
	default:
		return netip.Addr{}
	}
}

// quotedEcho reads the probe identifier and sequence off the quoted ICMP
// header, when the kernel quoted one that is an echo request of family.
func quotedEcho(family Family, data []byte) QuotedEcho {
	if len(data) < 8 {
		return QuotedEcho{}
	}
	echoType := byte(8)
	if family == FamilyIPv6 {
		echoType = 128
	}
	if data[0] != echoType {
		return QuotedEcho{}
	}
	return QuotedEcho{
		Present: true,
		ID:      binary.BigEndian.Uint16(data[4:6]),
		Seq:     binary.BigEndian.Uint16(data[6:8]),
	}
}

// KernelPathMTU reads the kernel's current path MTU estimate for dest, the
// value IP_MTU or IPV6_MTU answers. Linux answers that option only on a
// socket that holds a route to the destination, which means a connected
// one, and an unconnected probe socket gets ENOTCONN. The estimate is a
// property of the route, not of the socket that asks, so a UDP socket
// connected to dest reads the same cache entry a probe socket updated, and
// it needs no privilege: connect on UDP sends nothing and only binds the
// route. The socket is closed before this returns.
//
// A zero is never answered: no route, a failed connect or an absent
// estimate answer ErrPathMTUUnknown.
func KernelPathMTU(ctx context.Context, dest netip.Addr) (uint32, error) {
	if !dest.IsValid() {
		return 0, ErrPathMTUUnknown
	}
	var d net.Dialer
	// The port is never reached: connect on a datagram socket sends nothing.
	conn, err := d.DialContext(ctx, "udp", net.JoinHostPort(dest.String(), "9"))
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrPathMTUUnknown, err)
	}
	defer conn.Close() //nolint:errcheck // a throwaway socket that sent nothing

	sc, ok := conn.(syscall.Conn)
	if !ok {
		return 0, errNoErrQueueAccess
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("probe: raw descriptor: %w", err)
	}
	level, option := unix.IPPROTO_IP, unix.IP_MTU
	if dest.Is6() {
		level, option = unix.IPPROTO_IPV6, unix.IPV6_MTU
	}
	var mtu int
	var getErr error
	controlErr := raw.Control(func(fd uintptr) {
		mtu, getErr = unix.GetsockoptInt(int(fd), level, option)
	})
	if controlErr != nil {
		return 0, fmt.Errorf("probe: rawconn Control: %w", controlErr)
	}
	if getErr != nil {
		return 0, fmt.Errorf("%w: %w", ErrPathMTUUnknown, getErr)
	}
	// The kernel answers ENOTCONN rather than zero when it holds no route, so
	// a zero here would be a kernel the comment above does not describe.
	if mtu <= 0 {
		return 0, ErrPathMTUUnknown
	}
	return uint32(mtu), nil
}
