// Design: docs/architecture/testing/interop.md -- pure decisions over foreign daemon output.
// Related: check_rfc.go -- checkVPNFRR, the RFC 4659 checker that queries FRR and BIRD.
// Related: check_rfc_predicate.go -- parseFRRNextHops, the unicast reader this one unwraps to.
// Overview: the VPN-IPv6 next-hop readers, kept out of check_rfc_predicate.go,
// which holds no lab handle either and already reaches the file-size bound.
package bgp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// parseFRRVPNNextHops decodes FRR's `show bgp ipv6 vpn rd <rd> <prefix> json`.
// FRR 10.3.1 nests the prefix document under its Route Distinguisher,
// `{"65001:100":{"prefix":...,"paths":[...]}}`, so the document is unwrapped at
// the RD the caller queried before the single path is read. An answer without
// that key is an error, never an empty path list: FRR answering about another RD
// is not FRR holding no route.
func parseFRRVPNNextHops(output, rd string) ([]nextHop, error) {
	var byRD map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &byRD); err != nil {
		return nil, fmt.Errorf("decode FRR VPN route JSON: %w", err)
	}
	document, ok := byRD[rd]
	if !ok {
		return nil, fmt.Errorf("FRR VPN route JSON has no %s key", rd)
	}
	return parseFRRNextHops(string(document))
}

// birdVPNNextHops reads the BGP.next_hop line BIRD prints for one VPN route in
// `show route table <table> all`. BIRD heads a route with `<rd> <prefix>` and
// indents its attributes below it, so the block runs from that heading to the
// next unindented line. BIRD prints one address for a Next Hop that carried no
// link-local, and the global then the link-local for one that did
// (proto/bgp/attrs.c, bgp_format_next_hop). A missing route or a route with no
// BGP.next_hop line is an error rather than an empty answer.
func birdVPNNextHops(output, rd, prefix string) ([]netip.Addr, error) {
	heading := rd + " " + prefix + " "
	inRoute := false
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, heading) {
			inRoute = true
			continue
		}
		if !inRoute {
			continue
		}
		if line == "" || (line[0] != ' ' && line[0] != '\t') {
			break
		}
		value, found := strings.CutPrefix(strings.TrimSpace(line), "BGP.next_hop:")
		if !found {
			continue
		}
		fields := strings.Fields(value)
		addresses := make([]netip.Addr, 0, len(fields))
		for _, field := range fields {
			address, err := netip.ParseAddr(field)
			if err != nil {
				return nil, fmt.Errorf("invalid BIRD next hop %q: %w", field, err)
			}
			addresses = append(addresses, address)
		}
		return addresses, nil
	}
	if inRoute {
		return nil, fmt.Errorf("BIRD route %s %s has no BGP.next_hop line", rd, prefix)
	}
	return nil, fmt.Errorf("BIRD holds no route %s %s", rd, prefix)
}

// requireBIRDNextHops reports whether BIRD decoded exactly the wanted next-hop
// addresses: the global alone when linkLocal is the zero Addr, the global then
// the link-local otherwise. The count is compared with the addresses, so a
// link-local BIRD shows that the caller did not expect fails as surely as a
// missing one.
func requireBIRDNextHops(decoded []netip.Addr, global, linkLocal netip.Addr) error {
	want := []netip.Addr{global}
	if linkLocal.IsValid() {
		want = append(want, linkLocal)
	}
	if !slices.Equal(decoded, want) {
		return fmt.Errorf("BIRD decoded next hops %v, want %v", decoded, want)
	}
	return nil
}
