// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 cryptographic authentication.
// Related: auth_wiring.go -- signPacket, which the transport applies to every packet it sends.
// Related: rfc4577_test.go -- the key store and verifier checked in isolation.
//
// VALIDATES: RFC 4577 Section 6, "OSPF 'cryptographic authentication' SHOULD be used between
// a PE and a CE", on the engine's own transmit path: a link whose config names a key chain
// sends Hellos that carry cryptographic authentication and that a router holding the same
// chain accepts.
// PREVENTS: a link that configured a key chain but whose Hello loop sends AuType 0 packets,
// which the key-store-only unit in rfc4577_test.go cannot see.
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

// linkAuthBackend opens linkAuthHandles that keep the raw bytes of every packet sent.
// Safe for concurrent use: the Hello loop sends from its own goroutine.
type linkAuthBackend struct {
	mu    sync.Mutex
	sends [][]byte
}

func (b *linkAuthBackend) OpenInterface(_ string, _ transport.DropRecorder) (transport.InterfaceHandle, error) {
	return &linkAuthHandle{backend: b, recv: make(chan transport.RawPacket, 1)}, nil
}

func (b *linkAuthBackend) snapshot() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.sends)
}

type linkAuthHandle struct {
	backend *linkAuthBackend
	recv    chan transport.RawPacket
	once    sync.Once
}

func (h *linkAuthHandle) IfIndex() int { return 1 }
func (h *linkAuthHandle) Send(_ netip.Addr, raw []byte) error {
	h.backend.mu.Lock()
	defer h.backend.mu.Unlock()
	h.backend.sends = append(h.backend.sends, slices.Clone(raw))
	return nil
}
func (h *linkAuthHandle) Recv() <-chan transport.RawPacket { return h.recv }
func (h *linkAuthHandle) JoinAllSPFRouters() error         { return nil }
func (h *linkAuthHandle) JoinAllDRouters() error           { return nil }
func (h *linkAuthHandle) LeaveAllDRouters() error          { return nil }
func (h *linkAuthHandle) Close() error {
	h.once.Do(func() { close(h.recv) })
	return nil
}

// TestRFC4577ConfiguredLinkSendsAuthenticatedHello checks the transmit path of a keyed link.
// Goal: the first Hello the engine sends on an interface whose config names an HMAC-SHA-256
// key chain carries AuType 2 and Key ID 1, and a second router configured from the same
// config text accepts it. Method: parse the config, open the interfaces over a recording
// transport with a one-second Hello interval, take the first Hello's raw bytes, and verify
// them with a separate key store.
func TestRFC4577ConfiguredLinkSendsAuthenticatedHello(t *testing.T) {
	const text = `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"broadcast","hello-interval":1,` +
		`"authentication":{"mode":"md5","key-chain":"ce-link"}}}},` +
		`"key-chains":{"ce-link":{"name":"ce-link","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}}}}`
	cfg, err := parseOSPFConfig(ospfSec(text), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	backend := &linkAuthBackend{}
	eng := newEngine(transport.New(backend))
	defer eng.shutdown()
	eng.state = newFakeGRStore()
	eng.setConfig(cfg)
	addressedTopology(eng)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}

	// RFC requirement: RFC4577-6-2 positive -- a link configured with a key chain uses OSPF
	// cryptographic authentication on what it sends: the first Hello the engine's Hello loop
	// transmits on eth0 carries AuType 2 (cryptographic) with Key ID 1 of the configured
	// chain, and a receiver key store configured from the same config text accepts it.
	hello := firstSentHello(t, backend)
	decoded, err := ospfpacket.DecodePacket(slices.Clone(hello))
	if err != nil {
		t.Fatalf("the sent Hello does not decode: %v", err)
	}
	if decoded.Header.AuType != ospfpacket.AuTypeCryptographic {
		t.Fatalf("the Hello sent on the keyed link carries AuType %d, want %d (cryptographic)",
			decoded.Header.AuType, ospfpacket.AuTypeCryptographic)
	}
	if keyID := hello[ospfpacket.CommonHeaderLen-8+2]; keyID != 1 {
		t.Fatalf("the Hello carries Key ID %d, want 1 from chain ce-link", keyID)
	}
	receiver := newAuthStore()
	receiver.configure(cfg)
	if reason, ok := receiver.verify("eth0", ridOf("10.0.0.1"), [4]byte{10, 0, 0, 1}, hello); !ok {
		t.Fatalf("a router holding the same chain refused the Hello: %s", reason)
	}
}

// firstSentHello waits up to five seconds for the engine to send a Hello and returns its bytes.
func firstSentHello(t *testing.T, backend *linkAuthBackend) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, raw := range backend.snapshot() {
			if len(raw) > 1 && raw[1] == byte(ospfpacket.PacketTypeHello) {
				return raw
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no Hello sent within 5s of opening the interface")
	return nil
}
