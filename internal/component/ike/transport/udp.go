// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the two IKE sockets, the DF send and the error queue
// RFC: rfc/short/rfc7296.md -- IKE uses UDP port 500 (Section 2.1)
// Related: udp_linux.go, udp_other.go -- the platform half: IP_RECVERR, the DF toggle, the drain
package transport

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"syscall"

	"golang.org/x/net/ipv4"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/probe"
)

const (
	IKEPort    = 500
	MaxMsgSize = 3000 // RFC 7296 Section 2.1: handle up to 3000 bytes
)

var (
	ErrClosed      = errors.New("transport: closed")
	ErrSendFailed  = errors.New("transport: send failed")
	ErrNoLocalAddr = errors.New("transport: no local address configured")
)

// Packet is an inbound IKE message with its actual source and destination.
// Response callers MUST reuse LocalAddr with SendFrom, including its port.
type Packet struct {
	Data       []byte
	RemoteAddr *net.UDPAddr
	LocalAddr  *net.UDPAddr

	// NATT records that this datagram arrived on the NAT-T socket, port 4500.
	//
	// RFC 7296 Section 2.11 MUST: an implementation
	// "MUST specify the address and port at which the request was received as the source address and port in the response".
	// A handler therefore needs to know which of the two sockets delivered the
	// request, and the socket is what knows. Run stamps it from the transport's own
	// role, so no handler infers it from a port number.
	//
	// The zero value is the plain IKE socket. That is what a hand-built Packet in a
	// test means. It is also the unfloated default of RFC 7296 Section 2.23.
	NATT bool
}

// refusalQueueDepth bounds the refusals channel. One probe holds one SA's
// request window, and a refusal is one entry per DF datagram the path
// turned back, so the depth is the number of SAs that can be probing at
// once with room to spare; an entry that finds the channel full is dropped
// with a log line rather than blocking Run, which must keep reading IKE.
const refusalQueueDepth = 16

// SizeRefusal is one EMSGSIZE the kernel queued on this socket: a datagram
// sent with the DF bit was larger than the path, and either a router on the
// path said so with Fragmentation Needed or this host's kernel refused the
// send against its cached path MTU. Run reads the entry off the error queue
// and delivers it on Refusals for the engine to match against the SA that
// holds a probe.
type SizeRefusal struct {
	// Peer is the address and port the refused datagram was sent to, which
	// is the SA's remote endpoint: the socket is shared by every SA, and this
	// is how the engine tells whose probe was refused. A LOCAL refusal names
	// the address and port 0: the kernel fills that entry from the socket's
	// connected port (net/ipv4/ip_output.c __ip_append_data calls
	// ip_local_error with inet_dport), which an unconnected socket has not
	// got. The engine learns a local refusal from SendDF's own error, so the
	// event is confirmation, never the match.
	Peer netip.AddrPort
	// Outcome is ErrQueueMTUReported when MTU carries the reported next-hop
	// MTU, and ErrQueueMTUUnreported when the refusal named no usable value.
	Outcome probe.ErrQueueOutcome
	// MTU is the reported next-hop MTU in octets, meaningful only under
	// ErrQueueMTUReported.
	MTU uint32
	// Local is true when this host's own kernel refused the send against its
	// cached path MTU (the honor-cache mode), so nothing left the host and
	// MTU is the cache's estimate. Offender is then the zero Addr.
	Local bool
	// Offender is the router that answered, for a refusal from the network.
	Offender netip.Addr
	// NATT records which socket the refusal arrived on, as Packet.NATT does.
	NATT bool
}

// UDPTransport listens on a UDP socket and dispatches incoming IKE packets.
//
// Run reads the socket, and every sender writes it through Send, SendFrom or
// SendDF under mu, which also guards closed. SendDF toggles a socket-wide
// option around its one write, so the lock keeps another SA's datagram from
// leaving with the probe's DF setting. Safe for concurrent use.
type UDPTransport struct {
	logger *slog.Logger
	conn   *net.UDPConn

	// receiveDestination records whether ancillary data supplies the actual
	// destination. Without that capability only a concrete bind is allowed.
	receiveDestination bool

	// natT records that this socket is the NAT-T one, port 4500.
	//
	// RFC 3948 Section 2.2 puts a four-octet non-ESP marker on every IKE message
	// that port carries, so a sender needs to know which socket it holds.
	// The role is fixed at construction and never inferred from the bind port.
	// A port comparison reads the wrong answer under the ze.test.ike.port override,
	// where neither socket carries a well-known port.
	natT bool

	// bypassErr is why the kernel refused this socket its per-socket IPsec bypass
	// (dataplane.BypassIKESocket), nil when the socket carries it or the platform needs none.
	// Written by exemptFromIPsec, which newTransport calls before it returns the
	// transport and nothing calls after, so every reader sees it without mu.
	bypassErr error

	// mu serializes every write on the socket and guards closed. Send,
	// SendFrom and SendDF MUST hold it for the whole write: SendDF changes a
	// socket-wide option for the duration of its write, and a write that
	// interleaved from another goroutine would leave under that option.
	mu     sync.Mutex
	closed bool

	inbound chan Packet
	// refusals carries the size refusals Run reads off the error queue.
	// Bounded by refusalQueueDepth; an entry that finds it full is dropped.
	refusals chan SizeRefusal
}

