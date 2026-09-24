// Design: docs/architecture/isis/isis-5-adjacency.md -- RFC 1195 live capability and ISH paths.
// Goal: exercise delivered configuration, circuit origination and receive dispatch.
// Method: capture final PDUs at the transport backend and inspect live adjacency state.

package isis

import (
	"bytes"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// protocolBackend preserves the existing wire capture while assigning distinct
// interface indices. A circuit rebuilt by reconcile gets a fresh capture.
type protocolBackend struct {
	mu       sync.Mutex
	next     int
	circuits map[string]*protocolCapture
}

type protocolCapture struct {
	*capturingCircuit
	index int
}

func (c *protocolCapture) IfIndex() int { return c.index }

func (b *protocolBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	c := &protocolCapture{
		capturingCircuit: &capturingCircuit{name: name, recv: make(chan transport.RawFrame)},
		index:            b.next,
	}
	b.circuits[name] = c
	return c, nil
}

func protocolEngine(t *testing.T, data string) (*engine, *protocolBackend) {
	t.Helper()
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	backend := &protocolBackend{circuits: make(map[string]*protocolCapture)}
	eng := newEngine(transport.New(backend))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.shutdown)
	return eng, backend
}

func protocolLiveCircuit(t *testing.T, eng *engine, name string) *circuit.Circuit {
	t.Helper()
	eng.circuitsMu.RLock()
	c := eng.circuitByName[name]
	eng.circuitsMu.RUnlock()
	if c == nil {
		t.Fatalf("missing live circuit %s", name)
	}
	return c
}

func protocolCaptureFor(t *testing.T, backend *protocolBackend, name string) *protocolCapture {
	t.Helper()
	backend.mu.Lock()
	c := backend.circuits[name]
	backend.mu.Unlock()
	if c == nil {
		t.Fatalf("missing transport capture %s", name)
	}
	return c
}

func protocolTLV(t *testing.T, tlvs []packet.TLV) []byte {
	t.Helper()
	for _, tlv := range tlvs {
		if tlv.Type == packet.TLVProtocolsSupported {
			return bytes.Clone(tlv.Value)
		}
	}
	t.Fatal("no Protocols Supported TLV")
	return nil
}

func protocolLastHello(t *testing.T, c *protocolCapture) []byte {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range slices.Backward(c.sent) {
		p, err := packet.DecodePDU(v)
		if err != nil {
			continue
		}
		if p.LANHello != nil {
			protocols := protocolTLV(t, p.LANHello.TLVs)
			packet.ReleaseTLVs(p.LANHello.TLVs)
			return protocols
		}
		if p.P2PHello != nil {
			protocols := protocolTLV(t, p.P2PHello.TLVs)
			packet.ReleaseTLVs(p.P2PHello.TLVs)
			return protocols
		}
		if p.LSP != nil {
			packet.ReleaseTLVs(p.LSP.TLVs)
		}
	}
	t.Fatal("no transmitted IIH")
	return nil
}

const protocolDualConfig = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{
"eth0":{"level":"l1","hello-interval":"3600"},
"eth1":{"level":"l1","hello-interval":"3600","address-family":{"ipv6-unicast":{}}}}}}}`

const protocolIPv4Config = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{
"eth0":{"level":"l1","hello-interval":"3600"},
"eth1":{"level":"l1","hello-interval":"3600"}}}}}`

