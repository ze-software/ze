// VALIDATES: spec-ospf-ext-14 AC-12, A-11 -- `show ospf ipv6 instance` enumerates each
// active OSPFv3 address-family instance with its AF (from the Instance-ID range), Instance
// ID, area count, and neighbor count; a single base IPv6-unicast instance yields exactly one
// row; a second AF yields two.
// VALIDATES: spec-ospfv3-views-every-address-family AC-1 to AC-5, A-1 -- every other OSPFv3
// view answers for each running address family, each row led by its `address-family` and
// `instance-id`, and `address-family <af>` narrows it to one running AF or is refused.
// PREVENTS: an instance listing that hides the address family, or hard-depends on multi-AF;
// a view that reads only the IPv6-unicast engine while another AF's adjacency is Full.
package ospf

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestV3InstanceListingSingleAF(t *testing.T) {
	eng := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	m := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: eng}}

	rows := m.instanceListing()
	if len(rows) != 1 {
		t.Fatalf("instance rows = %d, want 1", len(rows))
	}
	if rows[0].AddressFamily != afIPv6Unicast.String() {
		t.Fatalf("address family = %q, want %q", rows[0].AddressFamily, afIPv6Unicast.String())
	}
}

func TestV3InstanceListingMultiAF(t *testing.T) {
	e6 := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	e4 := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv4Unicast)
	e4.cfg.RouterID = types.RouterID{10, 0, 0, 1}
	m := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: e6, afIPv4Unicast: e4}}

	rows := m.instanceListing()
	if len(rows) != 2 {
		t.Fatalf("instance rows = %d, want 2", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.AddressFamily] = true
	}
	if !seen[afIPv6Unicast.String()] || !seen[afIPv4Unicast.String()] {
		t.Fatalf("both AFs should be listed: %+v", rows)
	}
}

// TestOSPFv3ViewsAnswerEveryAddressFamily checks that every OSPFv3 view answers for each
// running address family, and that each row names the instance it came from (RFC 5838
// Section 2, AC-1, AC-3, A-1).
//
// Method: two AF engines (ipv6-unicast at Instance ID 0, ipv4-unicast at Instance ID 64)
// each bring one neighbor to Full through the real receive path and compute a route. Every
// entry of v3Views is answered with no argument and marshaled, as the pipes see it. Each
// row must lead with `address-family` then `instance-id`, carry the engine's own Instance
// ID, repeat no key, and both AFs must answer. The IPv4-unicast database record must hold
// that instance's self-originated LSAs.
func TestOSPFv3ViewsAnswerEveryAddressFamily(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		set := twoAFViewSet(t)
		want := map[string]uint8{afNameIPv6Unicast: 0, afNameIPv4Unicast: 64}
		for command, view := range v3Views {
			answer, err := view(set, nil, command, nil)
			if err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			seen := map[string]bool{}
			for _, row := range v3ViewRows(t, command, answer) {
				id, ok := want[row.addressFamily]
				if !ok {
					t.Fatalf("%s: row of address family %q, want one of %v", command, row.addressFamily, want)
				}
				if row.instanceID != id {
					t.Fatalf("%s: %s row has instance-id %d, want the engine's own %d", command, row.addressFamily, row.instanceID, id)
				}
				seen[row.addressFamily] = true
			}
			if len(seen) != len(want) {
				t.Fatalf("%s: rows came from %v, want rows from both running address families", command, seen)
			}
		}

		answer, err := v3Views[cmdShowIPv6Database](set, nil, cmdShowIPv6Database, nil)
		if err != nil {
			t.Fatalf("%s: %v", cmdShowIPv6Database, err)
		}
		selfOriginated := 0
		for _, row := range v3ViewRows(t, cmdShowIPv6Database, answer) {
			if row.addressFamily != afNameIPv4Unicast {
				continue
			}
			var lsas []struct {
				LocalOriginated bool `json:"local-originated"`
			}
			if err := json.Unmarshal(row.fields["lsas"], &lsas); err != nil {
				t.Fatalf("ipv4-unicast database lsas: %v", err)
			}
			for _, lsa := range lsas {
				if lsa.LocalOriginated {
					selfOriginated++
				}
			}
		}
		if selfOriginated == 0 {
			t.Fatal("the ipv4-unicast database record holds no LSA that instance originated")
		}
	})
}

