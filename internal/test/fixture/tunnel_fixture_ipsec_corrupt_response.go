// Design: docs/architecture/testing/ci-format.md -- compiled fixtures a .ci drives through `le test fixture`.
// Related: register_ipsec_auth_corrupt_response.go -- the registration
// Related: test/ipsec/ike-auth-corrupt-response.ci -- the run that starts this relay
// Related: test/ipsec/ike-auth-unauthenticated-doctor.ci -- the run in withhold mode
//
// The relay of test/ipsec/ike-auth-corrupt-response.ci. Two Ze daemons each name the
// relay's address as their remote-address, so every IKE datagram between them crosses
// it. Before it forwards each first IKE_AUTH response to the initiator, the relay sends
// three copies with the last octet changed. In withhold mode it sends one damaged copy
// of every IKE_AUTH response, retransmissions included, and never forwards a genuine
// one, so the initiator stays in its handshake dropping forgeries for as long as the
// test reads its state.
//
// The last octets of an SK-protected message
// are its Integrity Checksum Data (RFC 7296 Section 3.14), so each copy fails the
// integrity check while its clear header still carries the live SPIs and Message ID:
// the forgery RFC 7296 Section 2.4 says an endpoint must survive.
//
// Why both daemons float to the NAT-T port through it. Ze computes the NAT_DETECTION
// hashes over the CONFIGURED addresses (cycleLocalAddress and cycleRemoteAddress,
// internal/component/ike/engine/cycle_addr.go). The initiator hashes 127.0.0.1 and the
// relay, the responder hashes 127.0.0.2 and the relay, so each side's hash of the
// other differs from what it expects. detectResponderNAT (responder.go) and
// handleSAInitResponse (fsm.go) then call floatToNATTPort, and IKE_AUTH leaves from
// nattAddr, the port after the test's IKE port. The relay listens on both.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// The three loopback addresses. The initiator and the responder bind theirs through
// `local-address` under the ze.test.ike.port override (ikeListenHost, testport.go).
var (
	corruptRelayAddr     = netip.MustParseAddr("127.0.0.3")
	corruptInitiatorAddr = netip.MustParseAddr(addrLoopback)
	corruptResponderAddr = netip.MustParseAddr("127.0.0.2")
)

const (
	// corruptCopies is how many damaged copies precede each first IKE_AUTH response.
	corruptCopies = 3
	// corruptPassthrough is the argument of the control run: the relay forwards every
	// datagram unchanged, which proves the relay alone lets the tunnel establish.
	corruptPassthrough = "passthrough"
	// corruptWithhold is the argument of the doctor run: every IKE_AUTH response is
	// replaced by one damaged copy, so the handshake never completes.
	corruptWithhold = "withhold"
	// corruptLifetimeMax bounds the relay when nothing cancels it. The runner's
	// background timeout and its teardown end it first in every .ci run.
	corruptLifetimeMax = 5 * time.Minute
	// corruptReadSlice is how long one read waits before the loop rechecks the context.
	corruptReadSlice = 200 * time.Millisecond
	// corruptResponsesMax bounds the record of corrupted responses. One handshake
	// cycle adds one entry, and the reconnect backoff keeps a run far below it.
	corruptResponsesMax = 256
	// corruptDatagramMax is the largest UDP payload the relay reads.
	corruptDatagramMax = 65535
)

// The RFC 7296 Section 3.1 IKE header, in the clear on both ports:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       IKE SA Initiator's SPI                  |  0-7
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       IKE SA Responder's SPI                  |  8-15
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Next Payload | MjVer | MnVer | Exchange Type |     Flags     |  16-19
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Message ID                           |  20-23
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                            Length                             |  24-27
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// On the NAT-T port the header follows the four zero octets of the non-ESP marker
// (RFC 3948 Section 2.2).
const (
	ikeHeaderOctets      = 28
	ikeExchangeOffset    = 18
	ikeFlagsOffset       = 19
	ikeMessageIDOffset   = 20
	ikeExchangeIKEAuth   = 35
	ikeFlagResponse      = 0x20
	nonESPMarkerOctets   = 4
	corruptResponseKeyLn = 12
)

