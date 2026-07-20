// VALIDATES: RFC 5340 (OSPF for IPv6) at the OSPFv3 engine seam -- the Hello carries the
// interface's Interface ID and correctly-set Options, the Database Description carries the
// same Options, the Network-LSA lists every router on the link, the Link-LSA lists every
// address the router has on that link, the NSSA/AS-External forwarding address is a global
// IPv6 address or absent, the OSPFv3 common header's Reserved octet is ignored on receive,
// and the IPv6 address family's interface cost / InfTransDelay are validated as positive.
// PREVENTS: an OSPFv3 Hello advertising the wrong (or a shared) Interface ID, an E/N/DC
// Options mismatch silently blocking adjacency, a Network-LSA omitting a fully adjacent
// router, a Link-LSA leaking another link's prefixes, an unspecified forwarding address
// being advertised, a non-zero Reserved octet rejecting a valid packet, and a zero cost /
// transmit-delay reaching the IPv6 engine.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospfiface "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/iface"
	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3packet "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

// optDC is the OSPFv3 demand-circuit Options bit (RFC 5340 §A.2, bit 5). ze implements no
// demand circuits (RFC 1793), so the bit is never set -- which is what "set correctly"
// means for a router without the capability. v3/types deliberately declares no constant
// for it, so the tests name the wire bit directly.
const optDC = ospfv3types.Options(0x000020)

// rfc5340Sender captures the packets an ospfiface.Interface transmits.
type rfc5340Sender struct {
	sent [][]byte
}

func (s *rfc5340Sender) SendPacket(_ string, _ netip.Addr, payload []byte) error {
	s.sent = append(s.sent, append([]byte(nil), payload...))
	return nil
}
func (s *rfc5340Sender) JoinAllDRouters(_ string) error  { return nil }
func (s *rfc5340Sender) LeaveAllDRouters(_ string) error { return nil }

// rfc5340SendHello builds a real OSPFv3 interface with the v6 Hello encoder, sends one
// Hello through Interface.SendHello, and returns the decoded OSPFv3 Hello body.
func rfc5340SendHello(t *testing.T, cfg ospfiface.Config) ospfv3packet.Hello {
	t.Helper()
	cfg.IsV6 = true
	if cfg.NetworkType == "" {
		cfg.NetworkType = ospfiface.NetworkPointToPoint
	}
	if cfg.HelloInterval == 0 {
		cfg.HelloInterval = DefaultHelloInterval
	}
	if cfg.DeadInterval == 0 {
		cfg.DeadInterval = DefaultDeadInterval
	}
	sender := &rfc5340Sender{}
	ifc := ospfiface.New(cfg, sender, ospfiface.NopMetrics())
	ifc.SetEncoder(v6Encoder{})
	require.NoError(t, ifc.SendHello())
	require.Len(t, sender.sent, 1, "the interface must transmit exactly one Hello")
	p, err := ospfv3packet.DecodePacket(sender.sent[0])
	require.NoError(t, err, "the emitted Hello must decode as an OSPFv3 packet")
	require.NotNil(t, p.Hello)
	return *p.Hello
}

