//go:build linux

// Design: docs/functional-tests.md -- an LDP neighbor on a veth pair
// Related: register_ldp_neighbor_linux.go -- the fixture names
// Related: ldp_session_end_linux.go -- the two scenarios that drive this neighbor
// Related: plugin_fixture_clamped_path_linux.go -- createTestNetns, configureLink, runInNetns

package fixture

// ldp_neighbor_linux.go plays one LDP neighbor of a Ze daemon over a veth pair:
// it sends Link Hellos, accepts the TCP session Ze opens on port 646, and either
// brings that session to operational or rejects Ze's Initialization.
//
// WHY TWO NAMESPACES. `ldp/neighbor-link` runs Ze in `<prefix>-z` and keeps the
// neighbor's link in `<prefix>-n`, joined by one veth pair. Both ends in one
// namespace do not work: the neighbor's Hello arrives on Ze's link from an
// address that namespace owns, and Linux drops it as a martian unless
// accept_local is set. The guest's root namespace, where the ldp suite runs,
// does not work either: every LDP daemon there binds 224.0.0.2:646, and a socket
// bound to a group hears that group's datagrams from any link some socket on the
// host joined it on (IP_MULTICAST_ALL is on by default). A sibling test's daemon
// would then form an adjacency with this neighbor, and its own empty-neighbor
// assertion would fail.
//
// WHY THE NEIGHBOR IS PASSIVE. RFC 5036 Section 2.5.2: "If A1 > A2, LSR1 plays
// the active role; otherwise, it is passive." The neighbor's transport address
// 10.99.0.1 is below Ze's 10.99.0.2, so Ze is the active LSR that dials, which
// is the only role Ze plays today (startSessionForAdj).
//
// WHY ITS OWN CODEC. internal/plugins/ldp is gated by ze_ldp and this package is
// always-on (feature-gates.txt), so it MUST NOT import it, and most of that
// package's encoders are unexported. A neighbor written from RFC 5036 also does
// not share a defect with the encoder it talks to.

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/ipv4"
)

// The topology `ldp/neighbor-link` builds. The scenarios and the .ci configs
// name the same values: Ze's link, LSR-ID and transport address, and the
// neighbor's.
const (
	ldpNeighborZeLink   = "ldpz0"
	ldpNeighborPeerLink = "ldpn0"
	ldpNeighborZeCIDR   = "10.99.0.2/24"
	ldpNeighborPeerCIDR = "10.99.0.1/24"
	// ldpNeighborPeerAddress is the neighbor's address and LSR-ID, as Ze names
	// it in a session event's peer-address and in `show ldp neighbor`.
	ldpNeighborPeerAddress = "10.99.0.1"
	ldpNeighborLinkMTU     = 1500
	ldpNeighborZeSuffix    = "-z"
	ldpNeighborPeerSuffix  = "-n"
	// ldpNeighborIdentifier is the neighbor's LDP Identifier as Ze writes it
	// (ldp.AdjacencyKey): its LSR-ID and label space 0, the key of every
	// session event and log line Ze emits about it.
	ldpNeighborIdentifier = "10.99.0.1:0"
	// ldpNamespaceKeyword is the argument word that names the neighbor's
	// network namespace, as in `netns <name>`.
	ldpNamespaceKeyword = "netns"
)

var (
	ldpNeighborZeLSR   = [4]byte{10, 99, 0, 2}
	ldpNeighborPeerLSR = [4]byte{10, 99, 0, 1}
	ldpNeighborGroup   = netip.AddrFrom4([4]byte{224, 0, 0, 2})
)

// ldpWireVersion is the protocol version, RFC 5036 Section 3.1: "This version
// of the specification specifies LDP protocol version 1."
const ldpWireVersion uint16 = 1

