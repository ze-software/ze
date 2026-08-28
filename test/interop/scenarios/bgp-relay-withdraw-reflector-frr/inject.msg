# Raw BGP peer for the route-reflector relay-shape scenario. It SENDS nothing:
# it is the destination whose received bytes are the assertion.
#
# FRR is the route source here, not the receiver, and that is deliberate. FRR
# cannot observe this defect: measured on 2026-08-04, FRR 10.3.1 accepts a
# withdrawal carrying ORIGINATOR_ID and CLUSTER_LIST without a word, because its
# mandatory-attribute check only fires when NEXT_HOP or MP_REACH_NLRI is present.
# Scenario 53 uses FRR as the receiver, where a stamped NEXT_HOP does make it
# speak. Here the RFC 4456 attributes are the ones under test, so the receiving
# witness has to be one that reads bytes rather than one that applies policy.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=240s
# Pinned so it can never collide with FRR's own BGP Identifier -- RFC 4456
# Section 8 tells a receiver to IGNORE a route whose ORIGINATOR_ID is its own id.
option=open:value=router-id:id=172.30.0.9

# (1) The reflected ADVERTISEMENT of FRR's 10.20.0.0/24. Matched loosely on the
# prefix, because the attribute block is the SUBJECT of the internal/le/interoplab/bgp checker's
# positive
# assertion rather than of this barrier, and pinning it here would duplicate that
# assertion in a place whose failure the harness reports less clearly.
expect=bgp:conn=1:seq=1:contains=180A1400

# (2) The reflected WITHDRAWAL, byte-exact: withdrawn-len 0004, prefix 18 0A 14
# 00, attr-len 0000. This is the assertion, not a barrier. RFC 4271 Section 4.3
# says a message that withdraws only "will not include path attributes", and RFC
# 4456 Section 8 conditions ORIGINATOR_ID and CLUSTER_LIST on reflecting a ROUTE.
# One stamped attribute makes the body `0004180A1400000E8009...` and this rule
# stops matching, so the peer errors out, the session drops, and the internal/le/interoplab/bgp
# checker's own
# byte-exact wait then fails too. the internal/le/interoplab/bgp checker drives the withdrawal with
# `no network`
# on FRR.
expect=bgp:conn=1:seq=2:contains=0004180A14000000

# Stay established while the internal/le/interoplab/bgp checker finishes reading. Nothing but
# KEEPALIVEs
# follows the withdrawal, and Ze sends one every 30s (a third of the 90s hold). A
# peer whose script is exhausted exits, and its session dropping would end the
# reflection under the internal/le/interoplab/bgp checker's feet.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
