# Raw BGP injector for the next-hop self + `local ip auto` scenario.
#
# One announcement, 10.10.0.0/24 with NEXT_HOP 172.30.0.9 (this container). Ze
# forwards it to FRR, whose peer carries `next-hop self` and no configured local
# address, and the checker requires FRR to hold it with NEXT_HOP 172.30.0.2
# (Ze's connected endpoint), never 172.30.0.9.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER -- FRR is established and the forward rail carries traffic.
# 10.20.0.0/24 is FRR's OWN prefix (frr.conf `network 10.20.0.0/24`), relayed
# here by Ze. A KEEPALIVE rule would release right after the OPEN, long before
# FRR is up (internal/test/peer/checker.go ExpectedOrKeepalive).
expect=bgp:conn=1:seq=1:contains=180A1400

# The forwarded route. ORIGIN=IGP, AS_PATH=[65004], NEXT_HOP=172.30.0.9
# (AC1E0009), NLRI=10.10.0.0/24.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00

# End-of-RIB.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the checker reads FRR. A peer whose script is
# exhausted exits, and Ze would then withdraw the route. Ze sends a KEEPALIVE
# every 30s, so four of them hold the session for 2-4 minutes.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
