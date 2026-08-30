# Raw BGP injector for the RFC 6793 Section 4.2.2 AGGREGATOR downgrade scenario.
#
# It announces 10.0.0.0/24 with an AGGREGATOR carrying the non-mappable AS
# 4200000000, then End-of-RIB, then holds the session open while Ze forwards the
# route to BIRD and the checker reads BIRD's view of it.
#
# The UPDATE, field by field:
#
#   FF x16          marker
#   003A            length 58
#   02              type UPDATE
#   0000            withdrawn routes length 0
#   001F            total path attribute length 31
#   40 01 01 00     ORIGIN, well-known transitive, IGP
#   40 02 06        AS_PATH, well-known transitive, 6 octets
#      02 01 0000FDEC   one AS_SEQUENCE of one four-octet AS, 65004
#   40 03 04 AC1E0009   NEXT_HOP 172.30.0.9, this container
#   C0 07 08        AGGREGATOR, optional TRANSITIVE, 8 octets
#      FA56EA00     aggregating AS 4200000000, non-mappable
#      AC1E0009     aggregator address 172.30.0.9
#   18 0A 00 00     NLRI 10.0.0.0/24
#
# The eight-octet AGGREGATOR is correct HERE: this session negotiated four-octet
# AS support. What the scenario tests is what Ze emits on the OTHER session,
# which did not.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003A020000001F4001010040020602010000FDEC400304AC1E0009C00708FA56EA00AC1E0009180A0000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while Ze forwards to BIRD and the checker reads its
# route detail. A peer whose script is exhausted exits, and its session dropping
# would make Ze withdraw the route before BIRD is asked about it. Three KEEPALIVE
# waits keep it up for roughly 90s (a third of the 90s hold each).
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