// RFC requirement: RFC5340-4.2.1.1-1 positive -- before a Hello is sent, the interface's own
// Interface ID is copied into the Hello: an interface configured with Interface ID 42 emits a
// Hello whose Interface ID field is 42, and a second interface emits its own value, so the
// field tracks the interface rather than a constant (buildHelloPacketLocked copies
// cfg.InterfaceID into the neutral Hello, iface/iface.go:581-598; v6Encoder.EncodeHello writes
// it to the OSPFv3 Hello, encoder_v6.go:44-72).
// RFC requirement: RFC5340-4.2.1.1-1 negative -- the value copied is the LOCAL Interface ID,
// not one echoed from a neighbor: after receiving a neighbor Hello advertising Interface ID
// 99, the interface still advertises its own 42 (ReceiveDecodedHello stores the neighbor's
// Interface ID on the neighbor, iface/iface.go:486, and never touches cfg.InterfaceID).
func TestRFC5340HelloCarriesInterfaceID(t *testing.T) {
	base := ospfiface.Config{
		Name: "eth0", RouterID: ridOf("10.0.0.1"), AreaID: types.BackboneArea, InterfaceID: 42,
	}
	got := rfc5340SendHello(t, base)
	assert.Equal(t, ospfv3types.InterfaceID(42), got.InterfaceID, "the Hello must carry this interface's Interface ID")

	other := base
	other.Name = "eth1"
	other.InterfaceID = 43
	assert.Equal(t, ospfv3types.InterfaceID(43), rfc5340SendHello(t, other).InterfaceID,
		"a second interface must advertise its OWN Interface ID, not a shared value")

	// Negative: a neighbor's Interface ID must not become the value we advertise.
	sender := &rfc5340Sender{}
	cfg := base
	cfg.IsV6 = true
	cfg.HelloInterval = DefaultHelloInterval
	cfg.DeadInterval = DefaultDeadInterval
	cfg.NetworkType = ospfiface.NetworkPointToPoint
	ifc := ospfiface.New(cfg, sender, ospfiface.NopMetrics())
	ifc.SetEncoder(v6Encoder{})
	reason := ifc.ReceiveDecodedHello(ridOf("10.0.0.2"), netip.MustParseAddr("fe80::2"), packet.Hello{
		InterfaceID: 99, HelloInterval: DefaultHelloInterval, DeadInterval: uint32(DefaultDeadInterval),
		Options: types.Options(0).Set(types.OptionE), Priority: 1,
	}, time.Unix(1, 0))
	require.Empty(t, reason, "a matching neighbor Hello must be accepted")
	require.NoError(t, ifc.SendHello())
	require.Len(t, sender.sent, 1)
	p, err := ospfv3packet.DecodePacket(sender.sent[0])
	require.NoError(t, err)
	assert.Equal(t, ospfv3types.InterfaceID(42), p.Hello.InterfaceID,
		"the neighbor's Interface ID 99 must never replace this interface's own 42")
}

// RFC requirement: RFC5340-4.2.1.1-2 positive -- the Hello Options bits are set correctly for
// the area: a regular area sets the E-bit and an NSSA area sets the N-bit, alongside the V6 and
// R bits an active IPv6 router always sets (expectedOptionsLocked, iface/iface.go:895-905;
// neutralToV6Options, encoder_v6.go:77-86).
// RFC requirement: RFC5340-4.2.1.1-2 negative -- the bits are not set unconditionally: a stub
// area clears the E-bit, a non-NSSA area clears the N-bit, and the DC-bit is never set because
// ze implements no demand circuits, so its advertised capability is accurate
// (expectedOptionsLocked stub/NSSA branches, iface/iface.go:897-901; neutralToV6Options sets
// only V6|R|E|N, encoder_v6.go:77-86).
func TestRFC5340HelloOptionsBits(t *testing.T) {
	mk := func(areaType string) ospfv3types.Options {
		return rfc5340SendHello(t, ospfiface.Config{
			Name: "eth0", RouterID: ridOf("10.0.0.1"), AreaID: types.BackboneArea, InterfaceID: 42,
			AreaType: areaType,
		}).Options
	}

	normal := mk("")
	assert.True(t, normal.External(), "a regular area must set the E-bit")
	assert.False(t, normal.NSSA(), "a regular area must not set the N-bit")
	assert.True(t, normal.V6() && normal.Router(), "an active IPv6 router sets V6 and R")

	nssa := mk(ospfiface.AreaNSSA)
	assert.True(t, nssa.NSSA(), "an NSSA area must set the N-bit")
	assert.False(t, nssa.External(), "an NSSA area must not set the E-bit")

	stub := mk(ospfiface.AreaStub)
	assert.False(t, stub.External(), "a stub area must clear the E-bit")
	assert.False(t, stub.NSSA(), "a stub area must not set the N-bit")

	for name, opts := range map[string]ospfv3types.Options{"normal": normal, "nssa": nssa, "stub": stub} {
		assert.Zero(t, opts&optDC, "%s: the DC-bit must stay clear (no demand-circuit support)", name)
	}
}