// TestOSPFv3ViewsKeepTheRowsOfOneAddressFamily checks that with only ipv6-unicast running
// a view answers today's row plus the two identity keys, in front (AC-3).
//
// Method: one IPv6-unicast engine; the graceful-restart answer is marshaled and compared
// byte for byte with the engine's own snapshot behind the two keys.
func TestOSPFv3ViewsKeepTheRowsOfOneAddressFamily(t *testing.T) {
	e6 := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	set := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: e6}}

	answer, err := v3Views[cmdShowIPv6GracefulRestart](set, nil, cmdShowIPv6GracefulRestart, nil)
	if err != nil {
		t.Fatalf("%s: %v", cmdShowIPv6GracefulRestart, err)
	}
	raw, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	today, err := json.Marshal(e6.grSnapshot())
	if err != nil {
		t.Fatalf("marshal today's row: %v", err)
	}
	want := `[{"address-family":"ipv6-unicast","instance-id":0,` + string(today[1:]) + `]`
	if string(raw) != want {
		t.Fatalf("answer\n%s\nwant\n%s", raw, want)
	}
}

// TestOSPFv3ViewsNarrowToOneAddressFamily checks that `address-family <af>` narrows a view
// to that running AF, and that any other argument is refused as an operator error (AC-2,
// AC-5).
//
// Method: two idle AF engines (ipv6-unicast, ipv4-unicast at Instance ID 64). The database
// view narrowed to ipv4-unicast answers that one record. Naming ipv4-multicast, which is
// not running, is refused with the running AFs and `show ospf ipv6 instance` as the fix.
// An unknown name, a missing value and a stray token are each refused, never ignored.
func TestOSPFv3ViewsNarrowToOneAddressFamily(t *testing.T) {
	set := twoAFIdleSet(t)

	answer, err := v3Views[cmdShowIPv6Database](set, nil, cmdShowIPv6Database, []string{keywordAddressFamily, afNameIPv4Unicast})
	if err != nil {
		t.Fatalf("narrowed database: %v", err)
	}
	rows := v3ViewRows(t, cmdShowIPv6Database, answer)
	if len(rows) != 1 || rows[0].addressFamily != afNameIPv4Unicast || rows[0].instanceID != 64 {
		t.Fatalf("narrowed database rows = %+v, want the one ipv4-unicast record at instance-id 64", rows)
	}

	_, err = v3Views[cmdShowIPv6Neighbor](set, nil, cmdShowIPv6Neighbor, []string{keywordAddressFamily, afNameIPv4Multicast})
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("ipv4-multicast refusal = %v (%T), want an operator error", err, err)
	}
	for _, part := range []string{afNameIPv4Multicast, afNameIPv6Unicast + ", " + afNameIPv4Unicast} {
		if !strings.Contains(refusal.Error(), part) {
			t.Fatalf("refusal %q does not name %q", refusal.Error(), part)
		}
	}
	if refusal.Fix() != cmdShowIPv6Instance {
		t.Fatalf("refusal fix = %q, want %q", refusal.Fix(), cmdShowIPv6Instance)
	}

	for _, args := range [][]string{
		{keywordAddressFamily, "bogus"},
		// The registry's enumeration refuses the config's bare "ipv6" first; the plugin's
		// own parse refuses it too, for a caller that reaches the plugin another way.
		{keywordAddressFamily, afNameIPv6},
		{keywordAddressFamily},
		{"bogus"},
		{keywordAddressFamily, afNameIPv4Unicast, "extra"},
	} {
		_, err := v3Views[cmdShowIPv6Neighbor](set, nil, cmdShowIPv6Neighbor, args)
		if !errors.As(err, &refusal) {
			t.Fatalf("args %q: answer error = %v, want an operator error", args, err)
		}
	}
}

