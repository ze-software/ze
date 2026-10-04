// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 cryptographic authentication and replay.
// Related: auth_wiring.go -- signPacket and verifyPacket, the engine hooks that pick the Apad source.
// Related: auth_keystore.go -- authStore.configure, which reads the interface address for signing.
//
// VALIDATES: RFC 7474 Section 5: the sender initializes the first 4 octets of Apad to the IP
// source address it sends from, the remainder is 0x878FE1F3 repeated, and the receiver
// initializes them to the IP source address of the incoming packet's IP header.
// PREVENTS: an engine that binds the router ID, a zero address or its own address into Apad,
// which every packet-level test would still pass.
//
// Method: only the OS address source is substituted, through a registered iface backend whose
// GetInterface reports eth0 as 192.0.2.1/24. Configuration, the key chain, the address read
// (interfaceIPv4Address), the transport signer hook and the dispatcher are the production path.
package ospf

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const rfc7474ApadBackendName = "ospf-rfc7474-apad-test"

// rfc7474ApadSecret is the key chain's one HMAC-SHA-256 secret, key ID 1.
var rfc7474ApadSecret = []byte("apad-key")

// rfc7474AddressBackend reports eth0 as 192.0.2.1/24, ifindex 1, no link speed; the
// backend methods the engine does not call are absent.
type rfc7474AddressBackend struct{ iface.Backend }

func rfc7474Eth0() iface.InterfaceInfo {
	addresses := []iface.AddrInfo{{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"}}
	return iface.InterfaceInfo{Name: "eth0", OsName: "eth0", Index: 1, State: "up", MTU: 1500, Addresses: addresses}
}

func (*rfc7474AddressBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if name != "eth0" {
		return nil, fmt.Errorf("unknown test interface %s", name)
	}
	info := rfc7474Eth0()
	return &info, nil
}

func (*rfc7474AddressBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	return []iface.InterfaceInfo{rfc7474Eth0()}, nil
}

func (*rfc7474AddressBackend) LinkSpeedDuplex(string) (int, string) { return 0, "" }

func (*rfc7474AddressBackend) Close() error { return nil }

var registerRFC7474AddressBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend(rfc7474ApadBackendName, func() (iface.Backend, error) {
		return &rfc7474AddressBackend{}, nil
	})
})

// rfc7474RawBackend opens handles that keep the raw bytes of every packet the engine sends.
// Safe for concurrent use: the Hello loop sends from its own goroutine.
type rfc7474RawBackend struct {
	mu    sync.Mutex
	sends [][]byte
}

func (b *rfc7474RawBackend) OpenInterface(_ string, _ transport.DropRecorder) (transport.InterfaceHandle, error) {
	return &rfc7474RawHandle{backend: b, recv: make(chan transport.RawPacket, 1)}, nil
}

func (b *rfc7474RawBackend) firstHello(t *testing.T) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		b.mu.Lock()
		for _, raw := range b.sends {
			if len(raw) > 1 && packet.PacketType(raw[1]) == packet.PacketTypeHello {
				b.mu.Unlock()
				return raw
			}
		}
		b.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("no Hello sent within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type rfc7474RawHandle struct {
	backend *rfc7474RawBackend
	recv    chan transport.RawPacket
	once    sync.Once
}

func (h *rfc7474RawHandle) IfIndex() int { return 1 }
func (h *rfc7474RawHandle) Send(_ netip.Addr, raw []byte) error {
	h.backend.mu.Lock()
	defer h.backend.mu.Unlock()
	h.backend.sends = append(h.backend.sends, slices.Clone(raw))
	return nil
}
func (h *rfc7474RawHandle) Recv() <-chan transport.RawPacket { return h.recv }
func (h *rfc7474RawHandle) JoinAllSPFRouters() error         { return nil }
func (h *rfc7474RawHandle) JoinAllDRouters() error           { return nil }
func (h *rfc7474RawHandle) LeaveAllDRouters() error          { return nil }
func (h *rfc7474RawHandle) Close() error {
	h.once.Do(func() { close(h.recv) })
	return nil
}

