// VALIDATES: RFC 5392 Section 3.3.2 on the origination path: an inter-AS TE link whose
// neighboring ASBR is configured with an IPv6 Remote ASBR ID only is originated (through
// teOriginateType6 and buildInterASTELink) with the IPv6 Remote ASBR ID sub-TLV (type 24).
// PREVENTS: an origination that drops remote-asbr-ipv6 when it builds the Link TLV, which the
// packet-level unit cannot see because it hands the encoder a TELSA built by hand.
package ospf

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC5392-3.3.2-2 positive -- with inter-as remote-as 65001 and only
// remote-asbr-ipv6 2001:db8::9 configured (no IPv4 Remote ASBR ID), the originated Inter-AS
// Link TLV carries the IPv6 Remote ASBR ID sub-TLV: the LSA body holds the literal octets
// 00 18 00 10 20 01 0d b8 00 00 00 00 00 00 00 00 00 00 00 09 (type 24, length 16, the
// address), the decoded link has the IPv6 Remote ASBR ID and no IPv4 one.
//
// Goal: prove the configured v6-only ASBR reaches the wire. Method: real config text into a
// test engine, teOriginateType6, literal octets on the originated body, then a decode.
func TestRFC5392OriginatedV6OnlyLinkCarriesIPv6RemoteASBRID(t *testing.T) {
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
	    "traffic-engineering":{"enable":true,"inter-as":{"remote-as":"65001","remote-asbr-ipv6":"2001:db8::9"}}}}}}}`
	eng := teEngineWithTopology(t, cfg, nil)
	out := eng.teOriginateType6(types.RouterID{1, 1, 1, 1})
	var link *opaqueOrigination
	for i := range out {
		if !out[i].Withdraw {
			link = &out[i]
		}
	}
	if link == nil {
		t.Fatalf("no inter-AS Link LSA originated for the v6-only ASBR")
	}
	want := []byte{0x00, 0x18, 0x00, 0x10, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}
	if !bytes.Contains(link.Body, want) {
		t.Fatalf("originated body lacks IPv6 Remote ASBR ID sub-TLV % x: % x", want, link.Body)
	}
	l := decodeOrigTELSA(t, *link).Link
	if !l.HasRemoteASBRv6 {
		t.Fatalf("decoded inter-AS link has no IPv6 Remote ASBR ID: %+v", l)
	}
	if l.HasRemoteASBRv4 {
		t.Fatalf("decoded inter-AS link carries an IPv4 Remote ASBR ID that was never configured: %+v", l)
	}
	if !l.HasRemoteAS || l.RemoteAS != 65001 {
		t.Fatalf("decoded inter-AS link Remote AS = %v/%d, want 65001", l.HasRemoteAS, l.RemoteAS)
	}
}
