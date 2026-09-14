// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- the AES CCM mode the IKEv2 Encrypted payload takes
// Related: internal/component/ike/crypto/aead.go -- the IKEv2 transform that keys this mode

// Package ccm implements Counter with CBC-MAC, the authenticated encryption block
// cipher mode of RFC 3610 and NIST SP 800-38C.
//
// The Go standard library carries GCM and no CCM, and RFC 5282 Section 3.2 obliges an
// IKEv2 implementation that negotiates AES CCM to produce its ICV, so the mode is
// written here. The package is a leaf: it holds no config, registers nothing and keeps
// no state beyond the block cipher a caller hands it.
//
// CCM is two passes over one key. CBC-MAC authenticates the nonce, the additional data
// and the message; CTR mode then encrypts the message and the MAC. RFC 3610 Section 2
// names the two parameters: M, the octets of the authentication field, and L, the
// octets of the length field. The nonce is 15-L octets, so New takes the nonce length
// and derives L from it.
package ccm

import (
	"crypto/cipher"
	"crypto/subtle"
	"encoding/binary"
	"errors"
)

// blockOctets is the block size CCM is defined for. RFC 3610 Section 1: "CCM is only
// defined for use with 128-bit block ciphers, such as AES." So this is a constant
// rather than a parameter: a cipher with another block size is out of scope.
const blockOctets = 16

var (
	// ErrBlockSize refuses a block cipher CCM is not defined for.
	ErrBlockSize = errors.New("ccm: the block cipher is not 128 bits wide")
	// ErrTagOctets refuses an authentication field length RFC 3610 Section 2 does not
	// list: "Valid values are 4, 6, 8, 10, 12, 14, and 16 octets."
	ErrTagOctets = errors.New("ccm: the authentication field must be 4, 6, 8, 10, 12, 14 or 16 octets")
	// ErrNonceOctets refuses a nonce length outside the range L allows. RFC 3610
	// Section 2: "Valid values of L range between 2 octets and 8 octets (the value L=1
	// is reserved)", and Section 2.1 gives the nonce 15-L octets.
	ErrNonceOctets = errors.New("ccm: the nonce must be 7 to 13 octets")
	// ErrOpen is the only thing Open answers for a message that does not authenticate.
	// RFC 3610 Section 2.5: "If the T value is not correct, the receiver MUST NOT
	// reveal any information except for the fact that T is incorrect." One error for
	// every failure is what keeps that promise.
	ErrOpen = errors.New("ccm: message authentication failed")
)

// mode is one CCM instance: a block cipher with the M and L of RFC 3610 Section 2
// fixed. Safe for concurrent use, because it holds no state between calls and
// cipher.Block is itself safe for concurrent use.
type mode struct {
	block       cipher.Block
	tagOctets   int // M, the octets of the authentication field.
	nonceOctets int // 15-L, so the length field L follows from it.
}

// New returns a CCM AEAD over block with an authentication field of tagOctets and a
// nonce of nonceOctets. It refuses every parameter RFC 3610 Section 2 does not allow
// rather than clamping one, because a mode keyed at a length the peer does not use
// produces a tag that never verifies and says nothing about why.
func New(block cipher.Block, tagOctets, nonceOctets int) (cipher.AEAD, error) {
	if block.BlockSize() != blockOctets {
		return nil, ErrBlockSize
	}
	// RFC 3610 Section 2: "The first choice is M, the size of the authentication
	// field. ... Valid values are 4, 6, 8, 10, 12, 14, and 16 octets."
	if tagOctets < 4 || tagOctets > 16 || tagOctets%2 != 0 {
		return nil, ErrTagOctets
	}
	// RFC 3610 Section 2.1: "A nonce N of 15-L octets", with L from 2 to 8.
	if nonceOctets < blockOctets-1-8 || nonceOctets > blockOctets-1-2 {
		return nil, ErrNonceOctets
	}
	return &mode{block: block, tagOctets: tagOctets, nonceOctets: nonceOctets}, nil
}

// NonceSize answers the nonce length this instance takes, in octets.
func (m *mode) NonceSize() int { return m.nonceOctets }

// Overhead answers the octets Seal appends to the plaintext, which is M.
func (m *mode) Overhead() int { return m.tagOctets }

// lengthOctets is L, the octets the counter block reserves for the message length.
func (m *mode) lengthOctets() int { return blockOctets - 1 - m.nonceOctets }

