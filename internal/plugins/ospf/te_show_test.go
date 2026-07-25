// VALIDATES: spec-ospf-ext-2 `show ospf te-database` render and the opaque-area TE decode
// hook -- the TED view lists router addresses plus links with Link ID, local/remote
// address, link type, TE metric, bandwidths, admin group, and (for inter-AS) remote AS/
// ASBR; and a stored TE opaque LSA body decodes inline (Router-Address or Link + sub-TLVs)
// rather than as raw hex.
// PREVENTS: a te-database view that omits attributes, or an opaque-area TE LSA shown as hex.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func teDatabaseView1(t *testing.T, eng *engine) teDatabaseView {
	t.Helper()
	rows := eng.teDatabaseSnapshot()
	if len(rows) != 1 {
		t.Fatalf("te-database wrapping = %d, want 1", len(rows))
	}
	view, ok := rows[0].(teDatabaseView)
	if !ok {
		t.Fatalf("te-database row type = %T", rows[0])
	}
	return view
}

func TestTEShowRendersTED(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "2.2.2.2")
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 0, adv,
		packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{9, 9, 9, 9}}.Encode(), true, false))
	eng.teOnReceive(teReceived(OpaqueScopeArea, packet.TEOpaqueType, 1, adv, teLinkBody(t), true, false))

	view := teDatabaseView1(t, eng)
	if len(view.RouterAddresses) != 1 || view.RouterAddresses[0].Address != "9.9.9.9" {
		t.Fatalf("router addresses = %+v", view.RouterAddresses)
	}
	if len(view.Links) != 1 {
		t.Fatalf("links = %d, want 1", len(view.Links))
	}
	l := view.Links[0]
	if l.AdvertisingRouter != "2.2.2.2" || l.LinkID != "2.2.2.2" || l.LinkType != "point-to-point" {
		t.Fatalf("link identity = %+v", l)
	}
	if len(l.LocalAddresses) != 1 || l.LocalAddresses[0] != "10.0.0.1" {
		t.Fatalf("local addresses = %v", l.LocalAddresses)
	}
	if l.TEMetric == nil || *l.TEMetric != 42 {
		t.Fatalf("te metric = %v", l.TEMetric)
	}
	if l.MaxBandwidth == nil {
		t.Fatalf("max bandwidth missing")
	}
	if !l.Usable {
		t.Fatalf("Type-10 link should be usable")
	}
}

func TestTEShowInterASRemoteFields(t *testing.T) {
	eng, _ := newRedistEngine(t, teCfgJSON)
	adv := mustRouterID(t, "3.3.3.3")
	body := packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9},
	}}.Encode()
	eng.teOnReceive(teReceived(OpaqueScopeAS, packet.InterAsTEOpaqueType, 1, adv, body, true, false))
	view := teDatabaseView1(t, eng)
	if len(view.Links) != 1 {
		t.Fatalf("links = %d", len(view.Links))
	}
	l := view.Links[0]
	if l.RemoteAS == nil || *l.RemoteAS != 65001 || l.RemoteASBRv4 != "203.0.113.9" {
		t.Fatalf("inter-AS remote fields = %+v", l)
	}
}

func TestTEOpaqueAreaDecodeInline(t *testing.T) {
	// A stored TE opaque-area LSA decodes inline (not raw hex): the enumerated body decodes
	// to a Link TLV with its sub-TLVs.
	body := packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasLinkID: true, LinkID: [4]byte{2, 2, 2, 2}, HasTEMetric: true, TEMetric: 9,
	}}.Encode()
	view := ospflsdb.OpaqueLSAView{
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea,
		AdvertisingRouter: mustRouterID(t, "2.2.2.2"), OpaqueType: packet.TEOpaqueType, OpaqueID: 1, Body: body,
	}
	decoded, ok := teDecodeOpaqueLSA(view)
	if !ok {
		t.Fatalf("TE opaque LSA did not decode")
	}
	if decoded.LinkType != "point-to-point" || decoded.LinkID != "2.2.2.2" {
		t.Fatalf("decoded link = %+v", decoded)
	}
	if decoded.TEMetric == nil || *decoded.TEMetric != 9 {
		t.Fatalf("decoded te-metric = %v", decoded.TEMetric)
	}
}
