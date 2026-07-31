// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- DH key exchange groups
// RFC: rfc/short/rfc7296.md -- Key Exchange payload, public value length (Section 3.4)
// RFC: rfc/full/rfc5903.txt -- ECP public value encoding, X || Y (Section 7); no short summary exists

package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

var (
	ErrInvalidPublicKey = errors.New("invalid DH public key")
	ErrUnsupportedGroup = errors.New("unsupported DH group")
	// ErrPublicKeyLength reports a peer Diffie-Hellman value whose length is not the
	// length of the group modulus. RFC 7296 Section 3.4 makes that pad an obligation of
	// the sender. A value of another length therefore names another group, or a peer
	// that did not pad. Both cases end in a secret the two sides do not share, so the
	// exponentiation never runs. It wraps ErrInvalidPublicKey. A value of the wrong
	// length is an invalid public key, so a caller that tests for the general case
	// still matches this one.
	ErrPublicKeyLength = fmt.Errorf("%w: length does not match the group modulus", ErrInvalidPublicKey)
)

type DHExchange struct {
	GroupID    DHGroupID
	PublicKey  []byte
	privateEC  *ecdh.PrivateKey
	privateBig *big.Int
}

// RFC 3526 Section 3: MODP 2048-bit group (group 14).
var modp2048Prime = func() *big.Int {
	p, _ := new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1"+
			"29024E088A67CC74020BBEA63B139B22514A08798E3404DD"+
			"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245"+
			"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D"+
			"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F"+
			"83655D23DCA3AD961C62F356208552BB9ED529077096966D"+
			"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9"+
			"DE2BCBF6955817183995497CEA956AE515D2261898FA0510"+
			"15728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	return p
}()

var modp2048Generator = big.NewInt(2)

// modp2048Len is the octet length of the MODP-2048 prime modulus. RFC 7296 Section 3.4
// fixes every public value of the group at this length. It is derived from the modulus
// rather than written down, so the two cannot drift apart.
var modp2048Len = len(modp2048Prime.Bytes())

// sec1UncompressedTag is the leading octet SEC 1 Version 2.0 Section 2.3.3 puts in
// front of an uncompressed elliptic curve point. Go's crypto/ecdh speaks that
// encoding; an IKEv2 KE payload does not carry it.
const sec1UncompressedTag = 0x04

// ecpCoordBits transcribes the component bit length table of RFC 5903 Section 7:
//
//	Diffie-Hellman group                component bit length
//	------------------------            --------------------
//	256-bit Random ECP Group                   256
//	384-bit Random ECP Group                   384
//	521-bit Random ECP Group                   528
//
// Only the groups Ze offers appear here. The public value length is derived from
// this table rather than written down, so a group added later cannot carry a stale
// constant.
var ecpCoordBits = map[DHGroupID]int{
	DH_ECP_256: 256,
	DH_ECP_384: 384,
}

// ecpPublicLen is the octet length of an ECP group's Diffie-Hellman public value as
// it travels in a KE payload. RFC 5903 Section 7: "The Diffie-Hellman public value
// is obtained by concatenating the x and y values", each component padded to the bit
// length above. So the value is two coordinates wide and carries no SEC 1 tag octet:
// 64 octets for group 19, 96 for group 20.
func ecpPublicLen(groupID DHGroupID) (int, bool) {
	bits, ok := ecpCoordBits[groupID]
	if !ok {
		return 0, false
	}
	return 2 * ((bits + 7) / 8), true
}

// ecpWireFromSEC1 converts the SEC 1 uncompressed point that crypto/ecdh produces,
// 0x04 || X || Y, into the bare X || Y that RFC 5903 Section 7 puts on the wire.
// A value that is not the expected point is refused rather than reshaped: the caller
// generated it locally, so a surprise here is a bug in Ze, not a peer's doing.
func ecpWireFromSEC1(sec1 []byte, wireLen int) ([]byte, error) {
	if len(sec1) != wireLen+1 || sec1[0] != sec1UncompressedTag {
		return nil, ErrInvalidPublicKey
	}
	wire := make([]byte, wireLen)
	copy(wire, sec1[1:])
	return wire, nil
}

// ecpSEC1FromWire converts the bare X || Y of a peer's KE payload back into the SEC 1
// uncompressed point crypto/ecdh accepts.
//
// The length must be exactly the group's. RFC 5903 Section 7 defines one encoding for
// an ECP public value and the SEC 1 tagged form is not it, so a 65- or 97-octet value
// is refused with the same error as any other wrong length. Ze does not widen its
// accepted set past the standard's: strongSwan refuses the tagged form too, so
// tolerating it would let Ze complete an exchange no conforming peer would, and would
// hide the sending peer's defect instead of surfacing it. Refusing is also what
// fail-closed guarding asks of a value that feeds key derivation.
func ecpSEC1FromWire(wire []byte, wireLen int) ([]byte, error) {
	if len(wire) != wireLen {
		return nil, ErrPublicKeyLength
	}
	sec1 := make([]byte, wireLen+1)
	sec1[0] = sec1UncompressedTag
	copy(sec1[1:], wire)
	return sec1, nil
}