// messageFits reports whether a message of octets can be encoded in the length field.
// RFC 3610 Section 2.1: "The message m, consisting of a string of l(m) octets where 0
// <= l(m) < 2^(8L)." A longer message has no encoding, so it is refused here rather
// than truncated into the length field.
func (m *mode) messageFits(octets int) bool {
	if m.lengthOctets() >= 8 {
		return true
	}
	return uint64(octets) < uint64(1)<<(8*m.lengthOctets())
}

// Seal encrypts and authenticates plaintext, appends the result to dst and answers the
// extended slice. dst MUST NOT overlap plaintext.
//
// The nonce MUST be NonceSize() octets and MUST be unique for this key. RFC 3610
// Section 2.1: "Within the scope of any encryption key K, the nonce value MUST be
// unique." Both lengths are the caller's invariant rather than peer input, so a wrong
// one is a Ze defect and panics.
func (m *mode) Seal(dst, nonce, plaintext, additionalData []byte) []byte {
	if len(nonce) != m.nonceOctets {
		panic("BUG: ccm: Seal was given a nonce of the wrong length")
	}
	if !m.messageFits(len(plaintext)) {
		panic("BUG: ccm: Seal was given a message longer than the length field can encode")
	}

	tag := m.tag(nonce, plaintext, additionalData)

	head, out := sliceForAppend(dst, len(plaintext)+m.tagOctets)

	// RFC 3610 Section 2.3: "The message is encrypted by XORing the octets of message m
	// with the first l(m) octets of the concatenation of S_1, S_2, S_3, ... . Note that
	// S_0 is not used to encrypt the message."
	var first, start [blockOctets]byte
	m.counterBlock(&first, nonce, 0)
	m.block.Encrypt(first[:], first[:])
	m.counterBlock(&start, nonce, 1)
	cipher.NewCTR(m.block, start[:]).XORKeyStream(out, plaintext)

	// RFC 3610 Section 2.3: "U := T XOR first-M-bytes( S_0 )".
	for i := range m.tagOctets {
		out[len(plaintext)+i] = tag[i] ^ first[i]
	}
	return head
}

// Open authenticates and decrypts ciphertext, appends the plaintext to dst and answers
// the extended slice. It answers ErrOpen and no plaintext when the message does not
// authenticate. dst MUST NOT overlap ciphertext.
//
// The nonce MUST be NonceSize() octets, for the reason Seal states.
func (m *mode) Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if len(nonce) != m.nonceOctets {
		panic("BUG: ccm: Open was given a nonce of the wrong length")
	}
	if len(ciphertext) < m.tagOctets {
		return nil, ErrOpen
	}
	messageOctets := len(ciphertext) - m.tagOctets
	if !m.messageFits(messageOctets) {
		return nil, ErrOpen
	}

	var first, start [blockOctets]byte
	m.counterBlock(&first, nonce, 0)
	m.block.Encrypt(first[:], first[:])

	// The authentication field on the wire is U. RFC 3610 Section 2.3 built it as T XOR
	// the first M octets of S_0, so the same XOR recovers T.
	var want [blockOctets]byte
	for i := range m.tagOctets {
		want[i] = ciphertext[messageOctets+i] ^ first[i]
	}

	head, out := sliceForAppend(dst, messageOctets)
	m.counterBlock(&start, nonce, 1)
	cipher.NewCTR(m.block, start[:]).XORKeyStream(out, ciphertext[:messageOctets])

	tag := m.tag(nonce, out, additionalData)
	if subtle.ConstantTimeCompare(tag[:m.tagOctets], want[:m.tagOctets]) != 1 {
		// RFC 3610 Section 2.5: "The receiver MUST NOT reveal the decrypted message,
		// the value T, or any other information."
		clear(out)
		return nil, ErrOpen
	}
	return head, nil
}

// counterBlock writes the block A_i of RFC 3610 Section 2.3: the flags octet, the
// nonce, then the counter in the last L octets, most significant octet first.
//
// The flags octet of an A block is L' alone. RFC 3610 Section 2.3: "Bits 3, 4, and 5
// are also set to zero, ensuring that all the A blocks are distinct from B_0, which has
// the non-zero encoding of M in this position." That distinctness is what stops a
// counter block colliding with the authentication block.
func (m *mode) counterBlock(out *[blockOctets]byte, nonce []byte, counter uint64) {
	clear(out[:])
	out[0] = byte(m.lengthOctets() - 1)
	copy(out[1:], nonce)
	putTrailingUint(out, m.lengthOctets(), counter)
}

