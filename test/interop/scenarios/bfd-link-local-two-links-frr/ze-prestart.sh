# Runs in Ze's container before ze starts (prepare.go, prestartSuffix).
#
# Two links: VLANs 101 and 102 over eth0 split the lab bridge into two
# segments, and Ze's link-local is on both. eth0 carries no session.
ip link add link eth0 name eth1 type vlan id 101
ip link add link eth0 name eth2 type vlan id 102
ip -6 address add @ZE_LINK_LOCAL@/64 dev eth1 nodad
ip -6 address add @ZE_LINK_LOCAL@/64 dev eth2 nodad
ip link set eth1 up
ip link set eth2 up
# Drop every BFD Control packet Ze sends with Your Discriminator zero: the
# UDP payload's octets 8 to 11 (RFC 5880 Section 4.1), so transport-header
# bit 128 for 32 bits. FRR then learns Ze's discriminator only from a packet
# Ze sends after it selected FRR's first packet by address and link (RFC 5881
# Section 3), and the session comes Up only if that selection works. The
# table is outside the ze_ prefix, so Ze's firewall never sweeps it.
nft -f - <<'NFT'
table inet bfdfirst {
	chain output {
		type filter hook output priority -300; policy accept;
		udp dport 3784 @th,128,32 0 drop
	}
}
NFT
