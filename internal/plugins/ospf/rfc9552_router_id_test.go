// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF router-ID export.
// Related: rfc9552_bgpls_export_test.go -- native LSDB and snapshot fixtures.
// RFC: rfc/short/rfc9552.md -- Sections 5.2.1 and 5.3.2.1.

package ospf

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/ls_export"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC9552OSPFRouterIDsReachExporter checks the real native LSDB, snapshot,
// registered exporter and complete encoded attributes, not a supplied TLV echo.
// RFC 9552 Section 5.2.1: "When configured, these auxiliary TE Router-IDs
// (TLV 1028/1029) MUST be included in the node attribute described in Section
// 5.3.1 and MAY be included in the link attribute described in Section 5.3.2."
// RFC 9552 Section 5.3.2.1: "All auxiliary Router-IDs of both the local and
// the remote node MUST be included in the link attribute of each Link NLRI."
// MUTATION: omit trafficEngineering's 1028 append, change linkAttributes' remote
// type offset from +2 to +0, or truncate originateAttributes after its first TLV.
func TestRFC9552OSPFRouterIDsReachExporter(t *testing.T) {
	// RFC requirement: RFC9552-5.2.1-1 positive -- native OSPFv2 TE Router Address LSAs produce each node's exact IPv4 Router-ID and complete encoded BGP-LS Attribute, without inventing IPv6 Router-IDs.
	// RFC requirement: RFC9552-5.3.2.1-1 positive -- both native OSPFv2 link directions export the exact local and remote TE Router-IDs through the complete encoded BGP-LS Attribute.
	e, bus, events := bgplsTestSource(t, false)
	rfc9552InstallRouterPair(t, e)
	for _, router := range []byte{2, 3} {
		rfc9552InstallRouterAddress(t, e, router, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
	}
	want := map[string]string{
		"1/02020202/":         "040000010004040004c0000202",
		"1/03030303/":         "040000010004040004c0000203",
		"2/02020202/03030303": "04040004c000020204060004c000020304470002000a",
		"2/03030303/02020202": "04040004c000020304060004c000020204470002000a",
	}
	// RFC 9552 Sections 5.2.1 and 5.3.2.1.
	rfc9552AssertNativeRouterIDs(t, bus, events, want)
}

// TestRFC9552OSPFAuxiliaryRouterIDsNotCollapsed strengthens the existing native
// multi-LSA seed with exact values and final export bytes. This is deliberately
// a robustness case: RFC 3630 Section 2.4.1 says the Router Address "must appear
// in exactly one Traffic Engineering LSA originated by a router." It is not the
// conforming positive fixture above. If a received LSDB nevertheless contains
// several IDs, RFC 9552 Section 5.3.1.4 says: "If there is more than one auxiliary
// Router-ID of a given type, then each one is encoded as a separate TLV."
// MUTATION: make bgplsAddAttribute deduplicate by Type alone instead of Type
// and Value, losing the second native Router-ID on both nodes and links.
func TestRFC9552OSPFAuxiliaryRouterIDsNotCollapsed(t *testing.T) {
	e, bus, events := bgplsTestSource(t, false)
	rfc9552InstallRouterPair(t, e)
	for _, router := range []byte{2, 3} {
		rfc9552InstallRouterAddress(t, e, router, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
		lsa := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeOpaqueArea,
			LinkStateID:       types.LinkStateID{packet.TEOpaqueType, 0, 0, 2},
			AdvertisingRouter: types.RouterID{router, router, router, router},
			Sequence:          types.InitialSequenceNumber},
			Body: packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{198, 51, 100, router}}.Encode()}
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install additional received Router Address LSA")
		}
	}
	want := map[string]string{
		"1/02020202/":         "040000010004040004c000020204040004c6336402",
		"1/03030303/":         "040000010004040004c000020304040004c6336403",
		"2/02020202/03030303": "04040004c000020204040004c633640204060004c000020304060004c633640304470002000a",
		"2/03030303/02020202": "04040004c000020304040004c633640304060004c000020204060004c633640204470002000a",
	}
	// RFC 9552 Sections 5.3.1.4 and 5.3.2.1.
	rfc9552AssertNativeRouterIDs(t, bus, events, want)
}