// installOSPFAddressBackend supplies eth0 as 192.0.2.1/24 through the production
// iface registry, independent of host interfaces. Callers MUST NOT run in parallel
// and MUST stop their engines before cleanup restores the previous backend.
func installOSPFAddressBackend(t *testing.T) {
	t.Helper()
	if err := registerRFC7474AddressBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend(rfc7474ApadBackendName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
	if got := interfaceIPv4Address("eth0"); got != [4]byte{192, 0, 2, 1} {
		t.Fatalf("precondition: eth0 address %v, want 192.0.2.1 from the iface backend", got)
	}
	if got := interfaceNetworkMask("eth0"); got != [4]byte{255, 255, 255, 0} {
		t.Fatalf("precondition: eth0 mask %v, want /24 from the iface backend", got)
	}
}

// rfc7474ApadEngine loads the address backend, then runs an engine whose broadcast eth0 is
// authenticated by an AuType 3 (extended-sequence) HMAC-SHA-256 key chain.
func rfc7474ApadEngine(t *testing.T) (*engine, *rfc7474RawBackend) {
	t.Helper()
	// MUST stop the engine before installOSPFAddressBackend restores the backend.
	installOSPFAddressBackend(t)
	const data = `{"ospf":{"router-id":"10.0.0.1",` +
		`"areas":{"area":{"0":{"area-id":"0","authentication":{"key-chain":"kc1"}}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0","network-type":"broadcast","hello-interval":1,"authentication":{"mode":"inherit"}}}},` +
		`"key-chains":{"kc1":{"name":"kc1","extended-sequence":true,"key":{` +
		`"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"apad-key"}}}}}}`
	cfg, err := parseOSPFConfig(ospfSec(data), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	backend := &rfc7474RawBackend{}
	eng := newEngine(transport.New(backend))
	t.Cleanup(eng.shutdown)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	return eng, backend
}

// rfc7474Apad builds a 32-octet Apad: source, then remainder repeated 7 times.
func rfc7474Apad(source [4]byte, remainder uint32) []byte {
	apad := make([]byte, sha256.Size)
	copy(apad, source[:])
	for off := 4; off < len(apad); off += 4 {
		binary.BigEndian.PutUint32(apad[off:], remainder)
	}
	return apad
}

// rfc7474ExpectedDigest computes, independently of the packet codec, the RFC 7474 AuType 3
// HMAC-SHA-256 digest of an OSPF packet of length octets: the key is the secret followed by
// the OSPFv2 Cryptographic Protocol ID 00 01 (Section 6), and the HMAC covers the packet,
// its 8-octet sequence number, then apad (Section 5).
func rfc7474ExpectedDigest(wire []byte, length int, apad []byte) []byte {
	key := append(slices.Clone(rfc7474ApadSecret), 0x00, 0x01)
	mac := hmac.New(sha256.New, key)
	mac.Write(wire[:length+8])
	mac.Write(apad)
	return mac.Sum(nil)
}

// rfc7474SentDigest returns the first Hello the engine sent and the digest it appended
// after the packet and its 8-octet sequence number.
func rfc7474SentDigest(t *testing.T, backend *rfc7474RawBackend) (wire []byte, length int, digest []byte) {
	t.Helper()
	wire = backend.firstHello(t)
	if packet.AuType(binary.BigEndian.Uint16(wire[14:16])) != packet.AuTypeCryptographicESN {
		t.Fatalf("sent Hello AuType %d, want 3", binary.BigEndian.Uint16(wire[14:16]))
	}
	length = int(binary.BigEndian.Uint16(wire[2:4]))
	if len(wire) != length+8+sha256.Size {
		t.Fatalf("sent Hello is %d octets, want the %d-octet packet, an 8-octet sequence and a 32-octet digest", len(wire), length)
	}
	return wire, length, wire[length+8:]
}

// RFC requirement: RFC7474-5-2 positive -- the AuType 3 Hello the engine sends on eth0 (192.0.2.1/24 from the iface backend) carries the HMAC-SHA-256 of the packet followed by an Apad of 192.0.2.1 then 0x878FE1F3 repeated 7 times, computed here without the packet codec.
func TestRFC7474SentApadIsInterfaceSource(t *testing.T) {
	_, backend := rfc7474ApadEngine(t)
	wire, length, digest := rfc7474SentDigest(t, backend)
	want := rfc7474ExpectedDigest(wire, length, rfc7474Apad([4]byte{192, 0, 2, 1}, 0x878FE1F3))
	if !hmac.Equal(digest, want) {
		t.Fatalf("sent digest % x, want % x (Apad 192.0.2.1 then 0x878FE1F3)", digest, want)
	}
}

// RFC requirement: RFC7474-5-2 negative -- the digest of the Hello the engine sends on eth0 is not the one an Apad of the router ID 10.0.0.1, of 0.0.0.0, or of 192.0.2.1 followed by zeros would give: neither another address nor another remainder is bound.
func TestRFC7474SentApadNotRouterIDOrZero(t *testing.T) {
	_, backend := rfc7474ApadEngine(t)
	wire, length, digest := rfc7474SentDigest(t, backend)
	for _, wrong := range [][4]byte{{10, 0, 0, 1}, {}} {
		if hmac.Equal(digest, rfc7474ExpectedDigest(wire, length, rfc7474Apad(wrong, 0x878FE1F3))) {
			t.Fatalf("sent digest binds %v into Apad, not the sending address 192.0.2.1", wrong)
		}
	}
	if hmac.Equal(digest, rfc7474ExpectedDigest(wire, length, rfc7474Apad([4]byte{192, 0, 2, 1}, 0))) {
		t.Fatal("sent digest uses a zero Apad remainder, not 0x878FE1F3")
	}
}

// rfc7474SignedHello builds a Hello from 2.2.2.2 in the backbone, signed with AuType 3,
// key ID 1, sequence 1<<32|1, and an Apad starting with apadSource.
func rfc7474SignedHello(t *testing.T, apadSource [4]byte) []byte {
	t.Helper()
	hello := packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 1, Options: types.OptionE,
		Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: ridOf("2.2.2.2"),
		AreaID: types.BackboneArea, AuType: packet.AuTypeCryptographicESN}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	key := packet.AuthKey{KeyID: 1, Algorithm: packet.AuthHMACSHA256, Secret: rfc7474ApadSecret}
	signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographicESN, key, 1<<32|1, apadSource)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return signed
}

