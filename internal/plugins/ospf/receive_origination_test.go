// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- receive-side origination notifications.
package ospf

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: Hello and Full-state processing leave receive progress independent
// of a blocked topology read. Changes during that read cause another pass without
// a timer tick.
// PREVENTS: synchronous neighbor origination starving the engine's receive loop,
// or a notification arriving during active origination being discarded.
func TestReceiveProgressWhileOriginationBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point","cost":"10"},"eth1":{"area":"0","network-type":"point-to-point","cost":"10"},"eth2":{"area":"0","network-type":"point-to-point","cost":"10"}}}}}`), nil)
		if err != nil {
			t.Fatalf("parseOSPFConfig: %v", err)
		}
		fb := &fakeBackend{}
		eng := newEngine(transport.New(fb))
		eng.setConfig(cfg)
		oldSR, _ := srWire.get(cfg.RouterID)
		srWire.set(cfg.RouterID, sr.SRConfig{Enabled: true, SRLB: []sr.LabelRange{{Base: 40000, Size: 4}}})
		eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return nil })
		release := make(chan struct{})
		releaseWork := sync.OnceFunc(func() { close(release) })
		defer func() {
			releaseWork()
			eng.shutdown()
			eng.srAdj.neighborLost("eth0", ridOf("10.0.0.2"))
			srWire.set(cfg.RouterID, oldSR)
		}()
		if err := eng.openInterfaces(); err != nil {
			t.Fatalf("openInterfaces: %v", err)
		}
		synctest.Wait()

		entered := make(chan struct{})
		var calls atomic.Uint64
		eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
			if calls.Add(1) == 1 {
				// Hold an empty snapshot in flight. A subsequent notification must
				// publish the enrolled topology after this stale read completes.
				close(entered)
				<-release
				return nil
			}
			<-release
			topology := eng.lsdbTopology()
			for idx := range topology {
				topology[idx].Address = [4]byte{10, 0, byte(idx), 1}
				topology[idx].NetworkMask = [4]byte{255, 255, 255, 0}
			}
			return topology
		})
		peer := ridOf("10.0.0.2")
		send := func(name string, payload []byte) {
			fb.mu.Lock()
			handle := fb.handles[name]
			fb.mu.Unlock()
			handle.recv <- transport.RawPacket{
				IfIndex: handle.ifindex,
				Src:     netip.AddrFrom4([4]byte(peer)),
				Payload: payload,
			}
		}
		sendHello := func(name string) { send(name, helloFromPeer(peer, types.BackboneArea)) }
		sendHello("eth0")
		synctest.Wait()
		select {
		case <-entered:
		default:
			t.Fatal("Hello did not request topology origination")
		}
		sendHello("eth1")
		sendHello("eth2")
		synctest.Wait()
		rows := eng.neighborSnapshot()
		if len(rows) != 3 {
			t.Fatalf("neighbors while origination is blocked = %d, want one on each of three interfaces", len(rows))
		}

		twoWay := packet.Hello{
			HelloInterval: DefaultHelloInterval, DeadInterval: uint32(DefaultDeadInterval),
			Options: types.OptionE, Priority: 1, Neighbors: []types.RouterID{cfg.RouterID},
		}
		sendPacket := func(name string, p packet.Packet) {
			p.Header.RouterID = peer
			payload := make([]byte, p.EncodedLen())
			p.WriteTo(payload, 0)
			send(name, payload)
		}
		sendPacket("eth0", packet.Packet{Hello: &twoWay})
		sendPacket("eth0", packet.Packet{DBDesc: &packet.DBDesc{
			InterfaceMTU: 1500, Options: types.OptionE,
			Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7,
		}})
		sendPacket("eth0", packet.Packet{DBDesc: &packet.DBDesc{
			InterfaceMTU: 1500, Options: types.OptionE,
			Flags: packet.DDFlagMaster, DDSequence: 8,
		}})
		synctest.Wait()
		if snap, ok := eng.neighbors.Lookup("eth0", peer); !ok || snap.State != neighborStateFull {
			t.Fatalf("eth0 after DD exchange: present %v, state %s, want Full", ok, snap.State)
		}
		// Full is recorded before its event callback returns. A later packet on
		// another interface proves that callback has released the receive loop.
		sendPacket("eth1", packet.Packet{Hello: &twoWay})
		synctest.Wait()
		if snap, ok := eng.neighbors.Lookup("eth1", peer); !ok || snap.State != "exstart" {
			t.Fatalf("eth1 after eth0 reaches Full: present %v, state %s, want ExStart", ok, snap.State)
		}

		if _, ok := eng.srAdj.adjFor("eth0", peer); !ok {
			t.Fatal("Full adjacency did not allocate an Adj-SID")
		}
		// A one-way Hello leaves Full and withdraws its Adj-SID. The next
		// interface's progress must not wait for the SR withdrawal's origination.
		sendHello("eth0")
		synctest.Wait()
		sendPacket("eth2", packet.Packet{Hello: &twoWay})
		synctest.Wait()
		if snap, ok := eng.neighbors.Lookup("eth2", peer); !ok || snap.State != "exstart" {
			t.Fatalf("eth2 after eth0 leaves Full: present %v, state %s, want ExStart", ok, snap.State)
		}
		if _, ok := eng.srAdj.adjFor("eth0", peer); ok {
			t.Fatal("Adj-SID remains advertised after leaving Full")
		}
		// Re-establish Full while origination remains blocked, so the eventual
		// Router-LSA must describe the final adjacency state.
		sendPacket("eth0", packet.Packet{Hello: &twoWay})
		sendPacket("eth0", packet.Packet{DBDesc: &packet.DBDesc{
			InterfaceMTU: 1500, Options: types.OptionE,
			Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 9,
		}})
		sendPacket("eth0", packet.Packet{DBDesc: &packet.DBDesc{
			InterfaceMTU: 1500, Options: types.OptionE,
			Flags: packet.DDFlagMaster, DDSequence: 10,
		}})
		synctest.Wait()
		if snap, ok := eng.neighbors.Lookup("eth0", peer); !ok || snap.State != neighborStateFull {
			t.Fatalf("eth0 after renewed DD exchange: present %v, state %s, want Full", ok, snap.State)
		}

		releaseWork()
		synctest.Wait()
		// The synthetic clock has not advanced: only the pending notification can
		// publish this Router-LSA, so the maintenance ticker cannot hide a lost request.
		if links, ok := selfRouterLinkCount(t, eng, cfg.RouterID); !ok || links != 4 {
			t.Fatalf("Router-LSA after pending notification: present %v, links %d, want three stubs and the Full adjacency", ok, links)
		}
	})
}