// NewUDPTransport creates a transport listening on the given local address.
// The socket carries plain IKE, so it adds no non-ESP marker.
func NewUDPTransport(localAddr string, logger *slog.Logger) (*UDPTransport, error) {
	return newTransport(localAddr, false, logger)
}

// NewNATTTransport creates the NAT-T transport, the one RFC 7296 Section 2.23
// reserves for UDP-encapsulated ESP and IKE.
//
// IsNATT reports true for the result, so every sender that holds it frames its
// messages with the non-ESP marker of RFC 3948 Section 2.2.
func NewNATTTransport(localAddr string, logger *slog.Logger) (*UDPTransport, error) {
	return newTransport(localAddr, true, logger)
}

func newTransport(localAddr string, natT bool, logger *slog.Logger) (*UDPTransport, error) {
	addr, err := net.ResolveUDPAddr("udp4", localAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	t := &UDPTransport{
		logger:   logger,
		conn:     conn,
		natT:     natT,
		inbound:  make(chan Packet, 64),
		refusals: make(chan SizeRefusal, refusalQueueDepth),
	}
	if err := t.installDestinationCapture(); err != nil {
		conn.Close() //nolint:errcheck // the socket is discarded with the error
		return nil, err
	}
	if err := t.installErrorQueue(); err != nil {
		conn.Close() //nolint:errcheck // the socket is discarded with the error
		return nil, err
	}
	// RFC 4301 Section 5.2
	t.exemptFromIPsec(localAddr, dataplane.BypassIKESocket)
	return t, nil
}

// exemptFromIPsec sets the socket's per-socket IPsec bypass through bypass and records
// the outcome for BypassError. newTransport calls it before the socket is returned, so
// no datagram crosses the socket unexempted; a test passes a refusing bypass.
//
// A refusal is not fatal, as strongSwan's socket-default plugin logs and keeps a socket
// whose bypass failed: the socket still carries IKE wherever no policy captures it, and
// a node with no CAP_NET_ADMIN or no XFRM installs no policy at all. It is logged at
// Warn and kept, so the engine's ipsec-ike-socket-bypass doctor check reports it.
func (t *UDPTransport) exemptFromIPsec(localAddr string, bypass func(*net.UDPConn) error) {
	t.bypassErr = bypass(t.conn)
	if t.bypassErr == nil {
		return
	}
	t.logger.Warn("ike: could not exempt the IKE socket from IPsec: "+BypassCause(t.bypassErr)+
		". Under `vpn ipsec unmatched discard`, or a traffic selector covering the peer's IKE address, "+
		"the SPD captures ze's own IKE and the tunnel cannot be rekeyed or torn down. To fix it: "+
		BypassFix(t.bypassErr),
		"code", CodeDoctorIPsecIKESocketBypass, "address", localAddr, "error", t.bypassErr)
}

// bypassFault classifies why the kernel refused an IKE socket its bypass, which decides
// the fix (bypassFaultOf). It is closed: only bypassFaultOf produces it.
type bypassFault uint8

const (
	// bypassFaultUnspecified is the zero value, which bypassFaultOf never answers.
	bypassFaultUnspecified bypassFault = iota
	// bypassFaultCapability is a process without CAP_NET_ADMIN: EPERM.
	bypassFaultCapability
	// bypassFaultKernel is a kernel built without CONFIG_XFRM: ENOPROTOOPT.
	bypassFaultKernel
	// bypassFaultSecurityModule is a security module that refused the option: EACCES.
	bypassFaultSecurityModule
	// bypassFaultUnknown is any other answer: Ze cannot tell which fix applies, and says
	// so rather than name one that may not.
	bypassFaultUnknown
)

// bypassFaultOf classifies the error dataplane.BypassIKESocket answered, by the errno
// its setsockopt(IP_XFRM_POLICY) returned (net/ipv4/ip_sockglue.c and
// net/xfrm/xfrm_state.c, Linux master):
//
//   - EPERM: do_ip_setsockopt asks CAP_NET_ADMIN before anything else, "err = -EPERM;
//     if (!sockopt_ns_capable(sock_net(sk)->user_ns, CAP_NET_ADMIN)) break;", so a
//     process without it gets EPERM whatever the kernel was built with.
//   - ENOPROTOOPT: a kernel built without CONFIG_XFRM compiles xfrm_user_policy as the
//     include/net/xfrm.h stub, which answers "return -ENOPROTOOPT;". The capability is
//     asked first, so ENOPROTOOPT also proves ze holds CAP_NET_ADMIN.
//   - EACCES: neither the capability check nor xfrm_user_policy answers it. The security
//     module's hook does, before the option reaches IP: do_sock_setsockopt (net/socket.c)
//     runs "err = security_socket_setsockopt(sock, level, optname); if (err) goto
//     out_put;", SELinux's selinux_socket_setsockopt answers sock_has_perm(sock->sk,
//     SOCKET__SETOPT), whose avc_denied answers "return -EACCES;" (security/selinux/
//     avc.c), and AppArmor's apparmor_socket_setsockopt answers aa_sock_opt_perm, whose
//     aa_check_perms sets "error = -EACCES;" (security/apparmor/lib.c). So it is SELinux
//     or AppArmor, which no capability changes. espFormBypassRefusal
//     (dataplane/espform_open_linux.go) reads the same option's EACCES the same way.
//   - Anything else, EINVAL above all: xfrm_user_policy starts from -EINVAL and keeps it
//     when no key manager compiles the policy, which is a kernel with CONFIG_XFRM but no
//     xfrm_user, or a policy the kernel refused. Ze cannot tell which: Unknown.
//
// The errno constants come from syscall, so the classifier builds on every platform;
// off Linux BypassIKESocket never refuses, so it is never asked there.
func bypassFaultOf(err error) bypassFault {
	if errors.Is(err, syscall.EPERM) {
		return bypassFaultCapability
	}
	if errors.Is(err, syscall.ENOPROTOOPT) {
		return bypassFaultKernel
	}
	if errors.Is(err, syscall.EACCES) {
		return bypassFaultSecurityModule
	}
	return bypassFaultUnknown
}

// bypassFixSecurityModule lists this boot's denials by SELinux (an "avc:" record) and
// AppArmor (apparmor="DENIED"), the same command espFormFixSecurityModule
// (dataplane/espform_open_linux.go) gives for the same option's EACCES.
const bypassFixSecurityModule = `sudo journalctl -b --grep 'avc:|apparmor="DENIED"'` //nolint:gosec // G101: an operator fix command, not a credential

// bypassFixKernel is the kernel fix, which the restart for this process completes.
const bypassFixKernel = "boot a kernel built with CONFIG_XFRM (Ze's own kernel carries it), then restart ze: " //nolint:gosec // G101: an operator fix text, not a credential

// BypassCause answers, in the operator's words, why the kernel refused a socket the
// bypass with err, a BypassError that is not nil: a clause the Warn line and the
// engine's ipsec-ike-socket-bypass finding each put after their own subject. Safe for
// concurrent use.
func BypassCause(err error) string {
	switch bypassFaultOf(err) {
	case bypassFaultCapability:
		return "ze lacks CAP_NET_ADMIN, which the kernel asks of a process that sets a socket's IPsec policy"
	case bypassFaultKernel:
		return "the kernel is built without CONFIG_XFRM, so it has no IPsec socket policy to set"
	case bypassFaultSecurityModule:
		return "a security module (SELinux or AppArmor) refused ze the IP_XFRM_POLICY socket option"
	case bypassFaultUnknown:
		return "Ze could not tell why from the kernel's answer: ze may lack CAP_NET_ADMIN, " +
			"or the kernel may be built without CONFIG_XFRM"
	case bypassFaultUnspecified:
		panic("BUG: transport: bypassFaultOf answered bypassFaultUnspecified")
	}
	panic("BUG: transport: undeclared bypassFault")
}

// BypassFix answers the fix for a socket whose BypassError is err, not nil, chosen by
// its cause (bypassFaultOf). The bypass is set when a socket opens, so only a socket
// opened again carries it: every fix ends in a restart once the cause is gone.
// privilege.CapabilityRemedy words the capability fix for this process, the unit
// reinstall that also starts ze again only under ze.service. `ze signal restart` exits
// without starting a new process, so privilege.RestartRemedy words every other restart:
// systemctl only under ze.service, the stop-and-start steps otherwise. The Warn line
// above and the engine's ipsec-ike-socket-bypass doctor finding both call it, so the
// two say the same thing. Safe for concurrent use.
func BypassFix(err error) string {
	switch bypassFaultOf(err) {
	case bypassFaultCapability:
		return privilege.CapabilityRemedy("CAP_NET_ADMIN")
	case bypassFaultKernel:
		return bypassFixKernel + privilege.RestartRemedy()
	case bypassFaultSecurityModule:
		return "find the denial with " + bypassFixSecurityModule + ", allow ze the IP_XFRM_POLICY " +
			"socket option in that policy, then restart ze: " + privilege.RestartRemedy()
	case bypassFaultUnknown:
		return "if ze lacks CAP_NET_ADMIN: " + privilege.CapabilityRemedy("CAP_NET_ADMIN") +
			"; if the kernel lacks CONFIG_XFRM: " + bypassFixKernel + privilege.RestartRemedy()
	case bypassFaultUnspecified:
		panic("BUG: transport: bypassFaultOf answered bypassFaultUnspecified")
	}
	panic("BUG: transport: undeclared bypassFault")
}

// BypassError answers why the kernel refused this socket its per-socket IPsec bypass
// (RFC 4301 Section 5.2), nil when the socket carries it or the platform has no kernel
// SPD that could capture it (dataplane/ike_socket_bypass_other.go). Safe for concurrent use: the outcome is
// fixed before the transport is returned.
func (t *UDPTransport) BypassError() error {
	return t.bypassErr
}

// installDestinationCapture enables per-datagram destination metadata. On a
// platform without it, only a concrete bind identifies the destination safely.
func (t *UDPTransport) installDestinationCapture() error {
	if len(ipv4.NewControlMessage(ipv4.FlagDst)) == 0 {
		if t.localUDPAddr().IP.IsUnspecified() {
			return errors.New("transport: wildcard destination capture unavailable")
		}
		return nil
	}
	if err := ipv4.NewPacketConn(t.conn).SetControlMessage(ipv4.FlagDst, true); err != nil {
		return fmt.Errorf("transport: enable destination capture: %w", err)
	}
	t.receiveDestination = true
	return nil
}

// IsNATT reports whether this socket is the NAT-T one.
//
// It fails closed. A nil transport reads false, so a caller with no socket adds no
// marker and sends nothing (ai/rules/evidence.md).
func (t *UDPTransport) IsNATT() bool {
	if t == nil {
		return false
	}
	return t.natT
}

// Recv returns the channel of inbound packets.
func (t *UDPTransport) Recv() <-chan Packet {
	return t.inbound
}

// Send writes a raw IKE message to the remote address under the kernel's
// default DF policy. It holds mu across the write, so it never interleaves
// with a SendDF on the same socket. Safe for concurrent use.
func (t *UDPTransport) Send(data []byte, remote *net.UDPAddr) error {
	return t.SendFrom(data, nil, remote)
}

// SendFrom writes a raw IKE message with an explicit source. A nil local uses
// normal routing. Otherwise local MUST contain a concrete IPv4 address and the
// socket's bound port; a different port is never silently substituted. A socket
// bound to a concrete address permits only that address. Unsupported source
// selection returns an error without sending. Safe for concurrent use.
//
// RFC 7296 Section 2.11: "It MUST specify the address and port at which the
// request was received as the source address and port in the response."
// Response callers MUST pass Packet.LocalAddr as local.
func (t *UDPTransport) SendFrom(data []byte, local, remote *net.UDPAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrClosed
	}
	control, err := t.sourceControl(local)
	if err != nil {
		return errors.Join(ErrSendFailed, err)
	}
	return t.write(data, control, remote)
}

