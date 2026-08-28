# Raw BGP injector for the next-hop-self relay-shape scenario.
#
# Byte for byte the script scenario 52 uses. What differs is Ze's config: the FRR
# peer carries `next-hop self`, so applyFactsNextHop records a NEXT_HOP operation
# against everything relayed to FRR. FRR observes both halves on one session:
#
#   1. The ADVERTISEMENT. Ze forwards it to FRR with the next-hop REWRITTEN to
#      172.30.0.2, its own address. the internal/le/interoplab/bgp checker asserts that, which is
#      what proves
#      next-hop-self is actually engaged rather than silently inert.
#
#   2. The WITHDRAWAL, and the End-of-RIB before it. Neither advertises a route,
#      so RFC 4271 Section 4.3 says the message "will not include path
#      attributes". the internal/le/interoplab/bgp checker asserts FRR raised no attribute error
#      over either
#      and kept the session.
#
# ASN is pinned via inject-args (--asn 65004). The injected NEXT_HOP is
# 172.30.0.9 (this container); Ze replaces it, and the assertion that it did is
# the positive half.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER 1 -- FRR is established and the forward rail carries traffic. Ze dials
# this injector immediately, so everything below would otherwise run before FRR
# had a session at all, and FRR would see neither message.
#
# 10.20.0.0/24 is FRR's OWN prefix (frr.conf `network 10.20.0.0/24`), relayed
# here by Ze. Receiving it is proof rather than assumption. Matching on Ze's
# handshake KEEPALIVE would not be: an unmatched KEEPALIVE or EOR is silently
# accepted while this rule waits (internal/test/peer/checker.go
# ExpectedOrKeepalive), so a KEEPALIVE rule releases milliseconds after the OPEN
# and long before FRR is up.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) Announce 10.10.0.0/24, then End-of-RIB. FRR is up, so this is forwarded
# LIVE -- the scenario does not depend on a replay-on-peer-up path.
# ORIGIN=IGP, AS_PATH=[65004], NEXT_HOP=172.30.0.9, NLRI=10.10.0.0/24.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# BARRIER 2 -- give the internal/le/interoplab/bgp checker a window to observe the advertisement
# before it is
# withdrawn. Ze sends a KEEPALIVE every 30s (a third of the 90s hold), so two of
# them are 30-60s during which FRR holds the route. Withdrawing sooner would race
# the positive assertion, and a positive assertion that loses that race is
# indistinguishable from a missing prepend.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Withdraw 10.10.0.0/24: withdrawnLen=4, attrLen=0, no path attributes --
# exactly the RFC 4271 Section 4.3 shape. What Ze forwards to FRR is the subject
# of this scenario.
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A0A000000

# Hold the session open while the internal/le/interoplab/bgp checker finishes. A peer whose script
# is exhausted
# exits, and its session dropping would make Ze withdraw the route for the wrong
# reason -- which is exactly how an absence assertion passes vacuously.
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