// TestOSPFv3RouteViewPerAddressFamily checks that `show ospf ipv6 route` answers the
// computed routes of every running OSPFv3 AF, each tagged, and that `address-family`
// narrows it (AC-4).
//
// Method: the two-AF fixture computes 2001:db8:2::/64 on ipv6-unicast and 192.0.2.0/24 on
// ipv4-unicast. Each route must carry its own AF; narrowed to ipv4-unicast only the IPv4
// route answers.
func TestOSPFv3RouteViewPerAddressFamily(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		set := twoAFViewSet(t)

		answer, err := v3Views[cmdShowIPv6Route](set, nil, cmdShowIPv6Route, nil)
		if err != nil {
			t.Fatalf("%s: %v", cmdShowIPv6Route, err)
		}
		routes := v3RouteFamilies(t, v3ViewRows(t, cmdShowIPv6Route, answer))
		if routes[twoAFRouteIPv6] != afNameIPv6Unicast || routes[twoAFRouteIPv4] != afNameIPv4Unicast {
			t.Fatalf("route address families = %v, want %s on %s and %s on %s",
				routes, twoAFRouteIPv6, afNameIPv6Unicast, twoAFRouteIPv4, afNameIPv4Unicast)
		}

		answer, err = v3Views[cmdShowIPv6Route](set, nil, cmdShowIPv6Route, []string{keywordAddressFamily, afNameIPv4Unicast})
		if err != nil {
			t.Fatalf("narrowed %s: %v", cmdShowIPv6Route, err)
		}
		routes = v3RouteFamilies(t, v3ViewRows(t, cmdShowIPv6Route, answer))
		for prefix, af := range routes {
			if af != afNameIPv4Unicast {
				t.Fatalf("narrowed route %s answered from %s", prefix, af)
			}
		}
		if _, ok := routes[twoAFRouteIPv4]; !ok {
			t.Fatalf("narrowed routes = %v, want %s", routes, twoAFRouteIPv4)
		}
	})
}

// The prefix each AF of the two-AF fixture learns from its neighbor.
const (
	twoAFRouteIPv6 = "2001:db8:2::/64"
	twoAFRouteIPv4 = "192.0.2.0/24"
)

// v3RouteFamilies maps each route row's prefix to the address family it was tagged with.
func v3RouteFamilies(t *testing.T, rows []v3ViewRow) map[string]string {
	t.Helper()
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		var prefix string
		if err := json.Unmarshal(row.fields["prefix"], &prefix); err != nil {
			t.Fatalf("route prefix: %v", err)
		}
		out[prefix] = row.addressFamily
	}
	return out
}

// v3ViewRow is one marshaled OSPFv3 view row: its keys in order, its two identity values,
// and every field's raw JSON.
type v3ViewRow struct {
	keys          []string
	addressFamily string
	instanceID    uint8
	fields        map[string]json.RawMessage
}

// v3ViewRows marshals a view answer as the pipes receive it and decodes each row in key
// order, failing unless every row leads with `address-family` then `instance-id` and
// repeats no key.
func v3ViewRows(t *testing.T, command string, answer any) []v3ViewRow {
	t.Helper()
	raw, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("%s: marshal: %v", command, err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("%s: answer is not a JSON list: %v\n%s", command, err, raw)
	}
	out := make([]v3ViewRow, 0, len(rows))
	for _, rowRaw := range rows {
		row := v3ViewRow{fields: map[string]json.RawMessage{}}
		dec := json.NewDecoder(bytes.NewReader(rowRaw))
		if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
			t.Fatalf("%s: row is not a JSON object: %s", command, rowRaw)
		}
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				t.Fatalf("%s: row key: %v", command, err)
			}
			key, ok := tok.(string)
			if !ok {
				t.Fatalf("%s: row key %v is not a string", command, tok)
			}
			var value json.RawMessage
			if err := dec.Decode(&value); err != nil {
				t.Fatalf("%s: row value of %q: %v", command, key, err)
			}
			if _, repeated := row.fields[key]; repeated {
				t.Fatalf("%s: row repeats key %q: %s", command, key, rowRaw)
			}
			row.keys = append(row.keys, key)
			row.fields[key] = value
		}
		if len(row.keys) < 2 || row.keys[0] != keywordAddressFamily || row.keys[1] != fieldInstanceID {
			t.Fatalf("%s: row keys %v, want %s then %s first: %s", command, row.keys, keywordAddressFamily, fieldInstanceID, rowRaw)
		}
		if err := json.Unmarshal(row.fields[keywordAddressFamily], &row.addressFamily); err != nil {
			t.Fatalf("%s: %s: %v", command, keywordAddressFamily, err)
		}
		if err := json.Unmarshal(row.fields[fieldInstanceID], &row.instanceID); err != nil {
			t.Fatalf("%s: %s: %v", command, fieldInstanceID, err)
		}
		out = append(out, row)
	}
	return out
}