// sourceControl validates the entire requested endpoint before composing the
// ancillary data. A concrete bind needs no control message to select itself.
func (t *UDPTransport) sourceControl(local *net.UDPAddr) ([]byte, error) {
	if local == nil {
		return nil, nil
	}
	bound := t.localUDPAddr()
	if local.Port != bound.Port {
		return nil, errors.New("transport: source port differs from socket port")
	}
	if local.IP.To4() == nil {
		return nil, errors.New("transport: source must be an IPv4 address")
	}
	if local.IP.IsUnspecified() {
		return nil, errors.New("transport: source address is unspecified")
	}
	if local.IP.IsMulticast() {
		return nil, errors.New("transport: source address is multicast")
	}
	if local.IP.Equal(net.IPv4bcast) {
		return nil, errors.New("transport: source address is broadcast")
	}
	if local.Zone != "" {
		return nil, errors.New("transport: IPv4 source cannot carry a zone")
	}
	if !bound.IP.IsUnspecified() {
		if !local.IP.Equal(bound.IP) {
			return nil, errors.New("transport: source address differs from socket address")
		}
		return nil, nil
	}
	cm := ipv4.ControlMessage{Src: local.IP}
	control := cm.Marshal()
	if len(control) == 0 {
		return nil, errors.New("transport: explicit source selection unavailable")
	}
	return control, nil
}

