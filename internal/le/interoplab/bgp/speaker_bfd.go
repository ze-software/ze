// Design: docs/architecture/testing/interop.md -- the interop suites and their peers
// RFC: rfc/full/rfc5880.txt -- the BFD control packet and its state machine
// RFC: rfc/full/rfc5881.txt -- single-hop BFD over IPv4, ports and TTL
// Related: speaker.go -- the wire-level BGP speaker this responder runs beside
//
// A minimal BFD responder for the interop speaker.
//
// It exists because draft-ietf-idr-bgp-bfd-strict-mode is implemented by no
// daemon this lab can run: FRR 10.3.1 carries no BFD strict-mode capability
// (its bgpd binary declares only the RFC 9234 local-role strict-mode of an
// unrelated feature), and the draft's own Appendix A lists Junos, IOS-XR and
// Nokia. Proving that ze holds a BGP session until BFD is Up therefore needs a
// peer that (a) advertises capability 74 and (b) can bring a BFD session Up,
// and this file is the second half.
//
// It is written from RFC 5880 Section 4.1 (the packet), RFC 5880 Section 6.8.6
// (the receive procedures) and RFC 5881 Sections 4 and 5 (the ports and the
// TTL), never from ze's own encoder. A responder built by reading ze would
// prove that ze agrees with itself.
//
// What it deliberately does NOT implement, because no scenario needs it and an
// unused branch is a place for a defect to hide: authentication (RFC 5880
// Section 6.7), echo (Section 6.4), Demand mode (Section 6.6), the Poll
// sequence (Section 6.5), and the detection timer. It transmits at a fixed
// rate, answers what arrives, and never declares the far end down.
package bgp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"time"
)

// The BFD control packet, RFC 5880 Section 4.1.
const (
	bfdControlLength = 24
	bfdVersion       = 1
	// RFC 5881 Section 4: "The destination port MUST be 3784". A single-hop
	// session's packets arrive here.
	bfdSingleHopPort = 3784
	// bfdTransmitInterval is this end's fixed transmission rate. The draft's
	// scenarios measure seconds, so nothing here needs the RFC 5880 Section
	// 6.8.7 jitter or the slow-start rate.
	bfdTransmitInterval = 300 * time.Millisecond
)

// bfd.SessionState values, RFC 5880 Section 6.8.1.
const (
	bfdStateAdminDown uint8 = 0
	bfdStateDown      uint8 = 1
	bfdStateInit      uint8 = 2
	bfdStateUp        uint8 = 3
)

// bfdResponder answers a single-hop BFD session from the interop speaker's
// container. One session, one peer, no authentication.
//
// Concurrency: `up` and `upAt` are read by the BGP half of the speaker while
// the responder's own loop writes them, so both are atomic. Every other field
// belongs to the run loop.
type bfdResponder struct {
	// peer is the address the BGP peer runs on, which for a single-hop
	// session is the same address the BFD packets go to.
	peer net.IP

	// delay is how long the responder stays SILENT before it answers
	// anything. It is what creates the window a strict-mode scenario
	// asserts in: while nothing answers, ze's BFD session cannot leave
	// Down, so ze must be holding its BGP session.
	delay time.Duration

	// discriminator is this end's My Discriminator. RFC 5880 Section 6.8.6
	// refuses a packet whose My Discriminator is zero, so it is never zero.
	discriminator uint32

	// state is bfd.SessionState for this end.
	state uint8

	// remote is bfd.RemoteDiscr, which RFC 5880 Section 6.8.6 sets from the
	// received My Discriminator and which every packet this end sends
	// carries back as Your Discriminator.
	remote uint32

	// up latches once this end reaches the Up state. The BGP half of the
	// speaker reads it to say whether ze's KEEPALIVE arrived before or after
	// the BFD session came up, which is the whole assertion.
	up atomic.Bool

	// upAt is when that happened, as Unix nanoseconds, so the BGP half can
	// order it against the KEEPALIVE it saw.
	upAt atomic.Int64

	// silenceEndedAt is when this end started answering, as Unix nanoseconds.
	//
	// It, and not upAt, is the boundary the strict-mode oracle judges against.
	// RFC 5880 Section 6.8.6 brings the two ends Up one packet flight APART:
	// this end goes Down -> Init on ze's first Down packet, ze goes Down -> Up
	// on this end's Init, and only ze's Up brings this end Init -> Up. So ze
	// legitimately releases its BGP session a flight BEFORE this end reaches
	// Up, and measured in the lab that is about 3.7 ms. What no correct ze can
	// do is release while this end is SILENT: with nothing answering, ze's own
	// bfd.SessionState cannot leave Down at all.
	silenceEndedAt atomic.Int64
}

