# Raw BGP injector for the RFC 8092 Section 3 egress scenario.
#
# RFC 8092 Section 3: "Duplicate BGP Large Community values MUST NOT be
# transmitted." The injector sends 10.10.0.0/24 carrying 65000:1:1 and
# 65000:3:3. Ze's export policy toward GoBGP (ze.conf) adds 65000:1:1, which
# the route already carries, and 65000:2:2 from two community definitions. The
# checker in internal/le/interoplab/bgp (checkers.go,
# bgp-large-community-no-repeat-gobgp) requires GoBGP to hold exactly
# 65000:1:1, 65000:3:3, 65000:2:2, each once.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER -- 10.20.0.0/24, which the checker adds to GoBGP and Ze relays here,
# proves the GoBGP session is up before the route below is sent.
expect=bgp:conn=1:seq=1:contains=180A1400

# 10.10.0.0/24, LARGE_COMMUNITY C0 20 18 with 65000:1:1 and 65000:3:3.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004A020000002F4001010040020602010000FDEC400304AC1E0009C020180000FDE800000001000000010000FDE80000000300000003180A0A00
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the checker reads GoBGP.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
