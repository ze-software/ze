// RFC: rfc/full/rfc5903.txt -- ECP public value encoding and test vectors (Sections 7, 8)
// RFC: rfc/short/rfc7296.md -- Key Exchange payload (Section 3.4)

package crypto

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// The vectors below are copied verbatim from RFC 5903 Section 8. They are an authority
// outside Ze: every earlier ECP test round-tripped Ze against Ze, so a public value one
// octet too long canceled out and the suite stayed green while no other implementation
// could parse the KE payload.

const (
	// RFC 5903 Section 8.1, 256-Bit Random ECP Group (group 19).
	rfc5903P256PrivateI = "C88F01F5 10D9AC3F 70A292DA A2316DE5 44E9AAB8 AFE84049 C62A9C57 862D1433"
	rfc5903P256Gix      = "DAD0B653 94221CF9 B051E1FE CA5787D0 98DFE637 FC90B9EF 945D0C37 72581180"
	rfc5903P256Giy      = "5271A046 1CDB8252 D61F1C45 6FA3E59A B1F45B33 ACCF5F58 389E0577 B8990BB3"
	rfc5903P256Grx      = "D12DFB52 89C8D4F8 1208B702 70398C34 2296970A 0BCCB74C 736FC755 4494BF63"
	rfc5903P256Gry      = "56FBF3CA 366CC23E 8157854C 13C58D6A AC23F046 ADA30F83 53E74F33 039872AB"
	// RFC 5903 Section 8.1: the shared secret value is girx, the x coordinate alone.
	rfc5903P256Girx = "D6840F6B 42F6EDAF D13116E0 E1256520 2FEF8E9E CE7DCE03 812464D0 4B9442DE"

	// RFC 5903 Section 8.2, 384-Bit Random ECP Group (group 20).
	rfc5903P384PrivateI = "099F3C70 34D4A2C6 99884D73 A375A67F 7624EF7C 6B3C0F16 0647B674 14DCE655" +
		"E35B5380 41E649EE 3FAEF896 783AB194"
	rfc5903P384Gix = "667842D7 D180AC2C DE6F74F3 7551F557 55C7645C 20EF73E3 1634FE72 B4C55EE6" +
		"DE3AC808 ACB4BDB4 C88732AE E95F41AA"
	rfc5903P384Giy = "9482ED1F C0EEB9CA FC498462 5CCFC23F 65032149 E0E144AD A0241815 35A0F38E" +
		"EB9FCFF3 C2C947DA E69B4C63 4573A81C"
	rfc5903P384Grx = "E558DBEF 53EECDE3 D3FCCFC1 AEA08A89 A987475D 12FD950D 83CFA417 32BC509D" +
		"0D1AC43A 0336DEF9 6FDA41D0 774A3571"
	rfc5903P384Gry = "DCFBEC7A ACF31964 72169E83 8430367F 66EEBE3C 6E70C416 DD5F0C68 759DD1FF" +
		"F83FA401 42209DFF 5EAAD96D B9E6386C"
	rfc5903P384Girx = "11187331 C279962D 93D60424 3FD592CB 9D0A926F 422E4718 7521287E 7156C5C4" +
		"D6031355 69B9E9D0 9CF5D4A2 70F59746"
)

// rfcHex decodes a whitespace-formatted hex constant copied out of an RFC.
func rfcHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatalf("decode hex fixture: %v", err)
	}
	return b
}

// TestECPPublicValueOctetLength pins the exact width of the value Ze puts in a KE payload.
//
// VALIDATES: RFC 5903 Section 7 -- "The Diffie-Hellman public value is obtained by
// concatenating the x and y values", each component at the group's bit length. So the
// KE payload carries 64 octets for group 19 and 96 for group 20.
// PREVENTS: regression to crypto/ecdh's SEC 1 uncompressed encoding, 0x04 || X || Y,
// which is 65 and 97 octets. strongSwan answers that with "invalid DH public value size
// (65 bytes) for ECP_256" and NO_PROPOSAL_CHOSEN, so no ECP SA can ever come up.
func TestECPPublicValueOctetLength(t *testing.T) {
	cases := []struct {
		name    string
		group   DHGroupID
		want    int
		curve   ecdh.Curve
		modulus int
	}{
		{"group 19 ECP-256", DH_ECP_256, 64, ecdh.P256(), 32},
		{"group 20 ECP-384", DH_ECP_384, 96, ecdh.P384(), 48},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex, err := NewDHExchange(c.group)
			if err != nil {
				t.Fatalf("NewDHExchange: %v", err)
			}
			defer ex.Clear()

			if len(ex.PublicKey) != c.want {
				t.Errorf("public value = %d octets, want %d (RFC 5903 Section 7: x || y)",
					len(ex.PublicKey), c.want)
			}
			if len(ex.PublicKey) != 2*c.modulus {
				t.Errorf("public value is not two %d-octet coordinates", c.modulus)
			}
			// The value must still name a point on the curve once the SEC 1 tag is
			// restored. A length that happens to be right but carries the wrong bytes
			// would pass the check above and fail here.
			sec1 := append([]byte{sec1UncompressedTag}, ex.PublicKey...)
			if _, err := c.curve.NewPublicKey(sec1); err != nil {
				t.Errorf("0x04 || value is not a valid curve point: %v", err)
			}
		})
	}
}

