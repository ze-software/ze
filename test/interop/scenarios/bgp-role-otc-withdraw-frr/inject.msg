# Raw BGP injector for the RFC 9234 Section 5 egress scenario.
#
# It gives Ze one route to relay to FRR, then withdraws it, so FRR observes both
# halves of the same rule on one session:
#
#   1. The ADVERTISEMENT. Ze forwards it to FRR and must add the OTC Attribute
#      (RFC 9234 Section 5 egress rule 1: FRR is a Customer). check.py asserts FRR
#      reports OTC 65001 on the route.
#
#   2. The WITHDRAWAL. It advertises no route, so rule 1's condition is not met
#      and no OTC may be added. check.py asserts FRR's own decode of that UPDATE
#      reports no path attributes.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9 (this
# container).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER 1 -- FRR is established and the forward rail carries traffic. Ze dials
# this injector immediately, so everything below would otherwise run before FRR
# had a session at all: the first version of this file waited for "Ze's first
# KEEPALIVE" and matched the one BGP sends to confirm the OPEN, milliseconds in.
# The whole exchange then completed against an empty peer set (Ze logged "forward
# matched no target ... peers-known=1") and FRR never saw either message.
#
# 10.20.0.0/24 is FRR's OWN prefix (frr.conf `network 10.20.0.0/24`), relayed
# here by Ze. Receiving it is proof rather than assumption: FRR's session is up,
# and Ze is forwarding between the two peers. An unmatched KEEPALIVE or EOR is
# silently accepted while this rule waits (internal/test/peer/checker.go
# ExpectedOrKeepalive), so Ze's handshake KEEPALIVE no longer releases anything.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) Announce 10.10.0.0/24, then End-of-RIB. FRR is up, so this is forwarded
# LIVE -- the scenario does not depend on a replay-on-peer-up path.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# BARRIER 2 -- give check.py a window to observe the advertisement before it is
# withdrawn. Ze sends a KEEPALIVE every 30s (a third of the 90s hold), so two of
# them are 30-60s during which FRR holds the stamped route. Withdrawing sooner
# would race the positive assertion, and a positive assertion that loses that
# race is indistinguishable from a missing stamp.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Withdraw 10.10.0.0/24: withdrawnLen=4, attrLen=0, no path attributes.
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A0A000000

# Hold the session open while check.py finishes. A peer whose script is exhausted
# exits, and its session dropping would make Ze withdraw the route for the wrong
# reason -- which is exactly how an absence assertion passes vacuously.
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