// TestRFC9552OSPFRouterIDAbsenceReachExporter distinguishes IGP identity and
// numbered interface addresses from auxiliary TE Router-IDs. Expired LSAs and
// an unrelated opaque type cannot supply IDs. The live opposite node is a control.
// MUTATION: index router IDs by the remote node for both ends in linkAttributes,
// or retain MaxAge TE LSAs in publishBGPLSLocked.
func TestRFC9552OSPFRouterIDAbsenceReachExporter(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.1-1 negative -- native OSPFv2 links with no live local TE Router Address omit local Router-ID attributes while retaining the exact live remote ID, with the roles reversed on the reverse link and the final attribute bytes checked.
	for _, tc := range []struct {
		name    string
		install bool
		opaque  uint8
		age     types.LSAge
	}{
		{name: "absent"},
		{name: "expired", install: true, opaque: packet.TEOpaqueType, age: types.LSAge(types.MaxAge)},
		{name: "unrelated-opaque", install: true, opaque: 250},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, bus, events := bgplsTestSource(t, false)
			rfc9552InstallRouterPair(t, e)
			rfc9552InstallRouterAddress(t, e, 3, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
			if tc.install {
				rfc9552InstallRouterAddress(t, e, 2, types.LSTypeOpaqueArea, tc.opaque, tc.age)
			}
			want := map[string]string{
				"1/02020202/":         "0400000100",
				"1/03030303/":         "040000010004040004c0000203",
				"2/02020202/03030303": "04060004c000020304470002000a",
				"2/03030303/02020202": "04040004c000020304470002000a",
			}
			// RFC 9552 Section 5.3.2.1.
			rfc9552AssertNativeRouterIDs(t, bus, events, want)
		})
	}
}

// TestRFC9552OSPFRouterAddressRequiresTEScope supplies a valid Router Address
// body in the wrong LSA scopes or opaque type. RFC 3630 Section 2.1: "This proposal
// uses only Type 10 LSAs, which have an area flooding scope." RFC 5392 Section 3.2:
// "Both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA contain one top level TLV:
// 2 - Link TLV". A Router Address body in those carriers is not a TE Router-ID.
// This regression remains untagged until Main observes and resolves its result.
func TestRFC9552OSPFRouterAddressRequiresTEScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scope  types.LSType
		opaque uint8
	}{
		{"link-scope", types.LSTypeOpaqueLink, packet.TEOpaqueType},
		{"as-scope", types.LSTypeOpaqueAS, packet.TEOpaqueType},
		{"inter-as-router-address", types.LSTypeOpaqueArea, packet.InterAsTEOpaqueType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, bus, events := bgplsTestSource(t, false)
			rfc9552InstallRouterPair(t, e)
			rfc9552InstallRouterAddress(t, e, 2, tc.scope, tc.opaque, 0)
			want := map[string]string{
				"1/02020202/":         "0400000100",
				"1/03030303/":         "0400000100",
				"2/02020202/03030303": "04470002000a",
				"2/03030303/02020202": "04470002000a",
			}
			// RFC 9552 Sections 5.2.1 and 5.3.2.1; RFC 3630 Section 2.1.
			rfc9552AssertNativeRouterIDs(t, bus, events, want)
		})
	}
}