// RFC requirement: RFC5340-4.2.1.2-1 positive -- the Database Description Options are encoded
// from the same neutral Options as the Hello, so an area that sets the E-bit produces a DD
// carrying E together with V6 and R (v6Encoder.EncodeDBDesc -> packetOptions ->
// neutralToV6Options, encoder_v6.go:35-41, 77-86, 90-104).
// RFC requirement: RFC5340-4.2.1.2-1 negative -- the DD Options are not a constant: a stub
// area's DD clears the E-bit, and the DC-bit is never set in a DD because ze implements no
// demand circuits (neutralToV6Options sets only V6|R|E|N, encoder_v6.go:77-86).
func TestRFC5340DBDescOptionsBits(t *testing.T) {
	enc := v6Encoder{}
	decode := func(neutral types.Options) ospfv3types.Options {
		buf := enc.EncodeDBDesc(ridOf("10.0.0.1"), types.BackboneArea, packet.DBDesc{
			Options: neutral, InterfaceMTU: 1500, DDSequence: 7,
		})
		p, err := ospfv3packet.DecodePacket(buf)
		require.NoError(t, err)
		require.NotNil(t, p.DBDesc)
		return p.DBDesc.Options
	}

	external := decode(types.Options(0).Set(types.OptionE))
	assert.True(t, external.External(), "a regular area's DD must carry the E-bit")
	assert.True(t, external.V6() && external.Router(), "an active IPv6 router's DD sets V6 and R")
	assert.Zero(t, external&optDC, "the DD DC-bit must stay clear (no demand-circuit support)")

	stub := decode(types.Options(0))
	assert.False(t, stub.External(), "a stub area's DD must clear the E-bit")
	assert.Zero(t, stub&optDC, "the DD DC-bit must stay clear for a stub area too")
}

// RFC requirement: RFC5340-2.8-3 positive -- the Network-LSA lists ALL routers connected to the
// link: the DR itself plus every neighbor that has reached Full (v6OriginateNetwork,
// origination_v6.go:225-233).
// RFC requirement: RFC5340-2.8-3 negative -- the list is not "every neighbor seen": a neighbor
// that has not reached Full is excluded, so the Network-LSA never claims an adjacency the DR
// does not have (v6OriginateNetwork Full gate, origination_v6.go:227-231).
func TestRFC5340NetworkLSAListsAttachedRouters(t *testing.T) {
	e := newV6OriginEngine()
	area := types.BackboneArea
	self := types.RouterID{172, 30, 0, 2}
	full := types.RouterID{172, 30, 0, 3}
	twoWay := types.RouterID{172, 30, 0, 4}

	iface := v6BroadcastInterface(area, self)
	iface.Neighbors = []ospflsdb.NeighborInfo{
		{RouterID: full, Address: netip.MustParseAddr("fe80::3"), State: ospflsdb.NeighborStateFull},
		{RouterID: twoWay, Address: netip.MustParseAddr("fe80::4"), State: "2-way"},
	}

	key, ok := e.v6OriginateNetwork(area, self, ospfv3types.OptV6|ospfv3types.OptE|ospfv3types.OptR, iface)
	require.True(t, ok, "the DR must originate a Network-LSA")
	lsa, ok := e.lsdb.LookupLSA(area, key)
	require.True(t, ok, "the Network-LSA must be installed")
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	require.NoError(t, err)
	body, err := decoded.DecodeNetwork()
	require.NoError(t, err)

	got := make(map[types.RouterID]bool, len(body.AttachedRouters))
	for _, r := range body.AttachedRouters {
		got[types.RouterID(r)] = true
	}
	assert.True(t, got[self], "the DR itself must be listed")
	assert.True(t, got[full], "a Full neighbor on the link must be listed")
	assert.False(t, got[twoWay], "a neighbor that is not Full must not be listed")
	assert.Len(t, body.AttachedRouters, 2, "attached routers = DR + the one Full neighbor")
}