// newBFDResponder builds a responder for one peer.
func newBFDResponder(peer net.IP, delay time.Duration) *bfdResponder {
	return &bfdResponder{
		peer: peer,
		// RFC 5880 Section 6.8.1: "bfd.LocalDiscr MUST be unique among all
		// BFD sessions on this system, and MUST NOT be zero."
		discriminator: 0x5A455445, // "ZETE", a constant this lab owns
		delay:         delay,
		state:         bfdStateDown,
	}
}

// isUp reports whether this end has reached the Up state, and when.
func (r *bfdResponder) isUp() (bool, time.Time) {
	if !r.up.Load() {
		return false, time.Time{}
	}
	return true, time.Unix(0, r.upAt.Load())
}

// encodeControl writes one BFD control packet, RFC 5880 Section 4.1:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|Vers |  Diag   |Sta|P|F|C|A|D|M|  Detect Mult  |    Length     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       My Discriminator                        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                      Your Discriminator                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                    Desired Min TX Interval                    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                   Required Min RX Interval                    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                 Required Min Echo RX Interval                 |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Every flag is zero: no Poll, no Final, no Control Plane Independent, no
// Authentication, no Demand, no Multipoint. Echo is refused by advertising a
// Required Min Echo RX Interval of zero, which RFC 5880 Section 6.4 reads as
// "the transmitting system does not support the receipt of BFD Echo packets".
func (r *bfdResponder) encodeControl(state uint8, remote uint32) []byte {
	packet := make([]byte, bfdControlLength)
	packet[0] = bfdVersion << 5 // version 1, diagnostic 0 (No Diagnostic)
	packet[1] = state << 6
	packet[2] = 3 // Detect Mult; RFC 5880 Section 6.8.6 refuses zero
	packet[3] = bfdControlLength
	binary.BigEndian.PutUint32(packet[4:8], r.discriminator)
	binary.BigEndian.PutUint32(packet[8:12], remote)
	binary.BigEndian.PutUint32(packet[12:16], 300000) // Desired Min TX, microseconds
	binary.BigEndian.PutUint32(packet[16:20], 300000) // Required Min RX, microseconds
	binary.BigEndian.PutUint32(packet[20:24], 0)      // no Echo
	return packet
}

// receive applies the subset of RFC 5880 Section 6.8.6 a responder needs, and
// returns the state this end now holds plus whether the packet was accepted.
//
// RFC 5880 Section 6.8.6: "Set bfd.RemoteDiscr to the value of My
// Discriminator." and the state block:
//
//	"If bfd.SessionState is Down
//	    If received State is Down
//	        Set bfd.SessionState to Init
//	    Else if received State is Init
//	        Set bfd.SessionState to Up
//	Else if bfd.SessionState is Init
//	    If received State is Init or Up
//	        Set bfd.SessionState to Up"
//
// The packet is refused, and the state left alone, for each condition the same
// section lists: a version other than 1, a length below 24, a zero Detect Mult,
// the Multipoint bit set, and a zero My Discriminator.
func (r *bfdResponder) receive(packet []byte) (uint8, bool) {
	if len(packet) < bfdControlLength {
		return r.state, false
	}
	if packet[0]>>5 != bfdVersion {
		return r.state, false
	}
	if packet[2] == 0 {
		return r.state, false
	}
	if int(packet[3]) < bfdControlLength || int(packet[3]) > len(packet) {
		return r.state, false
	}
	if packet[1]&0x01 != 0 { // Multipoint
		return r.state, false
	}
	myDiscriminator := binary.BigEndian.Uint32(packet[4:8])
	if myDiscriminator == 0 {
		return r.state, false
	}

	r.remote = myDiscriminator
	received := packet[1] >> 6

	switch r.state {
	case bfdStateDown:
		switch received {
		case bfdStateDown:
			r.state = bfdStateInit
		case bfdStateInit:
			r.state = bfdStateUp
		}
	case bfdStateInit:
		if received == bfdStateInit || received == bfdStateUp {
			r.state = bfdStateUp
		}
	case bfdStateUp:
		// RFC 5880 Section 6.8.6 takes a session that is Up back to Down on a
		// received AdminDown, and on a received Down. Neither happens in a
		// scenario that never breaks the path, and this end never declares
		// the far end down on its own: it runs no detection timer.
		if received == bfdStateAdminDown || received == bfdStateDown {
			r.state = bfdStateDown
		}
	}

	if r.state == bfdStateUp && r.up.CompareAndSwap(false, true) {
		r.upAt.Store(time.Now().UnixNano())
	}
	return r.state, true
}