// write is the one write on the socket, under mu, with the error queue's
// side effect handled. With IP_RECVERR on, the kernel hands an ICMP error
// about an EARLIER datagram to the next send on the socket as that send's
// failure (net/core/sock.c sock_alloc_send_pskb returns the pending sk_err
// before it allocates), and the datagram is never sent. So a failed write
// drains the queue first: a drain that found entries says the error was a
// report about earlier traffic, now delivered on Refusals, and the write is
// made once more. A drain that found nothing leaves an error about this
// write, and so does one that found a LOCAL entry: only this socket's own
// refused send queues one, under mu, so it is this write's cache refusal
// and a second attempt would draw the same answer.
func (t *UDPTransport) write(data, control []byte, remote *net.UDPAddr) error {
	err := t.writeDatagram(data, control, remote)
	if err == nil {
		return nil
	}
	entries, local := t.drainErrorQueue()
	if entries == 0 {
		return errors.Join(ErrSendFailed, err)
	}
	if local {
		return errors.Join(ErrSendFailed, err)
	}
	err = t.writeDatagram(data, control, remote)
	if err == nil {
		return nil
	}
	t.drainErrorQueue()
	return errors.Join(ErrSendFailed, err)
}

// writeDatagram keeps nil-source sends usable on platforms without ancillary
// writes. Every caller MUST hold mu, including both attempts after an ICMP error.
func (t *UDPTransport) writeDatagram(data, control []byte, remote *net.UDPAddr) error {
	if len(control) == 0 {
		_, err := t.conn.WriteToUDP(data, remote)
		return err
	}
	_, _, err := t.conn.WriteMsgUDP(data, control, remote)
	return err
}