// corruptMode is what the relay does to an IKE_AUTH response bound for the initiator.
// Zero is no mode: corruptRelayArgs always answers one of the three named modes.
type corruptMode uint8

const (
	corruptModeUnspecified corruptMode = iota
	// corruptModeCopies sends corruptCopies damaged copies ahead of each first
	// response, then the genuine one.
	corruptModeCopies
	// corruptModePassthrough forwards every response unchanged (the control run).
	corruptModePassthrough
	// corruptModeWithhold sends one damaged copy in place of every response and
	// forwards no genuine one.
	corruptModeWithhold
)

// corruptResponseKey names one IKE_AUTH response: the initiator's SPI and the Message
// ID. A retransmitted response carries the same key and is forwarded unchanged.
type corruptResponseKey [corruptResponseKeyLn]byte

// corruptRelay is the state the two port workers share. Safe for concurrent use:
// every field is read and written under mu.
type corruptRelay struct {
	mode corruptMode

	mu        sync.Mutex
	responses map[corruptResponseKey]bool
	forwarded int
	withheld  int
	stray     int
	failure   error
}

// tunnelIPsecAuthCorruptResponse runs the relay until the context is canceled (the
// runner's SIGTERM at a stop or at teardown) or corruptLifetimeMax passes.
//
// Arguments: the test's IKE port ($PORT2), then optionally `passthrough` or `withhold`.
func tunnelIPsecAuthCorruptResponse(ctx context.Context, args []string) error {
	ikePort, mode, err := corruptRelayArgs(args)
	if err != nil {
		return err
	}
	relay := &corruptRelay{mode: mode, responses: make(map[corruptResponseKey]bool)}

	ikeConn, err := corruptRelayListen(ctx, ikePort)
	if err != nil {
		return err
	}
	defer ikeConn.Close() //nolint:errcheck // the relay is done with the socket; a close error changes no verdict
	nattConn, err := corruptRelayListen(ctx, ikePort+1)
	if err != nil {
		return err
	}
	defer nattConn.Close() //nolint:errcheck // the relay is done with the socket; a close error changes no verdict

	// The ready= text of the .ci: printed only once both sockets are bound.
	fmt.Println("RELAY: listening on", ikeConn.LocalAddr(), "and", nattConn.LocalAddr(), "passthrough", mode == corruptModePassthrough, "withhold", mode == corruptModeWithhold)

	deadline := time.Now().Add(corruptLifetimeMax)
	var workers sync.WaitGroup
	workers.Go(func() { relay.serve(ctx, deadline, ikeConn, ikePort, false) })
	workers.Go(func() { relay.serve(ctx, deadline, nattConn, ikePort+1, true) })
	workers.Wait()

	return relay.verdict()
}

// corruptRelayArgs reads the IKE port and the optional mode word.
func corruptRelayArgs(args []string) (uint16, corruptMode, error) {
	if len(args) == 0 {
		return 0, corruptModeUnspecified, errors.New("ipsec/auth-corrupt-response needs the test's IKE port: le test fixture ipsec/auth-corrupt-response $PORT2 [passthrough|withhold]")
	}
	if len(args) > 2 {
		return 0, corruptModeUnspecified, fmt.Errorf("ipsec/auth-corrupt-response takes the IKE port and an optional %q or %q, got %q", corruptPassthrough, corruptWithhold, args)
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return 0, corruptModeUnspecified, fmt.Errorf("ipsec/auth-corrupt-response: IKE port %q: %w", args[0], err)
	}
	// The NAT-T socket takes the next port, so 65535 has no partner (nattAddr, testport.go).
	if port == 0 {
		return 0, corruptModeUnspecified, fmt.Errorf("ipsec/auth-corrupt-response: IKE port %q must be above 0", args[0])
	}
	if port >= 65535 {
		return 0, corruptModeUnspecified, fmt.Errorf("ipsec/auth-corrupt-response: IKE port %q must be below 65535, the NAT-T port is the one after it", args[0])
	}
	if len(args) == 1 {
		return uint16(port), corruptModeCopies, nil
	}
	if args[1] == corruptPassthrough {
		return uint16(port), corruptModePassthrough, nil
	}
	if args[1] == corruptWithhold {
		return uint16(port), corruptModeWithhold, nil
	}
	return 0, corruptModeUnspecified, fmt.Errorf("ipsec/auth-corrupt-response: unknown argument %q, the only ones are %q and %q", args[1], corruptPassthrough, corruptWithhold)
}

