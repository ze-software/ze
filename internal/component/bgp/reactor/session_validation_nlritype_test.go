package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: typedNLRIEdit reads the nlrisyntax registry, not a family literal. With the
// real registrations, an EVPN Ethernet A-D NLRI of the wrong length is refused (session
// reset) and a malformed BGP-LS NLRI is removed. With the registry emptied, the same
// sections pass untouched: no family rule is applied by any other route. A family with
// neither a recognizer nor a rule leaves before the attribute is located.
// PREVENTS: a hard-coded family branch beside the registry (ai/rules/no-layering.md,
// spec Q6 (b)).
func TestTypedNLRIEditConsultsRegistry(t *testing.T) {
	s := nlriTypeTestSession()
	evpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	unicast := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
	code := uint8(attribute.AttrMPReachNLRI)

	// Ethernet A-D (type 1) with a 24-octet payload, where RFC 7432 Section 7.1 fixes 25.
	badEVPN := evpnWireNLRI(1, make([]byte, 24)...)
	// A Link-State Node NLRI (type 1) whose one TLV overruns the NLRI it sits in.
	badLS := lsWireNLRI(1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x01, 0x00, 0x00, 0x20)
	edit := func(fam family.Family, nlri []byte) ([]mpNLRIEdit, bool) {
		loc := message.MPNLRILocation{Present: true, AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI)}
		pathAttrs := mpReachAttrs(fam, nlri)
		attrs := nlritype.NewUpdateAttrs(pathAttrs)
		return s.typedNLRIEdit(nil, code, loc, pathAttrs, &attrs, nil)
	}

	if _, ok := edit(evpn, badEVPN); ok {
		t.Fatal("EVPN: a wrong Ethernet A-D length was accepted with the rule registered")
	}
	edits, ok := edit(bgpls, badLS)
	if !ok || len(edits) != 1 || edits[0].dropped != 1 || len(edits[0].nlri) != 0 {
		t.Fatalf("BGP-LS: ok=%v edits=%+v, want the malformed NLRI removed", ok, edits)
	}
	if edits, ok := edit(unicast, []byte{0x40, 0x20, 0x01, 0x0d, 0xb8}); !ok || len(edits) != 0 {
		t.Fatalf("IPv6 unicast: ok=%v edits=%+v, want untouched", ok, edits)
	}

	t.Cleanup(nlrisyntax.SnapshotForTest())
	if edits, ok := edit(evpn, badEVPN); !ok || len(edits) != 0 {
		t.Fatalf("EVPN with no rule: ok=%v edits=%+v, want untouched", ok, edits)
	}
	edits, ok = edit(bgpls, badLS)
	if !ok || len(edits) != 0 {
		t.Fatalf("BGP-LS with no rule: ok=%v edits=%+v, want untouched", ok, edits)
	}
}