// TestRFC9552OSPFInterASRouterIDsReachExporter distinguishes the external
// ASBR's advertised IPv4/IPv6 identifiers from an internal router with the same
// four-octet OSPF identity. RFC 5392 Section 3.3.3: "The IPv6 Remote ASBR ID
// sub-TLV specifies the identifier of the remote ASBR to which the advertised
// inter-AS link connects." An interface IPv6 address alone would not prove this.
// MUTATION: omit trafficEngineering's 1031 append or remove linkAttributes'
// Remote.ASN == 0 guard, leaking the internal collision's auxiliary Router-ID.
func TestRFC9552OSPFInterASRouterIDsReachExporter(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.1-1 positive -- a native OSPFv2 inter-AS TE link exports exact local IPv4 and remote IPv4/IPv6 Router-IDs in its complete encoded BGP-LS Attribute, without borrowing an internal same-ID router's auxiliary ID.
	for _, scope := range []types.LSType{types.LSTypeOpaqueArea, types.LSTypeOpaqueAS} {
		t.Run(scope.String(), func(t *testing.T) {
			e, bus, events := bgplsTestSource(t, false)
			rfc9552InstallRouterAddress(t, e, 2, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
			rfc9552InstallRouterAddress(t, e, 9, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
			lsa := packet.LSA{Header: packet.LSAHeader{Type: scope,
				LinkStateID:       types.LinkStateID{packet.InterAsTEOpaqueType, 0, 0, 1},
				AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
				Body: packet.TELSA{IsLink: true, Link: packet.TELink{
					HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
					HasRemoteAS: true, RemoteAS: 65100,
					HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{9, 9, 9, 9},
					HasRemoteASBRv6: true, RemoteASBRv6: [16]byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9},
					LocalIPs: [][4]byte{{10, 0, 0, 2}}, RemoteIPs: [][4]byte{{10, 0, 0, 9}},
				}}.Encode()}
			if !e.lsdb.Install(types.BackboneArea, lsa) {
				t.Fatal("install inter-AS TE LSA")
			}
			want := map[string]string{
				"1/02020202/":         "04040004c0000202",
				"1/09090909/":         "04040004c0000209",
				"2/02020202/09090909": "04040004c000020204060004090909090407001020010db8000000000000000000000009",
			}
			// RFC 9552 Section 5.3.2.1; RFC 5392 Sections 3.1.1 and 3.3.3.
			rfc9552AssertNativeRouterIDs(t, bus, events, want)
		})
	}
}

// TestRFC9552OSPFInterASIgnoresRouterAddressAlongsideLink preserves the valid
// Link TLV while ignoring a Router Address that has no meaning in opaque type 6.
// RFC 5392 Section 3.2: "Both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA
// contain one top level TLV: 2 - Link TLV". RFC 3630 Section 2.3.2:
// "Unrecognized types are ignored." Both TLV orders and both permitted scopes
// must retain the separately advertised local ID and the link's remote IDs.
func TestRFC9552OSPFInterASIgnoresRouterAddressAlongsideLink(t *testing.T) {
	for _, scope := range []types.LSType{types.LSTypeOpaqueArea, types.LSTypeOpaqueAS} {
		for _, addressFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("scope-%d/address-first-%t", scope, addressFirst), func(t *testing.T) {
				e, bus, events := bgplsTestSource(t, false)
				rfc9552InstallRouterAddress(t, e, 2, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0)
				address := packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{198, 51, 100, 99}}.Encode()
				link := packet.TELSA{IsLink: true, Link: packet.TELink{
					HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
					HasRemoteAS: true, RemoteAS: 65100,
					HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{9, 9, 9, 9},
					HasRemoteASBRv6: true, RemoteASBRv6: [16]byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9},
					LocalIPs: [][4]byte{{10, 0, 0, 2}}, RemoteIPs: [][4]byte{{10, 0, 0, 9}},
				}}.Encode()
				body := make([]byte, 0, len(address)+len(link))
				if addressFirst {
					body = append(body, address...)
					body = append(body, link...)
				} else {
					body = append(body, link...)
					body = append(body, address...)
				}
				lsa := packet.LSA{Header: packet.LSAHeader{Type: scope,
					LinkStateID:       types.LinkStateID{packet.InterAsTEOpaqueType, 0, 0, 1},
					AdvertisingRouter: types.RouterID{2, 2, 2, 2}, Sequence: types.InitialSequenceNumber},
					Body: body}
				if !e.lsdb.Install(types.BackboneArea, lsa) {
					t.Fatal("install mixed-body inter-AS TE LSA")
				}
				want := map[string]string{
					"1/02020202/":         "04040004c0000202",
					"2/02020202/09090909": "04040004c000020204060004090909090407001020010db8000000000000000000000009",
				}
				// RFC 9552 Section 5.3.2.1; RFC 5392 Section 3.2.
				rfc9552AssertNativeRouterIDs(t, bus, events, want)
			})
		}
	}
}