// ecpPublicKey decodes a peer's ECP public value from its KE payload wire form.
func ecpPublicKey(curve ecdh.Curve, groupID DHGroupID, remotePublic []byte) (*ecdh.PublicKey, error) {
	wireLen, ok := ecpPublicLen(groupID)
	if !ok {
		return nil, ErrUnsupportedGroup
	}
	sec1, err := ecpSEC1FromWire(remotePublic, wireLen)
	if err != nil {
		return nil, err
	}
	pub, err := curve.NewPublicKey(sec1)
	if err != nil {
		return nil, ErrInvalidPublicKey
	}
	return pub, nil
}

// ecpExchangePublic records the locally generated ECP public value on ex in KE payload
// wire form.
func ecpExchangePublic(ex *DHExchange, priv *ecdh.PrivateKey) error {
	wireLen, ok := ecpPublicLen(ex.GroupID)
	if !ok {
		return ErrUnsupportedGroup
	}
	public, err := ecpWireFromSEC1(priv.PublicKey().Bytes(), wireLen)
	if err != nil {
		return err
	}
	ex.privateEC = priv
	ex.PublicKey = public
	return nil
}

func NewDHExchange(groupID DHGroupID) (*DHExchange, error) {
	ex := &DHExchange{GroupID: groupID}
	switch groupID {
	case DH_MODP_2048:
		two := big.NewInt(2)
		pMinus2 := new(big.Int).Sub(modp2048Prime, two)
		priv, err := rand.Int(rand.Reader, pMinus2)
		if err != nil {
			return nil, err
		}
		priv.Add(priv, two)
		ex.privateBig = priv
		pub := new(big.Int).Exp(modp2048Generator, priv, modp2048Prime)
		ex.PublicKey = padBigInt(pub, modp2048Len)
	case DH_ECP_256:
		priv, err := ecdh.P256().GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := ecpExchangePublic(ex, priv); err != nil {
			return nil, err
		}
	case DH_ECP_384:
		priv, err := ecdh.P384().GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := ecpExchangePublic(ex, priv); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedGroup
	}
	return ex, nil
}

func (ex *DHExchange) SharedSecret(remotePublic []byte) ([]byte, error) {
	switch ex.GroupID {
	case DH_MODP_2048:
		// RFC 7296 Section 3.4: a MODP public value has the length of the prime
		// modulus, and the sender prepends zero octets to reach it. A value of another
		// length is refused before the exponentiation. Two cases reach here. The peer
		// did not pad, or the value belongs to another group. Neither ends in a secret
		// the two sides share, so the refusal is the only safe answer.
		if len(remotePublic) != modp2048Len {
			return nil, ErrPublicKeyLength
		}
		remotePub := new(big.Int).SetBytes(remotePublic)
		one := big.NewInt(1)
		pMinusOne := new(big.Int).Sub(modp2048Prime, one)
		if remotePub.Cmp(one) <= 0 || remotePub.Cmp(pMinusOne) >= 0 {
			return nil, ErrInvalidPublicKey
		}
		shared := new(big.Int).Exp(remotePub, ex.privateBig, modp2048Prime)
		return padBigInt(shared, modp2048Len), nil
	case DH_ECP_256:
		// RFC 5903 Section 7: the peer's value is the bare X || Y of its curve point.
		// crypto/ecdh wants that same point in SEC 1 uncompressed form, so the tag
		// octet is restored here and nowhere else. The shared secret it returns is the
		// x coordinate alone, which is what Section 7 asks for.
		remotePub, err := ecpPublicKey(ecdh.P256(), ex.GroupID, remotePublic)
		if err != nil {
			return nil, err
		}
		return ex.privateEC.ECDH(remotePub)
	case DH_ECP_384:
		remotePub, err := ecpPublicKey(ecdh.P384(), ex.GroupID, remotePublic)
		if err != nil {
			return nil, err
		}
		return ex.privateEC.ECDH(remotePub)
	default:
		return nil, ErrUnsupportedGroup
	}
}

func padBigInt(n *big.Int, size int) []byte {
	b := n.Bytes()
	if len(b) >= size {
		return b
	}
	padded := make([]byte, size)
	copy(padded[size-len(b):], b)
	return padded
}

func (ex *DHExchange) Clear() {
	if ex.privateBig != nil {
		ex.privateBig.SetInt64(0)
		ex.privateBig = nil
	}
	ex.privateEC = nil
	for i := range ex.PublicKey {
		ex.PublicKey[i] = 0
	}
}
