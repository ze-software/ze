# Raw BGP injector for the send-community suppression scenario.
#
# Every UPDATE below carries the SAME attribute block: ORIGIN, AS_PATH [65004],
# NEXT_HOP 172.30.0.9, COMMUNITY [65004:100 65004:200], EXTENDED_COMMUNITY
# [RT:65004:300] and LARGE_COMMUNITY [65004:1:2]. One source UPDATE holding all
# three community types is what makes the two destinations distinguishable: FRR
# (`send none`) must show none of them, BIRD (`send standard`) must show the
# standard one and neither of the other two.
#
# The ASN is pinned by inject-args (--asn 65004). Without it the peer adopts the
# ASN from Ze's OPEN and the eBGP relay becomes an iBGP one.
#
# The NEXT_HOP is written as hex (AC1E0009), so it does NOT follow the runner's
# rendered subnet prefix the way a dotted address in a text file does. This
# scenario therefore needs the default 172.30.0.0/24 lab subnet, exactly as
# scenario 47 does.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# (1) Announce 10.52.0.0/24 and 10.52.1.0/24 at once, then End-of-RIB. Both
# destinations are still coming up, so these two reach them through Ze's
# replay-on-peer-up path. The route server gates release of a client's routes on
# its EoR (see bgp-rs-replaying-gate.ci).
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF005802000000394001010040020602010000FDEC400304AC1E0009C00808FDEC0064FDEC00C8C010080002FDEC0000012CC0200C0000FDEC0000000100000002180A3400180A3401
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Wait for two of Ze's KEEPALIVEs (5 s apart at the 15 s hold ze.conf sets). By
# now FRR and BIRD are established, so the next UPDATE takes the live forward
# path rather than the replay path.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Announce 10.52.2.0/24 live, with the same three community types.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF005402000000394001010040020602010000FDEC400304AC1E0009C00808FDEC0064FDEC00C8C010080002FDEC0000012CC0200C0000FDEC0000000100000002180A3402

# Hold the session open while the internal/le/interoplab/bgp checker polls both daemons. A peer
# whose script is
# exhausted exits, and its session dropping makes Ze withdraw the relayed routes
# before the check runs. Twenty KEEPALIVEs at 5 s give the check 100 s, and the
# runner tears the lab down as soon as the check returns.
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=10:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=11:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=12:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=13:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=14:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=15:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=16:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=17:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=18:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=19:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=20:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=21:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=22:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=23:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=24:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=25:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