// rfc7474Receive dispatches wire on eth0 as if its IP header source were source, and returns
// the dispatcher's drop count for it and the neighbor rows after it.
func rfc7474Receive(eng *engine, source string, wire []byte) (uint64, int) {
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: 1, Src: netip.MustParseAddr(source), Payload: wire})
	return eng.dispatch.dropped() - before, len(eng.neighborSnapshot())
}

// RFC requirement: RFC7474-5-2 positive -- a Hello signed with an Apad of 192.0.2.2 and received on eth0 with IP header source 192.0.2.2 authenticates: it is not dropped and 2.2.2.2 becomes a neighbor.
func TestRFC7474ReceiveApadFromIPHeaderSource(t *testing.T) {
	eng, _ := rfc7474ApadEngine(t)
	dropped, neighbors := rfc7474Receive(eng, "192.0.2.2", rfc7474SignedHello(t, [4]byte{192, 0, 2, 2}))
	if dropped != 0 || neighbors != 1 {
		t.Fatalf("dropped %d, neighbors %d; want 0 and 1 for a Hello whose Apad is its IP source", dropped, neighbors)
	}
}

// RFC requirement: RFC7474-5-2 negative -- a Hello received on eth0 with IP header source 192.0.2.2 is dropped, and 2.2.2.2 never becomes a neighbor, when its Apad was 192.0.2.3 (another source on the same network), 192.0.2.1 (the receiving interface's own address) or 0.0.0.0.
func TestRFC7474ReceiveApadOtherThanIPHeaderSourceDropped(t *testing.T) {
	for _, apad := range [][4]byte{{192, 0, 2, 3}, {192, 0, 2, 1}, {}} {
		eng, _ := rfc7474ApadEngine(t)
		dropped, neighbors := rfc7474Receive(eng, "192.0.2.2", rfc7474SignedHello(t, apad))
		if dropped != 1 || neighbors != 0 {
			t.Fatalf("Apad %v from IP source 192.0.2.2: dropped %d, neighbors %d; want 1 and 0", apad, dropped, neighbors)
		}
	}
}