// RFC requirement: RFC1195-4.4-3 positive -- enabling IPv6 on only one interface
// produces identical IPv4+IPv6 Protocols Supported values in both interfaces'
// transmitted IIHs and the node's fragment-zero LSP.
func TestRFC1195NodeProtocolsAcrossInterfaces(t *testing.T) {
	eng, backend := protocolEngine(t, protocolDualConfig)
	want := []byte{packet.NLPIDIPv4, packet.NLPIDIPv6}
	for _, name := range []string{"eth0", "eth1"} {
		if err := protocolLiveCircuit(t, eng, name).SendHello(adjacency.Level1); err != nil {
			t.Fatal(err)
		}
		if got := protocolLastHello(t, protocolCaptureFor(t, backend, name)); !bytes.Equal(got, want) {
			t.Fatalf("%s protocols %x, want %x", name, got, want)
		}
	}
	eng.originate()
	id := types.NewLSPID(types.NewSourceID(eng.cfg.SystemID, 0), 0)
	entry := eng.lsdb.Lookup(lsdb.Level1, id)
	if entry == nil {
		t.Fatal("missing own fragment zero")
	}
	p, err := packet.DecodePDU(entry.Raw())
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(p.LSP.TLVs)
	if got := protocolTLV(t, p.LSP.TLVs); !bytes.Equal(got, want) {
		t.Fatalf("LSP protocols %x, want %x", got, want)
	}
}

// RFC requirement: RFC1195-4.4-3 negative -- removing the only IPv6-enabled
// interface family cannot leave stale IPv6 capability in an unchanged interface's
// transmitted IIH; after reload both circuits advertise only IPv4.
func TestRFC1195NodeProtocolsReload(t *testing.T) {
	eng, backend := protocolEngine(t, protocolDualConfig)
	unchanged := protocolLiveCircuit(t, eng, "eth0")
	reconcileTo(t, eng, protocolIPv4Config)
	if protocolLiveCircuit(t, eng, "eth0") != unchanged {
		t.Fatal("unmodified circuit was rebuilt rather than updated")
	}
	for _, name := range []string{"eth0", "eth1"} {
		if err := protocolLiveCircuit(t, eng, name).SendHello(adjacency.Level1); err != nil {
			t.Fatal(err)
		}
		if got := protocolLastHello(t, protocolCaptureFor(t, backend, name)); !bytes.Equal(got, []byte{packet.NLPIDIPv4}) {
			t.Fatalf("%s retained incompatible capability %x", name, got)
		}
	}
}

// RFC requirement: RFC1195-1.4-2 positive -- the configured IP router advertises
// IPv4 capability on the actual transmitted Hello even when no IP address is
// available from its transport backend.
func TestRFC1195PureIPAdvertisesIPv4(t *testing.T) {
	eng, backend := protocolEngine(t, protocolIPv4Config)
	if err := protocolLiveCircuit(t, eng, "eth0").SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	if got := protocolLastHello(t, protocolCaptureFor(t, backend, "eth0")); !bytes.Equal(got, []byte{packet.NLPIDIPv4}) {
		t.Fatalf("pure-IP node advertised %x", got)
	}
}

// RFC requirement: RFC1195-1.4-2 negative -- a per-interface IPv6-only family
// selection cannot make this RFC 1195 router omit its IPv4 capability on the wire.
func TestRFC1195IPv6SelectionDoesNotRemoveIPv4(t *testing.T) {
	eng, backend := protocolEngine(t, protocolDualConfig)
	if err := protocolLiveCircuit(t, eng, "eth1").SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	if got := protocolLastHello(t, protocolCaptureFor(t, backend, "eth1")); !bytes.Contains(got, []byte{packet.NLPIDIPv4}) {
		t.Fatalf("IPv6 selection removed IPv4: %x", got)
	}
}

