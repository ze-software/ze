// Design: docs/architecture/isis/isis-4-component-config.md -- panic containment in the engine's receive worker
//
// VALIDATES: a panic while the engine's receive worker handles one PDU is contained
// by the engine guard (logged, counted in ze_task_panics_total{component="isis",
// task="receive"}, crash-recorded), takes Down only the adjacencies the sending
// SNPA holds on the PDU's circuit, and the same worker goes on to handle the next
// PDU, so every other adjacency on that circuit and on other circuits stays Up.
package isis

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// receivePanicSNPA is the local MAC every test circuit carries, which a neighbor's
// LAN IIH echoes in TLV 6 to bring its adjacency Up.
var receivePanicSNPA = adjacency.SNPA{0x02, 0, 0, 0, 0, 1}

// receivePanicHello encodes a Level-1 LAN IIH from sys in area 49.0001 that
// echoes receivePanicSNPA, so the adjacency it forms reaches Up.
func receivePanicHello(t *testing.T, sys types.SystemID) []byte {
	t.Helper()
	area, err := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	if err != nil {
		t.Fatalf("area: %v", err)
	}
	areaVal := append([]byte{byte(area.Len())}, area.Bytes()...)
	v4 := netip.MustParseAddr("192.0.2.2").As4()
	h := packet.LANHello{
		PDUType:     packet.PDUTypeL1LANHello,
		CircuitType: packet.CircuitL1,
		SystemID:    sys,
		HoldingTime: types.HoldingTime(30),
		Priority:    64,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaVal},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVIPInterfaceAddress, Value: v4[:]},
			{Type: packet.TLVISNeighbors, Value: receivePanicSNPA[:]},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// addReceivePanicCircuit registers a Level-1 broadcast circuit on ifindex and
// brings one adjacency Up per neighbor, keyed by its System ID and learned from
// its SNPA.
func addReceivePanicCircuit(t *testing.T, e *engine, name string, ifindex int, neighbors map[types.SystemID]adjacency.SNPA) *circuit.Circuit {
	t.Helper()
	area, err := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	if err != nil {
		t.Fatalf("area: %v", err)
	}
	c := circuit.New(circuit.Config{
		Name:     name,
		IfIndex:  ifindex,
		SystemID: types.SystemID{0, 0, 0, 0, 0, 1},
		SNPA:     receivePanicSNPA,
		Areas:    []types.AreaID{area},
		IPv4:     netip.MustParseAddr("192.0.2.1"),
		Kind:     adjacency.KindBroadcast,
		Levels:   []adjacency.Level{adjacency.Level1},
		Level1:   circuit.LevelTimers{HelloInterval: 10, HoldMult: 3},
		Level2:   circuit.LevelTimers{HelloInterval: 10, HoldMult: 3},
	}, nil, nil)
	for sys, snpa := range neighbors {
		if tr := c.Receive(snpa, receivePanicHello(t, sys)); tr.State != adjacency.StateUp {
			t.Fatalf("setup: %s on %s is %v, want up", sys, name, tr.State)
		}
	}
	e.circuitsMu.Lock()
	e.circuits[ifindex] = c
	e.circuitByName[name] = c
	e.circuitsMu.Unlock()
	return c
}

// requireAdjacencyUp fails unless sys's adjacency on c is Up exactly when want is.
func requireAdjacencyUp(t *testing.T, c *circuit.Circuit, sys types.SystemID, want bool) {
	t.Helper()
	for _, row := range c.Table().Snapshot() {
		if row.SystemID != sys.String() {
			continue
		}
		if up := row.State == adjacency.StateUp.String(); up != want {
			t.Fatalf("%s on %s is %s, want up=%v", sys, c.Name(), row.State, want)
		}
		return
	}
	t.Fatalf("no adjacency from %s on %s", sys, c.Name())
}

// TestISISReceivePanicContained runs the real receive worker over two LAN circuits.
// eth0 holds neighbors from two SNPAs; eth1 holds a neighbor from the same SNPA as
// eth0's broken one. The CSNP handler is replaced through the dispatcher's
// registration seam with one that panics for the broken SNPA on eth0 and reports
// every other frame. One CSNP from the broken SNPA on eth0 panics: the guard counts
// and records it, only that SNPA's adjacency on eth0 goes Down, and the next CSNP,
// from the healthy SNPA, reaches the handler on the same worker.
func TestISISReceivePanicContained(t *testing.T) {
	before, declared := enginePanics().guard.Panics("receive")
	if !declared {
		t.Fatal(`task "receive" is not declared on the isis engine boundary`)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	e := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(e.shutdown)
	broken, healthy := adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, adjacency.SNPA{0x02, 0, 0, 0, 0, 3}
	brokenSys, healthySys, otherSys := types.SystemID{0, 0, 0, 0, 0, 2}, types.SystemID{0, 0, 0, 0, 0, 3}, types.SystemID{0, 0, 0, 0, 0, 4}
	const eth0, eth1 = 3, 4
	c0 := addReceivePanicCircuit(t, e, "eth0", eth0, map[types.SystemID]adjacency.SNPA{brokenSys: broken, healthySys: healthy})
	c1 := addReceivePanicCircuit(t, e, "eth1", eth1, map[types.SystemID]adjacency.SNPA{otherSys: broken})

	handled := make(chan transport.RawFrame, 1)
	e.dispatch.register(packet.PDUTypeL1CSNP, func(rf transport.RawFrame) {
		if rf.IfIndex == eth0 && rf.SrcMAC == broken {
			panic("test: CSNP handler seam panics for the broken SNPA on eth0")
		}
		handled <- rf
	})

	recv := make(chan transport.RawFrame)
	e.wg.Go(func() { e.receiveWorker(recv) })
	recv <- transport.RawFrame{IfIndex: eth0, SrcMAC: broken, PDU: minimalPDU(packet.PDUTypeL1CSNP)}
	recv <- transport.RawFrame{IfIndex: eth0, SrcMAC: healthy, PDU: minimalPDU(packet.PDUTypeL1CSNP)}

	select {
	case rf := <-handled:
		if rf.SrcMAC != healthy {
			t.Fatalf("handled a frame from %v, want the healthy SNPA %v", rf.SrcMAC, healthy)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the receive worker never handled the healthy CSNP after the broken one's panic")
	}

	if n, _ := enginePanics().guard.Panics("receive"); n != before+1 {
		t.Fatalf(`Panics("receive") = %d, want %d: the panic was not counted`, n, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "isis receive, unit PDU on ifindex " + strconv.Itoa(eth0) + " from " + net.HardwareAddr(broken[:]).String()
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
	requireAdjacencyUp(t, c0, brokenSys, false)
	requireAdjacencyUp(t, c0, healthySys, true)
	requireAdjacencyUp(t, c1, otherSys, true)
}
