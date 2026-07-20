// VALIDATES: RFC 5880 Section 4.1 wire-format invariants and the Section
// 6.8.6 structural reception checks that ParseControl performs before any
// session state is touched: version, length floors, length-versus-payload,
// detect multiplier, the Multipoint bit, and a zero My Discriminator.
// PREVENTS: a malformed or hostile Control packet reaching the FSM, and a
// blanket-accept parser that would let any of those six checks rot away
// unnoticed.
package packet

import (
	"errors"
	"testing"
)

// rfc5880Good is a minimal conformant Control packet used as the base for
// every mutation below. Callers mutate one field at a time so each test
// isolates exactly one reception check.
func rfc5880Good() Control {
	return Control{
		Version:                   Version,
		Diag:                      DiagNone,
		State:                     StateDown,
		DetectMult:                3,
		Length:                    MandatoryLen,
		MyDiscriminator:           0x0A0B0C0D,
		YourDiscriminator:         0,
		DesiredMinTxInterval:      1_000_000,
		RequiredMinRxInterval:     1_000_000,
		RequiredMinEchoRxInterval: 0,
	}
}

// rfc5880Wire encodes c into a fresh MandatoryLen buffer and applies mut to
// the raw bytes so a test can forge a field WriteTo would never emit.
func rfc5880Wire(c Control, mut func(b []byte)) []byte {
	b := make([]byte, MandatoryLen)
	c.WriteTo(b, 0)
	if mut != nil {
		mut(b)
	}
	return b
}

// RFC requirement: RFC5880-4.1-1 positive -- the Version field is 1. WriteTo
// (internal/component/bfd/packet/control.go:96) packs c.Version into the top
// three bits of byte 0, and ParseControl (control.go:173) accepts the packet
// only when that value equals the Version constant (control.go:25), which is 1.
// RFC requirement: RFC5880-6.8.6-1 positive -- the same producer accepts a
// received packet whose Version is 1, so reception proceeds.
func TestRFC5880VersionOneAccepted(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), nil)
	if got := data[0] >> 5; got != 1 {
		t.Fatalf("encoded version = %d, want 1", got)
	}
	c, _, err := ParseControl(data)
	if err != nil {
		t.Fatalf("ParseControl of a version-1 packet: %v", err)
	}
	if c.Version != 1 {
		t.Fatalf("decoded version = %d, want 1", c.Version)
	}
}

// RFC requirement: RFC5880-4.1-1 negative -- a packet whose Version is not 1
// is not a BFD packet. ParseControl (control.go:173-175) returns ErrBadVersion.
// RFC requirement: RFC5880-6.8.6-1 negative -- reception discards it: the
// engine's handleInbound (internal/component/bfd/engine/loop.go:73-76) returns
// on any ParseControl error, so no session state is touched.
func TestRFC5880VersionNotOneDiscarded(t *testing.T) {
	for _, ver := range []byte{0, 2, 3, 7} {
		data := rfc5880Wire(rfc5880Good(), func(b []byte) {
			b[0] = ver<<5 | (b[0] & 0x1F)
		})
		if _, _, err := ParseControl(data); !errors.Is(err, ErrBadVersion) {
			t.Fatalf("version %d: got err %v, want ErrBadVersion", ver, err)
		}
	}
}

// RFC requirement: RFC5880-4.1-2 positive -- the Multipoint bit is zero on
// transmit. WriteTo (control.go:115-117) sets FlagMultipoint only when
// c.Multipoint is true, and the session never builds such a packet: Build
// (internal/component/bfd/session/fsm.go:222) hardcodes Multipoint: false.
// RFC requirement: RFC5880-6.8.6-5 positive -- a received packet with M == 0
// passes the check at control.go:189 and is processed.
func TestRFC5880MultipointZeroAccepted(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), nil)
	if data[1]&FlagMultipoint != 0 {
		t.Fatalf("Multipoint bit set on transmit: byte1=%#x", data[1])
	}
	if _, _, err := ParseControl(data); err != nil {
		t.Fatalf("ParseControl of an M=0 packet: %v", err)
	}
}

// RFC requirement: RFC5880-4.1-2 negative -- a received packet with the
// Multipoint bit set is discarded. ParseControl (control.go:189-191) returns
// ErrMultipointSet.
// RFC requirement: RFC5880-6.8.6-5 negative -- same producer, so the M-bit
// check is not a blanket accept.
func TestRFC5880MultipointSetDiscarded(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), func(b []byte) { b[1] |= FlagMultipoint })
	if _, _, err := ParseControl(data); !errors.Is(err, ErrMultipointSet) {
		t.Fatalf("got err %v, want ErrMultipointSet", err)
	}
}