// readLoop is the responder's receive worker: one long-lived goroutine reading
// one socket, per ai/rules/goroutine-lifecycle.md. It exits when stop is closed
// or the socket is closed, and it closes packets on the way out so the run loop
// cannot block on a channel nobody fills.
func (r *bfdResponder) readLoop(listener *net.UDPConn, packets chan<- []byte, stop <-chan struct{}) {
	defer close(packets)
	buffer := make([]byte, 512)
	for {
		_ = listener.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		count, _, err := listener.ReadFromUDP(buffer)
		if count > 0 {
			select {
			case packets <- append([]byte(nil), buffer[:count]...):
			default: // the run loop is behind; drop, exactly as a real engine does
			}
		}
		if err == nil {
			continue
		}
		if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
			select {
			case <-stop:
				return
			default:
				continue
			}
		}
		return
	}
}

// run answers BFD until stop is closed. It returns the note lines the speaker's
// verdict carries, so a failure to open a socket is REPORTED rather than
// silently leaving the session down: a scenario whose BFD never came up because
// a socket could not be opened would otherwise read as ze holding the session
// correctly (ai/rules/principles.md).
//
// Two sockets, because RFC 5881 Section 4 gives the two directions different
// ports: "The source port MUST be in the range 49152 through 65535. The
// destination port MUST be 3784." So this end receives on 3784 and transmits
// from an ephemeral port the kernel picks inside that range.
func (r *bfdResponder) run(stop <-chan struct{}) []string {
	notes := []string{}

	listener, err := net.ListenUDP("udp4", &net.UDPAddr{Port: bfdSingleHopPort})
	if err != nil {
		return append(notes, "bfd-error: listen 3784: "+err.Error())
	}
	defer func() { _ = listener.Close() }()

	sender, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return append(notes, "bfd-error: open sender: "+err.Error())
	}
	defer func() { _ = sender.Close() }()

	// RFC 5881 Section 5: "BFD Control packets MUST be sent with a Time to
	// Live (TTL) or Hop Limit value of 255." Ze enforces the receive half of
	// that GTSM check, so a packet sent with the default TTL is dropped
	// before its state is ever read.
	if err := setUnicastTTL(sender, 255); err != nil {
		return append(notes, "bfd-error: set TTL 255: "+err.Error())
	}

	target := &net.UDPAddr{IP: r.peer, Port: bfdSingleHopPort}
	packets := make(chan []byte, 16)
	go r.readLoop(listener, packets, stop)

	// The silent window. Nothing is sent, so ze's session cannot leave Down,
	// which is the state the strict-mode hold is asserted against.
	if r.delay > 0 {
		select {
		case <-stop:
			return append(notes, "bfd-state: silent")
		case <-time.After(r.delay):
		}
		notes = append(notes, "bfd-silence-ended: yes")
	}
	r.silenceEndedAt.Store(time.Now().UnixNano())

	transmit := time.NewTicker(bfdTransmitInterval)
	defer transmit.Stop()
	for {
		select {
		case <-stop:
			return notes
		case packet, ok := <-packets:
			if !ok {
				return notes
			}
			r.receive(packet)
		case <-transmit.C:
			if _, err := sender.WriteToUDP(r.encodeControl(r.state, r.remote), target); err != nil {
				return append(notes, "bfd-error: send: "+err.Error())
			}
		}
	}
}

