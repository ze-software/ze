// VALIDATES: the two RFC 5880 authentication sequence variables a session
// owns: bfd.XmitAuthSeq advances in the unsigned 32-bit circular space on
// every Meticulous transmission, and bfd.AuthSeqKnown starts at zero on a new
// session.
// PREVENTS: a transmit sequence that saturates or stops at 0xFFFFFFFF, and a
// session that starts with a replay floor it never received, which would
// discard the peer's first authentic packet.
//
// Method: each test drives the session Machine through its own entry points
// (SetAuth, Sign, AdvanceAuthSeq, Verify) and reads the Sequence Number from
// the bytes Sign wrote.
package session

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880SeqAuthPair builds a signer and verifier pair of authType.
func rfc5880SeqAuthPair(t *testing.T, authType uint8) *AuthPair {
	t.Helper()
	cfg := auth.Settings{Type: authType, KeyID: 9, Secret: []byte("rfc5880-seq-key")}
	signer, err := auth.NewSigner(cfg)
	if err != nil {
		t.Fatalf("type %d: NewSigner: %v", authType, err)
	}
	verifier, err := auth.NewVerifier(cfg)
	if err != nil {
		t.Fatalf("type %d: NewVerifier: %v", authType, err)
	}
	return &AuthPair{Signer: signer, Verifier: verifier}
}

// rfc5880SignedByMachine writes one Control packet with m's authentication
// section and returns the bytes and the Control the peer parses.
func rfc5880SignedByMachine(m *Machine) ([]byte, packet.Control) {
	c := m.Build()
	buf := make([]byte, c.Length)
	c.WriteTo(buf, 0)
	m.Sign(buf, packet.MandatoryLen)
	return buf, c
}

// RFC requirement: RFC5880-6.7.3-4 positive -- for Meticulous Keyed MD5,
// starting from bfd.XmitAuthSeq 0xFFFFFFFE, four consecutive Sign plus
// AdvanceAuthSeq rounds put 0xFFFFFFFE, 0xFFFFFFFF, 0 and 1 in the transmitted
// Sequence Number field, and a Meticulous peer verifier accepts all four.
// RFC requirement: RFC5880-6.7.4-4 positive -- the same four rounds put the
// same wrapped sequence on the wire for Meticulous Keyed SHA1.
func TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		m, _ := newMachine(t, newFakeClock())
		pair := rfc5880SeqAuthPair(t, at)
		m.SetAuth(pair)
		m.vars.XmitAuthSeq = 0xFFFFFFFE

		var peer auth.SeqState
		for _, want := range []uint32{0xFFFFFFFE, 0xFFFFFFFF, 0, 1} {
			buf, c := rfc5880SignedByMachine(m)
			m.AdvanceAuthSeq()
			if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != want {
				t.Fatalf("type %d: transmitted Sequence Number %#x, want %#x", at, got, want)
			}
			if err := pair.Verifier.Verify(buf, c, &peer); err != nil {
				t.Fatalf("type %d: peer discarded sequence %#x: %v", at, want, err)
			}
		}
	}
}

// RFC requirement: RFC5880-6.8.1-12 positive -- a new session with
// authentication installed has bfd.AuthSeqKnown 0 and bfd.RcvAuthSeq 0, and
// accepts an authentic first packet at Sequence Number 0x80000000, which lies
// outside every replay window above 0; bfd.AuthSeqKnown is 1 after it.
func TestRFC5880NewSessionAuthSeqKnownZero(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	pair := rfc5880SeqAuthPair(t, packet.AuthTypeKeyedSHA1)
	m.SetAuth(pair)
	if m.rcvAuthSeq.Initialized() || m.rcvAuthSeq.Last() != 0 {
		t.Fatalf("new session: bfd.AuthSeqKnown %v, bfd.RcvAuthSeq %d, want false and 0",
			m.rcvAuthSeq.Initialized(), m.rcvAuthSeq.Last())
	}

	peer, _ := newMachine(t, newFakeClock())
	peer.SetAuth(rfc5880SeqAuthPair(t, packet.AuthTypeKeyedSHA1))
	peer.vars.XmitAuthSeq = 0x80000000
	buf, c := rfc5880SignedByMachine(peer)
	if err := m.Verify(buf, c); err != nil {
		t.Fatalf("first authentic packet at 0x80000000 discarded by a new session: %v", err)
	}
	if !m.rcvAuthSeq.Initialized() {
		t.Fatal("bfd.AuthSeqKnown still 0 after the first accepted packet")
	}
}

// RFC requirement: RFC5880-6.8.1-12 negative -- a session whose
// bfd.AuthSeqKnown is 1 with bfd.RcvAuthSeq 0, the state initialization to 1
// would create, discards the same authentic packet at 0x80000000 with
// auth.ErrSequenceOutsideWindow, so the zero start is what lets the new
// session above accept it.
func TestRFC5880AuthSeqKnownOneDiscardsFirstPacket(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	m.SetAuth(rfc5880SeqAuthPair(t, packet.AuthTypeKeyedSHA1))
	m.rcvAuthSeq.Advance(0)

	peer, _ := newMachine(t, newFakeClock())
	peer.SetAuth(rfc5880SeqAuthPair(t, packet.AuthTypeKeyedSHA1))
	peer.vars.XmitAuthSeq = 0x80000000
	buf, c := rfc5880SignedByMachine(peer)
	if err := m.Verify(buf, c); !errors.Is(err, auth.ErrSequenceOutsideWindow) {
		t.Fatalf("got %v, want auth.ErrSequenceOutsideWindow with bfd.AuthSeqKnown 1", err)
	}
}
