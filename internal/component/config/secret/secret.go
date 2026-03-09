// Design: docs/architecture/config/syntax.md — sensitive value encoding
//
// Package secret implements JunOS-compatible $9$ reversible encoding
// for sensitive configuration values (passwords, keys).
//
// The $9$ format is an obfuscation (not encryption) using a 65-character
// alphabet with position-relative encoding. Each encode uses a random salt,
// so the same plaintext produces different encoded output each time.
// The encoded value can always be decoded back to the original plaintext.
package secret

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

// Prefix is the marker for $9$-encoded values.
const Prefix = "$9$"

// family groups define the alphabet and extra-salt-char counts.
// Family 0 chars consume 3 extra salt chars, family 1 → 2, family 2 → 1, family 3 → 0.
var family = []string{
	"QzF3n6/9CAtpu0O",
	"B1IREhcSyrleKvMW8LXx",
	"7N-dVbwsY2g4oaJZGUDj",
	"iHkq.mPf5T",
}

// encoding weights per plaintext character position (cycles mod 7).
var encoding = [][]int{
	{1, 4, 32},
	{1, 16, 32},
	{1, 8, 32},
	{1, 64},
	{1, 32},
	{1, 4, 16, 128},
	{1, 32, 64},
}

var (
	ErrEmpty          = errors.New("secret: empty input")
	ErrNoPrefix       = errors.New("secret: missing $9$ prefix")
	ErrTooShort       = errors.New("secret: not enough characters")
	ErrInvalidChar    = errors.New("secret: invalid character in encoded string")
	ErrDecodeMismatch = errors.New("secret: gap/decode length mismatch")
)

// alphabet maps: rune↔index and rune→extra count.
type dict struct {
	numAlpha map[int]rune
	alphaNum map[rune]int
	extra    map[rune]int
}

func newDict() dict {
	d := dict{
		numAlpha: make(map[int]rune),
		alphaNum: make(map[rune]int),
		extra:    make(map[rune]int),
	}
	all := strings.Join(family, "")
	for i, r := range []rune(all) {
		d.numAlpha[i] = r
		d.alphaNum[r] = i
	}
	for i, fam := range family {
		for _, c := range fam {
			d.extra[c] = 3 - i
		}
	}
	return d
}

var alpha = newDict()

// gap returns the modular distance between two characters in the alphabet.
func gap(c1, c2 rune) int {
	return pmod(alpha.alphaNum[c2]-alpha.alphaNum[c1], len(alpha.numAlpha)) - 1
}

// pmod is Python-style positive modulus.
func pmod(d, m int) int {
	res := d % m
	if (res < 0 && m > 0) || (res > 0 && m < 0) {
		return res + m
	}
	return res
}

// randomAlphaChar picks a random character from the alphabet.
func randomAlphaChar() (rune, error) {
	b := make([]byte, 1)
	for {
		if _, err := rand.Read(b); err != nil {
			return 0, fmt.Errorf("secret: random: %w", err)
		}
		idx := int(b[0]) % (len(alpha.numAlpha) + 1) // slight bias, acceptable for obfuscation
		if idx < len(alpha.numAlpha) {
			return alpha.numAlpha[idx], nil
		}
	}
}

// Encode encodes a plaintext string using JunOS $9$ reversible obfuscation.
func Encode(plain string) (string, error) {
	var b strings.Builder
	b.WriteString(Prefix)

	// Pick random salt character
	salt, err := randomAlphaChar()
	if err != nil {
		return "", err
	}
	b.WriteRune(salt)

	// Write extra salt chars for the salt character's family
	extraCount := alpha.extra[salt]
	for range extraCount {
		r, err := randomAlphaChar()
		if err != nil {
			return "", err
		}
		b.WriteRune(r)
	}

	// Encode each plaintext byte
	prev := salt
	for i, ch := range []byte(plain) {
		enc := encoding[i%len(encoding)]
		// Find alphabet characters whose weighted gaps produce ch
		chars := encodeChar(ch, enc, prev)
		for _, c := range chars {
			b.WriteRune(c)
			prev = c
		}
	}

	return b.String(), nil
}

// encodeChar finds alphabet characters whose gaps (weighted by enc) sum to target mod 256.
// Greedy decomposition: highest-weight positions first, position 0 (weight=1) absorbs remainder.
func encodeChar(target byte, enc []int, prev rune) []rune {
	alphaSize := len(alpha.numAlpha)
	maxGap := alphaSize - 2 // gap range is [0, alphaSize-2]
	n := len(enc)

	// Decompose target into gap values (highest weight first)
	gaps := make([]int, n)
	remaining := int(target)
	for i := n - 1; i >= 0; i-- {
		gaps[i] = min(remaining/enc[i], maxGap)
		remaining -= gaps[i] * enc[i]
	}

	// Convert gaps to alphabet characters (forward order, each relative to previous)
	result := make([]rune, n)
	cur := prev
	for i, g := range gaps {
		idx := pmod(alpha.alphaNum[cur]+g+1, alphaSize)
		result[i] = alpha.numAlpha[idx]
		cur = result[i]
	}

	return result
}

// Decode decodes a $9$-encoded string back to plaintext.
func Decode(encoded string) (string, error) {
	if encoded == "" {
		return "", ErrEmpty
	}
	if !strings.HasPrefix(encoded, Prefix) {
		return "", ErrNoPrefix
	}

	chars := encoded[len(Prefix):]
	runes := []rune(chars)
	if len(runes) < 1 {
		return "", ErrTooShort
	}

	// First char is salt
	saltRune := runes[0]
	extraCount, ok := alpha.extra[saltRune]
	if !ok {
		return "", ErrInvalidChar
	}
	runes = runes[1:]

	// Skip extra salt chars
	if len(runes) < extraCount {
		return "", ErrTooShort
	}
	runes = runes[extraCount:]

	// Decode body
	var plain []byte
	prev := saltRune
	pos := 0
	for len(runes) > 0 {
		enc := encoding[pos%len(encoding)]
		if len(runes) < len(enc) {
			return "", ErrTooShort
		}

		nibble := runes[:len(enc)]
		runes = runes[len(enc):]

		gaps := make([]int, len(enc))
		cur := prev
		for i, r := range nibble {
			if _, ok := alpha.alphaNum[r]; !ok {
				return "", fmt.Errorf("%w: %c", ErrInvalidChar, r)
			}
			gaps[i] = gap(cur, r)
			cur = r
		}
		prev = nibble[len(nibble)-1]

		ch, err := gapDecode(gaps, enc)
		if err != nil {
			return "", err
		}
		plain = append(plain, ch)
		pos++
	}

	return string(plain), nil
}

// gapDecode converts weighted gaps back to a plaintext byte.
func gapDecode(gaps, enc []int) (byte, error) {
	if len(gaps) != len(enc) {
		return 0, ErrDecodeMismatch
	}
	num := 0
	for i, g := range gaps {
		num += g * enc[i]
	}
	return byte(num % 256), nil
}

// IsEncoded reports whether s has the $9$ prefix.
func IsEncoded(s string) bool {
	return len(s) >= len(Prefix) && s[:len(Prefix)] == Prefix
}
