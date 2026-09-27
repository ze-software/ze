// VALIDATES: RFC 5340 Appendix A.4.7 on the NSSA translator: a Type-5 translated from a
// received NSSA-LSA never carries a link-local forwarding address.
// PREVENTS: translateNSSAV6 copying a peer's fe80::/10 forwarding address into the
// backbone, where no router outside that link can reach it.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// translateV6WithForwarding runs one translation pass on an elected NSSA ABR over a
// received P=1 NSSA-LSA carrying forwarding address fa, and reports whether a Type-5 was
// originated and the forwarding address it carries.
func translateV6WithForwarding(t *testing.T, fa netip.Addr) (netip.Addr, bool) {
	t.Helper()
	eng, self := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa","nssa":{"translate-role":"always"}}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.9"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	lsid := v6SummaryLSID(78)
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:78::/64"), 0)
	if !ok {
		t.Fatalf("prefix conversion failed")
	}
	pfx.Options |= ospfv3types.OptPrefixP
	body := ospfv3packet.ExternalLSA{Metric: 44, Prefix: pfx, ForwardingAddr: fa.As16(), HasForwardingAddr: true}
	installV6NSSAForTest(t, eng, nssa, types.RouterID{10, 0, 9, 2}, lsid, body, 1, ospfv3types.InitialSequenceNumber)

	eng.translateNSSA(transTime)
	translated, found := decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(self, lsid))
	if !found {
		return netip.Addr{}, false
	}
	return netip.AddrFrom16(translated.ForwardingAddr), true
}

// TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding drives translateNSSAV6 through
// translateNSSA with a global and with a link-local forwarding address.
func TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding(t *testing.T) {
	// RFC requirement: RFC5340-A.4.7-1 positive -- a global forwarding address is carried
	// into the translated Type-5 unchanged.
	global := netip.MustParseAddr("2001:db8:9::2")
	got, ok := translateV6WithForwarding(t, global)
	if !ok {
		t.Fatalf("a P=1 NSSA-LSA with a global forwarding address was not translated")
	}
	if got != global {
		t.Fatalf("translated forwarding address = %v, want %v", got, global)
	}

	// RFC requirement: RFC5340-A.4.7-1 negative -- a received NSSA-LSA whose forwarding
	// address is link-local (fe80::/10) is not translated, so no Type-5 carries it.
	got, ok = translateV6WithForwarding(t, netip.MustParseAddr("fe80::2"))
	if ok {
		t.Fatalf("translated Type-5 advertises link-local forwarding address %v; RFC 5340 A.4.7 forbids it", got)
	}
}

// TestRFC5340TranslatorAdvertisesGlobalForwarding pins the second half of A.4.7: the
// forwarding address a translator advertises is a global IPv6 address.
func TestRFC5340TranslatorAdvertisesGlobalForwarding(t *testing.T) {
	// RFC requirement: RFC5340-A.4.7-2 positive -- the translated Type-5's forwarding
	// address is a global unicast address.
	got, ok := translateV6WithForwarding(t, netip.MustParseAddr("2001:db8:9::3"))
	if !ok || !got.IsGlobalUnicast() {
		t.Fatalf("translated forwarding address = %v (originated %v), want a global address", got, ok)
	}

	// RFC requirement: RFC5340-A.4.7-2 negative -- a link-local forwarding address is not
	// global, so the translator advertises no Type-5 rather than the link-local address.
	if got, ok := translateV6WithForwarding(t, netip.MustParseAddr("fe80::3")); ok {
		t.Fatalf("translator advertised non-global forwarding address %v", got)
	}
}