const protocolP2PConfig = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{
"eth0":{"level":"l1","hello-interval":"3600","circuit-type":"point-to-point"}}}}}`

func protocolPeerISH(t *testing.T, password string) []byte {
	t.Helper()
	net, err := types.ParseNET("49.0001.0000.0000.0002.00")
	if err != nil {
		t.Fatal(err)
	}
	h := packet.ISH{NET: net, HoldingTime: 30, TLVs: []packet.TLV{
		{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
	}}
	buf := make([]byte, 254)
	pdu := buf[:h.WriteTo(buf, 0)]
	if password != "" {
		pdu, err = packet.SignISH(pdu, packet.Key{Algorithm: packet.AuthAlgoCleartext, Secret: []byte(password)})
		if err != nil {
			t.Fatal(err)
		}
	}
	return pdu
}

// RFC requirement: RFC1195-4.4-2 positive -- a configured P2P circuit transmits
// an ISO 9542 ISH with its configured NET and IPv4 capability before its IIH,
// and an ISH delivered through the production dispatcher initializes its peer.
func TestRFC1195ISHTransmitReceive(t *testing.T) {
	eng, backend := protocolEngine(t, protocolP2PConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	capture := protocolCaptureFor(t, backend, "eth0")
	capture.mu.Lock()
	var sent []byte
	for i, pdu := range capture.sent {
		if len(pdu) == 0 {
			continue
		}
		if pdu[0] == packet.ESISProtocolDiscriminator {
			if capture.dests[i] != transport.AllESs {
				capture.mu.Unlock()
				t.Fatal("ISH not sent to ISO AllESs group")
			}
			sent = bytes.Clone(pdu)
			break
		}
		if len(pdu) > 4 {
			if packet.PDUType(pdu[4]&0x1f) == packet.PDUTypeP2PHello {
				capture.mu.Unlock()
				t.Fatal("IIH transmitted before initial ISH")
			}
		}
	}
	capture.mu.Unlock()
	h, err := packet.DecodeISH(sent)
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(h.TLVs)
	if !h.NET.Equal(eng.cfg.NETs[0]) {
		t.Fatalf("ISH NET %s does not match configured NET", h.NET)
	}
	if got := protocolTLV(t, h.TLVs); !bytes.Equal(got, []byte{packet.NLPIDIPv4}) {
		t.Fatalf("ISH protocols %x", got)
	}
	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), SrcMAC: [6]byte{2, 0, 0, 0, 0, 2}, PDU: protocolPeerISH(t, "")})
	rows := c.Table().Snapshot()
	if len(rows) != 1 {
		t.Fatalf("ISH did not initialize one adjacency: %+v", rows)
	}
	if rows[0].State != "initializing" || rows[0].Protocols != adjacency.ProtocolIPv4 {
		t.Fatalf("ISH discovery state %+v", rows[0])
	}
	// RFC 5303 keeps its three-way state Down until an IIH has been heard;
	// ISO 9542 discovery must not become proof of a completed Hello exchange.
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	wire, _, ok := capture.lastSent()
	if !ok {
		t.Fatal("no IIH after ISH discovery")
	}
	p, err := packet.DecodePDU(wire)
	if err != nil || p.P2PHello == nil {
		t.Fatalf("decoding post-ISH IIH: %v", err)
	}
	defer packet.ReleaseTLVs(p.P2PHello.TLVs)
	for _, tlv := range p.P2PHello.TLVs {
		if tlv.Type != packet.TLVP2PThreeWay {
			continue
		}
		threeWay, err := packet.DecodeP2PThreeWayTLV(tlv.Value)
		if err != nil {
			t.Fatal(err)
		}
		if threeWay.State != packet.AdjThreeWayDown || threeWay.HasNeighbor {
			t.Fatalf("ISH discovery bypassed three-way handshake: %+v", threeWay)
		}
		return
	}
	t.Fatal("post-ISH IIH omitted three-way state")
}

// RFC requirement: RFC1195-4.4-2 negative -- malformed ISO 9542 ISH packets
// delivered through the dispatcher cannot initialize an adjacency; valid ISHs
// are also rejected on broadcast circuits rather than treated as LAN IIHs.
func TestRFC1195ISHRejectsCorruptionAndWrongCircuit(t *testing.T) {
	for _, data := range []string{protocolP2PConfig, protocolIPv4Config} {
		eng, _ := protocolEngine(t, data)
		c := protocolLiveCircuit(t, eng, "eth0")
		pdu := protocolPeerISH(t, "")
		if data == protocolP2PConfig {
			pdu[7] ^= 1
		}
		eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: pdu})
		if rows := c.Table().Snapshot(); len(rows) != 0 {
			t.Fatalf("invalid ISH created adjacency %+v", rows)
		}
	}
}
