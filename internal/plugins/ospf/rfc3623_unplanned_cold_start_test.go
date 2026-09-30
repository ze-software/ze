// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- unplanned restart.
// Related: instance.go -- openInterfaces, which holds the Hellos until the Grace-LSAs are sent.
// Related: gr_restarter.go -- maybeUnplannedRestart and grOriginateGraceLSAs.
//
// VALIDATES: RFC 3623 Section 5, "The grace-LSAs must be originated and be sent *before* the
// restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be
// flooded to the AllSPFRouters multicast address (224.0.0.5)", on the engine's real
// cold-start path: openInterfaces with unplanned-restart support enabled.
// PREVENTS: an unplanned-restart pass that runs before the interfaces are known and so
// originates no Grace-LSA at all, or one that lets a Hello leave first.
package ospf

import (
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// coldStartSend is one packet a coldStartHandle was asked to send.
type coldStartSend struct {
	target netip.Addr
	packet ospfpacket.Packet
}

// coldStartBackend opens coldStartHandles that record every packet the engine sends.
// Safe for concurrent use: the Hello loop sends from its own goroutine.
type coldStartBackend struct {
	mu    sync.Mutex
	sends []coldStartSend
	fail  []error
}

func (b *coldStartBackend) OpenInterface(_ string, _ transport.DropRecorder) (transport.InterfaceHandle, error) {
	return &coldStartHandle{backend: b, recv: make(chan transport.RawPacket, 1)}, nil
}

func (b *coldStartBackend) record(target netip.Addr, raw []byte) {
	decoded, err := ospfpacket.DecodePacket(slices.Clone(raw))
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.fail = append(b.fail, err)
		return
	}
	b.sends = append(b.sends, coldStartSend{target: target, packet: decoded})
}

func (b *coldStartBackend) snapshot() ([]coldStartSend, []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.sends), slices.Clone(b.fail)
}

type coldStartHandle struct {
	backend *coldStartBackend
	recv    chan transport.RawPacket
	once    sync.Once
}

func (h *coldStartHandle) IfIndex() int { return 1 }
func (h *coldStartHandle) Send(target netip.Addr, raw []byte) error {
	h.backend.record(target, raw)
	return nil
}
func (h *coldStartHandle) Recv() <-chan transport.RawPacket { return h.recv }
func (h *coldStartHandle) JoinAllSPFRouters() error         { return nil }
func (h *coldStartHandle) JoinAllDRouters() error           { return nil }
func (h *coldStartHandle) LeaveAllDRouters() error          { return nil }
func (h *coldStartHandle) Close() error {
	h.once.Do(func() { close(h.recv) })
	return nil
}

// carriesGraceLSA reports whether a sent packet is a Link State Update holding a Grace-LSA.
func carriesGraceLSA(sent ospfpacket.Packet) bool {
	if sent.LSUpdate == nil {
		return false
	}
	for _, lsa := range sent.LSUpdate.LSAs {
		if lsa.OpaqueType() == ospfpacket.GraceOpaqueType {
			return true
		}
	}
	return false
}

// TestRFC3623UnplannedColdStartOriginatesGraceLSA checks the cold-start origination and order.
// Goal: an engine that starts with unplanned-restart support and no planned restart fact
// holds a Grace-LSA on its broadcast interface, and the first packet it sends there is the
// Link State Update carrying it, to AllSPFRouters, with the first Hello after it. Method:
// parse a one-interface config with a one-second Hello interval, enable planned-and-unplanned
// support, run openInterfaces over a recording transport, wait for the first Hello, and read
// the send order and the interface's link-scope LSAs.
func TestRFC3623UnplannedColdStartOriginatesGraceLSA(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0","network-type":"broadcast","hello-interval":1}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	backend := &coldStartBackend{}
	eng := newEngine(transport.New(backend))
	defer eng.shutdown()
	eng.state = newFakeGRStore()
	eng.gr.now = func() time.Time { return time.Unix(1_000_000, 0) }
	eng.setConfig(cfg)
	gr := grTestConfig()
	gr.RestarterSupport = grSupportPlannedAndUnplanned
	eng.gr.configure(gr)
	addressedTopology(eng)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	if !eng.gr.inRestart() {
		t.Fatal("an unplanned cold start did not enter in-restart")
	}
	held := false
	for _, lsa := range eng.lsdb.LinkLSAs("eth0") {
		if lsa.OpaqueType() == ospfpacket.GraceOpaqueType {
			held = true
		}
	}
	if !held {
		t.Fatal("the unplanned cold start originated no Grace-LSA on eth0")
	}

	// RFC requirement: RFC3623-5-2 positive -- on an unplanned cold start of a broadcast
	// interface the first packet sent is a Link State Update carrying the Grace-LSA, addressed
	// to AllSPFRouters (224.0.0.5), and the first Hello is sent after it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		sends, fails := backend.snapshot()
		if len(fails) > 0 {
			t.Fatalf("the engine sent a packet that does not decode: %v", fails)
		}
		helloAt := slices.IndexFunc(sends, func(s coldStartSend) bool { return s.packet.Hello != nil })
		if helloAt >= 0 {
			if !carriesGraceLSA(sends[0].packet) {
				t.Fatalf("the first packet sent on eth0 is not the Grace-LSA Link State Update: %+v", sends[0].packet)
			}
			if sends[0].target != transport.AllSPFRouters {
				t.Fatalf("the Grace-LSA went to %v, want AllSPFRouters %v", sends[0].target, transport.AllSPFRouters)
			}
			if helloAt == 0 {
				t.Fatal("a Hello was sent before the Grace-LSA")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no Hello within 5s of the cold start; sent %d packets", len(sends))
		}
		time.Sleep(20 * time.Millisecond)
	}
}
