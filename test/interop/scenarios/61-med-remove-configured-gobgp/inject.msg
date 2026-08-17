# Raw BGP injector for RFC 4271 Section 5.1.4's CONFIGURED removal.
#
# The injector is an EXTERNAL peer of Ze (--asn 65005, against Ze's 65001), and
# both routes below carry the SAME attribute block except for one COMMUNITY:
# ORIGIN, AS_PATH [65005], NEXT_HOP 172.30.0.9 and MULTI_EXIT_DISC 100.
#
# 10.61.0.0/24 also carries COMMUNITY 65005:1, which is the condition the
# DROP-MED policy matches. 10.61.1.0/24 does not, and it is the in-run positive
# control: same session, same rail, same policy chain, metric kept.
#
# The metric is 100 rather than 0. GoBGP prints nothing for an absent
# MULTI_EXIT_DISC, so a check for the value 0 could not tell a removed attribute
# from an absent one.
#
# The NEXT_HOP is written as hex (AC1E0009), so it does NOT follow the runner's
# rendered subnet prefix the way a dotted address in a text file does. This
# scenario therefore needs the default 172.30.0.0/24 lab subnet, as scenarios 47,
# 52, 54 and 60 do.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=300s

# THE WAIT FAVORS LIVE FORWARDING, BUT DOES NOT CLAIM THAT THE DESTINATION IS
# UP. The runner starts GoBGP after Ze, so four KEEPALIVEs give the ordinary
# forwarding path time to become ready. Correctness does not depend on that
# elapsed time: Adj-RIB-In stores the post-import UPDATE and bgp-rs replays it
# after the GoBGP peer-up state arrives if live forwarding had no destination.
# Ze's 15 s hold sends the KEEPALIVEs below every 5 s.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) 10.61.0.0/24, tagged 65005:1, so the policy removes its metric.
#   ORIGIN:    40010100            IGP
#   AS_PATH:   40020602010000FDED  AS_SEQUENCE[65005]
#   NEXT_HOP:  400304AC1E0009      172.30.0.9
#   MED:       80040400000064      100
#   COMMUNITY: C00804FDED0001      65005:1
#   NLRI:      180A3D00            10.61.0.0/24
# attr len = 4+9+7+7+7 = 34 = 0x22; total = 19+2+2+34+4 = 61 = 0x3D.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003D02000000224001010040020602010000FDED400304AC1E000980040400000064C00804FDED0001180A3D00

# (2) 10.61.1.0/24, untagged, so the policy leaves it alone. This is the
# control: an absence at GoBGP means the mechanism ran, not that the relay is
# broken.
#   NLRI:      180A3D01            10.61.1.0/24
# attr len = 27 = 0x1B; total = 19+2+2+27+4 = 54 = 0x36.
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDED400304AC1E000980040400000064180A3D01

# End-of-RIB.
action=send:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while check.py polls GoBGP. A peer whose script is
# exhausted exits, and its session dropping makes Ze withdraw the relayed routes
# before the check runs. Twenty-four KEEPALIVEs at 5 s give the check 120 s, and
# the runner tears the lab down as soon as the check returns.
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
