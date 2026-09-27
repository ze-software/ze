// VALIDATES: RFC 3101 Section 3.2 step (2) on both address families: a translator yields a
// Type-7 only to a strictly-higher-Router-ID NSSA translator (a router whose Router-LSA in the
// NSSA carries the B-bit) that originated a functionally equivalent Type-5, meaning the same
// destination and mask, the same metric and the same non-zero forwarding address (owner
// decision D-12).
// PREVENTS: translation suppressed by any higher-Router-ID Type-5 that merely shares the Link
// State ID, which leaves the NSSA route out of the backbone when the other Type-5 describes a
// different cost, a different forwarding address, or comes from a router that translates
// nothing.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// yieldCaseV2 names one higher-Router-ID Type-5 the elected translator meets.
type yieldCaseV2 struct {
	translator bool // the higher router's Router-LSA in the NSSA carries the B-bit
	mask       string
	metric     uint32
	forwarding string
}

// translatesBesideV2 runs one translation pass on the elected OSPFv2 translator 10.0.6.9 over
// a P=1 Type-7 for 10.25.0.0/16, metric 10, forwarding address 10.5.0.2, while 10.0.6.250
// advertises the Type-5 described by other. It reports whether 10.0.6.9 originated a Type-5.
func translatesBesideV2(t *testing.T, other yieldCaseV2) bool {
	t.Helper()
	eng, nssa := nssaTransEngine(t, "10.0.6.9")
	self := ridOf("10.0.6.9")
	higher := ridOf("10.0.6.250")
	eng.lsdb.OriginateNSSA(nssa, ridOf("10.0.6.2"), ip4Of("10.25.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
	if other.translator {
		require.True(t, eng.lsdb.Install(nssa, packet.LSA{
			Header: packet.LSAHeader{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(higher), AdvertisingRouter: higher, Sequence: types.InitialSequenceNumber},
			Router: &packet.RouterLSA{Flags: packet.RouterFlagB},
		}))
	}
	_, _, err := eng.lsdb.OriginateExternal(higher, ip4Of("10.25.0.0"), ip4Of(other.mask), types.OptionE, false, other.metric, ip4Of(other.forwarding), 0)
	require.NoError(t, err)
	eng.translateNSSA(transTime)
	return eng.lsdb.SelfExternalCount(self) == 1
}

func TestRFC3101TranslatorYieldsOnlyToEquivalentType5(t *testing.T) {
	// RFC requirement: RFC3101-3.2-2 negative -- a strictly-higher-Router-ID NSSA translator
	// that originated a Type-5 with the same destination, cost and non-zero forwarding address
	// makes this translator yield: it generates no Type-5.
	if translatesBesideV2(t, yieldCaseV2{translator: true, mask: "255.255.0.0", metric: 10, forwarding: "10.5.0.2"}) {
		t.Fatalf("translated although a higher-Router-ID translator originated an equivalent Type-5")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- a higher-Router-ID translator's Type-5 with a
	// different cost is not functionally equivalent, so this translator still generates one.
	if !translatesBesideV2(t, yieldCaseV2{translator: true, mask: "255.255.0.0", metric: 20, forwarding: "10.5.0.2"}) {
		t.Fatalf("yielded to a Type-5 with a different metric")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- a different forwarding address is not
	// functionally equivalent either.
	if !translatesBesideV2(t, yieldCaseV2{translator: true, mask: "255.255.0.0", metric: 10, forwarding: "10.5.0.3"}) {
		t.Fatalf("yielded to a Type-5 with a different forwarding address")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- the same Link State ID with another mask is
	// a different destination.
	if !translatesBesideV2(t, yieldCaseV2{translator: true, mask: "255.255.255.0", metric: 10, forwarding: "10.5.0.2"}) {
		t.Fatalf("yielded to a Type-5 for a different destination")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- an equivalent Type-5 from a router that is
	// not an NSSA translator (no B-bit Router-LSA in the NSSA) does not make this one yield.
	if !translatesBesideV2(t, yieldCaseV2{translator: false, mask: "255.255.0.0", metric: 10, forwarding: "10.5.0.2"}) {
		t.Fatalf("yielded to a Type-5 from a router that is not an NSSA translator")
	}
}

// translatesBesideV6 is the OSPFv3 form of translatesBesideV2: the elected translator
// 10.0.9.9 meets a Type-5 from 10.0.9.99 for 2001:db8:78::/64 with the given metric and
// forwarding address, under a different Link State ID than the Type-7 (OSPFv3 Link State IDs
// carry no addressing, so a peer picks its own).
func translatesBesideV6(t *testing.T, translator bool, metric uint32, forwarding string) bool {
	t.Helper()
	eng, self := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa","nssa":{"translate-role":"always"}}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.9"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	higher := types.RouterID{10, 0, 9, 99}
	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:78::/64"), 0)
	require.True(t, ok)
	nssaPrefix := prefix
	nssaPrefix.Options |= ospfv3types.OptPrefixP
	nssaBody := ospfv3packet.ExternalLSA{Metric: 44, Prefix: nssaPrefix, ForwardingAddr: netip.MustParseAddr("2001:db8:9::2").As16(), HasForwardingAddr: true}
	installV6NSSAForTest(t, eng, nssa, types.RouterID{10, 0, 9, 2}, v6SummaryLSID(78), nssaBody, 1, ospfv3types.InitialSequenceNumber)
	if translator {
		rtr := ospfv3packet.RouterLSA{Flags: ospfv3packet.RouterFlagB, Options: ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptN}
		require.True(t, eng.lsdb.Install(nssa, v6SelfLSA(ospfv3packet.LSA{
			Header: ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeRouter, AdvertisingRouter: ospfv3types.RouterID(higher), Sequence: ospfv3types.InitialSequenceNumber},
			Router: &rtr,
		})))
	}
	otherBody := ospfv3packet.ExternalLSA{Metric: metric, Prefix: prefix, ForwardingAddr: netip.MustParseAddr(forwarding).As16(), HasForwardingAddr: true}
	require.True(t, eng.lsdb.Install(types.BackboneArea, v6SelfLSA(ospfv3packet.LSA{
		Header:   ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeASExternal, LinkStateID: ospfv3types.LinkStateID(v6SummaryLSID(91)), AdvertisingRouter: ospfv3types.RouterID(higher), Sequence: ospfv3types.InitialSequenceNumber},
		External: &otherBody,
	})))
	eng.translateNSSA(transTime)
	_, found := decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(self, v6SummaryLSID(78)))
	return found
}

func TestRFC3101TranslatorYieldsOnlyToEquivalentType5V6(t *testing.T) {
	// RFC requirement: RFC3101-3.2-2 negative -- OSPFv3: an equivalent Type-5 (same prefix,
	// cost and forwarding address) from a higher-Router-ID NSSA translator, under another Link
	// State ID, makes this translator yield.
	if translatesBesideV6(t, true, 44, "2001:db8:9::2") {
		t.Fatalf("translated although a higher-Router-ID translator originated an equivalent Type-5")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- OSPFv3: a different cost is not equivalent.
	if !translatesBesideV6(t, true, 45, "2001:db8:9::2") {
		t.Fatalf("yielded to a Type-5 with a different metric")
	}
	// RFC requirement: RFC3101-3.2-2 positive -- OSPFv3: a router that is not an NSSA
	// translator does not make this one yield.
	if !translatesBesideV6(t, false, 44, "2001:db8:9::2") {
		t.Fatalf("yielded to a Type-5 from a router that is not an NSSA translator")
	}
}
