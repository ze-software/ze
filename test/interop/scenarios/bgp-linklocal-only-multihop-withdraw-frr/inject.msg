# Raw BGP injector for the Link-Local-only multihop withdrawal scenario.
#
# The injector mirrors Ze's OPEN, so it negotiates IPv6 unicast, four-octet ASNs
# and the Link-Local Next Hop capability (77) that Ze's `link-local-nexthop`
# leaf sends. ASN is pinned via inject-args (--asn 65004).
#
# Every UPDATE below is ORIGIN IGP, AS_PATH [65004], and one MP_REACH_NLRI for
# IPv6 unicast holding a 16-octet Next Hop field and one /48:
#
#   400101 00                     ORIGIN igp
#   400206 02 01 0000FDEC         AS_PATH AS_SEQUENCE [65004]
#   800E1C 0002 01 10 <nh16> 00 30 <prefix6>
#
# The fields carry no lab address, so the scenario runs on any selected network.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# BARRIER 1 -- give FRR's session time to come up. The first KEEPALIVE is Ze's
# handshake one and arrives at once; the second is Ze's 30s keepalive. The
# checker routes Ze to FRR's loopback and FRR retries its connect every 5s, so
# the FRR session is up well inside that window.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) GENERATION 1: 2001:db8:5701::/48, Next Hop 2001:db8:9::9 (Global). Ze
# relays it to FRR with the next hop unchanged.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0043020000002C4001010040020602010000FDEC800E1C0002011020010DB8000900000000000000000009003020010DB85701

# BARRIER 2 -- two of Ze's KEEPALIVEs, 30-60s during which the checker sees
# generation 1 at FRR before it is replaced.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) GENERATION 2: the same prefix, Next Hop fe80::9 ONLY. FRR is more than one
# hop from Ze, so Ze must withdraw the prefix from FRR. This is the subject.
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0043020000002C4001010040020602010000FDEC800E1C00020110FE800000000000000000000000000009003020010DB85701

# (3) CONTROL: 2001:db8:5702::/48, Next Hop 2001:db8:9::9, sent after the
# subject on the same session. FRR decoding it proves generation 2 went through
# the same rail before it, so the absence of a second announcement is evidence.
action=send:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0043020000002C4001010040020602010000FDEC800E1C0002011020010DB8000900000000000000000009003020010DB85702

# Hold the session open while the checker finishes. A peer whose script is
# exhausted exits, and its session dropping would make Ze withdraw the route for
# the wrong reason, which is how a withdrawal assertion passes vacuously.
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=10:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
