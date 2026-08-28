# Raw BGP injector for the RFC 4271 Section 5.1.5 relay scenario.
#
# The injector is an INTERNAL peer of Ze (--asn 65001, the same AS Ze runs).
# Without the flag the peer adopts the ASN from Ze's OPEN, which is the same
# value here, but pinning it keeps the session internal by configuration rather
# than by coincidence.
#
# Every UPDATE below carries the SAME attribute block: ORIGIN, AS_PATH [65004],
# NEXT_HOP 172.30.0.9 and LOCAL_PREF 200. The AS_PATH is non-empty on purpose:
# an internal peer relays routes it learned elsewhere, and both destinations are
# EXTERNAL peers of Ze, so an empty path would be an odd thing to hand them.
#
# LOCAL_PREF is 200 rather than 100. GoBGP's own default local preference is
# 100, so a check for the value 100 could not tell a leaked attribute from a
# default; 200 can only have come off the wire.
#
# The NEXT_HOP is written as hex (AC1E0009), so it does NOT follow the runner's
# rendered subnet prefix the way a dotted address in a text file does. This
# scenario therefore needs the default 172.30.0.0/24 lab subnet, exactly as
# scenarios 47 and 52 do.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# NOTHING IS ANNOUNCED UNTIL BOTH DESTINATIONS ARE UP, and that wait is the
# point of the scenario rather than a start-up convenience. The forward rail
# skips a destination that is not established yet (forwardUpdateCore,
# reactor/reactor_api_forward.go), so a prefix announced early either never
# arrives or arrives by some other path. An earlier draft of this file did
# announce early and stayed GREEN with the forward rail's strip removed --
# measured on 2026-08-04 -- so whatever delivered it there was not the rail
# under test. The wait is what makes every prefix travel that rail.
#
# Ze sends a KEEPALIVE every 5 s at the 15 s hold ze.conf sets, so four of them
# is a 20 s wait. FRR and GoBGP both reach Established inside that on the lab
# bridge, and the internal/le/interoplab/bgp checker waits on each session besides.
#
# EACH PREFIX IS ANNOUNCED ONCE. A re-announce used to sit at the end of this
# file to hide a Ze defect: the announce reached nobody and logged `BUG:
# ForwardUpdatesDirect: msgID missing from cache`
# (reactor/reactor_api_forward_batch.go). The cause was in the plugin
# Coordinator: it dropped every cache-consumer declaration made before the BGP
# reactor attached, which is every one of them, so bgp-rs ran as a FIFO
# consumer and the End-of-RIB behind the announce cumulatively evicted the
# announce's cache entry before bgp-rs flushed it. Fixed in
# plugin/coordinator.go. A test shaped around a bug is not evidence, so the
# repeat is gone.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) Announce 10.54.0.0/24 and 10.54.1.0/24, then End-of-RIB.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003A020000001B4001010040020602010000FDEC400304AC1E0009400504000000C8180A3600180A3601
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# (2) Announce 10.54.2.0/24 a KEEPALIVE later. An UPDATE that follows the
# End-of-RIB takes the same live forward path, and a scenario whose only
# announce sits in the initial burst would not say so.
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDEC400304AC1E0009400504000000C8180A3602

# Hold the session open while the internal/le/interoplab/bgp checker polls both daemons. A peer
# whose script
# is exhausted exits, and its session dropping makes Ze withdraw the relayed
# routes before the check runs. Twenty KEEPALIVEs at 5 s give the check 100 s,
# and the runner tears the lab down as soon as the check returns.
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
expect=bgp:conn=1:seq=26:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=27:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=28:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=29:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=30:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=31:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
