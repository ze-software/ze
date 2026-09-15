// Design: docs/architecture/diagnostics/active-probes.md -- one socket construction for every prober
// Related: df.go -- the DFMode this construction installs
// Related: socket_linux.go, socket_other.go -- dfControl and listenDatagramICMP, the platform half
// Related: doctor.go -- the doctor check that reports which socket kind this host can open
//
// Every active probe (show ping, monitor ping, show traceroute, monitor
// traceroute, show probe-round, and the resolve variants) opens its ICMP
// socket here and nowhere else. A prober that opened its own socket would
// have to repeat the DF and error-queue options, and the one that forgot
// would fragment silently while reporting a measured path.
//
// Two socket kinds exist. The raw ICMP socket needs CAP_NET_RAW. When the
// kernel refuses it for privilege, and only then, OpenICMP opens Linux's
// unprivileged ICMP socket instead (SOCK_DGRAM, IPPROTO_ICMP), which the
// kernel permits when the caller's group is inside net.ipv4.ping_group_range.
// The two kinds differ in three ways a prober must not have to know: who
// picks the echo identifier, which net.Addr type the conn speaks, and
// whether ICMP errors reach the ordinary read. Socket hides the first two
// and names the third through Kind.

package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"
)

// ErrFamilyRequired is what OpenICMP answers for FamilyAny: an ICMP socket is
// opened for one family, so the caller derives it from the destination with
// FamilyOf.
var ErrFamilyRequired = errors.New("probe: an ICMP socket needs one address family")

// ErrDatagramICMPUnsupported is what the unprivileged fallback answers off
// Linux: no other platform Ze builds for offers the ping socket, so a raw
// socket refused there stays refused by name.
var ErrDatagramICMPUnsupported = errors.New("probe: the unprivileged ICMP datagram socket exists only on Linux")

// SocketKind names which ICMP socket the kernel gave OpenICMP. The zero
// value is not a kind: a Socket is only ever built by OpenICMP, which sets it.
type SocketKind uint8

const (
	SocketUnspecified SocketKind = iota
	// SocketRaw is the raw ICMP socket, opened under CAP_NET_RAW. Ze writes
	// the echo identifier itself, and the kernel hands the socket every ICMP
	// datagram of its family, error messages included.
	SocketRaw
	// SocketDatagram is Linux's unprivileged ICMP socket. The kernel assigns
	// the identifier from the socket's bound port, overwrites the id field of
	// every echo sent, and delivers only echo replies carrying that id to the
	// ordinary read. An ICMP error quoting one of its echoes reaches the
	// error queue (net/ipv4/ping.c ping_err) and never the ordinary read, so
	// a prober that reads Time Exceeded off the socket cannot run on it.
	SocketDatagram
)

// String names the kind for an operator reading a message.
func (k SocketKind) String() string {
	switch k {
	case SocketRaw:
		return "raw"
	case SocketDatagram:
		return "datagram"
	default:
		return nameUnspecified
	}
}

// Socket is an open ICMP probe socket. It speaks *net.IPAddr on both kinds:
// WriteTo and ReadFrom translate to and from the *net.UDPAddr the datagram
// kind wants, so a prober builds its destination once, as an IP address.
//
// The caller owns the Socket and MUST call Close. Safe for concurrent use
// by one reader and one writer, which is what the underlying net conns
// promise.
type Socket struct {
	conn   net.PacketConn
	kind   SocketKind
	id     uint16
	family Family
}

// Identifier is the echo identifier every probe on this socket MUST carry
// in the header BuildICMPEcho writes. On the raw kind Ze chose it at open;
// on the datagram kind the kernel did, and rewrites the header field to it
// anyway, so a reply or a queued error is matched by this value on both
// kinds.
func (s *Socket) Identifier() uint16 { return s.id }

// Kind is the socket kind the kernel gave.
func (s *Socket) Kind() SocketKind { return s.kind }

// PacketConn is the conn itself, for the x/net wrappers that set the TTL
// and need the concrete *net.IPConn or *net.UDPConn. Reads and writes go
// through Socket, which translates the address type; a caller that reads
// the conn directly sees the kind's own address type.
func (s *Socket) PacketConn() net.PacketConn { return s.conn }

// WriteTo sends p to addr, an *net.IPAddr on both kinds.
func (s *Socket) WriteTo(p []byte, addr net.Addr) (int, error) {
	if s.kind == SocketDatagram {
		ipAddr, ok := addr.(*net.IPAddr)
		if !ok {
			return 0, fmt.Errorf("probe: write to %T, want *net.IPAddr", addr)
		}
		return s.conn.WriteTo(p, &net.UDPAddr{IP: ipAddr.IP, Zone: ipAddr.Zone})
	}
	return s.conn.WriteTo(p, addr)
}

// ReadFrom reads one datagram and names its sender as an *net.IPAddr on both
// kinds. On the datagram kind the data starts at the ICMP header, as it does
// on the raw kind: the kernel restores the header before queueing a reply
// (net/ipv4/ping.c ping_rcv) and Go strips the IPv4 header off a raw read.
func (s *Socket) ReadFrom(p []byte) (int, net.Addr, error) {
	n, from, err := s.conn.ReadFrom(p)
	udpAddr, ok := from.(*net.UDPAddr)
	if !ok {
		return n, from, err
	}
	return n, &net.IPAddr{IP: udpAddr.IP, Zone: udpAddr.Zone}, err
}

