// VALIDATES: spec-ospf-ext-4 AC-15 -- `show ospf database opaque-area` / `opaque-as` decode
// stored Extended Prefix (Type 7) / Extended Link (Type 8) bodies into Route Type, prefix,
// flags, Link Type/ID/Data, and sub-TLV rows, not raw hex.
// PREVENTS: an operator seeing only opaque hex for Extended Prefix/Link LSAs.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestExtDatabaseRenderDecoded(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	router := types.RouterID{1, 1, 1, 1}

	// Install a self Extended Prefix (area) and Extended Link (area) Opaque LSA through the
	// ext-1 carrier so they land in the LSDB exactly as a received one would.
	prefixBody := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AF: packet.ExtPrefixAFIPv4Unicast,
		Flags: packet.ExtPrefixFlagN, AddressPrefix: [4]byte{10, 1, 1, 0},
		SubTLVs: []packet.ExtSubTLV{{Type: 3, Value: []byte{0xaa, 0xbb, 0xcc, 0xdd}}},
	}}})
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: router, OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1,
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea, Options: types.OptionO, Body: prefixBody,
	}); !ok {
		t.Fatalf("originate Extended Prefix opaque LSA failed")
	}
	linkBody := packet.EncodeExtLinkLSA(packet.ExtLinkTLV{LinkType: packet.RouterLinkTypeP2P, LinkID: [4]byte{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1}})
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: router, OpaqueType: packet.ExtLinkOpaqueType, OpaqueID: 1,
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea, Options: types.OptionO, Body: linkBody,
	}); !ok {
		t.Fatalf("originate Extended Link opaque LSA failed")
	}

	db := eng.extOpaqueDecode(OpaqueScopeArea)
	if len(db.ExtendedPrefix) != 1 || len(db.ExtendedPrefix[0].Prefixes) != 1 {
		t.Fatalf("Extended Prefix not decoded: %+v", db.ExtendedPrefix)
	}
	p := db.ExtendedPrefix[0].Prefixes[0]
	if p.RouteType != "intra-area" || p.Prefix != "10.1.1.0/24" {
		t.Fatalf("prefix row = %+v, want intra-area 10.1.1.0/24", p)
	}
	// N-Flag set on a /24 non-host must be normalized away in the decoded view (RFC 7684 sec 2.1).
	for _, f := range p.Flags {
		if f == "node" {
			t.Fatalf("N-Flag must be ignored on a non-host prefix in the decoded view")
		}
	}
	if len(p.SubTLVs) != 1 || p.SubTLVs[0].Type != 3 || p.SubTLVs[0].Hex != "aabbccdd" {
		t.Fatalf("sub-TLV row = %+v, want type 3 hex aabbccdd", p.SubTLVs)
	}
	if len(db.ExtendedLink) != 1 {
		t.Fatalf("Extended Link not decoded: %+v", db.ExtendedLink)
	}
	l := db.ExtendedLink[0]
	if l.LinkType != networkPointToPoint || l.LinkID != "2.2.2.2" || l.LinkData != "10.0.0.1" {
		t.Fatalf("link row = %+v", l)
	}

	// appendExtOpaqueDecode appends the decoded section to the opaque-area response.
	out := eng.appendExtOpaqueDecode([]any{"base"}, OpaqueScopeArea)
	if len(out) != 2 {
		t.Fatalf("appendExtOpaqueDecode did not append the decoded section: %d elements", len(out))
	}
}