// RFC requirement: RFC5340-2.8-4 positive -- the Link-LSA lists all of this router's addresses
// on the link: the link-local address in the dedicated field plus every global prefix
// configured on that interface (v6OriginateLinkLSA, origination_v6_link.go:41-65;
// v6PrefixesForLinkLSA, origination_v6_link.go:75-103).
// RFC requirement: RFC5340-2.8-4 negative -- the list is scoped to THIS link: an address that
// exists only on a different interface never appears in this interface's Link-LSA, because the
// prefix set is read from that interface alone (v6LinkLSAPrefixes, origination_v6_link.go:67-73).
func TestRFC5340LinkLSAListsLinkAddresses(t *testing.T) {
	e := newV6OriginEngine()
	area := types.BackboneArea
	self := types.RouterID{172, 30, 0, 2}

	iface := v6BroadcastInterface(area, self)
	iface.IPv6LinkLocal = netip.MustParseAddr("fe80::1")
	iface.IPv6Prefixes = []netip.Prefix{
		netip.MustParsePrefix("2001:db8:1::/64"),
		netip.MustParsePrefix("2001:db8:2::/64"),
		netip.MustParsePrefix("fe80::/64"),
	}
	e.lsdb.SetTx(func(string, netip.Addr, []byte) error { return nil })

	key, ok := e.v6OriginateLinkLSA(self, iface)
	require.True(t, ok, "an OSPFv3 interface must originate a Link-LSA")
	lsa, ok := e.lsdb.LookupLinkLSA("eth0", key)
	require.True(t, ok, "the Link-LSA must live in the eth0 link-local store")
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	require.NoError(t, err)
	link, err := decoded.DecodeLink()
	require.NoError(t, err)

	assert.Equal(t, netip.MustParseAddr("fe80::1"), netip.AddrFrom16(link.LinkLocalAddr),
		"the link-local address of this link must be carried in the Link-LSA")
	got := make(map[netip.Prefix]bool, len(link.Prefixes))
	for _, p := range link.Prefixes {
		pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
		require.True(t, ok)
		got[pfx] = true
	}
	assert.True(t, got[netip.MustParsePrefix("2001:db8:1::/64")], "every global prefix on the link must be listed")
	assert.True(t, got[netip.MustParsePrefix("2001:db8:2::/64")], "every global prefix on the link must be listed")

	// Negative: a prefix that only exists on eth1 must not appear in eth0's Link-LSA.
	other := iface
	other.Name = "eth1"
	other.InterfaceID = 11
	other.IPv6LinkLocal = netip.MustParseAddr("fe80::11")
	other.IPv6Prefixes = []netip.Prefix{netip.MustParsePrefix("2001:db8:9::/64")}
	otherKey, ok := e.v6OriginateLinkLSA(self, other)
	require.True(t, ok)
	require.NotEqual(t, key, otherKey, "each link gets its own Link-LSA")
	otherLSA, ok := e.lsdb.LookupLinkLSA("eth1", otherKey)
	require.True(t, ok)
	otherDecoded, err := ospfv3packet.DecodeLSA(otherLSA.RawBytes)
	require.NoError(t, err)
	otherLink, err := otherDecoded.DecodeLink()
	require.NoError(t, err)
	otherGot := make(map[netip.Prefix]bool, len(otherLink.Prefixes))
	for _, p := range otherLink.Prefixes {
		pfx, ok := v6PrefixToNetip(p, afIPv6Unicast)
		require.True(t, ok)
		otherGot[pfx] = true
	}
	assert.True(t, otherGot[netip.MustParsePrefix("2001:db8:9::/64")], "eth1's own prefix is in eth1's Link-LSA")
	assert.False(t, otherGot[netip.MustParsePrefix("2001:db8:1::/64")],
		"eth0's prefix must not be advertised in eth1's Link-LSA (OSPFv3 runs per link)")
	assert.False(t, got[netip.MustParsePrefix("2001:db8:9::/64")],
		"eth1's prefix must not be advertised in eth0's Link-LSA (OSPFv3 runs per link)")
	assert.Equal(t, netip.MustParseAddr("fe80::11"), netip.AddrFrom16(otherLink.LinkLocalAddr),
		"each Link-LSA carries the link-local address of its own link")
}