// SetDeadline bounds the next read and write.
func (s *Socket) SetDeadline(t time.Time) error { return s.conn.SetDeadline(t) }

// Close closes the socket.
func (s *Socket) Close() error { return s.conn.Close() }

// DrainErrors hands every entry queued on the socket's error queue to visit,
// bounded by ErrQueueDrainMax, and never blocks. Both kinds queue the quoted
// echo starting at its ICMP header (raw_err and ping_err hand ip_icmp_error
// the same pointer), so the identifier and sequence are read the same way.
func (s *Socket) DrainErrors(visit func(QueuedError)) error {
	return drainErrorQueue(s.conn, s.family, visit)
}

// openRawICMP and openDatagramICMP are the two socket constructions OpenICMP
// tries, in that order. They are variables so a test can stand a refusal in
// for either without dropping CAP_NET_RAW or editing ping_group_range: the
// fallback guard and the doctor check are driven through them.
var (
	openRawICMP      = listenRawICMP
	openDatagramICMP = listenDatagramICMP
)

// OpenICMP opens the ICMP packet socket a prober sends on. family selects
// ICMPv4 or ICMPv6. bind is the local address the socket is bound to, and the
// zero Addr leaves the choice to the kernel. df is the Don't Fragment mode:
// DFOff opens the socket with the DF bit clear, and the other modes install
// IP_MTU_DISCOVER and IP_RECVERR on Linux so a router's Fragmentation Needed
// answer reaches the socket's error queue. The zero DFMode is refused with
// ErrDFUnspecified.
//
// DFOff is an option too, not the absence of one: Linux's default for a
// socket is IP_PMTUDISC_WANT, which sets the DF bit on every datagram that
// fits the path (observed on the wire by TestProbeDFBitOnTheWire), so a
// probe that never named a mode carried DF anyway. DFOff installs
// IP_PMTUDISC_DONT so the bit is clear and the kernel fragments freely.
//
// The raw socket is tried first. When the kernel refuses it for privilege,
// the unprivileged datagram socket is opened with the same options. A raw
// refusal for any other reason is the answer, and no fallback is tried:
// privilegeRefused is that guard. When both are refused the error names
// both reasons, so an operator learns whether the fix is CAP_NET_RAW or
// net.ipv4.ping_group_range.
//
// The caller owns the returned Socket and MUST close it.
func OpenICMP(ctx context.Context, family Family, bind netip.Addr, df DFMode) (*Socket, error) {
	if df == DFUnspecified {
		return nil, ErrDFUnspecified
	}
	network, err := family.icmpNetwork()
	if err != nil {
		return nil, err
	}
	control, err := dfControl(family, df)
	if err != nil {
		return nil, err
	}
	bindAddr := ""
	if bind.IsValid() {
		bindAddr = bind.String()
	}
	conn, rawErr := openRawICMP(ctx, network, bindAddr, control)
	if rawErr == nil {
		id, idErr := rawIdentifier()
		if idErr != nil {
			conn.Close() //nolint:errcheck // the open failed; the identifier error is the answer
			return nil, idErr
		}
		return &Socket{conn: conn, kind: SocketRaw, id: id, family: family}, nil
	}
	if !privilegeRefused(rawErr) {
		return nil, fmt.Errorf("probe: open raw %s socket: %w", family, rawErr)
	}
	conn, id, dgramErr := openDatagramICMP(family, bind, df)
	if dgramErr != nil {
		// Both refusals are wrapped, so errors.Is answers for either cause.
		return nil, fmt.Errorf("probe: open %s socket: raw refused (%w, needs CAP_NET_RAW) and unprivileged datagram refused (%w, needs the process group inside net.ipv4.ping_group_range)", family, rawErr, dgramErr)
	}
	return &Socket{conn: conn, kind: SocketDatagram, id: id, family: family}, nil
}

// rawIdentifier picks the echo identifier of a raw socket: two random
// octets, so two probers open at once on one host answer to different
// identifiers, as they do on the datagram kind where the kernel assigns one
// per socket. The process id, which every prober used before, is one value
// for every socket in the daemon.
func rawIdentifier() (uint16, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("probe: echo identifier: %w", err)
	}
	return binary.BigEndian.Uint16(b[:]), nil
}

// privilegeRefused is the guard on the unprivileged fallback: it reports
// whether the raw socket was refused for want of privilege, which is EPERM
// (no CAP_NET_RAW) or EACCES (a policy such as SELinux or a seccomp filter
// refusing the socket type). Every other refusal, an unsupported family, an
// address that cannot be bound, a descriptor limit, is a failure the
// fallback would only hide, so it stays a failure: a datagram socket opened
// over one would answer as if the raw one had, on a host whose real defect
// nobody was told about.
func privilegeRefused(err error) bool {
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	if errors.Is(err, syscall.EACCES) {
		return true
	}
	return false
}

// listenRawICMP opens the raw ICMP socket through net.ListenConfig, with
// control installing the DF options before the socket is bound.
func listenRawICMP(ctx context.Context, network, bindAddr string, control func(network, address string, c syscall.RawConn) error) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: control}
	return lc.ListenPacket(ctx, network, bindAddr)
}

// icmpNetwork is the net.ListenConfig network name of the raw ICMP socket for
// f. FamilyAny names no socket and is refused.
func (f Family) icmpNetwork() (string, error) {
	switch f {
	case FamilyIPv4:
		return NetworkICMPv4, nil
	case FamilyIPv6:
		return NetworkICMPv6, nil
	default:
		return "", ErrFamilyRequired
	}
}
