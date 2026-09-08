// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS method handler
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3 fragmentation, Section 2.1.9

// RFC 9190 Section 2.1.9 amends the EAP-TLS fragmentation of RFC 5216 Section
// 2.1.5 in two directions, and this file proves both of them over
// tlsFragmenter (eap_tls.go), which is the one producer and the one consumer of
// EAP-TLS TypeData on BOTH roles.
//
// The send direction is a prohibition: a message that fits in one fragment
// carries no L bit and no TLS Message Length, so its TLS data starts at offset
// 1 rather than offset 5. The receive direction is an obligation: an
// unfragmented message is accepted whichever way the far end wrote it, because
// the same sentence lets a peer keep the RFC 5216 shape.
//
// VALIDATES: what ze puts on the wire for a one-fragment message, that a
// genuinely fragmented message still declares its length, and that both inbound
// shapes reassemble to the same octets.
// PREVENTS: the L bit going out on every message ze sends, which is what
// nextFragment did until 2026-09-08 -- it read "first fragment" alone and never
// asked whether the first was also the last. Every EAP-TLS message ze emitted
// carried four octets of length no receiver was owed, and the interop lab stayed
// green throughout because RFC9190-2.1.9-2 obliges the far end to accept it
// either way.

package eap

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// eapTLSMessageWalk classifies one direction's EAP-TLS TypeData stream into
// whole messages, so a test can tell an unfragmented message from a
// continuation without re-deriving the fragmenter's own arithmetic.
//
// It tracks the M flag exactly as a receiver does: a packet arriving outside a
// message opens one, and the message ends at the first packet with M clear.
type eapTLSMessageWalk struct {
	inMessage bool
}

// eapTLSFragmentRole is what one TypeData is within its message.
type eapTLSFragmentRole int

const (
	// eapTLSWholeMessage is a message that opened and closed in one packet.
	eapTLSWholeMessage eapTLSFragmentRole = iota
	// eapTLSFirstOfMany is the first fragment of a message that continues.
	eapTLSFirstOfMany
	// eapTLSContinuation is any later fragment of a fragmented message.
	eapTLSContinuation
)

func (w *eapTLSMessageWalk) classify(td []byte) eapTLSFragmentRole {
	more := td[0]&eapTLSFlagM != 0
	switch {
	case w.inMessage:
		w.inMessage = more
		return eapTLSContinuation
	case more:
		w.inMessage = true
		return eapTLSFirstOfMany
	default:
		return eapTLSWholeMessage
	}
}

// TestEAPTLSSetsNoLengthBitOnAnUnfragmentedMessage reads the octets the
// fragmenter emits for a message that fits in one fragment, and then reads every
// EAP-TLS message a real exchange put on the wire from both seats.
//
// RFC requirement: RFC9190-2.1.9-1 negative -- RFC 9190 Section 2.1.9:
// "Implementations MUST NOT set the L bit in unfragmented messages, but they
// MUST accept unfragmented messages with and without the L bit set." A message
// short enough for one fragment leaves nextFragment with the L bit clear, with
// its TLS data at offset 1 rather than offset 5, and with no four-octet TLS
// Message Length in front of it. Across a complete TLS 1.3 exchange driven from
// both seats, no whole message and no continuation fragment carries the bit.
func TestEAPTLSSetsNoLengthBitOnAnUnfragmentedMessage(t *testing.T) {
	data := []byte("a TLS record short enough to leave in one EAP-TLS message")

	var f tlsFragmenter
	f.startSending(data)
	td := f.nextFragment()

	if td[0]&eapTLSFlagM != 0 {
		t.Fatalf("the one-fragment message set the M flag, flags=%#02x", td[0])
	}
	if td[0]&eapTLSFlagL != 0 {
		t.Errorf("the unfragmented message set the L flag, flags=%#02x: RFC 9190 Section 2.1.9 forbids it", td[0])
	}
	if len(td) != 1+len(data) {
		t.Errorf("the unfragmented message is %d octets for %d octets of TLS data, want %d: "+
			"a four-octet TLS Message Length is still in front of the payload", len(td), len(data), 1+len(data))
	}
	if !bytes.Equal(td[1:], data) {
		t.Errorf("the TLS data does not start at offset 1; the message carries %q behind its flags octet", td[1:])
	}
	if f.waitFragAck {
		t.Error("the fragmenter waits for a fragment ACK after a message it did not fragment")
	}

	// The same rule over a live exchange, because the fragmenter is embedded in
	// tlsMethod and in PeerSession and every EAP-TLS message either role sends
	// leaves through it.
	pki := newEAPTLSPKI(t)
	flight := driveEAPTLSFlight(t, pki.serverConfig(), newAttackPeer(pki), 0, 40)
	if !flight.peerDone {
		t.Fatalf("the exchange did not complete, so no wire is available to read: %v", flight.peerErr)
	}

	for _, side := range []struct {
		name    string
		packets []*Packet
	}{
		{name: "authenticator", packets: flight.serverSent},
		{name: "peer", packets: flight.peerSent},
	} {
		var walk eapTLSMessageWalk
		for i, pkt := range side.packets {
			if pkt == nil || pkt.Type != TypeTLS || len(pkt.TypeData) == 0 {
				continue
			}
			switch walk.classify(pkt.TypeData) {
			case eapTLSWholeMessage:
				if pkt.TypeData[0]&eapTLSFlagL != 0 {
					t.Errorf("%s packet %d is a whole message with flags %#02x: the L bit is set on a message that was never fragmented",
						side.name, i, pkt.TypeData[0])
				}
			case eapTLSContinuation:
				if pkt.TypeData[0]&eapTLSFlagL != 0 {
					t.Errorf("%s packet %d is a continuation fragment with flags %#02x: only a first fragment may declare a length",
						side.name, i, pkt.TypeData[0])
				}
			case eapTLSFirstOfMany:
			}
		}
	}
}