// RFC 5036 wire values the neighbor sends or reads (Sections 3.1, 3.3, 3.4.6,
// 3.5.1 to 3.5.4, 3.9).
const (
	ldpWirePort             = 646
	ldpWirePDUHeaderOctets  = 10
	ldpWireMessageHdrOctets = 8
	ldpWireTLVHeaderOctets  = 4
	// ldpWirePDULengthMax is the largest PDU Length read before the session
	// negotiated another: RFC 5036 Section 3.1, "Prior to completion of the
	// negotiation, the maximum allowable length is 4096 bytes."
	ldpWirePDULengthMax = 4096

	ldpWireNotification   uint16 = 0x0001
	ldpWireHello          uint16 = 0x0100
	ldpWireInitialization uint16 = 0x0200
	ldpWireKeepAlive      uint16 = 0x0201
	ldpWireTypeMask       uint16 = 0x7fff // clears the U-bit of a message or TLV type

	ldpWireStatusTLV        uint16 = 0x0300
	ldpWireCommonHelloTLV   uint16 = 0x0400
	ldpWireIPv4TransportTLV uint16 = 0x0401
	ldpWireCommonSessionTLV uint16 = 0x0500
	ldpWireCommonSessionLen        = 14

	// ldpWireStatusFatal is the E-bit of a Status Code (RFC 5036 Section 3.4.6).
	ldpWireStatusFatal uint32 = 0x80000000
	// ldpWireRejectedMaxPDU is Session Rejected/Parameters Max PDU Length
	// (RFC 5036 Section 3.9, 0x00000012).
	ldpWireRejectedMaxPDU uint32 = 0x00000012
)

// The neighbor's own session and Hello parameters.
const (
	// ldpNeighborHelloHold is the Hello Hold Time the neighbor proposes, in
	// seconds. Ze holds the adjacency for it, so a scenario that pauses its
	// Hellos for one step keeps the adjacency.
	ldpNeighborHelloHold uint16 = 30
	// ldpNeighborKeepalive is the KeepAlive Time the neighbor proposes, in
	// seconds. Each scenario ends inside it; the drain worker answers Ze's
	// KeepAlives besides.
	ldpNeighborKeepalive uint16 = 60
	// ldpNeighborHelloEvery is the neighbor's Hello cadence. Ze dials on a
	// Hello, so this is the granularity of every redial the scenarios observe.
	ldpNeighborHelloEvery = time.Second
	// ldpNeighborReadWait bounds one read of a handshake message from Ze.
	ldpNeighborReadWait = 10 * time.Second
)

// ldpNeighborLinkDriver is `ldp/neighbor-link netns <prefix> run <argv...>`. It
// builds `<prefix>-z` and `<prefix>-n`, joins them with a veth pair, runs argv
// in `<prefix>-z` and removes both namespaces when argv exits. Its exit is
// argv's.
func ldpNeighborLinkDriver(ctx context.Context, args []string) error {
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		return err
	}
	if err := refuseLDPNeighborLinkOptions(plan); err != nil {
		return err
	}
	// The thread stays locked for the life of the process: a namespace switch is
	// per thread, and argv MUST be forked from this thread to inherit Ze's.
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return fmt.Errorf("get the current network namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort close at exit

	zeSide, err := createTestNetns(plan.prefix+ldpNeighborZeSuffix, orig)
	if err != nil {
		return err
	}
	defer zeSide.remove()

	peerSide, err := createTestNetns(plan.prefix+ldpNeighborPeerSuffix, orig)
	if err != nil {
		return err
	}
	defer peerSide.remove()

	if err := wireLDPNeighborLink(zeSide, peerSide); err != nil {
		return err
	}
	return runInNetns(ctx, plan, zeSide, orig)
}

// refuseLDPNeighborLinkOptions refuses every clamped-path keyword the LDP link
// has no use for, so a copied line fails before any namespace exists.
func refuseLDPNeighborLinkOptions(plan *netnsRunPlan) error {
	const refusal = "ldp/neighbor-link takes only netns <prefix> and run <argv...>"
	if plan.routeMTU != 0 {
		return errors.New(refusal + ": route-mtu given")
	}
	if len(plan.farDaemons) != 0 {
		return errors.New(refusal + ": far-daemon given")
	}
	if plan.ipv6 {
		return errors.New(refusal + ": ipv6 given")
	}
	if len(plan.peers) != 0 {
		return errors.New(refusal + ": peer given")
	}
	if plan.withoutNetRaw {
		return errors.New(refusal + ": without-net-raw given")
	}
	return nil
}

// wireLDPNeighborLink creates the veth pair with one end in each namespace,
// addresses both ends and brings them up.
func wireLDPNeighborLink(zeSide, peerSide *testNetns) error {
	pair := &netlink.Veth{
		Name:          ldpNeighborZeLink,
		PeerName:      ldpNeighborPeerLink,
		PeerNamespace: netlink.NsFd(int(peerSide.ns)),
	}
	if err := zeSide.handle.LinkAdd(pair); err != nil {
		return fmt.Errorf("create %s/%s: %w", ldpNeighborZeLink, ldpNeighborPeerLink, err)
	}
	if err := zeSide.configureLink(ldpNeighborZeLink, ldpNeighborLinkMTU, ldpNeighborZeCIDR); err != nil {
		return err
	}
	return peerSide.configureLink(ldpNeighborPeerLink, ldpNeighborLinkMTU, ldpNeighborPeerCIDR)
}