// TestECPKEPayloadMatchesRFC5903Vector checks the bytes Ze would send against the KEi
// payload the RFC prints for a known private key.
//
// VALIDATES: RFC 5903 Section 8.1 and 8.2 -- for private key i the KE payload data is
// exactly gix || giy, with no tag octet and no padding beyond the coordinate width.
// PREVENTS: any re-encoding of the local public value. This is the one test whose
// expected bytes were not produced by Ze.
func TestECPKEPayloadMatchesRFC5903Vector(t *testing.T) {
	cases := []struct {
		name    string
		curve   ecdh.Curve
		group   DHGroupID
		private string
		gix     string
		giy     string
	}{
		{"group 19 ECP-256", ecdh.P256(), DH_ECP_256, rfc5903P256PrivateI, rfc5903P256Gix, rfc5903P256Giy},
		{"group 20 ECP-384", ecdh.P384(), DH_ECP_384, rfc5903P384PrivateI, rfc5903P384Gix, rfc5903P384Giy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			priv, err := c.curve.NewPrivateKey(rfcHex(t, c.private))
			if err != nil {
				t.Fatalf("NewPrivateKey from RFC 5903 vector: %v", err)
			}
			ex := &DHExchange{GroupID: c.group}
			if err := ecpExchangePublic(ex, priv); err != nil {
				t.Fatalf("ecpExchangePublic: %v", err)
			}

			want := append(rfcHex(t, c.gix), rfcHex(t, c.giy)...)
			if !bytes.Equal(ex.PublicKey, want) {
				t.Errorf("KE payload data = %x\nwant (RFC 5903 gix || giy) = %x", ex.PublicKey, want)
			}
		})
	}
}

// TestECPSharedSecretFromPeerWireValue drives the receive path with the RFC's own peer
// value: 64 or 96 bare octets, exactly what a conforming implementation sends.
//
// VALIDATES: RFC 5903 Section 7 -- "The Diffie-Hellman shared secret value consists of
// the x value of the Diffie-Hellman common value", here girx.
// PREVENTS: the receive half of the same defect. Before the fix crypto/ecdh was handed
// the peer's bare X || Y and rejected it as an invalid public key, so Ze could not
// complete an ECP exchange with any conforming peer in either direction.
func TestECPSharedSecretFromPeerWireValue(t *testing.T) {
	cases := []struct {
		name    string
		curve   ecdh.Curve
		group   DHGroupID
		private string
		grx     string
		gry     string
		girx    string
	}{
		{"group 19 ECP-256", ecdh.P256(), DH_ECP_256, rfc5903P256PrivateI, rfc5903P256Grx, rfc5903P256Gry, rfc5903P256Girx},
		{"group 20 ECP-384", ecdh.P384(), DH_ECP_384, rfc5903P384PrivateI, rfc5903P384Grx, rfc5903P384Gry, rfc5903P384Girx},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			priv, err := c.curve.NewPrivateKey(rfcHex(t, c.private))
			if err != nil {
				t.Fatalf("NewPrivateKey from RFC 5903 vector: %v", err)
			}
			ex := &DHExchange{GroupID: c.group, privateEC: priv}

			peerWire := append(rfcHex(t, c.grx), rfcHex(t, c.gry)...)
			secret, err := ex.SharedSecret(peerWire)
			if err != nil {
				t.Fatalf("SharedSecret(%d-octet peer value): %v", len(peerWire), err)
			}
			if want := rfcHex(t, c.girx); !bytes.Equal(secret, want) {
				t.Errorf("shared secret = %x\nwant (RFC 5903 girx) = %x", secret, want)
			}
		})
	}
}

