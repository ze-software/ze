// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- link-scoped LS Request replies.
package neighbor

import (
	"bytes"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOpaqueLinkRequestReturnsArrivalLinkBody(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	cfg.Options |= types.OptionO
	tbl.ConfigureInterface(cfg)
	peer := rid(t, "10.0.0.2")
	db := ospflsdb.New(nil)
	body := []byte{0, 1, 0, 0}
	header, installed := db.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: cfg.RouterID, OpaqueType: 99, OpaqueID: 1, Scope: types.LSTypeOpaqueLink,
		Area: cfg.AreaID, Interface: cfg.Name, Body: body,
	})
	if !installed {
		t.Fatal("arrival-link opaque LSA was not originated")
	}
	if _, installed := db.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: cfg.RouterID, OpaqueType: 99, OpaqueID: 1, Scope: types.LSTypeOpaqueLink,
		Area: cfg.AreaID, Interface: "other-link", Body: []byte{0, 2, 0, 0},
	}); !installed {
		t.Fatal("other-link opaque LSA was not originated")
	}
	tbl.SetLSDB(db)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	driveFull(t, tbl, cfg, peer)
	sender.sent = nil
	request := packet.LSReq{Requests: []packet.LSRequestEntry{{Type: header.Type, LinkStateID: header.LinkStateID, AdvertisingRouter: header.AdvertisingRouter}}}
	if reason := tbl.HandleLSReq(cfg.Name, peer, request); reason != "" {
		t.Fatalf("known opaque link request rejected: %s", reason)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("LS Request emitted %d packets, want one update", len(sender.sent))
	}
	response := sentPacket(t, sender, 0)
	if response.LSUpdate == nil || len(response.LSUpdate.LSAs) != 1 {
		t.Fatalf("LS Request response is not a single-LSA update: %+v", response)
	}
	lsa := response.LSUpdate.LSAs[0]
	if lsa.Header.Key() != header.Key() || len(lsa.RawBytes) < types.LSAHeaderLen || !bytes.Equal(lsa.RawBytes[types.LSAHeaderLen:], body) {
		t.Fatalf("LS Request returned wrong link body: key=%+v raw=%x", lsa.Header.Key(), lsa.RawBytes)
	}
}