// ldpNeighbor is the neighbor: its Hello socket, its session listener, the
// Hello sender and every session it accepted, all opened in the neighbor's
// namespace. openLDPNeighbor returns it; close MUST be called, after the daemon
// stopped, so a session the scenario left operational ends with the daemon and
// never as a failed setup attempt. Safe for concurrent use.
type ldpNeighbor struct {
	listener *net.TCPListener
	hellos   *ldpHelloer

	mu       sync.Mutex
	sessions []*ldpNeighborSession
}

// openLDPNeighbor opens the neighbor's sockets inside namespace and starts its
// Hello sender, disabled until the scenario enables it.
func openLDPNeighbor(ctx context.Context, namespace string) (*ldpNeighbor, error) {
	var hello *net.UDPConn
	var listener *net.TCPListener
	err := inNamedNetns(namespace, func() error {
		var openErr error
		hello, listener, openErr = openLDPNeighborSockets(ctx)
		return openErr
	})
	if err != nil {
		return nil, err
	}
	return &ldpNeighbor{listener: listener, hellos: startLDPHelloer(hello)}, nil
}

// openLDPNeighborSockets opens the Hello socket, pinned to the neighbor's link
// with TTL 1, and the listener on the neighbor's transport address. The calling
// thread MUST be in the neighbor's namespace.
func openLDPNeighborSockets(ctx context.Context) (*net.UDPConn, *net.TCPListener, error) {
	link, err := net.InterfaceByName(ldpNeighborPeerLink)
	if err != nil {
		return nil, nil, fmt.Errorf("find %s: %w", ldpNeighborPeerLink, err)
	}
	address := netip.AddrFrom4(ldpNeighborPeerLSR)
	var config net.ListenConfig

	packets, err := config.ListenPacket(ctx, "udp4", netip.AddrPortFrom(address, 0).String())
	if err != nil {
		return nil, nil, fmt.Errorf("open the Hello socket: %w", err)
	}
	hello, ok := packets.(*net.UDPConn)
	if !ok {
		packets.Close() //nolint:errcheck // best-effort close on the error path
		return nil, nil, fmt.Errorf("the Hello socket is a %T, not a UDP socket", packets)
	}
	// RFC 5036 Section 2.4.1: "LDP Link Hellos are sent as UDP packets addressed
	// to the well-known LDP discovery port for the "all routers on this subnet"
	// group multicast address."
	if err := pinLDPHelloSocket(hello, link); err != nil {
		hello.Close() //nolint:errcheck // best-effort close on the error path
		return nil, nil, err
	}

	stream, err := config.Listen(ctx, "tcp4", netip.AddrPortFrom(address, ldpWirePort).String())
	if err != nil {
		hello.Close() //nolint:errcheck // best-effort close on the error path
		return nil, nil, fmt.Errorf("listen on TCP %d: %w", ldpWirePort, err)
	}
	listener, ok := stream.(*net.TCPListener)
	if !ok {
		stream.Close() //nolint:errcheck // best-effort close on the error path
		hello.Close()  //nolint:errcheck // best-effort close on the error path
		return nil, nil, fmt.Errorf("the session listener is a %T, not a TCP listener", stream)
	}
	return hello, listener, nil
}

// pinLDPHelloSocket sends the socket's multicast out link, link-scoped.
func pinLDPHelloSocket(hello *net.UDPConn, link *net.Interface) error {
	packets := ipv4.NewPacketConn(hello)
	if err := packets.SetMulticastInterface(link); err != nil {
		return fmt.Errorf("send Hellos out %s: %w", link.Name, err)
	}
	if err := packets.SetMulticastTTL(1); err != nil {
		return fmt.Errorf("set the Hello TTL to 1: %w", err)
	}
	if err := packets.SetMulticastLoopback(false); err != nil {
		return fmt.Errorf("keep Hellos off the neighbor's own sockets: %w", err)
	}
	return nil
}