// SendDF writes a raw IKE message to the remote address with the DF mode df
// installed for that one datagram: DFHonorCache sets the DF bit and lets
// the kernel refuse a datagram larger than its cached path MTU with
// EMSGSIZE, DFBypassCache sets the bit and puts the datagram on the wire at
// full size, DFOff clears the bit so the kernel fragments. The socket's
// IP_MTU_DISCOVER is restored to its prior value before this returns, on
// every path, so a later Send by another SA leaves under the kernel default.
// It holds mu across the toggle and the write, so no other write on the
// socket can leave under the toggled option. Safe for concurrent use.
// The local endpoint follows SendFrom's contract: nil selects normal routing;
// an explicit source MUST name a concrete IPv4 address and this socket's port.
//
// Off Linux it answers probe.ErrDFUnsupported and writes nothing. A send the
// kernel refused against its cached path MTU is reported with EMSGSIZE
// joined to ErrSendFailed, and that is the caller's signal: the refusal's
// LOCAL entry reaches Refusals as well, with Local set and, on this
// unconnected socket, the peer's port unknown to the kernel.
func (t *UDPTransport) SendDF(data []byte, local, remote *net.UDPAddr, df probe.DFMode) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrClosed
	}
	control, err := t.sourceControl(local)
	if err != nil {
		return errors.Join(ErrSendFailed, err)
	}
	return t.writeWithDF(data, control, remote, df)
}

// Refusals returns the channel of size refusals the kernel queued for
// datagrams this socket sent with the DF bit. The engine reads it beside
// Recv. It is bounded by refusalQueueDepth and never blocks Run.
func (t *UDPTransport) Refusals() <-chan SizeRefusal {
	return t.refusals
}