// corruptRelayListen binds the relay address on one port.
func corruptRelayListen(ctx context.Context, port uint16) (*net.UDPConn, error) {
	address := netip.AddrPortFrom(corruptRelayAddr, port).String()
	var config net.ListenConfig
	packetConn, err := config.ListenPacket(ctx, "udp4", address)
	if err != nil {
		return nil, fmt.Errorf("relay bind %s: %w", address, err)
	}
	conn, ok := packetConn.(*net.UDPConn)
	if !ok {
		packetConn.Close() //nolint:errcheck // the socket is unusable; the error below is the report
		return nil, fmt.Errorf("relay bind %s: the socket is a %T, not UDP", address, packetConn)
	}
	return conn, nil
}

// serve relays one port until the context ends or the deadline passes. natt says the
// port frames IKE behind the non-ESP marker. Each worker owns its two buffers.
func (relay *corruptRelay) serve(ctx context.Context, deadline time.Time, conn *net.UDPConn, port uint16, natt bool) {
	datagram := make([]byte, corruptDatagramMax)
	damaged := make([]byte, corruptDatagramMax)
	// Bounded by the context and by deadline: each pass waits at most corruptReadSlice.
	for ctx.Err() == nil && time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(corruptReadSlice)); err != nil {
			relay.fail(fmt.Errorf("relay port %d: set read deadline: %w", port, err))
			return
		}
		octets, source, err := conn.ReadFromUDPAddrPort(datagram)
		if err != nil {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				continue
			}
			relay.fail(fmt.Errorf("relay port %d: read: %w", port, err))
			return
		}
		target, toInitiator, known := corruptRelayTarget(source.Addr())
		if !known {
			relay.countStray()
			continue
		}
		destination := netip.AddrPortFrom(target, port)
		message := datagram[:octets]
		if toInitiator {
			forward, err := relay.corruptAuthResponse(conn, destination, message, damaged, natt)
			if err != nil {
				relay.fail(err)
				return
			}
			if !forward {
				continue
			}
		}
		if _, err := conn.WriteToUDPAddrPort(message, destination); err != nil {
			relay.fail(fmt.Errorf("relay port %d: forward to %s: %w", port, destination, err))
			return
		}
		relay.countForwarded()
	}
}

// corruptRelayTarget answers where a datagram from source goes, and whether that is the
// initiator. A source that is neither daemon is not relayed.
func corruptRelayTarget(source netip.Addr) (netip.Addr, bool, bool) {
	if source == corruptInitiatorAddr {
		return corruptResponderAddr, false, true
	}
	if source == corruptResponderAddr {
		return corruptInitiatorAddr, true, true
	}
	return netip.Addr{}, false, false
}

// corruptAuthResponse acts on a datagram bound for the initiator as the relay's mode
// says, and answers whether the caller forwards the genuine message. Only an IKE_AUTH
// response is acted on; every other datagram is forwarded.
func (relay *corruptRelay) corruptAuthResponse(conn *net.UDPConn, destination netip.AddrPort, message, damaged []byte, natt bool) (bool, error) {
	key, isAuthResponse := ikeAuthResponseKey(message, natt)
	if !isAuthResponse {
		return true, nil
	}
	first, full := relay.recordResponse(key)
	if full {
		return false, fmt.Errorf("relay recorded %d IKE_AUTH responses, its bound: the initiator restarted its handshake that many times", corruptResponsesMax)
	}
	messageID := uint32(key[8])<<24 | uint32(key[9])<<16 | uint32(key[10])<<8 | uint32(key[11])
	switch relay.mode {
	case corruptModePassthrough:
		if first {
			fmt.Println("RELAY-PASSTHROUGH: forwarded the IKE_AUTH response unchanged, message ID", messageID)
		}
		return true, nil
	case corruptModeCopies:
		if !first {
			return true, nil
		}
		copyOf := damageLastOctet(message, damaged)
		for range corruptCopies {
			if _, err := conn.WriteToUDPAddrPort(copyOf, destination); err != nil {
				return false, fmt.Errorf("relay send a damaged copy to %s: %w", destination, err)
			}
		}
		fmt.Println("RELAY-CORRUPTED: sent", corruptCopies, "copies of the IKE_AUTH response with the last octet changed, message ID", messageID, "natt", natt)
		return true, nil
	case corruptModeWithhold:
		// Every response, a retransmitted one included, is replaced: a genuine response
		// that slipped through would establish the SA and clear the count under test.
		copyOf := damageLastOctet(message, damaged)
		if _, err := conn.WriteToUDPAddrPort(copyOf, destination); err != nil {
			return false, fmt.Errorf("relay send a damaged copy to %s: %w", destination, err)
		}
		relay.countWithheld()
		if first {
			fmt.Println("RELAY-WITHHELD: sent a damaged copy in place of the IKE_AUTH response, message ID", messageID, "natt", natt)
		}
		return false, nil
	case corruptModeUnspecified:
	}
	panic("BUG: corrupt relay holds no mode")
}

