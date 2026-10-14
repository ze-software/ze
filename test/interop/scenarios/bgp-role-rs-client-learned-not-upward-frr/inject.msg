# Raw BGP injector for the RFC 9234 Section 3.1 rs-client scenario.
#
# RFC 9234 Section 3.1: routes learned from an RS-Client must not be sent to
# a Provider, a Peer or a Route Server. Ze's role toward this injector is rs
# (ze.conf), and the injector announces role RS-Client (capability 9, value 2).
# Ze's role toward FRR is customer, so FRR is Ze's Provider and the route below
# MUST NOT reach it. The checker in internal/le/interoplab/bgp (checkers.go,
# bgp-role-rs-client-learned-not-upward-frr) requires FRR to hold Ze's own
# control route 10.77.4.0/24 and never 10.10.0.0/24, after Ze's RIB has
# counted the injected route.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s
option=open:value=add-capability:code=9:hex=02

# 10.10.0.0/24 and End-of-RIB, sent as soon as Ze opens the session.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the checker reads FRR. A dropped session would
# withdraw the route, and its absence at FRR would then prove nothing.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
