// Design: docs/architecture/ospf/ospf-2-wire.md -- offline OSPFv2 packet decode CLI.
// spec-ospf-ext-14: extended with `--opaque` (render an IPv4 opaque LSA's Opaque Type/ID +
// generic TLVs, RFC 5250) and `--v3` (render an OSPFv3 LSA's scope-aware LS Type + 20-octet
// header + typed/generic body, RFC 5340). Both run offline, with no running engine.

package cli

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

const maxStdinBytes = 256 * 1024

func errln(msg string) {
	var b textbuf.Buffer
	if _, werr := os.Stderr.WriteString(b.Str(msg).Byte('\n').String()); werr != nil {
		return
	}
}

func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "indent JSON output")
	opaque := fs.Bool("opaque", false, "decode an IPv4 opaque LSA (Opaque Type/ID + TLVs)")
	v3 := fs.Bool("v3", false, "decode an OSPFv3 (IPv6) LSA (scope-aware type + header + body)")
	fs.Usage = func() {
		errln("usage: ze ospf decode [--pretty] [--opaque | --v3] < hex")
		errln("  reads a hex OSPF packet/LSA from stdin, emits JSON on stdout")
		errln("  --opaque: interpret input as an IPv4 opaque LSA (RFC 5250)")
		errln("  --v3:     interpret input as an OSPFv3 LSA (RFC 5340)")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}
	wire, ok := readWire()
	if !ok {
		return 1
	}
	switch {
	case *opaque:
		return decodeOpaqueLSA(wire, *pretty)
	case *v3:
		return decodeV3LSA(wire, *pretty)
	default:
		return decodeV2Packet(wire, *pretty)
	}
}

func readWire() ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: read stdin: ").Err(err).String())
		return nil, false
	}
	if len(raw) > maxStdinBytes {
		var b textbuf.Buffer
		errln(b.Str("error: input exceeds ").Int(int64(maxStdinBytes)).Str(" bytes (max)").String())
		return nil, false
	}
	return toWire(raw), true
}

func emitJSON(v any, pretty bool) int {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: encode json: ").Err(err).String())
		return 1
	}
	return 0
}

func decodeV2Packet(wire []byte, pretty bool) int {
	p, err := packet.DecodePacket(wire)
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: decode packet: ").Err(err).String())
		return 1
	}
	return emitJSON(p.ToJSON(), pretty)
}

// offlineTLV is one generic (type, length, value-hex) opaque TLV row.
type offlineTLV struct {
	Type     uint16 `json:"type"`
	Length   int    `json:"length"`
	ValueHex string `json:"value-hex"`
}

// opaqueDecodeOutput renders an offline IPv4 opaque-LSA decode (RFC 5250).
type opaqueDecodeOutput struct {
	OpaqueType        uint8        `json:"opaque-type"`
	OpaqueID          uint32       `json:"opaque-id"`
	LinkStateID       string       `json:"link-state-id"`
	AdvertisingRouter string       `json:"advertising-router"`
	Age               uint16       `json:"age"`
	Length            uint16       `json:"length"`
	TLVs              []offlineTLV `json:"tlvs"`
	Malformed         bool         `json:"malformed,omitempty"`
	BodyHex           string       `json:"body-hex,omitempty"`
}

func decodeOpaqueLSA(wire []byte, pretty bool) int {
	l, err := packet.DecodeLSA(wire)
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: decode opaque LSA: ").Err(err).String())
		return 1
	}
	return emitJSON(renderOpaqueLSA(l), pretty)
}

// renderOpaqueLSA builds the offline opaque-LSA decode output (pure, no I/O).
func renderOpaqueLSA(l packet.LSA) opaqueDecodeOutput {
	out := opaqueDecodeOutput{
		OpaqueType:        l.OpaqueType(),
		OpaqueID:          l.OpaqueID(),
		LinkStateID:       l.Header.LinkStateID.String(),
		AdvertisingRouter: l.Header.AdvertisingRouter.String(),
		Age:               l.Header.Age.Age(),
		Length:            l.Header.Length,
	}
	tlvs, terr := packet.DecodeOpaqueTLVs(l.Body)
	for _, t := range tlvs {
		out.TLVs = append(out.TLVs, offlineTLV{Type: t.Type, Length: t.Length, ValueHex: hex.EncodeToString(t.Value)})
	}
	if terr != nil {
		out.Malformed = true
		out.BodyHex = hex.EncodeToString(l.Body)
	}
	return out
}