// RFC requirement: RFC5340-A.4.7-1 positive -- an NSSA/AS-external forwarding address that is a
// global IPv6 address is advertised as-is (v6OriginateNSSALSA, origination_v6_nssa.go:67-99).
// RFC requirement: RFC5340-A.4.7-1 negative -- the IPv6 Unspecified Address is never advertised
// as a forwarding address: an all-zero address clears HasForwardingAddr so the LSA carries no
// forwarding address at all (v6OriginateNSSALSA, origination_v6_nssa.go:71-73), and a
// link-local address is never selected as one in the first place
// (v6UsableForwardingAddress link-local exclusion, interface_addr.go:61-66, applied per
// candidate address by interfaceIPv6ForwardingAddress, interface_addr.go:78).
// RFC requirement: RFC5340-A.4.7-2 positive -- when a forwarding address IS advertised it is a
// global IPv6 address: only a global unicast address survives selection
// (interfaceIPv6ForwardingAddress, interface_addr.go:68-84) and it reaches the wire unchanged
// (v6OriginateNSSALSA, origination_v6_nssa.go:91-99).
// RFC requirement: RFC5340-A.4.7-2 negative -- a loopback, multicast, unspecified or link-local
// interface address yields no forwarding address (ok=false), so nothing non-global is ever
// advertised (v6UsableForwardingAddress, interface_addr.go:61-66).
func TestRFC5340ForwardingAddressIsGlobal(t *testing.T) {
	// The address-eligibility guard: only a global IPv6 address may become a forwarding address.
	assert.True(t, v6UsableForwardingAddress(netip.MustParseAddr("2001:db8::1")), "a global address is usable")
	for _, bad := range []string{"fe80::1", "::", "::1", "ff02::5"} {
		assert.False(t, v6UsableForwardingAddress(netip.MustParseAddr(bad)),
			"%s must never be advertised as an OSPFv3 forwarding address", bad)
	}

	e := newV6OriginEngine()
	nssa := types.AreaID{0, 0, 0, 9}
	self := types.RouterID{10, 0, 9, 1}
	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:9::/64"), 0)
	require.True(t, ok)
	global := netip.MustParseAddr("2001:db8:9::1").As16()

	require.True(t, e.v6OriginateNSSALSA(nssa, self, types.LinkStateID{0, 0, 0, 1}, prefix, false, 20, global, true, 0, false))
	body, ok := decodeV6External(t, e, nssa, v6NSSAKey(self, types.LinkStateID{0, 0, 0, 1}))
	require.True(t, ok)
	assert.True(t, body.HasForwardingAddr, "a global forwarding address must be advertised")
	assert.Equal(t, netip.MustParseAddr("2001:db8:9::1"), netip.AddrFrom16(body.ForwardingAddr))

	// The IPv6 Unspecified Address must not be advertised as a forwarding address.
	require.True(t, e.v6OriginateNSSALSA(nssa, self, types.LinkStateID{0, 0, 0, 2}, prefix, false, 20, [16]byte{}, true, 0, false))
	zero, ok := decodeV6External(t, e, nssa, v6NSSAKey(self, types.LinkStateID{0, 0, 0, 2}))
	require.True(t, ok)
	assert.False(t, zero.HasForwardingAddr, "the Unspecified Address must not be advertised as a forwarding address")
	assert.Equal(t, [16]byte{}, zero.ForwardingAddr)
}

// RFC requirement: RFC5340-A.4.8-1 positive -- an NSSA-LSA that is to be propagated by the NSSA
// area border router carries a global IPv6 forwarding address together with the P-bit that
// requests the propagation (v6OriginateNSSALSA propagate branch, origination_v6_nssa.go:78-90).
// RFC requirement: RFC5340-A.4.8-1 negative -- without a usable global forwarding address the
// LSA is NOT marked for propagation: the P-bit is cleared and no forwarding address is
// advertised, so a propagated NSSA-LSA can never lack the required global address
// (v6OriginateNSSALSA, origination_v6_nssa.go:79-80, 88-90).
func TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress(t *testing.T) {
	e := newV6OriginEngine()
	nssa := types.AreaID{0, 0, 0, 9}
	self := types.RouterID{10, 0, 9, 1}
	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:9::/64"), 0)
	require.True(t, ok)
	global := netip.MustParseAddr("2001:db8:9::1").As16()

	require.True(t, e.v6OriginateNSSALSA(nssa, self, types.LinkStateID{0, 0, 0, 1}, prefix, false, 20, global, true, 0, true))
	body, ok := decodeV6External(t, e, nssa, v6NSSAKey(self, types.LinkStateID{0, 0, 0, 1}))
	require.True(t, ok)
	assert.True(t, body.HasForwardingAddr, "a propagated NSSA-LSA must carry a forwarding address")
	assert.Equal(t, netip.MustParseAddr("2001:db8:9::1"), netip.AddrFrom16(body.ForwardingAddr))
	assert.NotZero(t, body.Prefix.Options&ospfv3types.OptPrefixP, "propagation is requested via the P-bit")

	require.True(t, e.v6OriginateNSSALSA(nssa, self, types.LinkStateID{0, 0, 0, 2}, prefix, false, 20, [16]byte{}, false, 0, true))
	none, ok := decodeV6External(t, e, nssa, v6NSSAKey(self, types.LinkStateID{0, 0, 0, 2}))
	require.True(t, ok)
	assert.False(t, none.HasForwardingAddr, "no global address means no forwarding address")
	assert.Zero(t, none.Prefix.Options&ospfv3types.OptPrefixP,
		"without a global forwarding address the NSSA-LSA must not be marked for propagation")
}