// inNamedNetns runs open on a thread switched into the named namespace, then
// switches the thread back, so the sockets open creates belong to that
// namespace for their lifetime. A thread that cannot switch back stays locked,
// and the goroutine's exit retires it.
func inNamedNetns(name string, open func() error) error {
	target, err := netns.GetFromName(name)
	if err != nil {
		return fmt.Errorf("open network namespace %s: %w", name, err)
	}
	defer target.Close() //nolint:errcheck // best-effort close of a namespace handle

	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		return fmt.Errorf("get the current network namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort close of a namespace handle

	if err := netns.Set(target); err != nil {
		runtime.UnlockOSThread()
		return fmt.Errorf("enter %s: %w", name, err)
	}
	openErr := open()
	if err := netns.Set(orig); err != nil {
		return errors.Join(openErr, fmt.Errorf("return from %s: %w", name, err))
	}
	runtime.UnlockOSThread()
	return openErr
}

// acceptSession waits until deadline for the next TCP connection Ze opens,
// then reads and checks Ze's Initialization on it.
func (n *ldpNeighbor) acceptSession(deadline time.Time) (*ldpNeighborSession, error) {
	if err := n.listener.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("arm the accept deadline: %w", err)
	}
	conn, err := n.listener.AcceptTCP()
	if err != nil {
		return nil, fmt.Errorf("accept a session on TCP %d: %w", ldpWirePort, err)
	}
	session := &ldpNeighborSession{conn: conn}
	n.mu.Lock()
	n.sessions = append(n.sessions, session)
	n.mu.Unlock()
	if err := session.readInitialization(); err != nil {
		return nil, err
	}
	return session, nil
}

// close stops the Hello sender, closes every accepted session and both
// sockets, and returns the first failure of any of them.
func (n *ldpNeighbor) close() error {
	failure := n.hellos.stop()
	n.mu.Lock()
	sessions := n.sessions
	n.sessions = nil
	n.mu.Unlock()
	for _, session := range sessions {
		failure = errors.Join(failure, session.close())
	}
	if err := n.listener.Close(); err != nil {
		failure = errors.Join(failure, fmt.Errorf("close the session listener: %w", err))
	}
	return failure
}

// ldpNeighborSession is one TCP connection Ze opened to the neighbor. Once
// establish succeeds, one drain worker reads it, answering each KeepAlive Ze
// sends, until close closes the connection and waits for the worker. close
// MUST be called; ldpNeighbor.close calls it for every session it accepted.
type ldpNeighborSession struct {
	conn *net.TCPConn
	// zeInitID is the Message ID of Ze's Initialization, which a rejection names.
	zeInitID uint32

	closeOnce sync.Once
	closeErr  error
	drained   chan struct{} // nil until establish starts the worker
	drainErr  error         // written by the worker before it closes drained
}

// readInitialization reads Ze's first message and requires it to be the
// Initialization of a session for this neighbor's adjacency.
func (s *ldpNeighborSession) readInitialization() error {
	pdu, err := readLDPPDU(s.conn, time.Now().Add(ldpNeighborReadWait))
	if err != nil {
		return fmt.Errorf("read Ze's Initialization: %w", err)
	}
	if pdu.lsr != ldpNeighborZeLSR {
		return fmt.Errorf("the session's first PDU names LSR %s, want Ze's %s",
			netip.AddrFrom4(pdu.lsr), netip.AddrFrom4(ldpNeighborZeLSR))
	}
	first := pdu.messages[0]
	if first.msgType != ldpWireInitialization {
		return fmt.Errorf("the first message Ze sent is type %#04x, want Initialization %#04x", first.msgType, ldpWireInitialization)
	}
	s.zeInitID = first.id
	return checkLDPInitialization(first.body)
}

// establish plays the passive role's acceptance and waits for Ze's. RFC 5036
// Section 2.5.3: "If they are, LSR1 replies with an Initialization message of
// its own to propose the parameters it wishes to use and a KeepAlive message to
// signal acceptance of LSR2's parameters." Ze's KeepAlive is its acceptance of
// the neighbor's; the drain worker then keeps the session read.
func (s *ldpNeighborSession) establish() error {
	var initialization [ldpWirePDUHeaderOctets + ldpWireMessageHdrOctets + ldpWireTLVHeaderOctets + ldpWireCommonSessionLen]byte
	if err := s.write(initialization[:writeLDPInitialization(initialization[:])]); err != nil {
		return fmt.Errorf("send the neighbor's Initialization: %w", err)
	}
	if err := s.sendKeepAlive(); err != nil {
		return fmt.Errorf("send the neighbor's KeepAlive: %w", err)
	}
	if err := s.awaitKeepAlive(); err != nil {
		return err
	}
	s.drained = make(chan struct{})
	go s.drain()
	return nil
}

