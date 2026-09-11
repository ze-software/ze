// Design: docs/architecture/wire/nlri.md -- family-specific route identity
// Related: register.go -- keys and withdrawal framing follow family registration
package nlrisplit

import (
	"errors"

	"github.com/ze-software/ze/internal/core/family"
)

// PrefixKeyScratchSize bounds normalized keys. Families with larger opaque
// NLRIs return a view of the input instead of copying it into scratch.
const PrefixKeyScratchSize = 64

var errPrefixKey = errors.New("nlrisplit: malformed route key")

// PrefixKeyFunc returns one route's identity, excluding non-key forwarding
// fields. raw excludes the ADD-PATH identifier but includes native NLRI framing.
// scratch must have PrefixKeyScratchSize bytes. The result aliases raw or scratch;
// a caller that retains it must copy it. Implementations never modify raw.
// withdraw distinguishes a labeled compatibility field from a label stack.
type PrefixKeyFunc func(raw, scratch []byte, withdraw bool) ([]byte, error)

// These maps are populated beside splitters during init, before any reader runs.
var prefixKeys = make(map[family.Family]PrefixKeyFunc)
var withdrawalSplitters = make(map[family.Family]Splitter)

// GetPrefixKey returns the registered route identity operation. The default
// preserves the whole NLRI for families whose wire fields all identify a route.
func GetPrefixKey(fam family.Family) PrefixKeyFunc {
	mu.RLock()
	defer mu.RUnlock()
	if key := prefixKeys[fam]; key != nil {
		return key
	}
	return keyOpaque
}

// GetWithdraw returns withdrawal framing. RFC 8277 Section 2.4 makes the
// three-octet compatibility field independent of the label-stack S bit.
func GetWithdraw(fam family.Family) Splitter {
	mu.RLock()
	defer mu.RUnlock()
	if split := withdrawalSplitters[fam]; split != nil {
		return split
	}
	return splitters[fam]
}

func keyOpaque(raw, _ []byte, _ bool) ([]byte, error) {
	return raw, nil
}

func keyCIDR(raw, scratch []byte, _ bool) ([]byte, error) {
	if len(raw) == 0 || len(raw) != 1+(int(raw[0])+7)/8 || len(raw) > len(scratch) {
		return nil, errPrefixKey
	}
	copy(scratch, raw)
	if bits := raw[0] % 8; bits != 0 {
		scratch[len(raw)-1] &= 0xff << (8 - bits)
	}
	return scratch[:len(raw)], nil
}

// keyLabeled removes the label stack for announcements and the single
// compatibility field for withdrawals. RFC 8277 Section 2.4 requires receivers
// to ignore the compatibility value, not just recognize 0x800000.
func keyLabeled(raw, scratch []byte, withdraw bool) ([]byte, error) {
	if len(raw) < 4 || len(raw) != 1+(int(raw[0])+7)/8 || len(scratch) < len(raw) {
		return nil, errPrefixKey
	}
	bits := int(raw[0])
	off := 1
	for {
		if bits < 24 || off+3 > len(raw) {
			return nil, errPrefixKey
		}
		bottom := raw[off+2]&1 != 0
		off += 3
		bits -= 24
		if withdraw || bottom {
			break
		}
	}
	scratch[0] = byte(bits)
	n := 1 + copy(scratch[1:], raw[off:])
	if remainder := bits % 8; remainder != 0 {
		scratch[n-1] &= 0xff << (8 - remainder)
	}
	return scratch[:n], nil
}

func keyVPN(raw, scratch []byte, withdraw bool) ([]byte, error) {
	key, err := keyLabeled(raw, scratch, withdraw)
	if err != nil {
		return nil, err
	}
	if key[0] < 64 {
		return nil, errPrefixKey
	}
	return key, nil
}

// Length octets frame a FlowSpec NLRI; they do not identify a different rule.
func keyFlowSpec(raw, _ []byte, _ bool) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errPrefixKey
	}
	off := 1
	length := int(raw[0])
	if length >= 240 {
		if len(raw) < 2 {
			return nil, errPrefixKey
		}
		off = 2
		length = int(raw[0]&0x0f)<<8 | int(raw[1])
	}
	if len(raw) != off+length {
		return nil, errPrefixKey
	}
	return raw[off:], nil
}
