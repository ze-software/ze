// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: shacrypt_test.go -- the specification's published test vectors

// Package shacrypt verifies a password against a SHA-crypt hash: the "$5$"
// (SHA-256) and "$6$" (SHA-512) formats crypt(3) writes into a shadow password
// file. It follows Ulrich Drepper's specification "Unix crypt using SHA-256 and
// SHA-512". Ze never writes this format: a password Ze hashes itself is bcrypt.
// The package exists so an account copied from another system keeps the
// password its user already has.
//
// It verifies and does not generate, so it never clamps a rounds value or
// truncates a salt the way crypt(3) does when it makes a hash. A stored hash is
// what crypt(3) printed, and crypt(3) prints the clamped rounds and the
// truncated salt, so a stored value outside those bounds was not written by it
// and Parse refuses it.
//
// The package imports the standard library only, so any tier may use it.
package shacrypt

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

// The specification's bounds. "The default number of rounds for both
// algorithms is 5,000." and "minimum for N = 1,000", "maximum for N =
// 999,999,999". "the SALT string can be a simple string of which up to 16
// characters are used."
const (
	roundsDefault = 5000
	roundsMin     = 1000
	roundsMax     = 999999999
	roundsDigits  = 9
	saltLenMax    = 16
	roundsPrefix  = "rounds="
)

// passwordLenMax bounds the password Matches hashes. The specification hashes
// the whole password once for each of its bytes (the P sequence), so the work
// grows with the square of the length, and the length is chosen by whoever
// attempts a login. No real password approaches it.
const passwordLenMax = 1024

// itoa64 is the crypt(3) base-64 alphabet, which differs from RFC 4648 in both
// its characters and their order.
const itoa64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// ErrNotSHACrypt reports a value that does not start with a SHA-crypt
// identifier ("$5$" or "$6$"). A caller that accepts several hash formats reads
// it as "try the next format". Any other Parse error means the value names
// SHA-crypt and is malformed.
var ErrNotSHACrypt = errors.New("not a SHA-crypt hash")

// noByte marks a position of a digest group that encodes a zero byte, which
// the specification writes as the literal 0 in its last b64_from_24bit call.
const noByte = -1

// group is one b64_from_24bit call of the specification: three digest byte
// positions (most significant first) and the number of characters it writes.
type group struct {
	high, middle, low int
	characters        int
}

// algorithm is one of the two SHA-crypt variants. They share every step and
// differ only in the hash function and in the order the final digest bytes are
// encoded.
type algorithm struct {
	identifier string
	newHash    func() hash.Hash
	digestLen  int
	groups     []group
}

// sha256Crypt encodes in the order of the specification's sha256crypt.c.
var sha256Crypt = algorithm{
	identifier: "$5$",
	newHash:    sha256.New,
	digestLen:  43,
	groups: []group{
		{0, 10, 20, 4}, {21, 1, 11, 4}, {12, 22, 2, 4}, {3, 13, 23, 4},
		{24, 4, 14, 4}, {15, 25, 5, 4}, {6, 16, 26, 4}, {27, 7, 17, 4},
		{18, 28, 8, 4}, {9, 19, 29, 4}, {noByte, 31, 30, 3},
	},
}

// sha512Crypt encodes in the order of the specification's sha512crypt.c.
var sha512Crypt = algorithm{
	identifier: "$6$",
	newHash:    sha512.New,
	digestLen:  86,
	groups: []group{
		{0, 21, 42, 4}, {22, 43, 1, 4}, {44, 2, 23, 4}, {3, 24, 45, 4},
		{25, 46, 4, 4}, {47, 5, 26, 4}, {6, 27, 48, 4}, {28, 49, 7, 4},
		{50, 8, 29, 4}, {9, 30, 51, 4}, {31, 52, 10, 4}, {53, 11, 32, 4},
		{12, 33, 54, 4}, {34, 55, 13, 4}, {56, 14, 35, 4}, {15, 36, 57, 4},
		{37, 58, 16, 4}, {59, 17, 38, 4}, {18, 39, 60, 4}, {40, 61, 19, 4},
		{62, 20, 41, 4}, {noByte, noByte, 63, 2},
	},
}

