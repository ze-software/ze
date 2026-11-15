// VALIDATES: spec-ospf-ext-3 AC-12/AC-13/AC-14 -- `show ospf database router-information`
// decodes the stored RI LSA bodies for both address families into named capability bits and a
// TLV list, and a truncated/malformed body renders what it can without crashing. Both RI views
// read every running OSPFv3 address-family engine and name each OSPFv3 entry's instance
// (spec-ospfv3-views-every-address-family A-4, AC-3).
// PREVENTS: a show that shows only one address family or one OSPFv3 instance, merges two
// instances' advertisements, hides the capability bits, or panics on a hostile received RI LSA.
package ospf

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func riShowView(t *testing.T, rows []any) riDatabaseView {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("RI snapshot wrapping = %d, want 1", len(rows))
	}
	view, ok := rows[0].(riDatabaseView)
	if !ok {
		t.Fatalf("RI snapshot is not a riDatabaseView: %T", rows[0])
	}
	return view
}

// riViewOf answers an RI view through view, the producer its OnExecuteCommand arm calls
// (routerInformationView for cmdShowDatabaseRI, v3RouterInformationView for
// cmdShowIPv6DatabaseRI).
func riViewOf(t *testing.T, view v3ViewFunc, set *v6EngineSet, v2 *engine, command string, cmdArgs []string) riDatabaseView {
	t.Helper()
	answer, err := view(set, v2, command, cmdArgs)
	if err != nil {
		t.Fatalf("%s %v: %v", command, cmdArgs, err)
	}
	rows, ok := answer.([]any)
	if !ok {
		t.Fatalf("%s answered %T, want []any", command, answer)
	}
	return riShowView(t, rows)
}

// riIPv6UnicastInstances answers the OSPFv3 instances an RI view reads when e is the only
// address-family engine, running as ipv6-unicast, selected the way the views select them.
func riIPv6UnicastInstances(t *testing.T, e *engine) []afInstance {
	t.Helper()
	set := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: e}}
	instances, err := set.selectInstances(cmdShowIPv6DatabaseRI, nil)
	if err != nil {
		t.Fatalf("select the ipv6-unicast instance: %v", err)
	}
	return instances
}

// newRIv2Engine builds an OSPFv2 engine that has originated its area-scope RI opaque LSA,
// with max-metric configured so the stub-router bit is advertised.
func newRIv2Engine(t *testing.T) *engine {
	t.Helper()
	v4, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"max-metric":{"router-lsa":{"always":"true"}},`+
		`"router-information":{"enabled":true,"scope":"area"},`+
		`"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`)
	if err := registerRIConsumer(v4); err != nil {
		t.Fatalf("registerRIConsumer: %v", err)
	}
	v4.originateSelfLSAs()
	return v4
}

func TestRIShowRender(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	v4 := newRIv2Engine(t)

	// v3 engine: RI enabled at area scope.
	v6 := newV6RIEngine(t)
	v6.setConfig(v6RIConfig(t, "area"))
	v6.v6OriginateRIScope(types.RouterID{1, 1, 1, 1}, OpaqueScopeArea, types.BackboneArea, map[ospflsdb.SelfLSARef]struct{}{})
	set := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: v6}}

	view := riViewOf(t, routerInformationView, set, v4, cmdShowDatabaseRI, nil)
	var haveV2, haveV3 bool
	for _, e := range view.RouterInformation {
		switch e.AF {
		case "v2":
			haveV2 = true
			if !containsStubRouter(e.Capabilities) {
				t.Errorf("v2 RI capabilities = %v, want stub-router", e.Capabilities)
			}
			if len(e.Instances) == 0 || len(e.Instances[0].TLVs) == 0 {
				t.Errorf("v2 RI entry has no decoded TLVs: %+v", e)
			}
		case "v3":
			haveV3 = true
		}
	}
	if !haveV2 || !haveV3 {
		t.Fatalf("RI view missing an address family: v2=%v v3=%v (%+v)", haveV2, haveV3, view.RouterInformation)
	}
}

