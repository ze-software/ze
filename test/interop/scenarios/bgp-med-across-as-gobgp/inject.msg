# Raw BGP injector for the RFC 4271 Section 5.1.4 relay scenario.
#
# The injector is an EXTERNAL peer of Ze (--asn 65004, against Ze's 65001), which
# is what Section 5.1.4 is about: "The MULTI_EXIT_DISC attribute received from a
# neighboring AS MUST NOT be propagated to other neighboring ASes."
#
# Every UPDATE below carries the SAME attribute block: ORIGIN, AS_PATH [65004],
# NEXT_HOP 172.30.0.9 and MULTI_EXIT_DISC 100. The AS_PATH holds the injector's
# own AS, so Ze sees a first-AS its session state agrees with.
#
# The metric is 100 rather than 0. GoBGP treats an absent MULTI_EXIT_DISC as
# nothing to print, so a check for the value 0 could not tell a leaked attribute
# from an absent one; 100 can only have come off the wire. Ze's OWN metric,
# announced by the fixture plan in internal/le/interoplab/bgp/helper.go, is 42, so the two are never
# confused.
#
# The NEXT_HOP is written as hex (AC1E0009), so it does NOT follow the runner's
# rendered subnet prefix the way a dotted address in a text file does. This
# scenario therefore needs the default 172.30.0.0/24 lab subnet, exactly as
# scenarios 47, 52 and 54 do.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# NOTHING IS ANNOUNCED UNTIL BOTH DESTINATIONS ARE UP, and that wait is the point
# of the scenario rather than a start-up convenience. The forward rail skips a
# destination that is not established yet (forwardUpdateCore,
# reactor/reactor_api_forward.go), so a prefix announced early either never
# arrives or arrives on the replay rail instead. Ze sends a KEEPALIVE every 5 s at
# the 15 s hold ze.conf sets, so four of them is a 20 s wait. FRR and GoBGP both
# reach Established inside that on the lab bridge, and the internal/le/interoplab/bgp checker waits
# on each
# session besides.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) Announce 10.60.0.0/24 and 10.60.1.0/24, then End-of-RIB.
#   ORIGIN:    40010100            IGP
#   AS_PATH:   40020602010000FDEC  AS_SEQUENCE[65004]
#   NEXT_HOP:  400304AC1E0009      172.30.0.9
#   MED:       80040400000064      100
#   NLRI:      180A3C00 180A3C01   10.60.0.0/24, 10.60.1.0/24
# attr len = 4+9+7+7 = 27 = 0x1B; total = 19+2+2+27+8 = 58 = 0x3A.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003A020000001B4001010040020602010000FDEC400304AC1E000980040400000064180A3C00180A3C01
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# (2) Announce 10.60.2.0/24 a KEEPALIVE later. An UPDATE that follows the
# End-of-RIB takes the same live forward path, and a scenario whose only announce
# sits in the initial burst would not say so.
# attr len = 27 = 0x1B; total = 19+2+2+27+4 = 54 = 0x36.
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDEC400304AC1E000980040400000064180A3C02

# Hold the session open while the internal/le/interoplab/bgp checker polls both daemons. A peer
# whose script is
# exhausted exits, and its session dropping makes Ze withdraw the relayed routes
# before the check runs. Twenty-four KEEPALIVEs at 5 s give the check 120 s, and
# the runner tears the lab down as soon as the check returns.
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
expect=bgp:conn=1:seq=32:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=33:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=34:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=35:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