// twoAFIdleSet holds an IPv6-unicast and an IPv4-unicast engine (Instance ID 64) with no
// interface open: every record view answers, no list view has rows.
func twoAFIdleSet(t *testing.T) *v6EngineSet {
	t.Helper()
	router := types.RouterID{10, 0, 0, 1}
	e6 := newV6DecodeEngine(t, router)
	e4 := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv4Unicast)
	e4.cfg.RouterID = router
	e4.lsdb.SetSelfRouterID(router)
	e4.dispatch.setInstanceID(64)
	return &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: e6, afIPv4Unicast: e4}}
}

// twoAFViewSet stands up the canonical two-AF config (twoAFConfig: ipv6-unicast at Instance
// ID 0 on eth0, ipv4-unicast at Instance ID 64 on eth1) over fake transports. Each AF brings
// one point-to-point neighbor to Full through the real receive path, receives that
// neighbor's Router-LSA, Link-LSA and Intra-Area-Prefix-LSA, and runs SPF, so every view
// has rows from both AFs. MUST be called inside synctest.Test: origination after Full runs
// on the maintenance worker's virtual clock.
func twoAFViewSet(t *testing.T) *v6EngineSet {
	t.Helper()
	v6 := newTwoAFMember(t, afIPv6Unicast, "eth0", 0)
	v4 := newTwoAFMember(t, afIPv4Unicast, "eth1", 64)
	v6.neighborFull(t)
	v4.neighborFull(t)
	// sleep(protocol): virtual time only; Full queues each engine's Router-LSA on its
	// maintenance worker, and SPF's two-way check needs that back-link.
	time.Sleep(time.Second)
	synctest.Wait()
	v6.neighborLSAs(t, netip.MustParsePrefix(twoAFRouteIPv6), [16]byte{0xfe, 0x80, 15: 2})
	v4.neighborLSAs(t, netip.MustParsePrefix(twoAFRouteIPv4), [16]byte{192, 0, 2, 2})
	v6.eng.spf.Run()
	v4.eng.spf.Run()
	return &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: v6.eng, afIPv4Unicast: v4.eng}}
}

// twoAFMember is one AF of the two-AF fixture: its engine, the fake transport's ifindex of
// its one interface, and its Instance ID.
type twoAFMember struct {
	eng      *engine
	ifindex  int
	instance uint8
}

// The fixture neighbor: its Router ID, its link-local source, the AllSPFRouters group it
// sends to.
var (
	twoAFPeer       = ridOf("10.0.0.2")
	twoAFPeerSource = netip.MustParseAddr("fe80::2")
	twoAFAllSPF     = netip.MustParseAddr("ff02::5")
)

// twoAFPeerIfaceID is the fixture neighbor's Interface ID. It is untyped because it is both
// the Hello's ospfv3types.InterfaceID and the uint32 the Link-LSA's Link State ID is built
// from (v6SummaryLSID).
const twoAFPeerIfaceID = 2

// newTwoAFMember builds the engine of one AF of twoAFConfig over a fake transport, with its
// interface open.
func newTwoAFMember(t *testing.T, af addressFamily, iface string, instance uint8) twoAFMember {
	t.Helper()
	families := twoAFConfig(t).v6Families()
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, af)
	eng.setMultiAF(true)
	for i := range families {
		if families[i].af == af {
			eng.setConfig(families[i].cfg)
		}
	}
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("%s openInterfaces: %v", af, err)
	}
	t.Cleanup(eng.shutdown)
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})
	fb.mu.Lock()
	handle := fb.handles[iface]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatalf("%s: no transport handle for %s", af, iface)
	}
	return twoAFMember{eng: eng, ifindex: handle.ifindex, instance: instance}
}

// twoAFOptions are the fixture neighbor's options: the AF-bit set, as RFC 5838 Section 2.4
// requires of a non-default AF and the default AF ignores.
func twoAFOptions() ospfv3types.Options {
	return (ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR).SetAF()
}