// TestECPRoundTripWithForeignPeer exchanges with a peer built straight on crypto/ecdh
// that speaks the RFC wire form, so neither side's encoding can hide the other's.
//
// VALIDATES: both halves agree on RFC 5903 Section 7 for a freshly generated key, not
// only for the RFC's fixed vector.
// PREVENTS: a symmetric re-break. The pre-existing ECP tests ran Ze against Ze, so a
// consistent off-by-one octet stayed invisible; this peer never uses Ze's encoder.
func TestECPRoundTripWithForeignPeer(t *testing.T) {
	cases := []struct {
		name  string
		curve ecdh.Curve
		group DHGroupID
		width int
	}{
		{"group 19 ECP-256", ecdh.P256(), DH_ECP_256, 64},
		{"group 20 ECP-384", ecdh.P384(), DH_ECP_384, 96},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ze, err := NewDHExchange(c.group)
			if err != nil {
				t.Fatalf("NewDHExchange: %v", err)
			}
			defer ze.Clear()

			peer, err := c.curve.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatalf("peer GenerateKey: %v", err)
			}
			// The peer sends what RFC 5903 Section 7 prescribes: the tag octet dropped.
			peerWire := peer.PublicKey().Bytes()[1:]
			if len(peerWire) != c.width {
				t.Fatalf("peer wire value = %d octets, want %d", len(peerWire), c.width)
			}

			zeSecret, err := ze.SharedSecret(peerWire)
			if err != nil {
				t.Fatalf("Ze SharedSecret from peer wire value: %v", err)
			}

			// The peer reconstructs Ze's point the same way any conforming stack would.
			zePoint, err := c.curve.NewPublicKey(append([]byte{sec1UncompressedTag}, ze.PublicKey...))
			if err != nil {
				t.Fatalf("peer could not parse Ze's KE payload: %v", err)
			}
			peerSecret, err := peer.ECDH(zePoint)
			if err != nil {
				t.Fatalf("peer ECDH: %v", err)
			}

			if !bytes.Equal(zeSecret, peerSecret) {
				t.Errorf("shared secrets differ:\n  ze   = %x\n  peer = %x", zeSecret, peerSecret)
			}
		})
	}
}

// TestECPRejectsNonConformingLength refuses every width but the group's own.
//
// VALIDATES: RFC 5903 Section 7 defines one encoding for an ECP public value, so a value
// of another length is refused rather than reshaped. The SEC 1 tagged form is included
// deliberately: Ze does not widen its accepted set past the standard's, because tolerating
// it would complete an exchange no conforming peer would and would hide the sender's bug.
// PREVENTS: truncating, padding, or silently sniffing the encoding of a value that feeds
// key derivation.
func TestECPRejectsNonConformingLength(t *testing.T) {
	cases := []struct {
		group DHGroupID
		width int
		other int
	}{
		{DH_ECP_256, 64, 96},
		{DH_ECP_384, 96, 64},
	}
	for _, c := range cases {
		t.Run(c.group.String(), func(t *testing.T) {
			ex, err := NewDHExchange(c.group)
			if err != nil {
				t.Fatalf("NewDHExchange: %v", err)
			}
			defer ex.Clear()

			// A genuine point of this group, in the SEC 1 tagged form Ze must refuse.
			tagged := append([]byte{sec1UncompressedTag}, ex.PublicKey...)

			bad := []struct {
				name  string
				value []byte
			}{
				{"empty", nil},
				{"one octet short", make([]byte, c.width-1)},
				{"one octet long", make([]byte, c.width+1)},
				{"the SEC 1 uncompressed encoding of a real point", tagged},
				{"the other ECP group's width", make([]byte, c.other)},
			}
			for _, b := range bad {
				secret, err := ex.SharedSecret(b.value)
				if !errors.Is(err, ErrPublicKeyLength) {
					t.Errorf("%s (%d octets): error = %v, want ErrPublicKeyLength",
						b.name, len(b.value), err)
				}
				if !errors.Is(err, ErrInvalidPublicKey) {
					t.Errorf("%s: error does not wrap ErrInvalidPublicKey", b.name)
				}
				if secret != nil {
					t.Errorf("%s: secret = %x, want nil", b.name, secret)
				}
			}
		})
	}
}

// TestECPRejectsRightLengthWrongPoint keeps the length check from becoming the only check.
//
// VALIDATES: a value of the correct width that is not on the curve is still refused.
// PREVENTS: a fix that pads or trusts any 64 octets, which would hand an invalid point
// to key derivation.
func TestECPRejectsRightLengthWrongPoint(t *testing.T) {
	for _, group := range []DHGroupID{DH_ECP_256, DH_ECP_384} {
		t.Run(group.String(), func(t *testing.T) {
			ex, err := NewDHExchange(group)
			if err != nil {
				t.Fatalf("NewDHExchange: %v", err)
			}
			defer ex.Clear()

			width, ok := ecpPublicLen(group)
			if !ok {
				t.Fatalf("ecpPublicLen(%v) not known", group)
			}
			offCurve := make([]byte, width)
			for i := range offCurve {
				offCurve[i] = 0xAA
			}
			secret, err := ex.SharedSecret(offCurve)
			if !errors.Is(err, ErrInvalidPublicKey) {
				t.Errorf("off-curve value: error = %v, want ErrInvalidPublicKey", err)
			}
			if secret != nil {
				t.Errorf("off-curve value: secret = %x, want nil", secret)
			}
		})
	}
}
