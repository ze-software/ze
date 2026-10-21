# Raw BGP injector for the MUP treat-as-withdraw scenario.
#
# draft-ietf-bess-mup-safi Section 3.1.3.1 makes a T1ST whose mandatory fields
# exceed its declared Length malformed, and the route is treated as withdrawn.
# Ze must therefore WITHDRAW the earlier well-formed generation of that route,
# not keep it. The checker in internal/le/interoplab/bgp (checkers.go,
# bgp-mup-malformed-t1st-withdraw-gobgp) requires GoBGP to hold K
# (10.0.0.1/32) after (1), then to lose K and hold L (10.0.0.2/32) after (2),
# with the GoBGP session still up. The bytes are those of
# test/plugin/mup-t1st-malformed-treat-as-withdraw.ci with this lab's ASN and
# next hop.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=240s

# BARRIER 1 -- 10.20.0.0/24, which the checker adds to GoBGP and Ze relays here,
# proves the GoBGP session is up before anything below is sent.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) Well-formed T1ST K: RD 65001:1, 10.0.0.1/32, TEID 1, QFI 9, endpoint
# 192.0.2.1, Source Address Length 0 (Length 0x18).
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004C02000000354001010040020602010000FDEC800E2500015504AC1E000900010003180000FDE900000001200A000001000000010920C000020100

# BARRIER 2 -- two of Ze's KEEPALIVEs (30-60s) give the checker its window to
# see K at GoBGP before the malformed generation arrives.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) K' (same key, Length 0x16: the endpoint overruns it) and well-formed L
# (10.0.0.2/32) in one MP_REACH_NLRI.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0066020000004F4001010040020602010000FDEC800E3F00015504AC1E000900010003160000FDE900000001200A000001000000010920C00002010003180000FDE900000001200A000002000000010920C000020100

# Hold the session open while the checker reads GoBGP. A dropped session would
# withdraw K for the wrong reason, which is how an absence passes vacuously.
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
