// VALIDATES: spec-ospf-af-unify -- a second (IPv6/OSPFv3) engine instance constructs over the
// REAL ospfv3 transport (with an in-memory backend) and the v6 codec, and enrolls its configured
// interface. This is the faithful counterpart to TestOSPFEngineIPv6HelloFormsNeighbor (which
// exercises the v6 codec over a stand-in transport): here the engine's Transport interface is
// satisfied by *ospfv3transport.Transport at runtime, not just at compile time.
// PREVENTS: the engine silently being unable to drive the ospfv3 transport (e.g. an interface
// method the engine needs that ospfv3/transport does not provide).
package ospf

import (
	"net/netip"
	"sync"
	"testing"

	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// fakeV6Backend is an in-memory ospfv3 transport backend (no raw IPv6 sockets) so a v6 engine
// instance can be exercised on any platform; it mirrors the v4 fakeBackend.
type fakeV6Backend struct {
	mu      sync.Mutex
	nextIdx int
	handles map[string]*fakeV6Handle
}

func (b *fakeV6Backend) OpenInterface(name string, _ ospfv3transport.DropRecorder) (ospfv3transport.InterfaceHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handles == nil {
		b.handles = make(map[string]*fakeV6Handle)
	}
	b.nextIdx++
	h := &fakeV6Handle{ifindex: b.nextIdx, src: netip.MustParseAddr("fe80::1"), recv: make(chan ospfv3transport.RawPacket, 8)}
	b.handles[name] = h
	return h, nil
}

type fakeV6Handle struct {
	ifindex int
	src     netip.Addr
	recv    chan ospfv3transport.RawPacket
	once    sync.Once
}

func (h *fakeV6Handle) IfIndex() int                           { return h.ifindex }
func (h *fakeV6Handle) LinkLocalSource() netip.Addr            { return h.src }
func (h *fakeV6Handle) Send(_, _ netip.Addr, _ []byte) error   { return nil }
func (h *fakeV6Handle) Recv() <-chan ospfv3transport.RawPacket { return h.recv }
func (h *fakeV6Handle) JoinAllSPFRouters() error               { return nil }
func (h *fakeV6Handle) JoinAllDRouters() error                 { return nil }
func (h *fakeV6Handle) LeaveAllDRouters() error                { return nil }
func (h *fakeV6Handle) Close() error                           { h.once.Do(func() { close(h.recv) }); return nil }

func TestOSPFEngineIPv6FamilyStarts(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 was not parsed into cfg.V6")
	}

	// The engine's Transport interface is satisfied by the real ospfv3 transport.
	eng := newEngineWithCodecAF(ospfv3transport.New(&fakeV6Backend{}), v6Codec{}, afIPv6Unicast)
	eng.setConfig(*cfg.V6)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	snaps := eng.interfaceSnapshot()
	if len(snaps) != 1 {
		t.Fatalf("v6 engine enrolled %d interfaces, want 1 (eth0 over the ospfv3 transport)", len(snaps))
	}
}