// TestRIViewOverEveryV6Engine proves both RFC 7770 RI views read every running OSPFv3
// address-family engine. The same router's area-scope RI LSA, held by the ipv6-unicast
// (Instance ID 0) and the ipv4-unicast (Instance ID 64) LSDBs with different capability
// bits, renders as two entries, each naming its own address-family and instance-id and
// carrying its own engine's bits. The address-family keyword narrows the OSPFv3 part to one
// instance; on the shared view it leaves the OSPFv2 part whole, and an OSPFv2 entry carries
// neither identity key. Method: twoAFIdleSet plus one RI LSA per engine, read through the
// two producers OnExecuteCommand's RI arms call (routerInformationView,
// v3RouterInformationView; neither is in v3Views), compared as one summary line per entry.
func TestRIViewOverEveryV6Engine(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	v4 := newRIv2Engine(t)
	set := twoAFIdleSet(t)
	router := types.RouterID{10, 0, 0, 1}
	originateV6RIArea(t, set, afIPv6Unicast, router, packet.RIInfoBitStubRouter)
	originateV6RIArea(t, set, afIPv4Unicast, router, packet.RIInfoBitGracefulRestart)
	v3Unicast6 := "v3 ipv6-unicast 0 stub-router"
	v3Unicast4 := "v3 ipv4-unicast 64 graceful-restart-capable"
	narrowTo4 := []string{keywordAddressFamily, afNameIPv4Unicast}

	every := riEntrySummaries(riViewOf(t, v3RouterInformationView, set, v4, cmdShowIPv6DatabaseRI, nil))
	if want := []string{v3Unicast6, v3Unicast4}; !slices.Equal(every, want) {
		t.Fatalf("%s = %q, want %q", cmdShowIPv6DatabaseRI, every, want)
	}
	narrowed := riEntrySummaries(riViewOf(t, v3RouterInformationView, set, v4, cmdShowIPv6DatabaseRI, narrowTo4))
	if want := []string{v3Unicast4}; !slices.Equal(narrowed, want) {
		t.Fatalf("%s %q = %q, want %q", cmdShowIPv6DatabaseRI, narrowTo4, narrowed, want)
	}

	shared := riEntrySummaries(riViewOf(t, routerInformationView, set, v4, cmdShowDatabaseRI, nil))
	var ospfv2 []string
	for _, line := range shared {
		if line == "v2" {
			ospfv2 = append(ospfv2, line)
		}
	}
	if len(ospfv2) == 0 {
		t.Fatalf("%s has no OSPFv2 entry: %q", cmdShowDatabaseRI, shared)
	}
	if want := append(slices.Clone(ospfv2), v3Unicast6, v3Unicast4); !slices.Equal(shared, want) {
		t.Fatalf("%s = %q, want %q", cmdShowDatabaseRI, shared, want)
	}
	sharedNarrowed := riViewOf(t, routerInformationView, set, v4, cmdShowDatabaseRI, narrowTo4)
	if got, want := riEntrySummaries(sharedNarrowed), append(slices.Clone(ospfv2), v3Unicast4); !slices.Equal(got, want) {
		t.Fatalf("%s %q = %q, want the whole OSPFv2 part then %q", cmdShowDatabaseRI, narrowTo4, got, want)
	}

	for _, entry := range sharedNarrowed.RouterInformation {
		text, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal RI entry: %v", err)
		}
		if entry.AF == "v2" {
			if strings.Contains(string(text), `"address-family"`) || strings.Contains(string(text), `"instance-id"`) {
				t.Errorf("OSPFv2 RI entry carries an OSPFv3 identity key: %s", text)
			}
			continue
		}
		if lead := `{"af":"v3","address-family":"ipv4-unicast","instance-id":64,"scope":`; !strings.HasPrefix(string(text), lead) {
			t.Errorf("OSPFv3 RI entry = %s, want it to start %s", text, lead)
		}
	}
}

