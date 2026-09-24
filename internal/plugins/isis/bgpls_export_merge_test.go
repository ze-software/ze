// Design: docs/architecture/wire/nlri-bgpls.md -- complete native attribute translation.

package isis

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

func TestISISBGPLSMergedSRLGArray(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	for fragment, localID := range []byte{11, 21} {
		// Addresses define one BGP-LS NLRI even when native fragments carry
		// different link identifiers. Each SRLG record matches one identifier.
		subs := []byte{4, 8, 0, 0, 0, localID, 0, 0, 0, localID + 1,
			6, 4, 192, 0, 2, 2, 8, 4, 192, 0, 2, 3}
		link := append([]byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 10, byte(len(subs))}, subs...)
		srlg := []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, localID, 0, 0, 0, localID + 1,
			0, 0, 0, byte(fragment + 1)}
		id[7] = byte(fragment)
		bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
			{Type: 22, Value: link}, {Type: 138, Value: srlg},
		})
	}
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 1 {
		t.Fatalf("address-identical adjacency did not merge: %+v", builder.snapshot.Links)
	}
	var groups []uint32
	arrays := 0
	for _, attr := range builder.snapshot.Links[0].Attributes {
		if attr.Type != 1096 {
			continue
		}
		arrays++
		if len(attr.Value)%4 != 0 {
			t.Fatalf("invalid SRLG array: %x", attr.Value)
		}
		for off := 0; off < len(attr.Value); off += 4 {
			groups = append(groups, binary.BigEndian.Uint32(attr.Value[off:]))
		}
	}
	slices.Sort(groups)
	if arrays != 1 || !slices.Equal(groups, []uint32{1, 2}) {
		t.Fatalf("merged adjacency SRLG arrays=%d groups=%v, want one array with both groups", arrays, groups)
	}
}
