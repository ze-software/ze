// Design: docs/architecture/l2tp/subscriber-session-model.md -- terminal
// accounting leaves other tunnels responsive: a silent accounting server
// MUST NOT stop the control packets of every other tunnel.
// Related: register_l2tp_slow_accounting.go -- registers this peer and its RADIUS server.
// Related: tunnel_fixture_l2tp_session_clear.go -- startSessionClearLink,
// awaitAccountingStart and the `ze cli` readers this peer reuses.
// Related: tunnel_fixture_l2tp_session.go -- the CDN body shape the LAC's
// hang-up follows.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelL2TPOpenProxySession and
// tunnelAccountingPeer, the LAC half of the session this peer ends.
// Related: test/draft/l2tp/slow-accounting-control.ci -- the only caller.

package fixture

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RFC 2661 challenge response uses the existing CHAP-MD5 wire contract
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"
)

// The two receipts the peer and the RADIUS server hand each other through
// the test's working directory. The HELLO receipt says the healthy tunnel's
// HELLO was acknowledged while the Stop was held; the Stop receipt says the
// server answered the Stop's retry.
const (
	slowAccountingHelloReceipt = "slow-accounting-hello.receipt"
	slowAccountingStopReceipt  = "slow-accounting-stop.receipt"
)

// slowAccountingPeerTunnel is the Assigned Tunnel ID of the healthy LAC. The
// session's LAC (tunnelL2TPOpenProxySession) uses 0x0321, so ze's ZLB names
// which of the two tunnels it acknowledges.
const slowAccountingPeerTunnel = uint16(0x0322)

// slowAccountingLACSession is the Assigned Session ID the ICRQ of
// tunnelL2TPOpenProxySession gives the session on the LAC's side. The LAC's
// CDN names its own session by it (RFC 2661 Section 6.12).
const slowAccountingLACSession = uint16(700)

// slowAccountingCDNAdministrative is the CDN Result Code RFC 2661 Section
// 4.4.2 lists as "3 - Call disconnected for administrative reasons".
// terminateCauseForCDN (internal/component/l2tp/session_fsm.go) maps it to
// Acct-Terminate-Cause 6, Admin Reset, which the server checks on the Stop.
const slowAccountingCDNAdministrative = uint16(3)

// This is the healthy LAC's first control retransmission deadline. A success
// requires the matching wire ACK before that protocol event, never a sleep or
// an elapsed-time success. RADIUS retains the configured five-second timeout
// and two attempts; this fixture changes no daemon cleanup deadline.
const slowAccountingControlRetry = time.Second

type slowAccountingRequest struct {
	immutable     []byte
	delay         uint32
	identifier    byte
	authenticator [16]byte
}

// Client.exchange sets the first Accounting-Request delay to zero. Its retry
// changes only delay, Identifier and Request Authenticator. Keep every other
// attribute octet, including order, so a new Stop operation cannot count merely
// because it reused an accounting ID. Verify the actual request authenticator,
// not just the mock's human-readable projection of selected attributes.
func slowAccountingRequestWire(packet []byte) (slowAccountingRequest, error) {
	var request slowAccountingRequest
	if len(packet) < 20 {
		return request, errors.New("short original Stop datagram")
	}
	if packet[0] != 4 {
		return request, errors.New("original Stop is not an Accounting-Request")
	}
	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return request, errors.New("original Stop length does not describe its full datagram")
	}
	request.identifier = packet[1]
	copy(request.authenticator[:], packet[4:20])
	unsigned := append([]byte(nil), packet...)
	clear(unsigned[4:20])
	want := md5.Sum(append(unsigned, []byte("testing123")...)) //nolint:gosec // RFC 2866 Section 3 Accounting-Request authenticator
	if request.authenticator != want {
		return request, errors.New("original Stop Request Authenticator does not verify")
	}
	request.immutable = append([]byte(nil), packet[20:]...)
	delays := 0
	for offset := 20; offset < len(packet); {
		if offset+2 > len(packet) {
			return request, errors.New("original Stop has a truncated attribute header")
		}
		length := int(packet[offset+1])
		if length < 2 {
			return request, errors.New("original Stop has an attribute shorter than its header")
		}
		if offset+length > len(packet) {
			return request, errors.New("original Stop has an attribute past its datagram")
		}
		if packet[offset] == 41 {
			delays++
			if length != 6 {
				return request, errors.New("the original Stop's Acct-Delay-Time is not a four-octet measurement")
			}
			request.delay = binary.BigEndian.Uint32(packet[offset+2 : offset+length])
			clear(request.immutable[offset-20+2 : offset-20+length])
		}
		offset += length
	}
	if delays != 1 {
		return request, errors.New("original Stop does not carry exactly one Acct-Delay-Time")
	}
	return request, nil
}

