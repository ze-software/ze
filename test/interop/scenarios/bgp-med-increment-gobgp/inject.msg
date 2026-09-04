# Raw BGP injector for the `increment { med 50 }` scenario.
#
# The injector is an EXTERNAL peer of Ze (--asn 65005, against Ze's 65001), and
# both routes below carry the SAME attribute block except for one COMMUNITY:
# ORIGIN, AS_PATH [65005], NEXT_HOP 172.30.0.9 and MULTI_EXIT_DISC 100.
#
# 10.63.0.0/24 also carries COMMUNITY 65005:1, which is the condition the
# INC-MED policy matches, so its metric becomes 100 + 50 = 150. 10.63.1.0/24
# does not, and it is the in-run positive control: same session, same rail, same
# policy chain, metric kept at 100.
#
# THE METRIC IS 100 AND THE INCREMENT IS 50, so the three outcomes this lab has
# to tell apart are three different numbers. 150 is the increment computing from
# the route's metric. 50 is the increment computing from an absent-attribute
# base, which is what a subject that does not name `med` produces. 100 is no
# arithmetic at all. A metric of 0 with an increment of 50 could not separate
# the first two.
#
# The NEXT_HOP is written as hex (AC1E0009), so it does NOT follow the runner's
# rendered subnet prefix the way a dotted address in a text file does. This
# scenario therefore needs the default 172.30.0.0/24 lab subnet, as
# bgp-med-across-as-gobgp and bgp-local-pref-strip-gobgp do.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# NOTHING IS ANNOUNCED UNTIL THE DESTINATION IS UP. The forward rail skips a
# destination that is not established yet (forwardUpdateCore,
# reactor/reactor_api_forward.go), so a prefix announced early either never
# arrives or arrives by some path other than the one under test. Ze sends a
# KEEPALIVE every 5 s at the 15 s hold ze.conf sets, so four of them is a 20 s
# wait, and the checker waits on the GoBGP session besides.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) 10.63.0.0/24, tagged 65005:1, so the policy increments its metric.
#   ORIGIN:    40010100            IGP
#   AS_PATH:   40020602010000FDED  AS_SEQUENCE[65005]
#   NEXT_HOP:  400304AC1E0009      172.30.0.9
#   MED:       80040400000064      100
#   COMMUNITY: C00804FDED0001      65005:1
#   NLRI:      180A3F00            10.63.0.0/24
# attr len = 4+9+7+7+7 = 34 = 0x22; total = 19+2+2+34+4 = 61 = 0x3D.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003D02000000224001010040020602010000FDED400304AC1E000980040400000064C00804FDED0001180A3F00

# (2) 10.63.1.0/24, untagged, so the policy leaves it alone. This is the
# control: a metric of 150 at GoBGP means the arithmetic ran on the route the
# operator named, not that Ze rewrites every metric it relays.
#   NLRI:      180A3F01            10.63.1.0/24
# attr len = 27 = 0x1B; total = 19+2+2+27+4 = 54 = 0x36.
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDED400304AC1E000980040400000064180A3F01

# End-of-RIB.
action=send:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the internal/le/interoplab/bgp checker polls
# GoBGP. A peer whose script is exhausted exits, and its session dropping makes
# Ze withdraw the relayed routes before the check runs. Twenty-four KEEPALIVEs
# at 5 s give the check 120 s, and the runner tears the lab down as soon as the
# check returns.
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
expect=bgp:conn=1:seq=26:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=27:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=28:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=29:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=30:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=31:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
