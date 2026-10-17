# Raw BGP injector for bgp-policy-modify-mp-frr: the UPDATEs whose
# multiprotocol sections Ze's import prefix-list must rebuild.
#
# A raw injector rather than a second FRR, because the claim needs each mixed
# set of prefixes inside ONE UPDATE, and the bytes below are that by
# construction: no daemon's packing decision stands between the scenario and
# what Ze receives. Every UPDATE carries ORIGIN IGP and AS_PATH [65004]
# (4-octet, the injector's ASN pinned in inject-args). An IPv6 next hop is
# ::ffff:172.30.0.9 and an IPv4 one 172.30.0.9.
#
# Ze's import chain on this peer is `bgp-filter-prefix:INJECT-IN`, which
# accepts 2001:db8:1::/48, 10.61.1.0/24 and everything under 2001:db8:100::/48,
# and rejects every other prefix by the implicit deny (ze.conf).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=240s
# Hold the session open after the last directive, so Ze keeps the routes while
# the checker reads FRR. A session that dropped would withdraw them.
option=linger:value=true

# End-of-RIB for IPv4 and IPv6 unicast at once: the route server releases a
# client's routes only after its End-of-RIB.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000201

# Nothing is sent before FRR is a live destination: wait for Ze to relay FRR's
# first sentinel, 2001:db8:f00::/48 (NLRI 30 20010DB80F00). The UPDATEs below
# then take Ze's live forward, never the peer-up replay.
expect=bgp:conn=1:seq=2:contains=3020010DB80F00

# (1) IPv6 keep-one-drop-one. MP_REACH_NLRI (AFI 2, SAFI 1, length 0x23)
# carries 2001:db8:1::/48 (accepted) and 2001:db8:dead::/48 (refused).
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004A02000000334001010040020602010000FDEC800E230002011000000000000000000000FFFFAC1E0009003020010DB800013020010DB8DEAD
# (a) IPv4 unicast carried in MP_REACH_NLRI (AFI 1, SAFI 1, next hop length 4),
# no NEXT_HOP attribute and no legacy NLRI: 10.61.1.0/24 (accepted) and
# 10.61.2.0/24 (refused).
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003802000000214001010040020602010000FDEC800E1100010104AC1E000900180A3D01180A3D02
# (d) All refused: 2001:db8:bad:1::/64 and 2001:db8:bad:2::/64, marked with
# COMMUNITY 65004:4242 (FDEC1092). Ze must drop the UPDATE, so no MP_REACH_NLRI
# without NLRI, and nothing carrying that community, reaches FRR. Sent BEFORE
# (c), whose community 65004:4141 the checker finds in the relay's capture of
# Ze's frames as proof that anything sent here would have arrived by then.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0055020000003E4001010040020602010000FDECC00804FDEC1092800E270002011000000000000000000000FFFFAC1E0009004020010DB80BAD00014020010DB80BAD0002
# (c) Past 255 octets. COMMUNITY 65004:4141 (FDEC102D), then MP_REACH_NLRI with
# the extended-length flag (0x90) and length 0x012A (298): thirty accepted
# /64s 2001:db8:100:1::/64 .. 2001:db8:100:1e::/64 and one refused
# 2001:db8:beef::/48. The rebuilt attribute is 291 octets, so it must leave Ze
# in the extended-length form too.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF015902000001424001010040020602010000FDECC00804FDEC102D900E012A0002011000000000000000000000FFFFAC1E0009004020010DB8010000014020010DB8010000024020010DB8010000034020010DB8010000044020010DB8010000054020010DB8010000064020010DB8010000074020010DB8010000084020010DB8010000094020010DB80100000A4020010DB80100000B4020010DB80100000C4020010DB80100000D4020010DB80100000E4020010DB80100000F4020010DB8010000104020010DB8010000114020010DB8010000124020010DB8010000134020010DB8010000144020010DB8010000154020010DB8010000164020010DB8010000174020010DB8010000184020010DB8010000194020010DB80100001A4020010DB80100001B4020010DB80100001C4020010DB80100001D4020010DB80100001E3020010DB8BEEF

# (b) The withdrawal waits for the checker: it has seen FRR hold
# 2001:db8:1::/48, then made FRR announce the second sentinel,
# 2001:db8:f01::/48 (NLRI 30 20010DB80F01).
expect=bgp:conn=1:seq=4:contains=3020010DB80F01

# (b) MP_UNREACH_NLRI (AFI 2, SAFI 1) withdrawing 2001:db8:1::/48.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0024020000000D800F0A0002013020010DB80001