// RFC requirement: RFC5340-A.3.1-2 positive -- the reserved header field is ignored on receive:
// an OSPFv3 Hello whose Reserved octet (offset 15) is 0xFF is decoded and processed exactly
// like one carrying zero, forming the neighbor (the decoded Header carries no Reserved field
// and DecodeHeader never reads offReserved: v3/packet/header.go:95-104, 107-140).
// RFC requirement: RFC5340-A.3.1-2 negative -- "ignored" is confined to the Reserved octet: its
// neighbor at offset 14 is the Instance ID and IS honored, so a packet carrying a different
// Instance ID is still discarded and forms no neighbor (dispatcher Instance ID demux,
// dispatcher.go:85-91).
func TestRFC5340ReservedHeaderOctetIgnoredOnReceive(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	require.NoError(t, err)
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	require.NoError(t, eng.openInterfaces())
	t.Cleanup(eng.shutdown)

	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	require.NotNil(t, handle, "eth0 transport handle missing")

	src := netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	hello := ospfv3packet.Hello{
		InterfaceID: 2, Priority: 1,
		Options:       ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR,
		HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval,
	}
	// stamp writes an OSPFv3 Hello with explicit Instance ID / Reserved octets and
	// finalizes the IPv6 upper-layer checksum over the mutated bytes.
	stamp := func(peer types.RouterID, instance, reserved byte) []byte {
		p := ospfv3packet.Packet{
			Header: ospfv3packet.Header{
				Type: ospfv3packet.PacketTypeHello, RouterID: ospfv3types.RouterID(peer),
				AreaID: ospfv3types.AreaID(cfg.Areas[0].AreaID),
			},
			Hello: &hello,
		}
		buf := make([]byte, p.EncodedLen())
		p.WriteTo(buf, 0)
		buf[14] = instance
		buf[15] = reserved
		ospfv3packet.FinalizePacketChecksum(src, dst, buf)
		return buf
	}

	// A non-zero Reserved octet must be ignored: the neighbor forms.
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: src, Dst: dst, Payload: stamp(ridOf("10.0.0.2"), 0, 0xFF)})
	assert.Len(t, eng.neighborSnapshot(), 1, "a non-zero Reserved octet must not affect processing")

	// The adjacent Instance ID octet is NOT ignored: a mismatch is discarded.
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: src, Dst: dst, Payload: stamp(ridOf("10.0.0.3"), 7, 0xFF)})
	assert.Len(t, eng.neighborSnapshot(), 1,
		"an Instance ID mismatch must still be discarded; only the Reserved octet is ignored")
}