// slowAccountingStop reads one Accounting-Request's RADIUS-RX line. stop is
// false for a Start or an Interim-Update, which the server answers at once.
// A Stop answers the Acct-Session-Id it ends, and an error unless it reports
// Acct-Terminate-Cause 6 (Admin Reset), the cause of the LAC's CDN.
func slowAccountingStop(line string) (owner string, stop bool, err error) {
	status, ok := tunnelRadiusValue(line, "Acct-Status-Type")
	if !ok {
		return "", false, errors.New("an Accounting-Request has no decoded status")
	}
	if status != accountingStatusStop {
		return "", false, nil
	}
	owner, ok = tunnelRadiusValue(line, "Acct-Session-Id")
	if !ok {
		return "", true, errors.New("a Stop has no original accounting identity")
	}
	cause, ok := tunnelRadiusValue(line, "Acct-Terminate-Cause")
	if !ok {
		return "", true, errors.New("the Stop carries no Acct-Terminate-Cause")
	}
	if cause != tunnelTerminateCauseAdminReset {
		return "", true, fmt.Errorf("the Stop reports Acct-Terminate-Cause %s, not %s (Admin Reset) for the LAC's administrative CDN", cause, tunnelTerminateCauseAdminReset)
	}
	return owner, true, nil
}

// slowAccountingRetry answers nil only when retry is the client's delayed
// retransmission of first: every attribute octet but Acct-Delay-Time equal, a
// larger delay, and a new Identifier and Request Authenticator. Both requests
// MUST come from slowAccountingRequestWire, which verified each signature.
func slowAccountingRetry(first, retry slowAccountingRequest) error {
	if !bytes.Equal(retry.immutable, first.immutable) {
		return errors.New("the Stop retry changed the original immutable accounting attributes")
	}
	if retry.delay <= first.delay {
		return errors.New("the Stop candidate is not a delayed retry of the original exchange")
	}
	if retry.identifier == first.identifier {
		return errors.New("the delayed Stop retry reused the first Identifier")
	}
	if retry.authenticator == first.authenticator {
		return errors.New("the delayed Stop retry reused the first Request Authenticator")
	}
	return nil
}

func slowAccountingMarkerAbsent(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected existing proof receipt %s", path)
}

// Link a fully written private receipt into its final name. The other process
// never observes the empty/truncated window of an in-place file write.
func slowAccountingPublish(path, owner string) error {
	contents, err := os.ReadFile(path)
	if err == nil {
		if string(contents) != owner {
			return errors.New("proof receipt already belongs to another accounting owner")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	staging := path + ".pending"
	file, err := os.OpenFile(staging, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(owner)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(staging))
	}
	linkErr := os.Link(staging, path)
	return errors.Join(linkErr, os.Remove(staging))
}

// tunnelSlowAccountingRadius is the RADIUS server of the test. It answers
// every Access-Request and Accounting-Start with the existing authenticated
// wire producer, withholds the session's first Accounting-Stop, and answers
// only a genuine retry of it, and only once the foreground peer proved the
// healthy tunnel's HELLO ACK.
//
// Arguments: <port>. The shared secret is the .ci's testing123.
func tunnelSlowAccountingRadius(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // runner joins this fixture and owns its isolated working directory
	log, err := os.OpenFile(tunnelRadiusRXLog, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close() //nolint:errcheck // every write is checked before a response is emitted
	buffer := make([]byte, 4096)
	stopOwner := ""
	var firstStop *slowAccountingRequest
	// The context owns this server; socket deadlines only let cancellation join it.
	for ctx.Err() == nil {
		if err := conn.SetReadDeadline(time.Now().Add(sessionClearDelay)); err != nil {
			return err
		}
		n, address, err := conn.ReadFromUDP(buffer)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() {
					continue
				}
			}
			return err
		}
		if n < 20 {
			return errors.New("short RADIUS request in slow-accounting fixture")
		}
		packet := buffer[:n]
		line := tunnelRadiusDescribe(packet)
		if _, err := fmt.Fprintln(log, line); err != nil {
			return err
		}
		responseCode := byte(2)
		answeredStop := false
		if packet[0] == 4 {
			responseCode = 5
			owner, stop, err := slowAccountingStop(line)
			if err != nil {
				return err
			}
			if stop {
				answeredStop = true
				if stopOwner == "" {
					stopOwner = owner
				}
				if stopOwner != owner {
					return errors.New("another accounting owner received a Stop")
				}
				request, err := slowAccountingRequestWire(packet)
				if err != nil {
					return err
				}
				if firstStop == nil {
					if request.delay != 0 {
						return errors.New("the first original Stop did not begin at client exchange delay zero")
					}
					firstStop = &request
					// Always withhold this first request even if the foreground
					// publishes its HELLO receipt while this process is descheduled.
					continue
				}
				if err := slowAccountingRetry(*firstStop, request); err != nil {
					return err
				}
				receipt, err := os.ReadFile(slowAccountingHelloReceipt)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return err
				}
				if string(receipt) != owner {
					return errors.New("the HELLO receipt does not name this original Stop")
				}
			}
		} else if packet[0] != 1 {
			return errors.New("unexpected RADIUS request code")
		}
		response := tunnelRadiusAnswer(responseCode, packet, nil, []byte("testing123"))
		if _, err := conn.WriteToUDP(response, address); err != nil {
			return err
		}
		if answeredStop {
			if err := slowAccountingPublish(slowAccountingStopReceipt, stopOwner); err != nil {
				return err
			}
		}
	}
	return nil
}