// awaitKeepAlive reads until Ze's KeepAlive arrives. A fatal Notification is
// Ze's refusal of the neighbor's parameters, and is reported with its code.
func (s *ldpNeighborSession) awaitKeepAlive() error {
	deadline := time.Now().Add(ldpNeighborReadWait)
	// Bounded by deadline: each pass reads one PDU, and the read fails past it.
	for {
		pdu, err := readLDPPDU(s.conn, deadline)
		if err != nil {
			return fmt.Errorf("read Ze's KeepAlive: %w", err)
		}
		for _, message := range pdu.messages {
			switch message.msgType {
			case ldpWireKeepAlive:
				return nil
			case ldpWireNotification:
				return fmt.Errorf("the neighbor's Initialization was answered with %w", ldpNotificationRefusal(message.body))
			}
		}
	}
}

// reject refuses Ze's Initialization and closes the connection. RFC 5036
// Section 2.5.3: "If the parameters are not acceptable, LSR1 responds by
// sending a Session Rejected/Parameters Error Notification message and closing
// the TCP connection."
func (s *ldpNeighborSession) reject() error {
	var notification [ldpWirePDUHeaderOctets + ldpWireMessageHdrOctets + ldpWireTLVHeaderOctets + 10]byte
	length := writeLDPNotification(notification[:], ldpWireStatusFatal|ldpWireRejectedMaxPDU, s.zeInitID, ldpWireInitialization)
	writeErr := s.write(notification[:length])
	if writeErr != nil {
		writeErr = fmt.Errorf("send the Session Rejected Notification: %w", writeErr)
	}
	return errors.Join(writeErr, s.close())
}

// drain is the session's worker: it reads every PDU Ze sends and answers each
// KeepAlive with one, until the connection closes.
func (s *ldpNeighborSession) drain() {
	defer close(s.drained)
	// Bounded by the connection: it ends when close closes it or Ze does.
	for {
		pdu, err := readLDPPDU(s.conn, time.Time{})
		if err != nil {
			s.drainErr = err
			return
		}
		for _, message := range pdu.messages {
			if message.msgType != ldpWireKeepAlive {
				continue
			}
			if err := s.sendKeepAlive(); err != nil {
				s.drainErr = err
				return
			}
		}
	}
}

// close closes the connection and waits for the drain worker. A drain that
// ended on anything but the close, or Ze ending the session, is reported.
func (s *ldpNeighborSession) close() error {
	s.closeOnce.Do(func() {
		if err := s.conn.Close(); err != nil {
			s.closeErr = fmt.Errorf("close the session: %w", err)
		}
		if s.drained == nil {
			return
		}
		<-s.drained
		if s.drainErr == nil {
			return
		}
		if errors.Is(s.drainErr, net.ErrClosed) {
			return
		}
		if errors.Is(s.drainErr, io.EOF) {
			return
		}
		s.closeErr = errors.Join(s.closeErr, fmt.Errorf("read the operational session: %w", s.drainErr))
	})
	return s.closeErr
}

func (s *ldpNeighborSession) sendKeepAlive() error {
	var keepalive [ldpWirePDUHeaderOctets + ldpWireMessageHdrOctets]byte
	return s.write(keepalive[:writeLDPKeepAlive(keepalive[:])])
}

func (s *ldpNeighborSession) write(pdu []byte) error {
	if err := s.conn.SetWriteDeadline(time.Now().Add(ldpNeighborReadWait)); err != nil {
		return err
	}
	_, err := s.conn.Write(pdu)
	return err
}

// ldpHelloer sends the neighbor's Link Hello every ldpNeighborHelloEvery while
// it is enabled. It owns one goroutine: startLDPHelloer starts it, and stop
// MUST be called to end it and close the socket. A Hello is sent only under mu,
// so once setEnabled(false) returns no Hello is in flight. Safe for concurrent
// use.
type ldpHelloer struct {
	conn   *net.UDPConn
	target *net.UDPAddr
	quit   chan struct{}
	done   chan struct{}

	mu      sync.Mutex
	enabled bool
	sent    int
	failure error // the first send that failed
}

