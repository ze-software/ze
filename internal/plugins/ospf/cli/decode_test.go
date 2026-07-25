// VALIDATES: spec-ospf-ext-14 AC-20 -- the offline `ze ospf decode --opaque` path renders
// an IPv4 opaque LSA's Opaque Type/ID plus its generic (type/length/value-hex) TLVs, with no
// running engine.
// PREVENTS: an offline opaque decode that hides the Opaque Type/ID or the TLV breakdown.
package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFOfflineDecode(t *testing.T) {
	body := []byte{0x00, 0x01, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04} // TLV type 1, len 4, value 01020304
	l := packet.LSA{
		Header: packet.LSAHeader{
			Type:        types.LSTypeOpaqueArea,
			LinkStateID: packet.OpaqueLinkStateID(250, 1),
			Length:      uint16(types.LSAHeaderLen + len(body)),
		},
		Body: body,
	}
	out := renderOpaqueLSA(l)
	if out.OpaqueType != 250 || out.OpaqueID != 1 {
		t.Fatalf("opaque type/id = %d/%d, want 250/1", out.OpaqueType, out.OpaqueID)
	}
	if len(out.TLVs) != 1 || out.TLVs[0].Type != 1 || out.TLVs[0].ValueHex != "01020304" {
		t.Fatalf("TLVs = %+v", out.TLVs)
	}
}