// deliverQueuedError turns one error-queue entry into a SizeRefusal on the
// channel. Only an EMSGSIZE is a size refusal; the kernel queues every ICMP
// error for the socket once IP_RECVERR is on (a port unreachable, a host
// unreachable), and those are logged at Debug and dropped because nothing
// in the engine acts on them. A full channel drops the entry with a log
// line: Run must return to the socket, and the engine's retransmit timer
// carries the DF-clear copy in any case.
func (t *UDPTransport) deliverQueuedError(entry probe.QueuedError) {
	if entry.Errno != syscall.EMSGSIZE {
		t.logger.Debug("ike transport: queued error is not a size refusal, dropped",
			"peer", entry.Dest, "errno", entry.Errno, "offender", entry.Offender)
		return
	}
	refusal := SizeRefusal{
		Peer:     entry.Dest,
		Outcome:  entry.Outcome,
		MTU:      entry.MTU,
		Local:    entry.Local,
		Offender: entry.Offender,
		NATT:     t.natT,
	}
	select {
	case t.refusals <- refusal:
	default:
		t.logger.Warn("ike transport: refusal queue full, dropping size refusal",
			"peer", refusal.Peer, "mtu", refusal.MTU)
	}
}

// Conn returns the underlying UDP connection, for a socket option set on it
// (EnableESPInUDP). Nothing writes through it: every write goes through Send,
// SendFrom or SendDF, under mu, the NAT keepalive included.
func (t *UDPTransport) Conn() *net.UDPConn {
	return t.conn
}

// LocalAddr returns the local address the transport is listening on.
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// Run reads packets from the UDP socket until Close is called. The loop has
// no bound of its own: it ends when the socket is closed.
//
// With IP_RECVERR on the socket, an ICMP error the kernel matched to a
// datagram this socket sent returns from the read as that errno once, with
// the entry on the error queue. A read error that is not the close is
// therefore first taken to the queue: a drain that found entries explains
// the error and delivers the size refusals; a drain that found nothing
// leaves an error the queue did not cause, which is logged as before.
func (t *UDPTransport) Run() {
	buf := make([]byte, MaxMsgSize)
	control := ipv4.NewControlMessage(ipv4.FlagDst)
	for {
		n, remoteAddr, localAddr, err := t.readDatagram(buf, control)
		if err != nil {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return
			}
			if entries, _ := t.drainErrorQueue(); entries > 0 {
				continue
			}
			t.logger.Warn("ike transport: read error", "error", err)
			continue
		}
		if n < 28 { // IKE header is 28 bytes minimum
			continue
		}
		pkt := Packet{
			Data:       make([]byte, n),
			RemoteAddr: remoteAddr,
			LocalAddr:  localAddr,
			NATT:       t.natT,
		}
		copy(pkt.Data, buf[:n])

		select {
		case t.inbound <- pkt:
		default:
			t.logger.Warn("ike transport: inbound queue full, dropping packet",
				"remote", remoteAddr)
		}
	}
}

// readDatagram reads the IP-header destination, not the wildcard socket bind.
// RFC 4555 Section 3.9: "The exchange responder MUST verify that the contents
// of the NO_NATS_ALLOWED notification match the addresses in the IP header."
// Missing metadata is an error, never an invented local address.
func (t *UDPTransport) readDatagram(data, control []byte) (int, *net.UDPAddr, *net.UDPAddr, error) {
	if !t.receiveDestination {
		n, remote, err := t.conn.ReadFromUDP(data)
		return n, remote, t.localUDPAddr(), err
	}
	n, controlLen, _, remote, err := t.conn.ReadMsgUDP(data, control)
	if err != nil {
		return 0, nil, nil, err
	}
	var cm ipv4.ControlMessage
	if err := cm.Parse(control[:controlLen]); err != nil {
		return 0, nil, nil, fmt.Errorf("transport: parse destination: %w", err)
	}
	if cm.Dst.To4() == nil {
		return 0, nil, nil, errors.New("transport: missing packet destination")
	}
	if cm.Dst.IsUnspecified() {
		return 0, nil, nil, errors.New("transport: unspecified packet destination")
	}
	local := &net.UDPAddr{IP: cm.Dst, Port: t.localUDPAddr().Port}
	return n, remote, local, nil
}

func (t *UDPTransport) localUDPAddr() *net.UDPAddr {
	addr, ok := t.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil
	}
	return addr
}

// Close shuts down the transport.
func (t *UDPTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return t.conn.Close()
}
