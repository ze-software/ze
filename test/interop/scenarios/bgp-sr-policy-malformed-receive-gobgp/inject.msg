# Raw BGP injector for the SR Policy malformed-receive scenario.
#
# RFC 9830 Section 5: "An SR Policy update that is determined not to be valid
# (and, therefore, malformed) based on the rules described in Section 4.2.1
# MUST be handled by the "treat-as-withdraw" strategy." Ze must therefore
# WITHDRAW the earlier well-formed generation of each affected policy at its
# downstream peer, never relay the malformed bytes. The checker in
# internal/le/interoplab/bgp (check_extras.go, bgp-sr-policy-malformed-receive-gobgp)
# requires GoBGP to hold A, B and C after (1), to lose A after (2) while B and C
# stay, to lose B after (3) while C stays, with the GoBGP session up throughout.
#
# Every UPDATE is AFI 1 / SAFI 73 in MP_REACH_NLRI with ORIGIN IGP, AS_PATH
# 65004, an IPv4-address Route Target 12.0.0.2:0 and, unless noted, the
# Tunnel Encapsulation attribute ze itself encodes in
# test/encode/bgp-srpolicy-1-encode.ci (one type-15 SR Policy TLV). The NLRI
# and attribute bytes are those of test/plugin/srpolicy-receive-nlri-length.ci
# and test/plugin/srpolicy-receive-no-tunnel-encap.ci with this lab's ASN and
# next hop, and with no LOCAL_PREF on this eBGP session.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=300s

# BARRIER 1 -- 10.20.0.0/24, which the checker adds to GoBGP and Ze relays here,
# proves the GoBGP session is up before anything below is sent.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) Three well-formed policies, length 0x60 (96 bits), distinguisher 0:
#     A color 100 endpoint 10.0.0.1, B color 300 endpoint 10.0.0.3, and the
#     control C color 400 endpoint 10.0.0.5, which nothing below touches.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0073020000005C4001010040020602010000FDEC800E1600014904AC1E0009006000000000000000640A000001C0100801020C0000020000C01728000F00240C060000000000640D06000005DC00008000110009060000000000010106000003E81000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0073020000005C4001010040020602010000FDEC800E1600014904AC1E00090060000000000000012C0A000003C0100801020C0000020000C01728000F00240C060000000000640D06000005DC00008000110009060000000000010106000003E81000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0073020000005C4001010040020602010000FDEC800E1600014904AC1E0009006000000000000001900A000005C0100801020C0000020000C01728000F00240C060000000000640D06000005DC00008000110009060000000000010106000003E81000

# BARRIER 2 -- two of Ze's KEEPALIVEs (30-60s) give the checker its window to
# see A, B and C at GoBGP before the malformed UPDATEs arrive.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) A again beside N, length 0x68 (104 bits, 13 octets): color 200, endpoint
#     10.0.0.2. The NLRI section walks and N has the wrong length, so the UPDATE
#     is treat-as-withdraw: A is withdrawn and N never reaches GoBGP.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0081020000006A4001010040020602010000FDEC800E2400014904AC1E0009006000000000000000640A0000016800000000000000C80A00000200C0100801020C0000020000C01728000F00240C060000000000640D06000005DC00008000110009060000000000010106000003E81000

# BARRIER 3 -- two more KEEPALIVEs: the checker sees A gone and B still held
# before B's malformed generation arrives.
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (3) B again with no Tunnel Encapsulation attribute (RFC 9830 Section 4.2.1):
#     treat-as-withdraw, so B is withdrawn.
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004802000000314001010040020602010000FDEC800E1600014904AC1E00090060000000000000012C0A000003C0100801020C0000020000

# Hold the session open while the checker reads GoBGP. A dropped session would
# withdraw A and B for the wrong reason, which is how an absence passes vacuously.
expect=bgp:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=10:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=11:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
