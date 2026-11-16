// Design: docs/architecture/l2tp/subscriber-session-model.md -- packed immutable logical session birth
package subscriberid

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// L2TPBirth names one logical L2TP session independently of its listener and
// reusable wire labels. Its private packed words are comparable map keys.
// Zero is invalid. Copies preserve identity, not admission or mutation authority;
// the original SessionAttempt/Control and receipt still govern those operations.
type L2TPBirth struct {
	high uint64
	low  uint64
}

// NewL2TPBirth mints a birth before the actual session is published. An all-zero
// draw is refused rather than publishing the invalid zero or reducing entropy.
// No global counter, interner, listener registry or reusable label contributes.
func NewL2TPBirth() (L2TPBirth, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return L2TPBirth{}, err
	}
	birth := L2TPBirth{high: binary.BigEndian.Uint64(raw[:8]), low: binary.BigEndian.Uint64(raw[8:])}
	if !birth.IsValid() {
		return L2TPBirth{}, errors.New("l2tp: random session birth is zero")
	}
	return birth, nil
}

// IsValid distinguishes the absent zero value at an unvalidated boundary.
func (birth L2TPBirth) IsValid() bool {
	if birth.high != 0 {
		return true
	}
	return birth.low != 0
}

// IsZero supports omitzero for optional birth fields on shared non-L2TP DTOs.
func (birth L2TPBirth) IsZero() bool { return !birth.IsValid() }

// ParseL2TPBirth validates the whole canonical DTO once at an external boundary.
// It never parses a TID/SID tuple, hashes a string or consults an owner registry.
func ParseL2TPBirth(text string) (L2TPBirth, error) {
	const prefix = "l2tp-"
	if len(text) != len(prefix)+32 {
		return L2TPBirth{}, errors.New("l2tp: invalid session birth length")
	}
	if text[:len(prefix)] != prefix {
		return L2TPBirth{}, errors.New("l2tp: invalid session birth prefix")
	}
	digits := text[len(prefix):]
	for _, digit := range digits {
		if digit >= '0' && digit <= '9' {
			continue
		}
		if digit >= 'a' && digit <= 'f' {
			continue
		}
		return L2TPBirth{}, errors.New("l2tp: session birth must use lowercase hexadecimal")
	}
	var raw [16]byte
	if _, err := hex.Decode(raw[:], []byte(digits)); err != nil {
		return L2TPBirth{}, err
	}
	birth := L2TPBirth{high: binary.BigEndian.Uint64(raw[:8]), low: binary.BigEndian.Uint64(raw[8:])}
	if !birth.IsValid() {
		return L2TPBirth{}, errors.New("l2tp: zero session birth is invalid")
	}
	return birth, nil
}

// String formats the external DTO or diagnostic representation. Invalid zero
// returns empty; it never mints a replacement or reconstructs wire labels.
func (birth L2TPBirth) String() string {
	if !birth.IsValid() {
		return ""
	}
	var raw [16]byte
	binary.BigEndian.PutUint64(raw[:8], birth.high)
	binary.BigEndian.PutUint64(raw[8:], birth.low)
	var text [37]byte
	copy(text[:], "l2tp-")
	hex.Encode(text[5:], raw[:])
	return string(text[:])
}

// MarshalText is a wire boundary; zero cannot be serialized as a real birth.
func (birth L2TPBirth) MarshalText() ([]byte, error) {
	if !birth.IsValid() {
		return nil, errors.New("l2tp: cannot encode absent session birth")
	}
	return []byte(birth.String()), nil
}

// UnmarshalText preserves an existing value on failure. A nil receiver is invalid.
func (birth *L2TPBirth) UnmarshalText(text []byte) error {
	if birth == nil {
		return errors.New("l2tp: nil session birth receiver")
	}
	parsed, err := ParseL2TPBirth(string(text))
	if err != nil {
		return err
	}
	*birth = parsed
	return nil
}