func rfc9552InstallRouterPair(t *testing.T, e *engine) {
	t.Helper()
	for _, endpoints := range [][2]byte{{2, 3}, {3, 2}} {
		local, remote := endpoints[0], endpoints[1]
		lsa := bgplsReachabilityRouter(types.RouterID{local, local, local, local},
			packet.RouterLink{Type: packet.RouterLinkTypeP2P,
				LinkID:   types.LinkStateID{remote, remote, remote, remote},
				LinkData: [4]byte{10, 0, 0, local}, Metric: 10})
		if !e.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("install native Router LSA")
		}
	}
}

func rfc9552InstallRouterAddress(t *testing.T, e *engine, router byte, scope types.LSType, opaque uint8, age types.LSAge) {
	t.Helper()
	lsa := packet.LSA{Header: packet.LSAHeader{Type: scope, Age: age,
		LinkStateID:       types.LinkStateID{opaque, 0, 0, 1},
		AdvertisingRouter: types.RouterID{router, router, router, router}, Sequence: types.InitialSequenceNumber},
		Body: packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{192, 0, 2, router}}.Encode()}
	if scope == types.LSTypeOpaqueLink {
		if _, ok := e.lsdb.OriginateLinkSelf("router-id-proof", types.BackboneArea, lsa.Header.Key(), lsa.Body,
			func(sequence types.LSSequenceNumber, purge bool) packet.LSA {
				lsa.Header.Sequence = sequence
				if purge {
					lsa.Header.Age = types.LSAge(types.MaxAge)
				}
				return lsa
			}); !ok {
			t.Fatal("install link-scoped opaque LSA")
		}
		return
	}
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install native Router Address LSA")
	}
}

// Snapshot checks compare the complete TLV multiset against literal wire
// expectations; order is the exporter's responsibility, not the LSDB's.
func rfc9552AssertNativeRouterIDs(t *testing.T, bus *fakeBus, events <-chan linkstateevents.Snapshot, want map[string]string) {
	t.Helper()
	snapshot := bgplsReplaySnapshot(t, bus, events)
	got := make(map[string][]linkstateevents.TLV)
	for _, node := range snapshot.Nodes {
		key := fmt.Sprintf("1/%x/", node.ID.RouterID)
		if _, found := got[key]; found {
			t.Fatalf("duplicate node identity %s", key)
		}
		got[key] = node.Attributes
	}
	for i := range snapshot.Links {
		link := &snapshot.Links[i]
		key := fmt.Sprintf("2/%x/%x", link.Local.RouterID, link.Remote.RouterID)
		if _, found := got[key]; found {
			t.Fatalf("duplicate link identity %s", key)
		}
		got[key] = link.Attributes
	}
	if len(got) != len(want) {
		t.Fatalf("native objects = %+v, want %v", got, want)
	}
	for key, literal := range want {
		attributes, found := got[key]
		if !found {
			t.Fatalf("native snapshot missing %s", key)
		}
		var actual []string
		for _, attr := range attributes {
			actual = append(actual, fmt.Sprintf("%04x/%x", attr.Type, attr.Value))
		}
		var expected []string
		for _, attr := range rfc9552ReadTLVs(t, rfc9552Unhex(t, literal)) {
			expected = append(expected, fmt.Sprintf("%04x/%x", attr.Type, attr.Value))
		}
		slices.Sort(actual)
		slices.Sort(expected)
		if !slices.Equal(actual, expected) {
			t.Fatalf("native %s attributes = %v, want %v", key, actual, expected)
		}
	}
	// RFC 9552 Sections 5.2.1 and 5.3.2.1: exercise the registered producer's
	// encodeTopology -> originateAttributes -> send path on the same native bus.
	commands := rfc9552ExportNativeCommands(t, bus, len(want))
	seen := make(map[string]bool)
	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) != 12 || fields[2] != "attr" || fields[3] != "set" || fields[10] != "add" {
			t.Fatalf("unexpected exporter command %q", command)
		}
		wire := rfc9552Unhex(t, fields[11])
		if len(wire) < 13 {
			t.Fatalf("short BGP-LS NLRI %x", wire)
		}
		if wire[4] != byte(linkstateevents.OSPFv2) {
			t.Fatalf("OSPF source protocol became %d", wire[4])
		}
		var local, remote []byte
		for _, desc := range rfc9552ReadTLVs(t, wire[13:]) {
			if desc.Type != 256 && desc.Type != 257 {
				continue
			}
			for _, sub := range rfc9552ReadTLVs(t, desc.Value) {
				if sub.Type == 515 {
					if desc.Type == 256 {
						local = sub.Value
					} else {
						remote = sub.Value
					}
				}
			}
		}
		key := fmt.Sprintf("%d/%x/%x", binary.BigEndian.Uint16(wire), local, remote)
		literal, found := want[key]
		if !found {
			t.Fatalf("unexpected exported identity %s", key)
		}
		if seen[key] {
			t.Fatalf("duplicate exported identity %s", key)
		}
		seen[key] = true
		value := rfc9552Unhex(t, literal)
		// RFC 9552 Section 5.3: flags 0x90, code 29, extended length, then
		// the entire literal BGP-LS Attribute. ORIGIN/AS_PATH are checked too.
		expected := append([]byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x90, 29, byte(len(value) >> 8), byte(len(value))}, value...)
		if actual := rfc9552Unhex(t, fields[4]); !bytes.Equal(actual, expected) {
			t.Fatalf("exported %s attributes = %x, want %x", key, actual, expected)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exported identities = %v, want %v", seen, want)
	}
}