// neighborFull drives the fixture neighbor to Full: a two-way Hello, then the master's
// initial and final Database Description (the peer's higher Router ID is master).
func (m twoAFMember) neighborFull(t *testing.T) {
	t.Helper()
	area := ospfv3types.AreaID(types.BackboneArea)
	m.dispatch(t, ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello, AreaID: area},
		Hello: &ospfv3packet.Hello{
			InterfaceID: twoAFPeerIfaceID, Priority: 1, Options: twoAFOptions(),
			HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval,
			Neighbors: []ospfv3types.RouterID{ospfv3types.RouterID(m.eng.cfg.RouterID)},
		},
	})
	for _, dd := range []ospfv3packet.DBDesc{
		{InterfaceMTU: 1500, Options: twoAFOptions(), Flags: ospfv3packet.DDFlagInit | ospfv3packet.DDFlagMore | ospfv3packet.DDFlagMaster, DDSequence: 7},
		{InterfaceMTU: 1500, Options: twoAFOptions(), Flags: ospfv3packet.DDFlagMaster, DDSequence: 8},
	} {
		m.dispatch(t, ospfv3packet.Packet{
			Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeDBDesc, AreaID: area},
			DBDesc: &dd,
		})
	}
	rows := m.eng.neighborSnapshot()
	if len(rows) != 1 {
		t.Fatalf("%s neighbor rows = %d, want 1", m.eng.af, len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok || snap.State != ospflsdb.NeighborStateFull {
		t.Fatalf("%s neighbor = %+v, want Full", m.eng.af, rows[0])
	}
}

// neighborLSAs floods the fixture neighbor's Router-LSA (a point-to-point link back to this
// router), its Link-LSA (linkAddress: an IPv6 link-local, or for an IPv4 AF the IPv4 next
// hop in the leading 32 bits, RFC 5838 Section 2.5) and an Intra-Area-Prefix-LSA for
// prefix.
func (m twoAFMember) neighborLSAs(t *testing.T, prefix netip.Prefix, linkAddress [16]byte) {
	t.Helper()
	wirePrefix, ok := netipToV6Prefix(prefix, 5)
	if !ok {
		t.Fatalf("netipToV6Prefix(%s) failed", prefix)
	}
	peer := ospfv3types.RouterID(twoAFPeer)
	seq := ospfv3types.InitialSequenceNumber
	routerLSA := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeRouter, AdvertisingRouter: peer, Sequence: seq},
		Router: &ospfv3packet.RouterLSA{
			Options: twoAFOptions(),
			Links: []ospfv3packet.RouterLink{{
				Type: ospfv3packet.RouterLinkTypeP2P, Metric: 1,
				NeighborRouterID: ospfv3types.RouterID(m.eng.cfg.RouterID),
			}},
		},
	}
	linkLSA := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeLink, AdvertisingRouter: peer, Sequence: seq,
			LinkStateID: ospfv3types.LinkStateID(v6SummaryLSID(twoAFPeerIfaceID)),
		},
		Link: &ospfv3packet.LinkLSA{RtrPriority: 1, Options: twoAFOptions(), LinkLocalAddr: linkAddress},
	}
	intraLSA := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeIntraAreaPrefix, AdvertisingRouter: peer, Sequence: seq,
			LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1},
		},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{
			ReferencedLSType: ospfv3types.LSTypeRouter, ReferencedAdvRouter: peer,
			Prefixes: []ospfv3packet.Prefix{wirePrefix},
		},
	}
	m.dispatch(t, ospfv3packet.Packet{
		Header:   ospfv3packet.Header{Type: ospfv3packet.PacketTypeLSUpdate, AreaID: ospfv3types.AreaID(types.BackboneArea)},
		LSUpdate: &ospfv3packet.LSUpdate{LSAs: []ospfv3packet.LSA{routerLSA, linkLSA, intraLSA}},
	})
}

// dispatch stamps the fixture neighbor's Router ID and this AF's Instance ID on p, encodes
// it, binds the IPv6 upper-layer checksum, and feeds it through the engine's real receive
// path.
func (m twoAFMember) dispatch(t *testing.T, p ospfv3packet.Packet) {
	t.Helper()
	p.Header.RouterID = ospfv3types.RouterID(twoAFPeer)
	p.Header.InstanceID = ospfv3types.InstanceID(m.instance)
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(twoAFPeerSource, twoAFAllSPF, buf)
	m.eng.dispatch.dispatch(transport.RawPacket{IfIndex: m.ifindex, Src: twoAFPeerSource, Dst: twoAFAllSPF, Payload: buf})
}
