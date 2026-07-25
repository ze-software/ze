// Design: docs/architecture/wire/attributes.md -- Tunnel Encapsulation attribute decode for test display

package decode

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// decodeTunnelEncap decodes Tunnel Encapsulation attribute (code 23, RFC 9012)
// into a human-readable string showing tunnel type TLVs and their sub-TLVs.
func decodeTunnelEncap(data []byte) string {
	te, err := attribute.ParseTunnelEncap(data)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("MALFORMED:").Hex(data)
		return tb.String()
	}
	if len(te.TLVs) == 0 {
		return "EMPTY"
	}

	var tb textbuf.Buffer
	for i := range te.TLVs {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str("TT=").Uint16(te.TLVs[i].TunnelType)

		if pref, ok := te.TLVs[i].Preference(); ok {
			tb.Str("[pref=").Uint32(pref).Byte(']')
		}

		stlvs, serr := te.TLVs[i].SubTLVs()
		if serr != nil {
			tb.Str("[sub-tlv-err]")
			continue
		}
		tb.Str("[stlvs=").Int(int64(len(stlvs))).Byte(']')
	}
	return tb.String()
}