func rfc9552ReadTLVs(t *testing.T, wire []byte) []linkstateevents.TLV {
	t.Helper()
	var tlvs []linkstateevents.TLV
	for len(wire) != 0 {
		if len(wire) < 4 {
			t.Fatalf("short TLV header %x", wire)
		}
		length := int(binary.BigEndian.Uint16(wire[2:4]))
		if length > len(wire)-4 {
			t.Fatalf("TLV length %d exceeds %d", length, len(wire)-4)
		}
		tlvs = append(tlvs, linkstateevents.TLV{Type: binary.BigEndian.Uint16(wire), Value: wire[4 : 4+length]})
		wire = wire[4+length:]
	}
	return tlvs
}

func rfc9552Unhex(t *testing.T, literal string) []byte {
	t.Helper()
	wire, err := hex.DecodeString(literal)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

// The engine-side double only acknowledges and captures update-route. The
// registered exporter consumes native LSDB replay and does all TLV encoding.
// Shutdown joins the worker before the final drain, making absence assertions
// independent of sleeps, channel emptiness races, or exporter ordering.
func rfc9552ExportNativeCommands(t *testing.T, bus *fakeBus, count int) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	previous := registry.GetEventBus()
	registry.SetEventBus(bus)
	t.Cleanup(func() { registry.SetEventBus(previous) })
	registration := registry.Lookup("bgp-ls-export")
	if registration == nil {
		t.Fatal("native exporter registration missing")
	}
	commands := make(chan string, 64)
	bridge := rpc.NewDirectBridge()
	bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodUpdateRoute {
			return nil, fmt.Errorf("unexpected exporter RPC %s", method)
		}
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		if strings.Contains(input.Command, " del ") {
			return json.Marshal(rpc.UpdateRouteOutput{Withdrawn: 1})
		}
		select {
		case commands <- input.Command:
			return json.Marshal(rpc.UpdateRouteOutput{Announced: 1})
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	go func() { done <- registration.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("native exporter did not stop")
		}
	})
	for stage := range 3 {
		select {
		case request := <-mux.Requests():
			if request == nil {
				t.Fatal("exporter startup connection closed")
			}
			if err := mux.SendOK(ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if stage == 0 {
			input := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: "bgp-ls-export", Data: `{"bgp-ls-export":{}}`}}}
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", input); err != nil {
				t.Fatal(err)
			}
		}
		if stage == 1 {
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	event := rpc.DeliverEventInput{Event: `{"type":"state","state":"up","peer":{"remote":{"address":"192.0.2.254"}}}`}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:deliver-event", payload); err != nil {
		t.Fatal(err)
	}
	var captured []string
	for range count {
		select {
		case command := <-commands:
			captured = append(captured, command)
		case <-ctx.Done():
			t.Fatalf("exporter produced %d/%d routes: %v", len(captured), count, ctx.Err())
		}
	}
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:bye", json.RawMessage(`{"reason":"router-ID proof complete"}`)); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case command := <-commands:
			captured = append(captured, command)
		default:
			return captured
		}
	}
}
