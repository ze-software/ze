// VALIDATES: spec-ospf-ext-12 (RFC 6549 sec 3) -- the OSPFv2 neighbor encoder stamps the
// engine's Instance ID into the common header of every DD / LSReq / LSUpdate it emits, so
// the neighbor FSM's transmit path carries the instance; the base instance 0 is unchanged.
package neighbor

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestDBDescCarriesInstanceID(t *testing.T) {
	r := rid(t, "1.1.1.1")
	a, err := types.ParseAreaID("0")
	if err != nil {
		t.Fatal(err)
	}
	dd := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit, DDSequence: 7}
	lsr := packet.LSReq{Requests: []packet.LSRequestEntry{{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID{1, 1, 1, 1}, AdvertisingRouter: r}}}
	lsu := packet.LSUpdate{}

	for _, id := range []uint8{0, 5, 255} {
		enc := NewV4Encoder(id)
		encodings := map[string][]byte{
			"dbdesc":   enc.EncodeDBDesc(r, a, dd),
			"lsreq":    enc.EncodeLSReq(r, a, lsr),
			"lsupdate": enc.EncodeLSUpdate(r, a, lsu),
		}
		for name, wire := range encodings {
			h, _, err := packet.DecodeHeader(wire)
			if err != nil {
				t.Fatalf("id %d %s: DecodeHeader: %v", id, name, err)
			}
			if h.InstanceID != id {
				t.Fatalf("id %d %s: header Instance ID = %d, want %d", id, name, h.InstanceID, id)
			}
			if h.RouterID != r || h.AreaID != a {
				t.Fatalf("id %d %s: header router/area = %+v", id, name, h)
			}
		}
	}
}