// RFC requirement: RFC5340-C.3-1 positive -- an IPv6 (OSPFv3) address-family interface with a
// positive output cost is accepted, and an unset cost (which the engine defaults) is accepted
// (validateConfigAF over cfg.V6, config.go:885-889, 945-952).
// RFC requirement: RFC5340-C.3-1 negative -- an OSPFv3 interface configured with output cost 0
// is rejected with ErrInterfaceCostZero, so a non-positive metric never reaches the IPv6 engine
// (validateConfigAF, config.go:886-889).
// RFC requirement: RFC5340-C.3-2 positive -- an OSPFv3 interface with InfTransDelay
// (transmit-delay) 1, the smallest positive value, is accepted (validateConfigAF,
// config.go:890-893).
// RFC requirement: RFC5340-C.3-2 negative -- an OSPFv3 interface configured with
// transmit-delay 0 is rejected with ErrTransmitDelayZero (validateConfigAF, config.go:890-893).
func TestRFC5340IPv6InterfaceCostAndTransmitDelay(t *testing.T) {
	mk := func(mut func(*interfaceConfig)) ospfConfig {
		ic := interfaceConfig{Name: "eth0", AreaID: types.BackboneArea, TransmitDelay: 1}
		mut(&ic)
		v6 := ospfConfig{
			present:    true,
			RouterID:   ridOf("10.0.0.1"),
			Areas:      []areaConfig{{AreaID: types.BackboneArea, NSSATranslateRole: translateRoleCandidate}},
			Interfaces: []interfaceConfig{ic},
		}
		return ospfConfig{
			present:  true,
			RouterID: ridOf("10.0.0.1"),
			Areas:    []areaConfig{{AreaID: types.BackboneArea, NSSATranslateRole: translateRoleCandidate}},
			V6:       &v6,
		}
	}

	require.NoError(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasCost = true; ic.Cost = 1 })),
		"an OSPFv3 interface cost of 1 is the smallest valid value")
	require.NoError(t, validateConfig(mk(func(*interfaceConfig) {})),
		"an unset OSPFv3 interface cost is defaulted by the engine, not rejected")
	require.ErrorIs(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasCost = true; ic.Cost = 0 })),
		ErrInterfaceCostZero, "an OSPFv3 interface cost of 0 must be rejected")

	require.NoError(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasTransmitDelay = true; ic.TransmitDelay = 1 })),
		"an OSPFv3 InfTransDelay of 1 is the smallest valid value")
	require.ErrorIs(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasTransmitDelay = true; ic.TransmitDelay = 0 })),
		ErrTransmitDelayZero, "an OSPFv3 InfTransDelay of 0 must be rejected")
}

// RFC requirement: RFC5340-C.3-3 positive -- a neighbor Hello whose HelloInterval equals this
// interface's is accepted, so routers sharing a link agree on HelloInterval
// (validateHelloLocked, iface/iface.go:870-872).
// RFC requirement: RFC5340-C.3-3 negative -- a neighbor Hello advertising a different
// HelloInterval is dropped with reason "hello-interval", so the adjacency never forms across a
// mismatch (validateHelloLocked, iface/iface.go:870-872).
// RFC requirement: RFC5340-C.3-4 positive -- a neighbor Hello whose RouterDeadInterval equals
// this interface's is accepted (validateHelloLocked, iface/iface.go:873-875).
// RFC requirement: RFC5340-C.3-4 negative -- a neighbor Hello advertising a different
// RouterDeadInterval is dropped with reason "dead-interval" (validateHelloLocked,
// iface/iface.go:873-875).
func TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink(t *testing.T) {
	ifc := ospfiface.New(ospfiface.Config{
		Name: "eth0", RouterID: ridOf("10.0.0.1"), AreaID: types.BackboneArea,
		NetworkType: ospfiface.NetworkPointToPoint, IsV6: true, InterfaceID: 42,
		HelloInterval: DefaultHelloInterval, DeadInterval: DefaultDeadInterval,
	}, &rfc5340Sender{}, ospfiface.NopMetrics())
	ifc.SetEncoder(v6Encoder{})

	peer := ridOf("10.0.0.2")
	src := netip.MustParseAddr("fe80::2")
	hello := func(helloInterval uint16, dead uint32) packet.Hello {
		return packet.Hello{
			InterfaceID: 2, Priority: 1, Options: types.Options(0).Set(types.OptionE),
			HelloInterval: helloInterval, DeadInterval: dead,
		}
	}

	assert.Empty(t, ifc.ReceiveDecodedHello(peer, src, hello(DefaultHelloInterval, uint32(DefaultDeadInterval)), time.Unix(1, 0)),
		"matching HelloInterval and RouterDeadInterval must be accepted")
	assert.Equal(t, "hello-interval",
		ifc.ReceiveDecodedHello(peer, src, hello(DefaultHelloInterval+1, uint32(DefaultDeadInterval)), time.Unix(2, 0)),
		"a differing HelloInterval must be dropped")
	assert.Equal(t, "dead-interval",
		ifc.ReceiveDecodedHello(peer, src, hello(DefaultHelloInterval, uint32(DefaultDeadInterval)+1), time.Unix(3, 0)),
		"a differing RouterDeadInterval must be dropped")
}