// damageLastOctet copies message into damaged and changes its last octet, the last
// octet of the Integrity Checksum Data (RFC 7296 Section 3.14). damaged MUST be at
// least as long as message.
func damageLastOctet(message, damaged []byte) []byte {
	copyOf := damaged[:len(message)]
	copy(copyOf, message)
	copyOf[len(copyOf)-1] ^= 0xff
	return copyOf
}

// ikeAuthResponseKey reads the clear header and answers the response's key when the
// datagram is an IKE_AUTH response. A NAT-T keepalive, an ESP packet and anything too
// short for a header answer false.
func ikeAuthResponseKey(datagram []byte, natt bool) (corruptResponseKey, bool) {
	var key corruptResponseKey
	header := datagram
	if natt {
		if len(datagram) < nonESPMarkerOctets {
			return key, false
		}
		// A non-zero first word is an ESP SPI: the datagram is not IKE.
		for _, octet := range datagram[:nonESPMarkerOctets] {
			if octet != 0 {
				return key, false
			}
		}
		header = datagram[nonESPMarkerOctets:]
	}
	if len(header) < ikeHeaderOctets {
		return key, false
	}
	if header[ikeExchangeOffset] != ikeExchangeIKEAuth {
		return key, false
	}
	if header[ikeFlagsOffset]&ikeFlagResponse == 0 {
		return key, false
	}
	copy(key[:8], header[:8])
	copy(key[8:], header[ikeMessageIDOffset:ikeMessageIDOffset+4])
	return key, true
}

// recordResponse marks key seen. It answers whether key is new, and whether the record
// is full, in which case nothing was recorded.
func (relay *corruptRelay) recordResponse(key corruptResponseKey) (bool, bool) {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	if relay.responses[key] {
		return false, false
	}
	if len(relay.responses) >= corruptResponsesMax {
		return true, true
	}
	relay.responses[key] = true
	return true, false
}

func (relay *corruptRelay) countForwarded() {
	relay.mu.Lock()
	relay.forwarded++
	relay.mu.Unlock()
}

func (relay *corruptRelay) countWithheld() {
	relay.mu.Lock()
	relay.withheld++
	relay.mu.Unlock()
}

func (relay *corruptRelay) countStray() {
	relay.mu.Lock()
	relay.stray++
	relay.mu.Unlock()
}

// fail keeps the first worker failure, the one the run reports.
func (relay *corruptRelay) fail(err error) {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	if relay.failure == nil {
		relay.failure = err
	}
}

// verdict answers the run's result once both workers stopped: the first worker
// failure, else an error when no IKE_AUTH response ever crossed the relay.
func (relay *corruptRelay) verdict() error {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	fmt.Println("RELAY-SUMMARY: forwarded", relay.forwarded, "datagrams, saw", len(relay.responses), "IKE_AUTH responses, withheld", relay.withheld, "of them, ignored", relay.stray, "from unknown sources")
	if relay.failure != nil {
		return relay.failure
	}
	if len(relay.responses) == 0 {
		return fmt.Errorf("the relay forwarded %d datagrams and saw no IKE_AUTH response: the daemons never reached IKE_AUTH through %s", relay.forwarded, corruptRelayAddr)
	}
	return nil
}
