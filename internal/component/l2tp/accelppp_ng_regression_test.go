// Design: docs/architecture/wire/l2tp.md -- L2TP control-message parsing under hostile input
// Related: hidden.go -- HiddenDecrypt, the Original Length check under test
// Related: avp.go -- AVPIterator and skipHiddenAVP under test
// Related: header.go -- ParseMessageHeader, the Length check under test

package l2tp

import (
	"bytes"
	"errors"
	"testing"
)

// The tests in this file replay the malformed inputs behind accel-ppp-ng's
// L2TP security fixes against Ze's control-message parsing. Each test names
// the accel-ppp-ng commit its input comes from.

// VALIDATES: a hidden AVP whose decrypted Original Length exceeds the octets
// that follow it is refused with ErrHiddenLenMismatch, a hidden value too short
// to hold the Original Length is refused with ErrInvalidAVPLen, and the
// untampered ciphertext still decrypts.
//
// Source: accel-ppp-ng a8ca0f3 (T8439), "L2TP - Add validation checks for
// Hidden AVP lengths": l2tp_recv trusted the decrypted length.
//
// RFC 2661 Section 4.3: "Length of Original Attribute Value:  This is length
// of the Original Attribute Value to be obscured in octets. This is necessary
// to determine the original length of the Attribute Value which is lost when
// the additional Padding is added."
func TestNGHiddenAVPOriginalLengthChecked(t *testing.T) {
	secret := []byte("tunnel-secret")
	rv := []byte("random-vector-0123")
	plaintext := []byte{0x00, 0x2A, 0x00, 0x07}

	ct := append([]byte(nil), HiddenEncrypt(make([]byte, 64), AVPAssignedSessionID, secret, rv, plaintext)...)
	got, err := HiddenDecrypt(make([]byte, 64), AVPAssignedSessionID, secret, rv, ct)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("untampered hidden AVP: got %x err %v, want %x", got, err, plaintext)
	}

	// XOR is positional: flipping a ciphertext bit flips the same plaintext
	// bit, so this raises the decrypted Original Length from 4 to 260.
	forged := append([]byte(nil), ct...)
	forged[0] ^= 0x01
	if _, err := HiddenDecrypt(make([]byte, 64), AVPAssignedSessionID, secret, rv, forged); !errors.Is(err, ErrHiddenLenMismatch) {
		t.Fatalf("Original Length 260 over 4 octets: error %v, want ErrHiddenLenMismatch", err)
	}

	for _, short := range [][]byte{nil, {0x00}} {
		if _, err := HiddenDecrypt(make([]byte, 64), AVPAssignedSessionID, secret, rv, short); !errors.Is(err, ErrInvalidAVPLen) {
			t.Fatalf("%d-octet hidden value: error %v, want ErrInvalidAVPLen", len(short), err)
		}
	}
}

// VALIDATES: the control-message parsers never consume a hidden value as
// plaintext: a mandatory hidden AVP is refused, a non-mandatory one skipped,
// and a plain AVP passed through.
//
// Source: accel-ppp-ng a8ca0f3 (T8439). Ze's control-message parsers do not
// decrypt hidden AVPs inline (HiddenDecrypt has no caller outside tests), so
// the over-read accel-ppp fixed has no path in Ze; skipHiddenAVP is that path.
func TestNGHiddenAVPNeverReadAsPlaintext(t *testing.T) {
	if skip, err := skipHiddenAVP("ICRQ", AVPAssignedSessionID, FlagHidden|FlagMandatory); !skip || err == nil {
		t.Fatalf("mandatory hidden AVP: skip %v err %v, want skip with an error", skip, err)
	}
	if skip, err := skipHiddenAVP("ICRQ", AVPCallingNumber, FlagHidden); !skip || err != nil {
		t.Fatalf("optional hidden AVP: skip %v err %v, want a silent skip", skip, err)
	}
	if skip, err := skipHiddenAVP("ICRQ", AVPCallingNumber, FlagMandatory); skip || err != nil {
		t.Fatalf("plain AVP: skip %v err %v, want it read", skip, err)
	}
}

// VALIDATES: an AVP whose Length is 0, or below the 6-octet header, stops the
// iterator with ErrInvalidAVPLen for good, and an L2TP header whose Length is
// 0 is refused, so no length-driven loop can stall on a zero length.
//
// Source: accel-ppp-ng 2fa6475 (T8550), "Zero length packet leading to
// infinite CPU loop" in SSTP. Ze has no SSTP; the same shape is the AVP walk.
//
// RFC 2661 Section 4.1: "The minimum Length of an AVP is 6."
func TestNGZeroLengthAVPStopsIterator(t *testing.T) {
	for _, length := range []byte{0, 1, 5} {
		it := NewAVPIterator([]byte{0x80, length, 0, 0, 0, 0, 0x80, length, 0, 0, 0, 0})
		for range 3 {
			if _, _, _, _, ok := it.Next(); ok {
				t.Fatalf("AVP Length %d yielded an AVP", length)
			}
		}
		if !errors.Is(it.Err(), ErrInvalidAVPLen) {
			t.Fatalf("AVP Length %d: Err %v, want ErrInvalidAVPLen", length, it.Err())
		}
	}

	// Positive control: a 6-octet AVP is yielded once, then clean exhaustion.
	it := NewAVPIterator([]byte{0x80, 0x06, 0, 0, 0, 0})
	if _, _, _, _, ok := it.Next(); !ok {
		t.Fatalf("6-octet AVP not yielded: %v", it.Err())
	}
	if _, _, _, _, ok := it.Next(); ok || it.Err() != nil {
		t.Fatalf("after the only AVP: ok %v err %v, want clean exhaustion", ok, it.Err())
	}

	// A control message (T=1, L=1, S=1) whose Length field is 0.
	hdr := []byte{0xC8, 0x02, 0x00, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	if _, err := ParseMessageHeader(hdr); !errors.Is(err, ErrShortBuffer) {
		t.Fatalf("control header with Length 0: error %v, want ErrShortBuffer", err)
	}
	hdr[3] = 12
	if _, err := ParseMessageHeader(hdr); err != nil {
		t.Fatalf("control header with Length 12: %v", err)
	}
}