func startLDPHelloer(conn *net.UDPConn) *ldpHelloer {
	h := &ldpHelloer{
		conn:   conn,
		target: net.UDPAddrFromAddrPort(netip.AddrPortFrom(ldpNeighborGroup, ldpWirePort)),
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go h.run()
	return h
}

func (h *ldpHelloer) run() {
	defer close(h.done)
	ticker := time.NewTicker(ldpNeighborHelloEvery)
	defer ticker.Stop()
	// Bounded by stop: it ends when quit closes.
	for {
		select {
		case <-h.quit:
			return
		case <-ticker.C:
			h.mu.Lock()
			if h.enabled {
				h.sendLocked() //nolint:errcheck // the failure is recorded for err()
			}
			h.mu.Unlock()
		}
	}
}

// setEnabled turns the cadence on or off. Turning it on sends one Hello at
// once, so Ze does not wait a full interval for the first one.
func (h *ldpHelloer) setEnabled(enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = enabled
	if enabled {
		h.sendLocked() //nolint:errcheck // the failure is recorded for err()
	}
}

// sendOne sends exactly one Hello whatever the cadence, and answers whether it
// left.
func (h *ldpHelloer) sendOne() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sendLocked()
}

// err answers the first Hello that failed to leave, so a scenario can tell a
// neighbor that went silent from a Ze that ignored it.
func (h *ldpHelloer) err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.failure
}

// sendLocked sends one Hello. The caller MUST hold mu.
func (h *ldpHelloer) sendLocked() error {
	var hello [ldpWirePDUHeaderOctets + ldpWireMessageHdrOctets + 2*(ldpWireTLVHeaderOctets+4)]byte
	h.sent++
	length := writeLDPHello(hello[:], uint32(h.sent))
	_, err := h.conn.WriteToUDP(hello[:length], h.target)
	if err != nil {
		err = fmt.Errorf("send Hello %d: %w", h.sent, err)
		if h.failure == nil {
			h.failure = err
		}
	}
	return err
}

// stop ends the worker, closes the socket and answers the first send failure.
func (h *ldpHelloer) stop() error {
	close(h.quit)
	<-h.done
	failure := h.err()
	if err := h.conn.Close(); err != nil {
		failure = errors.Join(failure, fmt.Errorf("close the Hello socket: %w", err))
	}
	return failure
}

// writeLDPPDUHeader writes the LDP PDU header for a body of bodyOctets after it.
// RFC 5036 Section 3.1: PDU Length is "the total length of this PDU in octets,
// excluding the Version and PDU Length fields."
//
//	0  Version (2)    2  PDU Length (2)
//	4  LSR-ID (4)     8  Label Space (2)    10 messages
func writeLDPPDUHeader(buf []byte, bodyOctets int) int {
	binary.BigEndian.PutUint16(buf[0:2], ldpWireVersion)
	binary.BigEndian.PutUint16(buf[2:4], uint16(6+bodyOctets))
	copy(buf[4:8], ldpNeighborPeerLSR[:])
	binary.BigEndian.PutUint16(buf[8:10], 0)
	return ldpWirePDUHeaderOctets
}

// writeLDPMessageHeader writes a message header at off. RFC 5036 Section 3.5:
// Message Length is the length "in octets, of Message ID, Mandatory Parameters,
// and Optional Parameters."
//
//	0  U|Message Type (2)    2  Message Length (2)    4  Message ID (4)
func writeLDPMessageHeader(buf []byte, off int, msgType uint16, parameterOctets int, id uint32) int {
	binary.BigEndian.PutUint16(buf[off:off+2], msgType)
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(4+parameterOctets))
	binary.BigEndian.PutUint32(buf[off+4:off+8], id)
	return off + ldpWireMessageHdrOctets
}

// writeLDPTLVHeader writes a TLV header at off with the U- and F-bits clear.
//
//	0  U|F|Type (2)    2  Length (2)    4  Value
func writeLDPTLVHeader(buf []byte, off int, tlvType uint16, valueOctets int) int {
	binary.BigEndian.PutUint16(buf[off:off+2], tlvType)
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(valueOctets))
	return off + ldpWireTLVHeaderOctets
}

// writeLDPHello writes the neighbor's Link Hello: a Common Hello Parameters TLV
// with the T- and R-bits clear (RFC 5036 Section 3.5.2, a Link Hello) and an
// IPv4 Transport Address TLV naming the neighbor's transport address.
//
//	10 Hello header (8)    18 Common Hello TLV: Hold Time (2), T|R|Reserved (2)
//	26 IPv4 Transport TLV: Address (4)    34 end
func writeLDPHello(buf []byte, id uint32) int {
	const parameters = 2 * (ldpWireTLVHeaderOctets + 4)
	off := writeLDPPDUHeader(buf, ldpWireMessageHdrOctets+parameters)
	off = writeLDPMessageHeader(buf, off, ldpWireHello, parameters, id)
	off = writeLDPTLVHeader(buf, off, ldpWireCommonHelloTLV, 4)
	binary.BigEndian.PutUint16(buf[off:off+2], ldpNeighborHelloHold)
	binary.BigEndian.PutUint16(buf[off+2:off+4], 0)
	off = writeLDPTLVHeader(buf, off+4, ldpWireIPv4TransportTLV, 4)
	copy(buf[off:off+4], ldpNeighborPeerLSR[:])
	return off + 4
}