// originateV6RIArea originates, in the LSDB of set's af engine, an area-scope OSPFv3 RI LSA
// (RI Instance 0) advertised by router whose informational capabilities carry bit alone.
func originateV6RIArea(t *testing.T, set *v6EngineSet, af addressFamily, router types.RouterID, bit uint) {
	t.Helper()
	e, ok := set.engineFor(af)
	if !ok {
		t.Fatalf("no %s engine in the set", af)
	}
	body := packet.EncodeRITLVs([]packet.RITLV{{
		Type:  packet.RITLVInformationalCapabilities,
		Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(bit)),
	}})
	originateV6Raw(t, e, ospfv3types.LSTypeRouterInformationArea, router, types.BackboneArea, body)
}

// riEntrySummaries renders each RI entry as one line: "v2" for an OSPFv2 entry, and
// "v3 <address-family> <instance-id> <capabilities>" for an OSPFv3 one.
func riEntrySummaries(view riDatabaseView) []string {
	out := make([]string, 0, len(view.RouterInformation))
	for _, e := range view.RouterInformation {
		if e.riOSPFv3Instance == nil {
			out = append(out, e.AF)
			continue
		}
		out = append(out, e.AF+" "+e.AddressFamily+" "+strconv.Itoa(int(e.InstanceID))+" "+strings.Join(e.Capabilities, ","))
	}
	return out
}

func TestRIShowMalformedTLV(t *testing.T) {
	// A valid type-1 TLV followed by a truncated second TLV: the renderer decodes the first
	// and flags the body malformed, never panicking (AC-14, bound-checked iterator).
	good := packet.EncodeRITLVs([]packet.RITLV{{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitStubRouter))}})
	body := append([]byte(nil), good...)
	body = append(body, 0x00, 0x08, 0xFF, 0xFF, 0x01) // a TLV header claiming a length past the end
	caps, tlvs, malformed := decodeRIBody(body)
	if !malformed {
		t.Fatalf("truncated RI body not flagged malformed")
	}
	if len(tlvs) != 1 || tlvs[0].Type != packet.RITLVInformationalCapabilities {
		t.Fatalf("partial decode = %+v, want the leading type-1 TLV", tlvs)
	}
	if caps&packet.RIInfoBitMask(packet.RIInfoBitStubRouter) == 0 {
		t.Fatalf("leading type-1 capabilities not decoded: %#08x", caps)
	}
}

func containsStubRouter(capabilities []string) bool {
	return slices.Contains(capabilities, "stub-router")
}

// TestRIGroupOrderBreaksAnInstanceIDTieByAddressFamily pins the address-family step of
// riGroupKeyLess: two OSPFv3 RI groups with the same Instance ID in different address
// families render ordered by address family, before scope decides anything.
//
// Method: buildRIView over two observations of one router at OSPFv3 Instance ID 0, one
// tagged ipv6-unicast at area scope and one tagged ipv4-unicast at link scope, given in
// both input orders. The address-family step puts ipv4-unicast first; an order that skipped
// it would fall through to scope and put the ipv6-unicast (area) group first.
func TestRIGroupOrderBreaksAnInstanceIDTieByAddressFamily(t *testing.T) {
	body := packet.EncodeRITLVs([]packet.RITLV{{
		Type:  packet.RITLVInformationalCapabilities,
		Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitStubRouter)),
	}})
	unicast6 := riObservation{
		af: "v3", ospfv3: &riOSPFv3Instance{AddressFamily: afNameIPv6Unicast, InstanceID: 0},
		scope: "area", area: "0.0.0.0", advRouter: "10.0.0.1", body: body,
	}
	unicast4 := riObservation{
		af: "v3", ospfv3: &riOSPFv3Instance{AddressFamily: afNameIPv4Unicast, InstanceID: 0},
		scope: "link", area: "0.0.0.0", advRouter: "10.0.0.1", body: body,
	}
	want := []string{"v3 ipv4-unicast 0 stub-router", "v3 ipv6-unicast 0 stub-router"}
	for _, obs := range [][]riObservation{{unicast6, unicast4}, {unicast4, unicast6}} {
		if got := riEntrySummaries(buildRIView(obs)); !slices.Equal(got, want) {
			t.Errorf("RI groups from %s then %s render %q, want %q",
				obs[0].ospfv3.AddressFamily, obs[1].ospfv3.AddressFamily, got, want)
		}
	}
}