// setUnicastTTL sets IP_TTL on the socket, which is how RFC 5881 Section 5's
// "MUST be sent with a TTL of 255" is met for an unconnected UDP socket. The Go
// standard library exposes no portable setter, so this reaches the file
// descriptor; both constants exist on every platform this lab builds for.
func setUnicastTTL(connection *net.UDPConn, ttl int) error {
	raw, err := connection.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	}); err != nil {
		return err
	}
	return sockErr
}

// runBFDResponder is the responder's owning worker. It runs the responder to
// completion and hands its notes back on one channel, so the caller's deferred
// stop can collect them (ai/rules/goroutine-lifecycle.md: one goroutine, one
// lifecycle, one stop path).
func runBFDResponder(responder *bfdResponder, stop <-chan struct{}, notes chan<- []string) {
	notes <- responder.run(stop)
}

// applyBFDStrictOracle is the speaker's verdict on
// draft-ietf-idr-bgp-bfd-strict-mode. It asserts BEHAVIOR rather than the
// presence of a capability byte: what the draft changes is WHEN the KEEPALIVE
// is sent, so that is what is judged.
//
// Three assertions, and the third is the one with teeth:
//
//  1. This end's BFD session reached Up. Without it nothing was tested, because
//     a peer that never answers holds every implementation, correct or not.
//  2. Section 8.5.1, the release: "sends a KEEPALIVE message ... and changes
//     its state to OpenConfirm." A session that never establishes fails.
//  3. Section 8.5.5, the hold: "DOES NOT send a KEEPALIVE message, and DOES NOT
//     start the KeepaliveTimer ... stays in OpenSent state
//     (OpenSentBfdUpPending)." The KEEPALIVE must arrive AFTER this end stopped
//     being silent.
//
// The third boundary is the silence, never this end's own Up time. RFC 5880
// Section 6.8.6 brings the two ends Up one packet flight apart, ze first, so a
// correct ze releases its session a few milliseconds BEFORE this end reaches
// Up. What a correct ze cannot do is release while nothing is answering: with
// no packets arriving, ze's own bfd.SessionState is stuck in Down, and the
// window is the whole --bfd-delay rather than one flight.
func applyBFDStrictOracle(responder *bfdResponder, firstKeepalive time.Time, verdict *speakerVerdict) {
	up, upAt := responder.isUp()
	if up {
		verdict.notes = append(verdict.notes, "bfd-up: yes")
	} else {
		verdict.notes = append(verdict.notes, "bfd-up: no")
	}
	if firstKeepalive.IsZero() {
		verdict.notes = append(verdict.notes, "ze-keepalive: none")
	} else {
		verdict.notes = append(verdict.notes, "ze-keepalive: yes")
	}

	if !up {
		verdict.fail("the BFD session never reached Up, so nothing was proven about the hold; " +
			"check the responder notes for a socket or TTL failure")
		return
	}
	if firstKeepalive.IsZero() {
		verdict.fail("draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.1: BFD reached Up and ze sent no KEEPALIVE, " +
			"so the session never left the OpenSent sub-state")
		return
	}
	silenceEnded := time.Unix(0, responder.silenceEndedAt.Load())
	if responder.delay > 0 && firstKeepalive.Before(silenceEnded) {
		verdict.fail(fmt.Sprintf("draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5: ze sent its KEEPALIVE %s "+
			"BEFORE this end answered a single BFD packet, so its own bfd.SessionState was still Down "+
			"and the session was not held", silenceEnded.Sub(firstKeepalive)))
		return
	}
	verdict.notes = append(verdict.notes,
		fmt.Sprintf("keepalive-after-silence: %s", firstKeepalive.Sub(silenceEnded)),
		fmt.Sprintf("keepalive-vs-local-bfd-up: %s", firstKeepalive.Sub(upAt)))
}
