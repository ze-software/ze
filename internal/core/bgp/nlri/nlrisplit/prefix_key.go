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

// compatibilityFamilies maps each family whose withdrawn NLRI carries the
// three-octet Compatibility field of RFC 8277 Section 2.4 directly after its
// length octet, where the announcement carried its label stack, to the splitter
// that frames its withdrawals and the writer that converts a field to that
// framing. It is the one declaration of that family set: CarriesCompatibility,
// CompatibilitySplitter, GetWithdrawalWriter and WithdrawalNLRIFromKey all
// read it.
//
// It is written only by this package's init (register.go) and never after, so
// it is frozen before any reader runs: the Go memory model orders a package's
// init before every function of an importing package, so the session writer
// reads it with no lock. Nothing may write it outside init.
var compatibilityFamilies = make(map[family.Family]compatibilityFraming)

// compatibilityFraming is how one family of compatibilityFamilies frames and
// writes its withdrawals. Both fields are set for every entry.
type compatibilityFraming struct {
	split Splitter
	write WithdrawalWriter
}

// cidrKeys holds the families whose route key is the [length][prefix] CIDR of
// the family's own address family, carried behind fields that are not part of
// the key: a label stack in an announcement, a Compatibility field in a
// withdrawal. RouteCIDR answers only for these.
var cidrKeys = make(map[family.Family]PrefixKeyFunc)

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

// CarriesCompatibility reports whether a withdrawn NLRI of fam carries the
// Compatibility field: [length:1][Compatibility:3][rest], after the ADD-PATH
// identifier when one is negotiated.
//
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
//
// CompatibilitySplitter frames such a section. It takes no lock:
// compatibilityFamilies is frozen after init.
func CarriesCompatibility(fam family.Family) bool {
	return compatibilityFamilies[fam].split != nil
}

// CompatibilitySplitter returns the splitter that frames a withdrawal of fam
// when its withdrawn NLRI carries the RFC 8277 Section 2.4 Compatibility field,
// and nil for every other family. It is GetWithdraw for those families with no
// lock taken, because compatibilityFamilies is frozen after init and every
// UPDATE Ze sends asks it once per MP_UNREACH_NLRI.
func CompatibilitySplitter(fam family.Family) Splitter {
	return compatibilityFamilies[fam].split
}

// RouteCIDR returns the [length][prefix] CIDR a route of fam names, with the
// fields in front of the prefix that do not identify the route stripped. raw is
// one NLRI without its ADD-PATH identifier, and withdraw says it was framed by
// the family's withdrawal splitter. scratch must have PrefixKeyScratchSize
// bytes, and the result aliases it. A family whose key is not such a CIDR
// answers ErrUnsupported, which is the caller's cue to keep the NLRI opaque.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." For a withdrawal of a labeled family the CIDR is what is
// left once the field is ignored, so it names the route whatever the field held.
func RouteCIDR(fam family.Family, raw, scratch []byte, withdraw bool) ([]byte, error) {
	mu.RLock()
	key := cidrKeys[fam]
	mu.RUnlock()
	if key == nil {
		return nil, ErrUnsupported
	}
	return key(raw, scratch, withdraw)
}

// KeysByCIDR reports whether fam names its routes by a CIDR that RouteCIDR
// answers. A consumer that keys an announcement by RouteCIDR asks this to key
// the family's withdrawal, which wireu hands over as an INET of that prefix,
// the same way.
func KeysByCIDR(fam family.Family) bool {
	mu.RLock()
	defer mu.RUnlock()
	return cidrKeys[fam] != nil
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
	off, bits, ok := labelFieldEnd(raw, withdraw)
	if !ok {
		return nil, errPrefixKey
	}
	scratch[0] = byte(bits)
	n := 1 + copy(scratch[1:], raw[off:])
	if remainder := bits % 8; remainder != 0 {
		scratch[n-1] &= 0xff << (8 - remainder)
	}
	return scratch[:n], nil
}

