// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- engine-to-neighbor NSM wiring
package ospf

import (
	"testing"
	"time"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFAdjacencyFull(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	peer, err := types.ParseRouterID("10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	eng.mu.Lock()
	ifc := eng.interfaces["eth0"]
	eng.mu.Unlock()
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if ifc == nil {
		t.Fatal("eth0 runtime interface missing")
	}
	if handle == nil {
		t.Fatal("eth0 fake transport handle missing")
	}
	hello := packet.Hello{
		HelloInterval: DefaultHelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(DefaultDeadInterval),
		Neighbors:     []types.RouterID{cfg.RouterID},
	}
	if reason := ifc.ReceiveHello(peer, hello, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	dispatchDBDesc(t, eng, handle.ifindex, peer, cfg.Areas[0].AreaID, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7})
	dispatchDBDesc(t, eng, handle.ifindex, peer, cfg.Areas[0].AreaID, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagMaster, DDSequence: 8})

	rows := eng.neighborSnapshot()
	if len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1", len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok {
		t.Fatalf("snapshot row type = %T, want neighbor.Snapshot", rows[0])
	}
	if snap.Interface != "eth0" || snap.RouterID != "10.0.0.2" || snap.State != "full" || snap.Address != "10.0.0.2" {
		t.Fatalf("snapshot = %+v, want full peer on eth0", snap)
	}
}

func dispatchDBDesc(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, dd packet.DBDesc) {
	t.Helper()
	p := packet.Packet{Header: packet.Header{RouterID: router, AreaID: area}, DBDesc: &dd}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Payload: buf})
}
