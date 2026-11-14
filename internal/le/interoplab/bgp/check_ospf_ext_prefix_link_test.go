package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// TestOSPFExtPrefixLinkVerdict drives the Extended Prefix/Link checker's verdict on
// answers shaped like Ze's `show ospf database opaque-area` (the generic LSDB rows,
// then the decoded section ext_render.go appends). Each refused case is one wrong
// reading the earlier substring check accepted: Ze's own Extended LSAs alone, FRR
// holding only one of the two types, or an FRR opaque LSA of another type in the
// generic rows. The accepted case holds both types from both routers.
func TestOSPFExtPrefixLinkVerdict(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	frr := netip.MustParseAddr("172.30.0.3")
	generic := `{"areas":[{"area":"0.0.0.0","lsas":[` +
		`{"type":"opaque-area","link-state-id":"4.0.0.0","advertising-router":"172.30.0.3"},` +
		`{"type":"opaque-area","link-state-id":"7.0.0.1","advertising-router":"172.30.0.2"}]}],"as-external":[]}`
	decoded := func(prefix, link string) string {
		return "[" + generic + `,{"extended-prefix":[` + prefix + `],"extended-link":[` + link + `]}]`
	}
	row := func(router string) string {
		return `{"advertising-router":"` + router + `","scope":"area","opaque-id":1}`
	}
	cases := []struct {
		name   string
		output string
		refuse string
	}{
		{"only Ze's own Extended LSAs", decoded(row("172.30.0.2"), row("172.30.0.2")), "Extended Prefix LSA advertised by 172.30.0.3"},
		{"FRR Extended Prefix without FRR Extended Link", decoded(row("172.30.0.2")+","+row("172.30.0.3"), row("172.30.0.2")), "Extended Link LSA advertised by 172.30.0.3"},
		{"FRR Extended Link without FRR Extended Prefix", decoded(row("172.30.0.2"), row("172.30.0.2")+","+row("172.30.0.3")), "Extended Prefix LSA advertised by 172.30.0.3"},
		{"FRR's rows without Ze's own", decoded(row("172.30.0.3"), row("172.30.0.3")), "Extended Prefix LSA advertised by 172.30.0.2"},
		{"no decoded section, FRR only in the generic rows", "[" + generic + "]", "Extended Prefix LSA advertised by 172.30.0.3"},
		{"not JSON", "extended-prefix extended-link 172.30.0.3", "decode Ze's opaque-area database"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ospfExtPrefixLinkVerdict(tc.output, frr, ze)
			if err == nil {
				t.Fatalf("verdict accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.refuse) {
				t.Fatalf("verdict = %v, want it to name %q", err, tc.refuse)
			}
		})
	}
	both := row("172.30.0.2") + "," + row("172.30.0.3")
	if err := ospfExtPrefixLinkVerdict(decoded(both, both), frr, ze); err != nil {
		t.Fatalf("verdict refused both Extended types from both routers: %v", err)
	}
}