// openSlowAccountingControl opens the healthy tunnel: SCCRQ, SCCRP, SCCCN,
// then one HELLO whose ACK proves the tunnel is established. The caller owns
// the returned socket and MUST close it; an error closes it here.
func openSlowAccountingControl(ctx context.Context, port int) (*tunnelAccountingPeer, error) {
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return nil, err
	}
	transferred := false
	defer func() {
		if !transferred {
			_ = conn.Close() //nolint:errcheck // failed construction retains socket cleanup
		}
	}()
	challenge := []byte("slow-control-lac")
	reply, _, err := tunnelL2TPExchange(ctx, conn, target, tunnelL2TPSCCRQ(slowAccountingPeerTunnel, "healthy-control-lac", challenge), 40, sessionClearDelay)
	if err != nil {
		return nil, err
	}
	avps, err := tunnelL2TPParseAVPs(reply)
	if err != nil {
		return nil, err
	}
	if tunnelL2TPMessageType(avps) != 2 {
		return nil, errors.New("the healthy tunnel's answer to its SCCRQ is not an SCCRP")
	}
	if len(avps[9]) != 2 {
		return nil, errors.New("the healthy tunnel's SCCRP has no Assigned Tunnel ID")
	}
	if len(avps[11]) == 0 {
		return nil, errors.New("the healthy tunnel's SCCRP has no Challenge")
	}
	peer := &tunnelAccountingPeer{conn: conn, target: target, localTID: binary.BigEndian.Uint16(avps[9]), ns: 1, nr: 1}
	digest := md5.Sum(append(append([]byte{3}, tunnelL2TPTestSecret()...), avps[11]...)) //nolint:gosec // RFC 2661 Section 4.4.3 CHAP value
	peer.sendControl(append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(3)), tunnelL2TPAVP(true, 13, digest[:])...), 0)
	if err := slowAccountingHello(ctx, peer); err != nil {
		return nil, err
	}
	transferred = true
	return peer, nil
}

// slowAccountingHello sends one HELLO on the healthy tunnel and waits, until
// the LAC's first retransmission is due, for ze's ZLB acknowledging it: tunnel
// slowAccountingPeerTunnel, session 0, Nr one past the HELLO's Ns.
func slowAccountingHello(ctx context.Context, peer *tunnelAccountingPeer) error {
	wantNR := peer.ns + 1
	wire := tunnelL2TPControl(peer.localTID, 0, peer.ns, peer.nr, tunnelL2TPAVP(true, 0, tunnelL2TPU16(6)))
	if _, err := peer.conn.WriteToUDP(wire, peer.target); err != nil {
		return err
	}
	peer.ns++
	deadline := time.Now().Add(slowAccountingControlRetry)
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	if err := peer.conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	buffer := make([]byte, 2048)
	// A single absolute protocol deadline bounds stale SCCCN ACKs; it is never renewed.
	for ctx.Err() == nil {
		n, address, err := peer.conn.ReadFromUDP(buffer)
		if err != nil {
			return fmt.Errorf("healthy HELLO lacked its ACK before the LAC retry: %w", err)
		}
		if !address.IP.Equal(peer.target.IP) {
			return errors.New("the healthy ACK came from another address")
		}
		if address.Port != peer.target.Port {
			return errors.New("the healthy ACK came from another port")
		}
		acked, err := slowAccountingHelloAck(buffer[:n], wantNR)
		if err != nil {
			return err
		}
		if acked {
			return nil
		}
	}
	return ctx.Err()
}

