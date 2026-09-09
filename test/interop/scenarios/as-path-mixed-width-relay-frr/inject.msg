# Raw BGP injector for the RFC 6793 Section 4.2.3 mixed-width relay scenario.
#
# It is the OLD BGP speaker. ze holds this session to two octets from its own
# side (ze.conf, capability asn4 false), so the AS path below is the pair
# Section 4.2.3 exists to reconcile: a two-octet AS_PATH carrying AS_TRANS where
# a four-octet AS number belongs, and an AS4_PATH carrying the real one.
#
# The UPDATE, field by field:
#
#   FF x16          marker
#   003C            length 60
#   02              type UPDATE
#   0000            withdrawn routes length 0
#   0021            total path attribute length 33
#   40 01 01 00     ORIGIN, well-known transitive, IGP
#   40 02 06        AS_PATH, well-known transitive, 6 octets
#      02 02 FDEC 5BA0   one AS_SEQUENCE of two TWO-octet AS numbers,
#                        65004 and AS_TRANS (23456)
#   40 03 04 AC1E0009   NEXT_HOP 172.30.0.9, this container
#   C0 11 0A        AS4_PATH, optional TRANSITIVE, 10 octets
#      02 02 0000FDEC FA56EA7B   one AS_SEQUENCE of two FOUR-octet AS numbers,
#                                65004 and 4200000123, the non-mappable one
#   18 0A 00 00     NLRI 10.0.0.0/24
#
# The AS4_PATH holds as many AS numbers as the AS_PATH, so Section 4.2.3 takes
# no leading AS number from the AS_PATH and the reconstructed path is the
# AS4_PATH itself: [65004, 4200000123]. FRR must see that, with ze's own 65001
# prepended in front of it, and must never see 23456.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive. A peer whose script is
# exhausted exits, and its session dropping would make ze withdraw the relayed
# route before FRR is asked about it. linger also discards whatever arrives,
# which is what keeps FRR's own re-advertisement from being read as a mismatch
# (the race bgp-rfc7606-relay-shape-frr records).
option=linger:value=true

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C0200000021400101004002060202FDEC5BA0400304AC1E0009C0110A02020000FDECFA56EA7B180A0000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