// RFC requirement: RFC5880-6.8.6-2 positive -- a packet whose Length is
// exactly 24 with A=0, or exactly 26 with A=1, meets the minimum and is
// accepted. ParseControl (control.go:176-182) computes minLen as MandatoryLen,
// raised to MandatoryLen+2 when the A bit is set.
func TestRFC5880LengthMinimumAccepted(t *testing.T) {
	plain := rfc5880Wire(rfc5880Good(), nil)
	if _, _, err := ParseControl(plain); err != nil {
		t.Fatalf("A=0 length 24: %v", err)
	}

	authed := rfc5880Good()
	authed.Auth = true
	authed.Length = MandatoryLen + 2
	buf := make([]byte, MandatoryLen+2)
	authed.WriteTo(buf, 0)
	if _, _, err := ParseControl(buf); err != nil {
		t.Fatalf("A=1 length 26: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.6-2 negative -- a Length below the minimum is
// discarded: 23 with A=0 and 24 with A=1 (the two-byte Auth Type + Auth Len
// header raises the floor). ParseControl (control.go:180-182) returns
// ErrLengthTooSmall for both.
func TestRFC5880LengthBelowMinimumDiscarded(t *testing.T) {
	short := rfc5880Wire(rfc5880Good(), func(b []byte) { b[3] = MandatoryLen - 1 })
	if _, _, err := ParseControl(short); !errors.Is(err, ErrLengthTooSmall) {
		t.Fatalf("A=0 length 23: got err %v, want ErrLengthTooSmall", err)
	}

	authed := rfc5880Good()
	authed.Auth = true
	authed.Length = MandatoryLen // one short of the A=1 floor of 26
	buf := make([]byte, MandatoryLen)
	authed.WriteTo(buf, 0)
	if _, _, err := ParseControl(buf); !errors.Is(err, ErrLengthTooSmall) {
		t.Fatalf("A=1 length 24: got err %v, want ErrLengthTooSmall", err)
	}
}

// RFC requirement: RFC5880-6.8.6-3 positive -- a Length equal to the
// encapsulating payload is legal. ParseControl (control.go:183-185) compares
// c.Length against len(data), which the transport hands over as exactly the
// received UDP payload (internal/component/bfd/transport/udp.go delivers
// Inbound.Bytes sliced to the datagram length).
func TestRFC5880LengthEqualsPayloadAccepted(t *testing.T) {
	c := rfc5880Good()
	c.Length = MandatoryLen + 4
	buf := make([]byte, MandatoryLen+4)
	c.WriteTo(buf, 0)
	if _, _, err := ParseControl(buf); err != nil {
		t.Fatalf("length equal to payload rejected: %v", err)
	}
}

// RFC requirement: RFC5880-6.8.6-3 negative -- a Length greater than the
// encapsulating payload is discarded. ParseControl (control.go:183-185)
// returns ErrLengthOverBuffer, which is what stops a forged Length from
// driving an over-read in the authentication section.
func TestRFC5880LengthOverPayloadDiscarded(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), func(b []byte) { b[3] = MandatoryLen + 8 })
	if _, _, err := ParseControl(data); !errors.Is(err, ErrLengthOverBuffer) {
		t.Fatalf("got err %v, want ErrLengthOverBuffer", err)
	}
}

// RFC requirement: RFC5880-6.8.6-4 positive -- a nonzero Detect Mult is
// accepted; ParseControl (control.go:186-188) rejects only zero, so 1 and 255
// both pass and round-trip.
func TestRFC5880DetectMultNonZeroAccepted(t *testing.T) {
	for _, mult := range []uint8{1, 3, 255} {
		data := rfc5880Wire(rfc5880Good(), func(b []byte) { b[2] = mult })
		c, _, err := ParseControl(data)
		if err != nil {
			t.Fatalf("detect mult %d: %v", mult, err)
		}
		if c.DetectMult != mult {
			t.Fatalf("detect mult decoded %d, want %d", c.DetectMult, mult)
		}
	}
}

// RFC requirement: RFC5880-6.8.6-4 negative -- a packet with Detect Mult zero
// is discarded. ParseControl (control.go:186-188) returns ErrZeroDetectMult;
// a zero multiplier would otherwise produce a zero detection time.
func TestRFC5880DetectMultZeroDiscarded(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), func(b []byte) { b[2] = 0 })
	if _, _, err := ParseControl(data); !errors.Is(err, ErrZeroDetectMult) {
		t.Fatalf("got err %v, want ErrZeroDetectMult", err)
	}
}

// RFC requirement: RFC5880-6.8.6-6 positive -- a nonzero My Discriminator is
// accepted and decoded verbatim. ParseControl (control.go:165,192-194) reads
// bytes 4..7 big-endian and rejects only the zero value.
func TestRFC5880MyDiscriminatorNonZeroAccepted(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), nil)
	c, _, err := ParseControl(data)
	if err != nil {
		t.Fatalf("ParseControl: %v", err)
	}
	if c.MyDiscriminator != 0x0A0B0C0D {
		t.Fatalf("MyDiscriminator = %#x, want 0x0A0B0C0D", c.MyDiscriminator)
	}
}

// RFC requirement: RFC5880-6.8.6-6 negative -- a packet whose My
// Discriminator is zero is discarded. ParseControl (control.go:192-194)
// returns ErrZeroMyDisc, so a peer that never learned its own discriminator
// cannot install itself as the remote discriminator of a live session.
func TestRFC5880MyDiscriminatorZeroDiscarded(t *testing.T) {
	data := rfc5880Wire(rfc5880Good(), func(b []byte) {
		b[4], b[5], b[6], b[7] = 0, 0, 0, 0
	})
	if _, _, err := ParseControl(data); !errors.Is(err, ErrZeroMyDisc) {
		t.Fatalf("got err %v, want ErrZeroMyDisc", err)
	}
}
