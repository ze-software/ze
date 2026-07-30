// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- DH key exchange groups

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
		ex.privateEC = priv
		ex.PublicKey = priv.PublicKey().Bytes()
	case DH_ECP_384:
		priv, err := ecdh.P384().GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		ex.privateEC = priv
		ex.PublicKey = priv.PublicKey().Bytes()
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
		remotePub, err := ecdh.P256().NewPublicKey(remotePublic)
		if err != nil {
			return nil, ErrInvalidPublicKey
		}
		return ex.privateEC.ECDH(remotePub)
	case DH_ECP_384:
		remotePub, err := ecdh.P384().NewPublicKey(remotePublic)
		if err != nil {
			return nil, ErrInvalidPublicKey
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