// TestFRROSPFExtFromZeVerdict drives assertion 3's verdict on answers shaped like FRR
// 10.7.1's `show ip ospf database opaque-area json` (show_lsa_detail and
// show_ip_ospf_database_header in ospfd/ospf_vty.c, show_opaque_info_detail in
// ospfd/ospf_opaque.c, show_vty_link_info in ospfd/ospf_ext.c). The LSAs are the
// lab's own: the passing recording of 2026-11-14 holds FRR's Router Information,
// Extended Prefix and Extended Link LSAs and Ze's Extended Prefix and Extended Link
// LSAs. The first refused case is the one the earlier substring check accepted: FRR
// holds only its own LSAs, and its Extended Link LSA names Ze as the link's Link ID.
func TestFRROSPFExtFromZeVerdict(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	// FRR's own LSAs, with the opaque values FRR's show functions add.
	const frrRI = `{"lsaAge":0,"options":"*|O|-|-|-|-|E|-","lsaFlags":1,"lsaType":"Area-Local Opaque-LSA",` +
		`"linkStateId":"4.0.0.0","advertisingRouter":"172.30.0.3","lsaSeqNumber":"80000001","checksum":"8247","length":76,` +
		`"opaqueType":"Router Information LSA","opaqueId":0,"opaqueLength":56,"opaqueLengthValid":true,"opaqueValues":{}}`
	const frrPrefix = `{"lsaAge":0,"options":"*|O|-|-|-|-|E|-","lsaFlags":1,"lsaType":"Area-Local Opaque-LSA",` +
		`"linkStateId":"7.0.0.1","advertisingRouter":"172.30.0.3","lsaSeqNumber":"80000001","checksum":"6dd0","length":44,` +
		`"opaqueType":"Extended Prefix Opaque LSA","opaqueId":1,"opaqueLength":24,"opaqueLengthValid":true,` +
		`"opaqueValues":{"extendedPrefix":{"routeType":1,"addressFamily":"0x0","flags":"0x40","address":"10.255.255.3","prefixLength":32,` +
		`"prefixSID":{"algorithm":0,"flags":"0x0","mtID":"0x0","index":100}}}}`
	const frrLink = `{"lsaAge":0,"options":"*|O|-|-|-|-|E|-","lsaFlags":1,"lsaType":"Area-Local Opaque-LSA",` +
		`"linkStateId":"8.0.0.1","advertisingRouter":"172.30.0.3","lsaSeqNumber":"80000001","checksum":"3ff","length":68,` +
		`"opaqueType":"Extended Link Opaque LSA","opaqueId":1,"opaqueLength":48,"opaqueLengthValid":true,` +
		`"opaqueValues":{"extendedLink":{"linkType":"0x1","linkID":"172.30.0.2","linkData":"172.30.0.3",` +
		`"adjacencySID":[{"flags":"0xe0","mtID":"0x0","weight":"0x0","label":15000},{"flags":"0x60","mtID":"0x0","weight":"0x0","label":15001}],` +
		`"remoteInterfaceAddress":"172.30.0.2"}}}`
	// Ze's LSAs as FRR stores them.
	const zePrefix = `{"lsaAge":14,"options":"*|O|-|-|-|-|-|-","lsaFlags":6,"lsaType":"Area-Local Opaque-LSA",` +
		`"linkStateId":"7.0.0.1","advertisingRouter":"172.30.0.2","lsaSeqNumber":"80000001","checksum":"c78a","length":32,` +
		`"opaqueType":"Extended Prefix Opaque LSA","opaqueId":1,"opaqueLength":12,"opaqueLengthValid":true,` +
		`"opaqueValues":{"extendedPrefix":{"routeType":1,"addressFamily":"0x0","flags":"0x0","address":"172.30.0.0","prefixLength":24,"prefixSID":{}}}}`
	const zeLink = `{"lsaAge":6,"options":"*|O|-|-|-|-|-|-","lsaFlags":6,"lsaType":"Area-Local Opaque-LSA",` +
		`"linkStateId":"8.0.0.1","advertisingRouter":"172.30.0.2","lsaSeqNumber":"80000001","checksum":"6829","length":36,` +
		`"opaqueType":"Extended Link Opaque LSA","opaqueId":1,"opaqueLength":16,"opaqueLengthValid":true,` +
		`"opaqueValues":{"extendedLink":{"linkType":"0x1","linkID":"172.30.0.3","linkData":"172.30.0.2","adjacencySID":[]}}}`
	database := func(lsas ...string) string {
		return `{"routerId":"172.30.0.3","areaLocalOpaqueLsa":{"areas":{"0.0.0.0":[` + strings.Join(lsas, ",") + `]}}}`
	}
	refused := []struct {
		name   string
		output string
		refuse string
	}{
		{"only FRR's own LSAs, its Extended Link naming Ze as Link ID", database(frrRI, frrPrefix, frrLink), "no Extended Prefix LSA (opaque type 7) advertised by 172.30.0.2"},
		{"Ze's Extended Prefix without its Extended Link", database(frrRI, zePrefix, frrPrefix, frrLink), "no Extended Link LSA (opaque type 8) advertised by 172.30.0.2"},
		{"Ze's Extended Link without its Extended Prefix", database(frrRI, frrPrefix, frrLink, zeLink), "no Extended Prefix LSA (opaque type 7) advertised by 172.30.0.2"},
		{"no areaLocalOpaqueLsa object", `{"routerId":"172.30.0.3"}`, "no areaLocalOpaqueLsa object"},
		{"the text form", "OSPF Router with ID (172.30.0.3)\n  Advertising Router: 172.30.0.2\n  Opaque-Type 7 (Extended Prefix Opaque LSA)\n", "decode FRR's opaque-area database"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := frrOSPFExtFromZeVerdict(tc.output, ze)
			if err == nil {
				t.Fatalf("verdict accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.refuse) {
				t.Fatalf("verdict = %v, want it to name %q", err, tc.refuse)
			}
		})
	}
	if err := frrOSPFExtFromZeVerdict(database(frrRI, zePrefix, frrPrefix, zeLink, frrLink), ze); err != nil {
		t.Fatalf("verdict refused Ze's Extended Prefix and Extended Link LSAs: %v", err)
	}
}