// Hash is a parsed SHA-crypt hash. Only Parse builds one, and the zero Hash
// matches no password.
type Hash struct {
	variant *algorithm
	rounds  int
	salt    string
	digest  string
}

// IsHash reports whether s is a well-formed SHA-crypt hash.
func IsHash(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// Parse reads a stored hash of the form $5$[rounds=N$]salt$digest or
// $6$[rounds=N$]salt$digest.
func Parse(encoded string) (Hash, error) {
	variant, rest, err := variantOf(encoded)
	if err != nil {
		return Hash{}, err
	}
	rounds := roundsDefault
	// "If the SALT strings starts with rounds=<N>$ where N is an unsigned
	// decimal number the numeric value of N is used to modify the algorithm
	// used."
	if after, found := strings.CutPrefix(rest, roundsPrefix); found {
		value, remainder, separated := strings.Cut(after, "$")
		if !separated {
			return Hash{}, errors.New("SHA-crypt hash: rounds=<N> is not followed by $")
		}
		rounds, err = parseRounds(value)
		if err != nil {
			return Hash{}, err
		}
		rest = remainder
	}
	salt, digest, separated := strings.Cut(rest, "$")
	if !separated {
		return Hash{}, errors.New("SHA-crypt hash: no $ between the salt and the digest")
	}
	if len(salt) > saltLenMax {
		return Hash{}, fmt.Errorf("SHA-crypt hash: the salt has %d characters, and the most is %d", len(salt), saltLenMax)
	}
	if len(digest) != variant.digestLen {
		return Hash{}, fmt.Errorf("SHA-crypt hash: the digest has %d characters, and %s needs %d", len(digest), variant.identifier, variant.digestLen)
	}
	if strings.Trim(digest, itoa64) != "" {
		return Hash{}, errors.New("SHA-crypt hash: the digest holds a character outside the crypt alphabet")
	}
	return Hash{variant: variant, rounds: rounds, salt: salt, digest: digest}, nil
}

// variantOf returns the algorithm the identifier names and the text after it.
func variantOf(encoded string) (*algorithm, string, error) {
	if rest, found := strings.CutPrefix(encoded, sha512Crypt.identifier); found {
		return &sha512Crypt, rest, nil
	}
	if rest, found := strings.CutPrefix(encoded, sha256Crypt.identifier); found {
		return &sha256Crypt, rest, nil
	}
	return nil, "", ErrNotSHACrypt
}

// parseRounds reads N of rounds=N. It accepts only the canonical decimal form
// crypt(3) prints: digits, no sign, no leading zero, inside the bounds.
func parseRounds(value string) (int, error) {
	if value == "" {
		return 0, errors.New("SHA-crypt hash: rounds= carries no number")
	}
	if len(value) > roundsDigits {
		return 0, fmt.Errorf("SHA-crypt hash: rounds=%s is above the maximum %d", value, roundsMax)
	}
	if strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("SHA-crypt hash: rounds=%s is not a decimal number", value)
	}
	if value[0] == '0' {
		return 0, fmt.Errorf("SHA-crypt hash: rounds=%s has a leading zero", value)
	}
	rounds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("SHA-crypt hash: rounds=%s: %w", value, err)
	}
	// "Any selection of N below the minimum will cause the use of 1,000 rounds
	// and a value of 1 billion and higher will cause 999,999,999 rounds to be
	// used." crypt(3) prints the value it used, so a stored hash carries one
	// inside the bounds.
	if rounds < roundsMin {
		return 0, fmt.Errorf("SHA-crypt hash: rounds=%d is below the minimum %d", rounds, roundsMin)
	}
	return rounds, nil
}

