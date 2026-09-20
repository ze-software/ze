// VALIDATES: RFC 5340 Section 4.4.3.6 -- `default-information originate` on an OSPFv3
// engine originates the default route as an AS-External-LSA whose wire LS Type is 0x4005,
// carrying ::/0 in an OSPFv3 prefix body, and evaluates its condition against the IPv6
// unicast Loc-RIB.
// PREVENTS: the OSPFv2 0x0005 key reaching an OSPFv3 engine, which RFC 5340 Appendix
// A.4.2.1 reads as U=0 with S2S1=00, "Link-Local Scoping - Flooded only on originating
// link", so the default would reach no router past the first hop; and an OSPFv3 engine
// deciding the conditional form from the IPv4 table it can never advertise into.
package ospf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

var testDefaultRouteV6 = netip.MustParsePrefix("::/0")

// newV6DefaultEngine builds an OSPFv3 engine whose config carries the given
// `default-information` body.
func newV6DefaultEngine(t *testing.T, defaultInfo string) (*engine, types.RouterID) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","default-information":`+defaultInfo+
		`,"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newV6RIEngine(t)
	eng.setConfig(cfg)
	return eng, cfg.RouterID
}

// v6DefaultLSA returns the stored AS-External-LSA this engine originated for ::/0.
func v6DefaultLSA(t *testing.T, eng *engine, router types.RouterID) (uint16, bool) {
	t.Helper()
	eng.mu.Lock()
	lsid, ok := eng.redistV6[defaultV6Prefix]
	eng.mu.Unlock()
	if !ok {
		return 0, false
	}
	lsa, found := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(router, lsid))
	if !found || lsa.Header.Age.IsMaxAge() {
		return 0, false
	}
	if len(lsa.RawBytes) < 4 {
		t.Fatalf("the OSPFv3 default LSA carries %d raw octets, too short for an LS Type", len(lsa.RawBytes))
	}
	return binary.BigEndian.Uint16(lsa.RawBytes[2:4]), true
}

// installRIBDefaultV6 inserts a non-OSPF ::/0 into the global Loc-RIB and registers its
// removal, so a conditional `default-information originate` on an OSPFv3 engine sees a
// real IPv6 default.
func installRIBDefaultV6(t *testing.T) {
	t.Helper()
	loc := locrib.Default()
	staticID := redistevents.RegisterProtocol("static")
	loc.InsertForward(family.IPv6Unicast, testDefaultRouteV6,
		locrib.Path{Source: staticID, NextHop: netip.MustParseAddr("2001:db8::fe"), AdminDistance: 1, Metric: 1}, nil)
	t.Cleanup(func() { loc.Remove(family.IPv6Unicast, testDefaultRouteV6, staticID, 0) })
}

// TestOSPFv3DefaultInformationUsesASExternalLSType checks the wire LS Type of the default
// an OSPFv3 engine originates. RFC 5340 Section 4.4.3.6: "The LS type of an
// AS-external-LSA is set to the value 0x4005." Reading the stored octets rather than the
// LSDB key is the point: the key is ze's own value and the octets are what a peer sees.
func TestOSPFv3DefaultInformationUsesASExternalLSType(t *testing.T) {
	eng, router := newV6DefaultEngine(t, `{"originate":true,"always":true,"metric":"7","metric-type":"type-2"}`)
	eng.applyDefaultInformation()

	got, ok := v6DefaultLSA(t, eng, router)
	if !ok {
		t.Fatalf("`default-information originate always` originated no OSPFv3 default LSA")
	}
	if got != uint16(ospfv3types.LSTypeASExternal) {
		t.Errorf("OSPFv3 default wire LS Type = 0x%04X, want 0x4005", got)
	}
	if got&0x6000 != 0x4000 {
		t.Errorf("OSPFv3 default wire LS Type 0x%04X does not carry AS flooding scope (S2S1=10)", got)
	}
}

// TestOSPFv2DefaultInformationKeepsType5 is the negative polarity of the test above: the
// LS Type is chosen by address family rather than stamped 0x4005 everywhere, so an OSPFv2
// engine with the same configuration still originates the one-octet RFC 2328 Type 5.
func TestOSPFv2DefaultInformationKeepsType5(t *testing.T) {
	eng, router := newRedistEngine(t,
		`{"ospf":{"router-id":"10.0.0.1","default-information":{"originate":true,"always":true,"metric":"7"}}}`)
	eng.applyDefaultInformation()

	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea,
		types.LSAKey{Type: types.LSTypeASExternal, AdvertisingRouter: router})
	if !ok {
		t.Fatalf("`default-information originate always` originated no OSPFv2 default LSA")
	}
	// RFC 2328 Appendix A.4.1: the OSPFv2 LSA header is LS Age (2), Options (1), LS Type (1).
	if len(lsa.RawBytes) < 4 {
		t.Fatalf("the OSPFv2 default LSA carries %d raw octets, too short for an LS Type", len(lsa.RawBytes))
	}
	if lsa.RawBytes[3] != 5 {
		t.Errorf("OSPFv2 default wire LS Type = %d, want 5", lsa.RawBytes[3])
	}
}

// TestOSPFv3DefaultInformationConditionReadsIPv6RIB checks that the conditional form asks
// the IPv6 unicast table. An IPv4 default in the Loc-RIB must not satisfy an OSPFv3
// engine's condition, and an IPv6 one must.
func TestOSPFv3DefaultInformationConditionReadsIPv6RIB(t *testing.T) {
	t.Run("ipv4_default_does_not_satisfy_it", func(t *testing.T) {
		installRIBDefault(t)
		eng, router := newV6DefaultEngine(t, `{"originate":true}`)
		eng.applyDefaultInformation()
		if _, ok := v6DefaultLSA(t, eng, router); ok {
			t.Errorf("an IPv4 default in the Loc-RIB must not make an OSPFv3 engine originate ::/0")
		}
	})

	t.Run("ipv6_default_satisfies_it", func(t *testing.T) {
		installRIBDefaultV6(t)
		eng, router := newV6DefaultEngine(t, `{"originate":true}`)
		eng.applyDefaultInformation()
		got, ok := v6DefaultLSA(t, eng, router)
		if !ok {
			t.Fatalf("an IPv6 default in the Loc-RIB must make an OSPFv3 engine originate ::/0")
		}
		if got != uint16(ospfv3types.LSTypeASExternal) {
			t.Errorf("OSPFv3 conditional default wire LS Type = 0x%04X, want 0x4005", got)
		}
	})
}

// TestOSPFv3DefaultInformationSharesItsLSAWithRedistribute checks the pair the two intents
// form: `redistribute` injecting ::/0 and `default-information originate` reach the one
// Link State ID, and a withdraw from one does not drop a default the other still wants.
func TestOSPFv3DefaultInformationSharesItsLSAWithRedistribute(t *testing.T) {
	eng, router := newV6DefaultEngine(t, `{"originate":true,"always":true,"metric":"7"}`)
	eng.applyDefaultInformation()
	fromDefaultInfo, ok := v6DefaultLSA(t, eng, router)
	if !ok {
		t.Fatalf("`default-information originate always` originated no OSPFv3 default LSA")
	}

	if err := eng.InjectExternal(testDefaultRouteV6, "static", 0); err != nil {
		t.Fatalf("redistributing ::/0 into OSPFv3: %v", err)
	}
	fromRedist, ok := v6DefaultLSA(t, eng, router)
	if !ok {
		t.Fatalf("redistributing ::/0 left no OSPFv3 default LSA")
	}
	if fromRedist != fromDefaultInfo {
		t.Errorf("the two intents wrote different LS Types: 0x%04X then 0x%04X", fromDefaultInfo, fromRedist)
	}

	if _, err := eng.WithdrawExternal(testDefaultRouteV6); err != nil {
		t.Fatalf("withdrawing the redistributed ::/0: %v", err)
	}
	if _, ok := v6DefaultLSA(t, eng, router); !ok {
		t.Errorf("withdrawing the redistributed ::/0 purged the default `default-information originate` still wants")
	}
}