// writeLDPInitialization writes the neighbor's Initialization: one Common
// Session Parameters TLV proposing protocol version 1, ldpNeighborKeepalive,
// Downstream Unsolicited with loop detection off, the default Max PDU Length,
// and Ze's LDP Identifier as the Receiver LDP Identifier.
//
//	18 Common Session TLV: Protocol Version (2), KeepAlive Time (2),
//	   A|D|Reserved (1), PVLim (1), Max PDU Length (2),
//	   Receiver LSR-ID (4), Receiver Label Space (2)    36 end
func writeLDPInitialization(buf []byte) int {
	const parameters = ldpWireTLVHeaderOctets + ldpWireCommonSessionLen
	off := writeLDPPDUHeader(buf, ldpWireMessageHdrOctets+parameters)
	off = writeLDPMessageHeader(buf, off, ldpWireInitialization, parameters, 1)
	off = writeLDPTLVHeader(buf, off, ldpWireCommonSessionTLV, ldpWireCommonSessionLen)
	binary.BigEndian.PutUint16(buf[off:off+2], ldpWireVersion)
	binary.BigEndian.PutUint16(buf[off+2:off+4], ldpNeighborKeepalive)
	buf[off+4] = 0
	buf[off+5] = 0
	// RFC 5036 Section 3.5.3: "A value of 255 or less specifies the default
	// maximum length of 4096 octets."
	binary.BigEndian.PutUint16(buf[off+6:off+8], 0)
	copy(buf[off+8:off+12], ldpNeighborZeLSR[:])
	binary.BigEndian.PutUint16(buf[off+12:off+14], 0)
	return off + ldpWireCommonSessionLen
}

// writeLDPKeepAlive writes a KeepAlive, which carries no parameter.
func writeLDPKeepAlive(buf []byte) int {
	off := writeLDPPDUHeader(buf, ldpWireMessageHdrOctets)
	return writeLDPMessageHeader(buf, off, ldpWireKeepAlive, 0, 1)
}

// writeLDPNotification writes a Notification carrying one Status TLV that names
// the message it answers (RFC 5036 Section 3.4.6).
//
//	18 Status TLV: Status Code (4), Message ID (4), Message Type (2)    32 end
func writeLDPNotification(buf []byte, status, answeredID uint32, answeredType uint16) int {
	const parameters = ldpWireTLVHeaderOctets + 10
	off := writeLDPPDUHeader(buf, ldpWireMessageHdrOctets+parameters)
	off = writeLDPMessageHeader(buf, off, ldpWireNotification, parameters, 1)
	off = writeLDPTLVHeader(buf, off, ldpWireStatusTLV, 10)
	binary.BigEndian.PutUint32(buf[off:off+4], status)
	binary.BigEndian.PutUint32(buf[off+4:off+8], answeredID)
	binary.BigEndian.PutUint16(buf[off+8:off+10], answeredType)
	return off + 10
}

// ldpReceivedMessage is one message of a PDU Ze sent: its type with the U-bit
// cleared, its Message ID, and the parameters after the Message ID.
type ldpReceivedMessage struct {
	msgType uint16
	id      uint32
	body    []byte
}

// ldpReceivedPDU is one PDU Ze sent. messages holds at least one message.
type ldpReceivedPDU struct {
	lsr        [4]byte
	labelSpace uint16
	messages   []ldpReceivedMessage
}