// v3DecodeOutput renders an offline OSPFv3 LSA decode (RFC 5340 section A.4.2.1 / A.4).
type v3DecodeOutput struct {
	LSTypeHex         string `json:"ls-type-hex"`
	Scope             string `json:"scope"`
	FunctionCode      uint16 `json:"function-code"`
	UBit              bool   `json:"u-bit"`
	LinkStateID       string `json:"link-state-id"`
	AdvertisingRouter string `json:"advertising-router"`
	Age               uint16 `json:"age"`
	Length            uint16 `json:"length"`
	Decoded           any    `json:"decoded,omitempty"`
	BodyHex           string `json:"body-hex,omitempty"`
}

func decodeV3LSA(wire []byte, pretty bool) int {
	l, err := ospfv3packet.DecodeLSA(wire)
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: decode OSPFv3 LSA: ").Err(err).String())
		return 1
	}
	return emitJSON(renderV3LSA(l), pretty)
}

// renderV3LSA builds the offline OSPFv3 LSA decode output (pure, no I/O).
func renderV3LSA(l ospfv3packet.LSA) v3DecodeOutput {
	t := uint16(l.Header.Type)
	out := v3DecodeOutput{
		LSTypeHex:         hexUint16(t),
		Scope:             v3OfflineScope(t),
		FunctionCode:      t & 0x1FFF,
		UBit:              t&0x8000 != 0,
		LinkStateID:       l.Header.LinkStateID.String(),
		AdvertisingRouter: l.Header.AdvertisingRouter.String(),
		Age:               uint16(l.Header.Age),
		Length:            l.Header.Length,
	}
	if body := v3OfflineTypedBody(&l); body != nil {
		out.Decoded = body
	} else {
		out.BodyHex = hex.EncodeToString(l.Body)
	}
	return out
}

// v3OfflineTypedBody decodes the common OSPFv3 base LSA bodies; nil for unknown types.
func v3OfflineTypedBody(l *ospfv3packet.LSA) any {
	switch l.Header.Type { //nolint:exhaustive // only the common base types render typed; the rest fall back to body-hex
	case ospfv3types.LSTypeRouter:
		if b, err := l.DecodeRouter(); err == nil {
			return b
		}
	case ospfv3types.LSTypeNetwork:
		if b, err := l.DecodeNetwork(); err == nil {
			return b
		}
	case ospfv3types.LSTypeIntraAreaPrefix:
		if b, err := l.DecodeIntraAreaPrefix(); err == nil {
			return b
		}
	case ospfv3types.LSTypeLink:
		if b, err := l.DecodeLink(); err == nil {
			return b
		}
	case ospfv3types.LSTypeASExternal, ospfv3types.LSTypeNSSA:
		if b, err := l.DecodeExternal(); err == nil {
			return b
		}
	}
	return nil
}

// scopeArea is the RFC 5340 A.4.2.1 area flooding-scope name (shared with the
// decode tests, which assert on it).
const scopeArea = "area"

// v3OfflineScope names the RFC 5340 section A.4.2.1 flooding scope from the S2/S1 bits.
func v3OfflineScope(t uint16) string {
	switch t & 0x6000 {
	case 0x0000:
		return "link-local"
	case 0x2000:
		return scopeArea
	case 0x4000:
		return "as"
	default:
		return "reserved"
	}
}

func hexUint16(v uint16) string {
	var b textbuf.Buffer
	return b.Str("0x").Hex([]byte{byte(v >> 8), byte(v)}).String()
}

func toWire(raw []byte) []byte {
	clean := stripWhitespace(string(raw))
	if clean != "" && len(clean)%2 == 0 && isHexString(clean) {
		if decoded, err := hex.DecodeString(clean); err == nil {
			return decoded
		}
	}
	return raw
}

func isHexString(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