// slowAccountingHelloAck judges one datagram ze sent the healthy tunnel. It
// MUST be a control ZLB for tunnel slowAccountingPeerTunnel and session 0.
// acked is true when its Nr is wantNR; a ZLB with another Nr acknowledges an
// earlier message, such as the SCCCN, so the caller keeps reading.
func slowAccountingHelloAck(datagram []byte, wantNR uint16) (acked bool, err error) {
	if len(datagram) != 12 {
		return false, fmt.Errorf("the healthy tunnel received a %d-octet response, not a ZLB", len(datagram))
	}
	if binary.BigEndian.Uint16(datagram[:2]) != 0xc802 {
		return false, errors.New("the healthy tunnel received a response that is not a control ZLB")
	}
	if binary.BigEndian.Uint16(datagram[4:6]) != slowAccountingPeerTunnel {
		return false, errors.New("the healthy ACK named another tunnel")
	}
	if binary.BigEndian.Uint16(datagram[6:8]) != 0 {
		return false, errors.New("the healthy ACK named a session")
	}
	return binary.BigEndian.Uint16(datagram[10:12]) == wantNR, nil
}

// slowAccountingHangUp sends the LAC's Call-Disconnect-Notify for the
// session. A CDN from the peer is handled on ze's L2TP reactor goroutine,
// which publishes the session's Down there (reactor.go, the peer hang-up path
// into deliverSessionDowns), so a Down reader that waited for the RADIUS
// server would hold every tunnel's control packets. An operator clear would
// publish on the CLI handler's goroutine instead, and could not show it.
// The caller MUST own the peer's socket: the link goroutine is stopped.
func slowAccountingHangUp(peer *tunnelAccountingPeer) error {
	// RFC 2661 Section 6.12: "The following AVPs MUST be present in the
	// CDN:" Message Type, Result Code and Assigned Session ID.
	result := append(tunnelL2TPU16(slowAccountingCDNAdministrative), tunnelL2TPU16(0)...)
	cdn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(tunnelL2TPMessageTypeCDN)), tunnelL2TPAVP(true, 1, result)...)
	cdn = append(cdn, tunnelL2TPAVP(true, tunnelL2TPAVPAssignedSessionID, tunnelL2TPU16(slowAccountingLACSession))...)
	if _, err := peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, peer.zeSID, peer.ns, peer.nr, cdn), peer.target); err != nil {
		return err
	}
	peer.ns++
	fmt.Println("OK: the LAC sent a CDN with Result Code 3 for session " + sessionIDText(peer.zeSID))
	return nil
}

// slowAccountingAwait polls probe every sessionClearDelay within its share of
// the test's budget, as sessionClearLink.await does for a running link. probe
// answers nil once the state holds, and otherwise what it saw instead.
func slowAccountingAwait(ctx context.Context, what string, probe func() error) error {
	last := errors.New("no attempt finished")
	attempts := WaitAttempts(sessionClearWaitShare, sessionClearDelay, 120)
	held := Poll(ctx, attempts, sessionClearDelay, func() bool {
		if err := probe(); err != nil {
			last = err
			return false
		}
		return true
	})
	if !held {
		return fmt.Errorf("%s never came; the last attempt saw: %w", what, last)
	}
	return nil
}

