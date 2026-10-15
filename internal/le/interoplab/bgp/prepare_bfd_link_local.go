// Design: docs/architecture/testing/interop.md -- BFD over link-local against FRR.
// Related: prepare.go -- scenarioPrestarts, renderPrestarts, assignLinkLocal, which run these.
// Related: check_bfd_link_local.go -- checkBFDLinkLocalTwoLinks, the checker these make discriminating.
package bgp

// bfdLinkLocalTwoLinksZePrestart runs in Ze's container before ze starts.
//
// Two links: VLANs 101 and 102 over eth0 split the lab bridge into two
// segments, and Ze's link-local is on both. eth0 carries no session.
//
// The nft table drops every BFD Control packet Ze sends with Your
// Discriminator zero: the UDP payload's octets 8 to 11 (RFC 5880 Section 4.1),
// so transport-header bit 128 for 32 bits. FRR then learns Ze's discriminator
// only from a packet Ze sends after it selected FRR's first packet by address
// and link (RFC 5881 Section 3), and the session comes Up only if that
// selection works. The table is outside the ze_ prefix, so Ze's firewall never
// sweeps it.
const bfdLinkLocalTwoLinksZePrestart = `ip link add link eth0 name eth1 type vlan id 101
ip link add link eth0 name eth2 type vlan id 102
ip -6 address add ` + zeLinkLocalPlaceholder + `/64 dev eth1 nodad
ip -6 address add ` + zeLinkLocalPlaceholder + `/64 dev eth2 nodad
ip link set eth1 up
ip link set eth2 up
nft -f - <<'NFT'
table inet bfdfirst {
	chain output {
		type filter hook output priority -300; policy accept;
		udp dport 3784 @th,128,32 0 drop
	}
}
NFT
`

// bfdLinkLocalTwoLinksFRRPrestart runs in FRR's container before FRR starts.
//
// The same two VLANs as Ze's prestart, and FRR's ONE link-local on both, so
// the address alone cannot tell the two sessions apart: only the link can.
const bfdLinkLocalTwoLinksFRRPrestart = `ip link add link eth0 name eth1 type vlan id 101
ip link add link eth0 name eth2 type vlan id 102
ip -6 address add ` + frrLinkLocalPlaceholder + `/64 dev eth1 nodad
ip -6 address add ` + frrLinkLocalPlaceholder + `/64 dev eth2 nodad
ip link set eth1 up
ip link set eth2 up
`

// bfdLinkLocalTwoLinksPrestarts is the scenario's prestart for each owner.
var bfdLinkLocalTwoLinksPrestarts = ownerPrestarts{
	zeLinkLocalOwner:  bfdLinkLocalTwoLinksZePrestart,
	frrLinkLocalOwner: bfdLinkLocalTwoLinksFRRPrestart,
}