// labelFieldEnd finds the end of the label field of one labeled NLRI, raw,
// which starts at its length octet: [length][label field][RD?][prefix]. It
// returns the offset of the first octet after the field and the bits the
// length octet counts after it. An announcement's field is its label stack,
// walked to the bottom-of-stack bit; a withdrawal's is the one three-octet
// Compatibility field of RFC 8277 Section 2.4, whatever it holds. It reports
// false when the length octet counts fewer bits than the field or the field
// runs past raw. It is the one label walk keyLabeled and the withdrawal writer
// share. The walk is bounded by len(raw): every step advances three octets.
func labelFieldEnd(raw []byte, withdraw bool) (off, bits int, ok bool) {
	bits = int(raw[0])
	off = 1
	for {
		if bits < 24 || off+3 > len(raw) {
			return 0, 0, false
		}
		bottom := raw[off+2]&1 != 0
		off += 3
		bits -= 24
		if withdraw || bottom {
			return off, bits, true
		}
	}
}

// vpnKeyBitsMin is the fewest bits a VPN route key counts after its label
// field: the 8-octet Route Distinguisher (RFC 4364 Section 4.3.4).
const vpnKeyBitsMin = 64

func keyVPN(raw, scratch []byte, withdraw bool) ([]byte, error) {
	key, err := keyLabeled(raw, scratch, withdraw)
	if err != nil {
		return nil, err
	}
	if key[0] < vpnKeyBitsMin {
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

var (
	errWithdrawalField       = errors.New("nlrisplit: malformed labeled NLRI field")
	errWithdrawalDestination = errors.New("nlrisplit: labeled withdrawal does not fit its destination")
	errWithdrawalLength      = errors.New("nlrisplit: key too long for a withdrawal length octet with the Compatibility field")
)

// WithdrawalWriter writes the withdrawal framing of every NLRI in field, one
// MP NLRI field of a labeled family, into dst and returns the octets written.
// addPath says the session that framed field negotiated ADD-PATH for the
// family, and withdraw says field is withdrawal framed (one Compatibility
// field per NLRI) rather than announcement framed (a label stack). Each NLRI
// keeps its path identifier; its label field becomes the one Compatibility
// field 0x800000 and its length octet drops 24 bits per label beyond the
// first. The result is never longer than field.
//
// It never writes past len(dst), never modifies field, and refuses the field
// with an error when an NLRI does not frame, its stack has no bottom-of-stack
// bit, it counts fewer bits than its label field, or dst is too short; dst
// then holds a partial write the caller MUST discard. dst MAY alias field at
// the same offset: every write lands on octets already read. Allocation-free.
// Safe for concurrent use.
type WithdrawalWriter func(dst, field []byte, addPath, withdraw bool) (int, error)

// GetWithdrawalWriter returns the withdrawal writer of fam, or nil for a
// family whose announcement and withdrawal framings are the same bytes. It
// takes no lock: compatibilityFamilies is frozen after init, and the forward
// rail asks it once per converted withdrawal.
func GetWithdrawalWriter(fam family.Family) WithdrawalWriter {
	return compatibilityFamilies[fam].write
}

// WithdrawalNLRIFromKey writes the withdrawal NLRI of one route of fam into
// dst from its native key, [bits][RD?][prefix] as GetPrefixKey returns it, and
// returns the octets written. The path identifier is the caller's: the key
// excludes it. A family GetWithdrawalWriter answers nil for answers
// ErrUnsupported, which tells the caller its stored NLRI is already the
// withdrawal framing. It needs 3 octets more than len(key) in dst.
func WithdrawalNLRIFromKey(fam family.Family, key, dst []byte) (int, error) {
	if compatibilityFamilies[fam].write == nil {
		return 0, ErrUnsupported
	}
	if len(key) == 0 || len(key) != 1+(int(key[0])+7)/8 {
		return 0, errPrefixKey
	}
	if int(key[0])+24 > maxLengthOctetBits {
		return 0, errWithdrawalLength
	}
	n, ok := writeWithdrawalNLRI(dst, int(key[0]), key[1:])
	if !ok {
		return 0, errWithdrawalDestination
	}
	return n, nil
}

// writeLabeledWithdrawal is the WithdrawalWriter of labeled unicast (SAFI 4).
func writeLabeledWithdrawal(dst, field []byte, addPath, withdraw bool) (int, error) {
	return writeWithdrawalField(dst, field, addPath, withdraw, 0)
}

// writeVPNWithdrawal is the WithdrawalWriter of MPLS VPN (SAFI 128), whose key
// holds the Route Distinguisher, so keyVPN's floor applies.
func writeVPNWithdrawal(dst, field []byte, addPath, withdraw bool) (int, error) {
	return writeWithdrawalField(dst, field, addPath, withdraw, vpnKeyBitsMin)
}

// writeWithdrawalField converts field one NLRI at a time, each changing form
// once, from its source framing to the withdrawal framing, with no key in
// between. keyBitsMin is the family's floor on the bits after the label field.
// The walk is bounded by len(field): every NLRI advances at least four octets.
//
// Announced NLRI (RFC 8277 Section 2.3) and withdrawn NLRI (Section 2.4), each
// after the four-octet Path Identifier under ADD-PATH:
//
//	+--------+---------------------------------+------+--------------+
//	| Length | Label(3) ... until S = 1        | RD?  | Prefix ~     |
//	+--------+---------------------------------+------+--------------+
//	| Length | Compatibility 0x800000 (3)      | RD?  | Prefix ~     |
//	+--------+---------------------------------+------+--------------+
func writeWithdrawalField(dst, field []byte, addPath, withdraw bool, keyBitsMin int) (int, error) {
	head := 0
	if addPath {
		head = 4
	}
	written := 0
	for read := 0; read < len(field); {
		end := bitLengthEntryEnd(field, read, head, maxLengthOctetBits)
		if end > len(field) {
			return 0, errWithdrawalField
		}
		nlri := field[read+head : end]
		off, bits, ok := labelFieldEnd(nlri, withdraw)
		if !ok {
			return 0, errWithdrawalField
		}
		if bits < keyBitsMin {
			return 0, errWithdrawalField
		}
		if written+head > len(dst) {
			return 0, errWithdrawalDestination
		}
		written += copy(dst[written:written+head], field[read:read+head])
		n, ok := writeWithdrawalNLRI(dst[written:], bits, nlri[off:])
		if !ok {
			return 0, errWithdrawalDestination
		}
		written += n
		read = end
	}
	return written, nil
}

// writeWithdrawalNLRI writes one withdrawn labeled NLRI into dst: the length
// octet, the Compatibility field, then rest, the RD (SAFI 128) and the prefix
// that bits counts. It reports false when dst is too short or the length octet
// cannot hold bits+24. It is the one declaration of the labeled withdrawal
// framing: the field writer, the key inverse, bgp-rib's sent identity and the
// reactor's owed withdrawal all reach the wire through it.
func writeWithdrawalNLRI(dst []byte, bits int, rest []byte) (int, bool) {
	n := 4 + len(rest)
	if bits+24 > maxLengthOctetBits {
		return 0, false
	}
	if n > len(dst) {
		return 0, false
	}
	// RFC 8277 Section 2.4: "the prefix length is not the same as the NLRI
	// length; to determine the prefix length of a prefix in an MP_UNREACH_NLRI,
	// the length of the Compatibility field must be subtracted from the length
	// of the NLRI."
	dst[0] = byte(bits + 24)
	// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD
	// be set to 0x800000."
	dst[1], dst[2], dst[3] = 0x80, 0x00, 0x00
	copy(dst[4:], rest)
	// RFC 4760 Section 5 frames the prefix as RFC 4271 Section 4.3 does, where
	// "Note that the value of trailing bits is irrelevant.", so clearing them
	// changes no match and gives one route one withdrawal, as keyLabeled keys it.
	if remainder := bits % 8; remainder != 0 {
		dst[n-1] &= 0xff << (8 - remainder)
	}
	return n, true
}