// TestEAPTLSKeepsTheLengthBitOnAFragmentedMessage sends a message four times the
// fragment size and reads its first fragment.
//
// RFC requirement: RFC9190-2.1.9-1 positive -- the prohibition is over
// UNFRAGMENTED messages alone, so a message that really is fragmented still
// carries the L bit and the four-octet TLS Message Length on its first fragment,
// which RFC 5216 Section 2.1.5 requires of it: "The L bit (length included) is
// set to indicate the presence of the four-octet TLS Message Length field, and
// MUST be set for the first fragment of a fragmented TLS message." A fragmenter
// that answered the prohibition by dropping the bit everywhere would leave a
// receiver with no declared length to check its reassembly against, and would
// pass the negative above.
func TestEAPTLSKeepsTheLengthBitOnAFragmentedMessage(t *testing.T) {
	data := make([]byte, 4*eapTLSFragmentSize)
	for i := range data {
		data[i] = byte(i % 251)
	}

	var f tlsFragmenter
	f.startSending(data)
	first := f.nextFragment()

	if first[0]&eapTLSFlagM == 0 {
		t.Fatalf("the first fragment of a %d-octet message cleared the M flag, flags=%#02x", len(data), first[0])
	}
	if first[0]&eapTLSFlagL == 0 {
		t.Fatalf("the first fragment of a fragmented message cleared the L flag, flags=%#02x", first[0])
	}
	if declared := int(binary.BigEndian.Uint32(first[1:5])); declared != len(data) {
		t.Errorf("the first fragment declares %d octets, want %d", declared, len(data))
	}
	if len(first) != 5+eapTLSFragmentSize {
		t.Errorf("the first fragment is %d octets, want %d: the payload does not start at offset 5", len(first), 5+eapTLSFragmentSize)
	}
	if !bytes.Equal(first[5:], data[:eapTLSFragmentSize]) {
		t.Error("the first fragment's payload is not the opening octets of the message")
	}

	// The whole message still reassembles, so the length the first fragment
	// declared is the length the receiver counts to.
	var receiver tlsFragmenter
	if err := receiver.reassemble(first); err != nil {
		t.Fatalf("reassemble the first fragment: %v", err)
	}
	for range 3 {
		if err := receiver.reassemble(f.nextFragment()); err != nil {
			t.Fatalf("reassemble a later fragment: %v", err)
		}
	}
	if !receiver.reassemblyComplete() {
		t.Fatal("the receiver holds fewer octets than the first fragment declared")
	}
	if !bytes.Equal(receiver.drainReassembled(), data) {
		t.Error("the reassembled message is not the message that was sent")
	}
}

// TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit feeds the
// receive path the same TLS data in both shapes RFC 9190 Section 2.1.9 permits.
//
// RFC requirement: RFC9190-2.1.9-2 positive -- RFC 9190 Section 2.1.9:
// "Implementations MUST NOT set the L bit in unfragmented messages, but they
// MUST accept unfragmented messages with and without the L bit set."
// tlsFragmenter.reassemble (eap_tls.go) takes an unfragmented message whose
// flags octet is bare and one that declares its length, reports each complete,
// and yields the same octets from either. A far end that still writes the RFC
// 5216 shape is therefore understood.
func TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit(t *testing.T) {
	data := []byte("one whole TLS message carried in a single EAP-TLS packet")

	bare := append([]byte{0}, data...)

	withLength := make([]byte, 5+len(data))
	binary.BigEndian.PutUint32(withLength[1:5], uint32(len(data)))
	copy(withLength[5:], data)
	withLength[0] = eapTLSFlagL

	for _, tc := range []struct {
		name     string
		typeData []byte
	}{
		{name: "without the L bit", typeData: bare},
		{name: "with the L bit", typeData: withLength},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f tlsFragmenter
			if err := f.reassemble(tc.typeData); err != nil {
				t.Fatalf("reassemble refused an unfragmented message %s: %v", tc.name, err)
			}
			if !f.reassemblyComplete() {
				t.Fatalf("an unfragmented message %s was reported incomplete", tc.name)
			}
			if got := f.drainReassembled(); !bytes.Equal(got, data) {
				t.Errorf("reassembled %q, want %q", got, data)
			}
		})
	}
}

// TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself feeds the receive
// path two messages that declare one thing and carry another.
//
// RFC requirement: RFC9190-2.1.9-2 negative -- accepting an unfragmented message
// in either shape is not accepting anything at all. A message whose L bit
// promises a four-octet TLS Message Length it is too short to hold, and one
// whose payload runs past the length it declared, are each refused with an error
// naming what disagreed. A reassembler that took every message would satisfy the
// positive above and would hand crypto/tls whatever an unauthenticated party
// sent.
func TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself(t *testing.T) {
	t.Run("the L bit with no length behind it", func(t *testing.T) {
		var f tlsFragmenter
		if err := f.reassemble([]byte{eapTLSFlagL, 0x00, 0x10}); err == nil {
			t.Fatal("reassemble accepted a message whose L bit promises a length field it does not carry")
		}
	})

	t.Run("more payload than the length declared", func(t *testing.T) {
		td := make([]byte, 5+40)
		td[0] = eapTLSFlagL
		binary.BigEndian.PutUint32(td[1:5], 10)

		var f tlsFragmenter
		if err := f.reassemble(td); err == nil {
			t.Fatal("reassemble accepted a message carrying 40 octets behind a declared length of 10")
		}
	})
}
