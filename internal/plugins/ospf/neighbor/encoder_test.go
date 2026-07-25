// VALIDATES: spec-ospf-af-unify Phase 5 -- the default (OSPFv2) neighbor encoder
// produces bytes the v2 codec decodes back to the same DBDesc, so routing the
// neighbor send path through the Encoder seam is byte-identical to the prior direct
// ospf/packet encode. PREVENTS: the neighbor send path silently diverging from
// ospf/packet when the encode seam was introduced (the v6 encoder plugs in here).
package neighbor

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFNeighborV4EncoderRoundTrip(t *testing.T) {
	r := rid(t, "1.1.1.1")
	a, err := types.ParseAreaID("0")
	if err != nil {
		t.Fatal(err)
	}

	dd := packet.DBDesc{
		InterfaceMTU: 1500,
		Options:      types.OptionE,
		Flags:        packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster,
		DDSequence:   7,
	}
	p, err := packet.DecodePacket(v4Encoder{}.EncodeDBDesc(r, a, dd))
	if err != nil || p.DBDesc == nil {
		t.Fatalf("DBDesc round-trip: %v", err)
	}
	if p.DBDesc.DDSequence != 7 || p.DBDesc.InterfaceMTU != 1500 || p.DBDesc.Flags != dd.Flags {
		t.Fatalf("decoded DBDesc = %+v", p.DBDesc)
	}
	if p.Header.RouterID != r || p.Header.AreaID != a {
		t.Fatalf("decoded header = %+v", p.Header)
	}
}