// slowAccountingAwaitGone waits until neither `show l2tp sessions` nor `show
// subscriber` holds the hung-up session. The same commands listed it before
// the CDN (awaitListedSession, awaitActiveSubscriber), so an empty answer is
// not a broken command.
func slowAccountingAwaitGone(ctx context.Context) error {
	err := slowAccountingAwait(ctx, "no session in `"+sessionClearShowSessions+"`", func() error {
		answer, err := sessionClearCLI(ctx, sessionClearShowSessions)
		if err != nil {
			return err
		}
		if rows := jsonObjectsWith(answer, "local-sid"); len(rows) != 0 {
			return errors.New("an answer still listing a session: " + describeJSON(answer))
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("OK: show l2tp sessions lists no session after the LAC's CDN")
	err = slowAccountingAwait(ctx, "no subscriber in `"+sessionClearShowSubscribers+"`", func() error {
		answer, err := sessionClearCLI(ctx, sessionClearShowSubscribers)
		if err != nil {
			return err
		}
		if rows := jsonObjectsWith(answer, "access-type"); len(rows) != 0 {
			return errors.New("an answer still listing a subscriber: " + describeJSON(answer))
		}
		summaries := jsonObjectsWith(answer, "total")
		if len(summaries) != 1 {
			return fmt.Errorf("%d summaries holding a total, want 1: %s", len(summaries), describeJSON(answer))
		}
		if summaries[0]["total"] != float64(0) {
			return errors.New("a summary whose total is not 0: " + describeJSON(answer))
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("OK: show subscriber lists no subscriber after the LAC's CDN, total 0")
	return nil
}

// tunnelL2TPSlowAccountingControl is the LAC side of
// test/draft/l2tp/slow-accounting-control.ci. Goal: a RADIUS server that
// does not answer one session's Accounting-Stop holds no other tunnel. Method:
// bring one PPP session up to its address, its Accounting-Start and its rows
// in the CLI; open a second, healthy tunnel; hang the session up with the
// LAC's own CDN, which ze handles on its reactor goroutine. Once the server
// has received, and withheld, the Stop, the healthy tunnel's HELLO MUST be
// acknowledged before the LAC's first retransmission. The server then answers
// only a real delayed retry of that Stop, and the healthy tunnel MUST still
// acknowledge a HELLO, and the session MUST be gone from both CLI views.
//
// Arguments: <l2tp-port>. The CLI takes the daemon's SSH address and
// credentials from the environment the runner gave this process.
func tunnelL2TPSlowAccountingControl(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	for _, path := range []string{slowAccountingHelloReceipt, slowAccountingStopReceipt} {
		if err := slowAccountingMarkerAbsent(path); err != nil {
			return err
		}
	}
	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown
	if err := peer.negotiateIPCP(ctx); err != nil {
		return err
	}
	address, ok := netip.AddrFromSlice(peer.negotiatedAddr.To4())
	if !ok {
		return fmt.Errorf("IPCP settled on %s, which is not an IPv4 address", peer.negotiatedAddr)
	}

	// stop is idempotent: the deferred call covers every early return, and
	// the explicit call below hands the socket back before the hang-up.
	link := startSessionClearLink(ctx, peer)
	defer link.stop()
	acctSessionID, err := link.awaitAccountingStart(ctx)
	if err != nil {
		return err
	}
	if err := link.awaitListedSession(ctx, peer.zeSID, address); err != nil {
		return err
	}
	if err := link.awaitActiveSubscriber(ctx, address); err != nil {
		return err
	}

	healthy, err := openSlowAccountingControl(ctx, port)
	if err != nil {
		return err
	}
	defer healthy.conn.Close() //nolint:errcheck // fixture owns the second wire peer

	link.stop()
	if err := link.failure(); err != nil {
		return err
	}
	if err := slowAccountingHangUp(peer); err != nil {
		return err
	}

	err = slowAccountingAwait(ctx, "the original unanswered Accounting-Stop", func() error {
		records, err := readAccountingRecords()
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.status == accountingStatusStop && record.sessionID == acctSessionID {
				return slowAccountingMarkerAbsent(slowAccountingStopReceipt)
			}
		}
		return errors.New("the real server has not received this admission's Stop")
	})
	if err != nil {
		return err
	}
	if err := slowAccountingHello(ctx, healthy); err != nil {
		return err
	}
	if err := slowAccountingMarkerAbsent(slowAccountingStopReceipt); err != nil {
		return err
	}
	if err := slowAccountingPublish(slowAccountingHelloReceipt, acctSessionID); err != nil {
		return err
	}
	fmt.Println("OK: healthy tunnel acknowledged HELLO while the original Accounting-Stop was unanswered")

	err = slowAccountingAwait(ctx, "the original Stop retry and server wire response", func() error {
		receipt, err := os.ReadFile(slowAccountingStopReceipt)
		if err != nil {
			return err
		}
		if string(receipt) != acctSessionID {
			return errors.New("server wire-response receipt lost the original accounting identity")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := slowAccountingHello(ctx, healthy); err != nil {
		return err
	}
	// The mock's successful UDP write is a server wire-response receipt, not
	// proof the daemon consumed it or joined its accounting owner. The real
	// accounting completion/retirement units remain an independent obligation.
	fmt.Println("OK: the original delayed Stop retry received a server wire response and the healthy tunnel still acknowledged HELLO")
	return slowAccountingAwaitGone(ctx)
}
