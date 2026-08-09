// Design: docs/architecture/isis/isis-12-ipv6.md -- IPv6 origination scope filtering (RFC 5308).
//
// RFC: rfc/short/rfc5308.md sec 2 -- "Link-local prefixes MUST NOT be advertised
//   using this TLV [236]."
// RFC: rfc/short/rfc5308.md sec 3 -- "For LSPs, this TLV [232] MUST contain only
//   the non-link-local IPv6 addresses assigned to the IS." (The link-local
//   addresses go in the IIH TLV 232, originated by the circuit layer.)
//
// These pure helpers enforce the RFC 5308 address-scope rules at the point where
// the engine builds a LevelState for an LSP: TLV 236 excludes link-local prefixes,
// and the LSP TLV 232 excludes link-local interface addresses. The codec
// (packet/tlv_ipv6.go) round-trips whatever it is given; the scope policy lives
// here (and in the circuit layer for the Hello side).

package lsdb

import "net/netip"

// NonLinkLocalV6Prefixes returns the subset of in that are non-link-local IPv6
// prefixes, ready to originate as TLV 236 entries (RFC 5308 sec 2: link-local
// prefixes MUST NOT be advertised in TLV 236). A prefix is link-local when its
// network address is in fe80::/10. Invalid or non-IPv6 prefixes are dropped.
func NonLinkLocalV6Prefixes(in []PrefixInfoV6) []PrefixInfoV6 {
	out := make([]PrefixInfoV6, 0, len(in))
	for _, p := range in {
		a := p.Prefix.Addr()
		if !p.Prefix.IsValid() || !a.Is6() || a.Is4In6() {
			continue
		}
		if a.IsLinkLocalUnicast() {
			continue // RFC 5308 sec 2: no link-local prefixes in TLV 236
		}
		out = append(out, p)
	}
	return out
}

// NonLinkLocalV6Addrs returns the subset of in that are non-link-local IPv6
// addresses, for the LSP TLV 232 (RFC 5308 sec 3: an LSP carries only
// non-link-local addresses). Invalid or non-IPv6 addresses are dropped. The
// link-local addresses are advertised separately in the IIH TLV 232 by the
// circuit layer (the Hello scope).
func NonLinkLocalV6Addrs(in []netip.Addr) []netip.Addr {
	out := make([]netip.Addr, 0, len(in))
	for _, a := range in {
		if !a.IsValid() || !a.Is6() || a.Is4In6() {
			continue
		}
		if a.IsLinkLocalUnicast() {
			continue // RFC 5308 sec 3: an LSP TLV 232 carries only non-link-local
		}
		out = append(out, a)
	}
	return out
}
