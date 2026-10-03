// VALIDATES: RFC 5340 §A.3.1 -- an OSPFv3 engine built exactly as production builds it
// (newEngineWithCodecAF over the real ospfv3 transport) finalizes the IPv6 upper-layer
// checksum on every packet it sends.
// PREVENTS: the regression this test was written for -- installAuthHooks used to install the
// OSPFv2 RFC 2328 App-D signer on EVERY engine including the OSPFv3 ones. The v3 transport
// treats a non-nil signer as "an RFC 7166 Authentication Trailer owns integrity" and skips
// FinalizePacketChecksum (v3/transport/transport.go:496-502), so every OSPFv3 packet left the
// box with a zero (invalid) IPv6 upper-layer checksum, which a conforming peer discards.
package ospf

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// v3SendBackend is an in-memory ospfv3 transport backend that records what was transmitted.
type v3SendBackend struct {
	mu sync.Mutex
	h  *v3SendHandle
}

func (b *v3SendBackend) OpenInterface(_ string, _ ospfv3transport.DropRecorder) (ospfv3transport.InterfaceHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.h = &v3SendHandle{
		ifindex: 7,
		src:     netip.MustParseAddr("fe80::1"),
		recv:    make(chan ospfv3transport.RawPacket, 8),
	}
	return b.h, nil
}

func (b *v3SendBackend) handle() *v3SendHandle {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.h
}

type v3SendHandle struct {
	ifindex int
	src     netip.Addr
	recv    chan ospfv3transport.RawPacket
	once    sync.Once
	mu      sync.Mutex
	sent    [][]byte
}

func (h *v3SendHandle) IfIndex() int                { return h.ifindex }
func (h *v3SendHandle) LinkLocalSource() netip.Addr { return h.src }

func (h *v3SendHandle) Send(_, _ netip.Addr, payload []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = append(h.sent, append([]byte(nil), payload...))
	return nil
}

func (h *v3SendHandle) Recv() <-chan ospfv3transport.RawPacket { return h.recv }
func (h *v3SendHandle) JoinAllSPFRouters() error               { return nil }
func (h *v3SendHandle) JoinAllDRouters() error                 { return nil }
func (h *v3SendHandle) LeaveAllDRouters() error                { return nil }
func (h *v3SendHandle) Close() error                           { h.once.Do(func() { close(h.recv) }); return nil }

func (h *v3SendHandle) first() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sent) == 0 {
		return nil
	}
	return h.sent[0]
}

// TestOSPFv3EngineFinalizesPacketChecksum drives the production construction path: an OSPFv3
// address-family engine (v6 codec + ospfv3 transport, register_multiaf.go:51-53) sends a packet
// and the IPv6 upper-layer checksum must be present and verify against the egress link-local
// source and the destination (RFC 5340 §A.3.1).
func TestOSPFv3EngineFinalizesPacketChecksum(t *testing.T) {
	be := &v3SendBackend{}
	tr := ospfv3transport.New(be)
	eng := newEngineWithCodecAF(tr, v6Codec{}, afIPv6Unicast)
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}}}`), nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.V6)
	eng.setConfig(*cfg.V6)
	require.NoError(t, eng.openInterfaces())
	t.Cleanup(eng.shutdown)

	dst := netip.MustParseAddr("ff02::5")
	hello := ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello},
		Hello:  &ospfv3packet.Hello{InterfaceID: 7, HelloInterval: 10, RouterDeadInterval: 40},
	}
	buf := make([]byte, hello.EncodedLen())
	hello.WriteTo(buf, 0)
	require.NoError(t, tr.SendPacket("eth0", dst, buf))

	h := be.handle()
	require.NotNil(t, h, "the ospfv3 transport never opened eth0")
	sent := h.first()
	require.NotNil(t, sent, "nothing was transmitted")

	assert.False(t, sent[12] == 0 && sent[13] == 0,
		"the OSPFv3 packet checksum field is still zero: the transport did not finalize it")
	assert.True(t, ospfv3packet.VerifyPacketChecksum(h.src, dst, sent),
		"the transmitted OSPFv3 packet must carry a checksum that verifies against its IPv6 source and destination")
}
