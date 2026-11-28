# Raw BGP injector for the RFC 8097 Section 2 default-eBGP inbound drop leg.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9 (AC1E0009),
# this injector container's own address on the base lab network (172.30.0.0/24).
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s
option=linger:value=true

# 10.97.1.0/24 with ORIGIN=IGP, AS_PATH=[65004], NEXT_HOP=172.30.0.9 (AC1E0009),
# EXTENDED_COMMUNITIES=[target:65001:7 (0002FDE900000007), 0x43/0x00:valid (4300000000000000)].
# attr len = 4+9+7+19 = 39 = 0x0027; total = 19+2+2+39+4 = 66 = 0x0042.
# Ze's ebgp-default-in peer has `export { final reject; }`, so this peer
# announces immediately on Established and lingers while Ze installs 10.97.1.0/24
# in bgp-rib and advertises all three prefixes to GoBGP and both wire speakers.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004202000000274001010040020602010000FDEC400304AC1E0009C010100002FDE9000000074300000000000000180A6101
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
