// VALIDATES: spec-ospf-ext-1 AC-14/A-8 -- opaque LSAs (types 9/10/11) never become SPF
// vertices: BuildGraph decodes only Router-LSAs and Network-LSAs, so an opaque LSA in the
// database contributes no router or network vertex and cannot change the route table.
// PREVENTS: an opaque body being mis-decoded into a topology vertex and corrupting SPF.
package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func opaqueLSAForSPF(t *testing.T, scope types.LSType, adv string) packet.LSA {
	t.Helper()
	return packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionE | types.OptionO,
			Type:              scope,
			LinkStateID:       packet.OpaqueLinkStateID(1, 0x10),
			AdvertisingRouter: testRID(t, adv),
			Sequence:          types.InitialSequenceNumber,
		},
		Opaque: &packet.OpaqueLSA{Type: scope, Data: []byte{0xde, 0xad, 0xbe, 0xef}},
	}
}

func TestOpaqueLSANotInSPFGraph(t *testing.T) {
	area := testArea()
	// A source with one real router plus a Type-10 and a Type-11 opaque LSA from routers
	// that have NO Router-LSA (5.5.5.5, 6.6.6.6): if opaque leaked into the graph it would
	// appear as a spurious vertex.
	src := testSource(t, area,
		routerLSA(t, "1.1.1.1", stubLink(t, "192.0.2.0", 5)),
		opaqueLSAForSPF(t, types.LSTypeOpaqueArea, "5.5.5.5"),
		opaqueLSAForSPF(t, types.LSTypeOpaqueAS, "6.6.6.6"),
	)

	g := BuildGraph(src, area)

	if _, ok := g.Routers[testRID(t, "5.5.5.5")]; ok {
		t.Fatalf("Type-10 opaque advertising router became a router vertex")
	}
	if _, ok := g.Routers[testRID(t, "6.6.6.6")]; ok {
		t.Fatalf("Type-11 opaque advertising router became a router vertex")
	}
	if len(g.Networks) != 0 {
		t.Fatalf("opaque LSAs produced network vertices: %+v", g.Networks)
	}
	// The one legitimate router vertex is present (sanity: the opaque LSAs did not
	// displace real topology).
	if _, ok := g.Routers[testRID(t, "1.1.1.1")]; !ok {
		t.Fatalf("real router vertex missing from graph")
	}
	if len(g.Routers) != 1 {
		t.Fatalf("graph has %d router vertices, want exactly 1 (opaque excluded)", len(g.Routers))
	}
}