// readLDPPDU reads one whole PDU from conn before deadline (none when zero) and
// splits it into its messages. A PDU longer than the pre-negotiation maximum,
// or a message that overruns its PDU, is an error.
func readLDPPDU(conn net.Conn, deadline time.Time) (ldpReceivedPDU, error) {
	var pdu ldpReceivedPDU
	if err := conn.SetReadDeadline(deadline); err != nil {
		return pdu, err
	}
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return pdu, err
	}
	if version := binary.BigEndian.Uint16(header[0:2]); version != ldpWireVersion {
		return pdu, fmt.Errorf("PDU version %d, want %d", version, ldpWireVersion)
	}
	length := int(binary.BigEndian.Uint16(header[2:4]))
	if length < 6+ldpWireMessageHdrOctets {
		return pdu, fmt.Errorf("PDU Length %d holds no message", length)
	}
	if length > ldpWirePDULengthMax {
		return pdu, fmt.Errorf("PDU Length %d exceeds %d", length, ldpWirePDULengthMax)
	}
	rest := make([]byte, length)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return pdu, err
	}
	copy(pdu.lsr[:], rest[0:4])
	pdu.labelSpace = binary.BigEndian.Uint16(rest[4:6])
	body := rest[6:]
	// Bounded by the PDU Length checked above: each pass consumes one message.
	for len(body) > 0 {
		if len(body) < ldpWireMessageHdrOctets {
			return pdu, fmt.Errorf("a %d-octet tail is shorter than a message header", len(body))
		}
		messageLength := int(binary.BigEndian.Uint16(body[2:4]))
		if messageLength < 4 {
			return pdu, fmt.Errorf("message length %d is shorter than its Message ID", messageLength)
		}
		end := 4 + messageLength
		if end > len(body) {
			return pdu, fmt.Errorf("message length %d overruns the PDU", messageLength)
		}
		pdu.messages = append(pdu.messages, ldpReceivedMessage{
			msgType: binary.BigEndian.Uint16(body[0:2]) & ldpWireTypeMask,
			id:      binary.BigEndian.Uint32(body[4:8]),
			body:    body[8:end],
		})
		body = body[end:]
	}
	return pdu, nil
}

// findLDPTLV answers the value of the first TLV of type tlvType in parameters.
func findLDPTLV(parameters []byte, tlvType uint16) ([]byte, error) {
	// Bounded by len(parameters): each pass consumes one TLV.
	for len(parameters) >= ldpWireTLVHeaderOctets {
		found := binary.BigEndian.Uint16(parameters[0:2]) & 0x3fff
		length := int(binary.BigEndian.Uint16(parameters[2:4]))
		end := ldpWireTLVHeaderOctets + length
		if end > len(parameters) {
			return nil, fmt.Errorf("TLV %#04x of length %d overruns its message", found, length)
		}
		if found == tlvType {
			return parameters[ldpWireTLVHeaderOctets:end], nil
		}
		parameters = parameters[end:]
	}
	return nil, fmt.Errorf("no TLV %#04x", tlvType)
}

// checkLDPInitialization requires Ze's Common Session Parameters to propose
// protocol version 1 and to name the neighbor's LDP Identifier as the receiver:
// Ze opened this session for the neighbor's adjacency.
func checkLDPInitialization(parameters []byte) error {
	session, err := findLDPTLV(parameters, ldpWireCommonSessionTLV)
	if err != nil {
		return fmt.Errorf("the Initialization Ze sent: %w", err)
	}
	if len(session) < ldpWireCommonSessionLen {
		return fmt.Errorf("the Common Session Parameters Ze sent hold %d octets, want %d", len(session), ldpWireCommonSessionLen)
	}
	if version := binary.BigEndian.Uint16(session[0:2]); version != ldpWireVersion {
		return fmt.Errorf("the Initialization Ze sent proposes protocol version %d, want %d", version, ldpWireVersion)
	}
	receiver := [4]byte(session[8:12])
	if receiver != ldpNeighborPeerLSR {
		return fmt.Errorf("the Initialization Ze sent names receiver %s, want the neighbor %s",
			netip.AddrFrom4(receiver), netip.AddrFrom4(ldpNeighborPeerLSR))
	}
	if space := binary.BigEndian.Uint16(session[12:14]); space != 0 {
		return fmt.Errorf("the Initialization Ze sent names receiver label space %d, want 0", space)
	}
	return nil
}

// ldpNotificationRefusal describes a Notification Ze sent: its Status Code, or
// why no Status Code could be read from it.
func ldpNotificationRefusal(parameters []byte) error {
	status, err := findLDPTLV(parameters, ldpWireStatusTLV)
	if err != nil {
		return fmt.Errorf("a Notification with no readable status: %w", err)
	}
	if len(status) < 4 {
		return fmt.Errorf("a Notification whose Status TLV holds %d octets", len(status))
	}
	return fmt.Errorf("a Notification, status %#08x", binary.BigEndian.Uint32(status[0:4]))
}
