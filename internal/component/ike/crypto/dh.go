// Design: plan/spec-ipsec-6-ikev2-crypto.md -- DH key exchange groups

package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"math/big"
)

var (
	ErrInvalidPublicKey = errors.New("invalid DH public key")
	ErrUnsupportedGroup = errors.New("unsupported DH group")
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
		ex.PublicKey = padBigInt(pub, 256)
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
		remotePub := new(big.Int).SetBytes(remotePublic)
		one := big.NewInt(1)
		pMinusOne := new(big.Int).Sub(modp2048Prime, one)
		if remotePub.Cmp(one) <= 0 || remotePub.Cmp(pMinusOne) >= 0 {
			return nil, ErrInvalidPublicKey
		}
		shared := new(big.Int).Exp(remotePub, ex.privateBig, modp2048Prime)
		return padBigInt(shared, 256), nil
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