// tag computes the CBC-MAC value T of RFC 3610 Section 2.2 over the first block, the
// additional data and the message. Only the first M octets are used.
func (m *mode) tag(nonce, plaintext, additionalData []byte) [blockOctets]byte {
	var mac cbcMAC
	mac.block = m.block

	// RFC 3610 Section 2.2: "Flags = 64*Adata + 8*M' + L'", where "The M' field is set
	// to (M-2)/2" and "L' = L-1", and "The Adata bit is set to zero if l(a)=0, and set
	// to one if l(a)>0."
	var b0 [blockOctets]byte
	b0[0] = byte((m.tagOctets-2)/2)<<3 | byte(m.lengthOctets()-1)
	if len(additionalData) > 0 {
		b0[0] |= 0x40
	}
	copy(b0[1:], nonce)
	putTrailingUint(&b0, m.lengthOctets(), uint64(len(plaintext)))
	mac.writeBlock(&b0)

	// RFC 3610 Section 2.2: "The blocks encoding a are formed by concatenating this
	// string that encodes l(a) with a itself, and splitting the result into 16-octet
	// blocks, and then padding the last block with zeroes if necessary."
	if len(additionalData) > 0 {
		var prefix [10]byte
		mac.write(additionalDataLength(&prefix, len(additionalData)))
		mac.write(additionalData)
		mac.pad()
	}

	// RFC 3610 Section 2.2: "If the message m consists of the empty string, then no
	// blocks are added in this step." pad adds nothing for an empty buffer.
	mac.write(plaintext)
	mac.pad()
	return mac.x
}

// additionalDataLength writes the encoding of l(a) into prefix and answers the octets
// it used. RFC 3610 Section 2.2 gives three forms, chosen by the length.
func additionalDataLength(prefix *[10]byte, octets int) []byte {
	// "If 0 < l(a) < (2^16 - 2^8), then the length field is encoded as two octets".
	if octets < 0xFF00 {
		binary.BigEndian.PutUint16(prefix[0:2], uint16(octets))
		return prefix[0:2]
	}
	// "If (2^16 - 2^8) <= l(a) < 2^32, then the length field is encoded as six octets
	// consisting of the octets 0xff, 0xfe, and four octets encoding l(a)".
	if uint64(octets) < 1<<32 {
		prefix[0], prefix[1] = 0xFF, 0xFE
		binary.BigEndian.PutUint32(prefix[2:6], uint32(octets))
		return prefix[0:6]
	}
	// "If 2^32 <= l(a) < 2^64, then the length field is encoded as ten octets
	// consisting of the octets 0xff, 0xff, and eight octets encoding l(a)".
	prefix[0], prefix[1] = 0xFF, 0xFF
	binary.BigEndian.PutUint64(prefix[2:10], uint64(octets))
	return prefix[0:10]
}

// putTrailingUint writes value into the last octets of block, most significant octet
// first. RFC 3610 Section 2.2 encodes both l(m) and the counter that way.
func putTrailingUint(block *[blockOctets]byte, octets int, value uint64) {
	for i := range octets {
		block[blockOctets-1-i] = byte(value >> (8 * i))
	}
}

// cbcMAC is the CBC-MAC of RFC 3610 Section 2.2: "X_1 := E( K, B_0 )" and "X_i+1 := E(
// K, X_i XOR B_i )". The state starts at zero, so the first block reproduces X_1.
type cbcMAC struct {
	block   cipher.Block
	x       [blockOctets]byte
	partial [blockOctets]byte
	held    int
}

func (c *cbcMAC) writeBlock(b *[blockOctets]byte) {
	for i := range b {
		c.x[i] ^= b[i]
	}
	c.block.Encrypt(c.x[:], c.x[:])
}

// write buffers data and processes every whole block it completes.
func (c *cbcMAC) write(data []byte) {
	for len(data) > 0 {
		copied := copy(c.partial[c.held:], data)
		c.held += copied
		data = data[copied:]
		if c.held == blockOctets {
			c.writeBlock(&c.partial)
			c.held = 0
		}
	}
}

// pad processes the held octets with zero padding. It does nothing when none are held,
// which is what keeps an empty message from adding a block.
func (c *cbcMAC) pad() {
	if c.held == 0 {
		return
	}
	clear(c.partial[c.held:])
	c.writeBlock(&c.partial)
	c.held = 0
}

// sliceForAppend extends in by octets and answers the whole slice with the extension.
// It is the standard library's own helper for an AEAD, restated because crypto/cipher
// keeps it unexported.
func sliceForAppend(in []byte, octets int) (head, tail []byte) {
	if total := len(in) + octets; cap(in) >= total {
		head = in[:total]
	} else {
		head = make([]byte, total)
		copy(head, in)
	}
	return head, head[len(in):]
}
