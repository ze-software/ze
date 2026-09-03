// Design: docs/architecture/core-design.md -- remove-private-as policy action
// RFC: rfc/short/rfc6996.md -- Private Use ASN ranges
// Related: filter_remove_private_as.go -- SDK entry point and filter handler

package filter_remove_private_as

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	removePrivateASDirective = "remove-private"
	removeModeStripText      = "strip"
	removeModePeerASText     = "peer-as"
)

type removeMode uint8

const (
	removeModeStrip removeMode = iota
	removeModePeerAS
)

func (m removeMode) String() string {
	if m == removeModePeerAS {
		return removeModePeerASText
	}
	return removeModeStripText
}

// RFC 6996 Section 5: Private Use ASNs are 64512-65534 and
// 4200000000-4294967294, inclusive.
func isPrivateASN(asn uint32) bool {
	return (asn >= 64512 && asn <= 65534) || (asn >= 4200000000 && asn <= 4294967294)
}

// rewriteASPathText takes the AS path as filtertext.ASPath returns it, and
// gives back the value in the shape the "as-path" field of the filter text
// format carries: bare for one ASN, bracketed for several. The reactor performs
// the authoritative wire rewrite; this text rewrite lets later text filters in
// the same chain see the intended path.
//
// The second return reports whether a Private Use ASN was rewritten. It is
// false for a path that carries none, which the first return then echoes back
// unchanged, and false for a path this reader cannot parse, which returns the
// empty string because there is no path to advertise.
func rewriteASPathText(asPath string, mode removeMode, peerAS uint32) (string, bool) {
	tokens := strings.Fields(asPath)
	if len(tokens) == 0 {
		return "", false
	}

	out := make([]uint32, 0, len(tokens))
	changed := false
	for _, tok := range tokens {
		asn64, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return "", false
		}
		asn := uint32(asn64) //nolint:gosec // bounded by ParseUint 32-bit
		if !isPrivateASN(asn) {
			out = append(out, asn)
			continue
		}
		changed = true
		if mode == removeModePeerAS {
			out = append(out, peerAS)
		}
	}
	if !changed {
		return asPath, false
	}
	return formatASPathTokens(out), true
}

func formatASPathTokens(asns []uint32) string {
	if len(asns) == 0 {
		return "[]"
	}
	var b textbuf.Buffer
	if len(asns) > 1 {
		b.Byte('[')
	}
	for i, asn := range asns {
		if i > 0 {
			b.Byte(' ')
		}
		b.Uint32(asn)
	}
	if len(asns) > 1 {
		b.Byte(']')
	}
	return b.String()
}

func buildDirectiveDelta(mode removeMode, asPathValue string, asPathChanged bool) string {
	var b textbuf.Buffer
	if asPathChanged {
		b.Str("as-path ").Str(asPathValue).Byte(' ')
	}
	b.Str(removePrivateASDirective).Byte(' ').Str(mode.String())
	return b.String()
}

func hasPrivateAS4PathPayload(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + withdrawnLen
	if len(payload) < attrLenOff+2 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrStart := attrLenOff + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return false
	}
	for off := attrStart; off < attrEnd; {
		if off+3 > attrEnd {
			return false
		}
		flags := payload[off]
		code := payload[off+1]
		hdrLen := 3
		valueLen := int(payload[off+2])
		if flags&0x10 != 0 {
			if off+4 > attrEnd {
				return false
			}
			hdrLen = 4
			valueLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		}
		valueStart := off + hdrLen
		valueEnd := valueStart + valueLen
		if valueEnd > attrEnd {
			return false
		}
		if code == 17 && as4PathValueHasPrivateASN(payload[valueStart:valueEnd]) {
			return true
		}
		off = valueEnd
	}
	return false
}

func as4PathValueHasPrivateASN(value []byte) bool {
	for off := 0; off < len(value); {
		if off+2 > len(value) {
			return false
		}
		count := int(value[off+1])
		off += 2
		for range count {
			if off+4 > len(value) {
				return false
			}
			if isPrivateASN(binary.BigEndian.Uint32(value[off:])) {
				return true
			}
			off += 4
		}
	}
	return false
}