// Matches reports whether password produces this hash. The digest comparison
// takes constant time. A password longer than passwordLenMax matches nothing.
func (h Hash) Matches(password []byte) bool {
	// The zero Hash has no variant: it was not built by Parse, so it matches
	// nothing rather than dereferencing nil.
	if h.variant == nil {
		return false
	}
	if len(password) > passwordLenMax {
		return false
	}
	computed := h.variant.digestOf(password, []byte(h.salt), h.rounds)
	return subtle.ConstantTimeCompare(computed, []byte(h.digest)) == 1
}

// digestOf runs the specification's algorithm and returns the encoded digest.
// The step comments name the numbered steps of the specification's "The
// algorithm" section.
func (a *algorithm) digestOf(key, salt []byte, rounds int) []byte {
	// Steps 4 to 8: digest B is the hash of key, salt, key.
	bHash := a.newHash()
	write(bHash, key)
	write(bHash, salt)
	write(bHash, key)
	bDigest := bHash.Sum(nil)

	// Steps 1 to 3 and 9 to 12: digest A. Steps 9 and 10 add B for each block of
	// the key and its first bytes for the rest, and step 11 walks the bits of
	// the key length.
	aHash := a.newHash()
	write(aHash, key)
	write(aHash, salt)
	write(aHash, repeatTo(bDigest, len(key)))
	for length := len(key); length > 0; length >>= 1 {
		if length&1 != 0 {
			write(aHash, bDigest)
		} else {
			write(aHash, key)
		}
	}
	result := aHash.Sum(nil)

	// Steps 13 to 16: the P sequence, from the key repeated once per key byte.
	pDigest := a.newHash()
	for range len(key) {
		write(pDigest, key)
	}
	pSequence := repeatTo(pDigest.Sum(nil), len(key))

	// Steps 17 to 20: the S sequence, from the salt repeated 16 + A[0] times.
	sDigest := a.newHash()
	for range 16 + int(result[0]) {
		write(sDigest, salt)
	}
	sSequence := repeatTo(sDigest.Sum(nil), len(salt))

	// Step 21: the rounds. The loop is bounded by rounds, which Parse holds
	// inside roundsMin and roundsMax.
	round := a.newHash()
	for count := range rounds {
		round.Reset()
		if count&1 != 0 {
			write(round, pSequence)
		} else {
			write(round, result)
		}
		if count%3 != 0 {
			write(round, sSequence)
		}
		if count%7 != 0 {
			write(round, pSequence)
		}
		if count&1 != 0 {
			write(round, result)
		} else {
			write(round, pSequence)
		}
		// Write copied result into the hash state, so Sum may reuse its array.
		result = round.Sum(result[:0])
	}
	return a.encode(result)
}

// encode writes the final digest in the crypt(3) base-64 alphabet, three bytes
// to four characters, in the byte order of the variant's groups.
func (a *algorithm) encode(sum []byte) []byte {
	out := make([]byte, 0, a.digestLen)
	for _, g := range a.groups {
		word := uint(byteAt(sum, g.high))<<16 | uint(byteAt(sum, g.middle))<<8 | uint(byteAt(sum, g.low))
		for range g.characters {
			out = append(out, itoa64[word&0x3f])
			word >>= 6
		}
	}
	return out
}

// byteAt returns sum[position], or zero for noByte.
func byteAt(sum []byte, position int) byte {
	if position == noByte {
		return 0
	}
	return sum[position]
}

// repeatTo returns block repeated until it is length bytes long, the last copy
// cut short. It is the shape of the specification's steps 9 and 10, 16 and 20.
func repeatTo(block []byte, length int) []byte {
	out := make([]byte, 0, length)
	for len(out) < length {
		out = append(out, block[:min(len(block), length-len(out))]...)
	}
	return out
}

// write adds b to h.
func write(h hash.Hash, b []byte) {
	h.Write(b) //nolint:errcheck // hash.Hash documents that Write never returns an error
}
