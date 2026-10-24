# Raw BGP injector: an AS_SET and an AS 0 inside an ATTR_SET (RFC 6368).
#
# ze holds this session to two octets (ze.conf, capability asn4 disable), so the
# outer AS_PATH below is two octets wide. RFC 6368 Section 5: "The AS_PATH and
# AGGREGATOR attributes contained within an ATTR_SET attribute MUST be encoded
# using 4-octet AS numbers, regardless of the capabilities advertised by the BGP
# speaker to which the ATTR_SET attribute is transmitted." The included AS_PATH
# below is therefore four octets wide.
#
# Every UPDATE carries the same outer attributes:
#
#   40 01 01 00          ORIGIN IGP
#   40 02 04 0201FDEC    AS_PATH, AS_SEQUENCE of one: 65004, this injector
#   40 03 04 AC1E0009    NEXT_HOP 172.30.0.9, this container
#
# UPDATE 1, the kept route 10.97.80.0/24 (length 75, attributes 48), adds:
#
#   C0 80 1B             ATTR_SET, optional transitive, 27 octets
#      0000FDF0              Origin AS 65008
#      40 01 01 00           included ORIGIN IGP
#      40 02 10              included AS_PATH, 16 octets
#         02 01 0000FDF0           AS_SEQUENCE of one: 65008
#         01 02 0000FDF1 0000FDF2  AS_SET of two: {65009, 65010}
#
# RFC 9774 Section 3 binds sets "in the AS_PATH or AS4_PATH", the UPDATE's own,
# and FRR and BIRD never look inside an ATTR_SET, so ze keeps the route.
#
# UPDATE 2, the withdrawn route 10.97.81.0/24 (length 69, attributes 42), adds:
#
#   C0 80 15             ATTR_SET, optional transitive, 21 octets
#      0000FDF0              Origin AS 65008
#      40 01 01 00           included ORIGIN IGP
#      40 02 0A              included AS_PATH, 10 octets
#         02 02 0000FDF0 00000000  AS_SEQUENCE of two: 65008, 0
#
# The included AS_PATH carries AS 0, so the included attribute is malformed
# itself and RFC 6368 Section 5 makes the ATTR_SET malformed: ze treats the
# route as withdrawn (validateAttrSetDepth,
# internal/component/bgp/message/rfc7606_optional_attrs.go).
#
# UPDATE 3, the control 10.97.82.0/24 with the outer attributes only, is sent
# last on the same session, so once a peer holds it ze has judged both routes.
#
# Then End-of-RIB. ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive, so ze does not withdraw the
# routes before the peers are asked about them; linger also discards what ze sends.
option=linger:value=true

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004B0200000030400101004002040201FDEC400304AC1E0009C0801B0000FDF04001010040021002010000FDF001020000FDF10000FDF2180A6150
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0045020000002A400101004002040201FDEC400304AC1E0009C080150000FDF04001010040020A02020000FDF000000000180A6151
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0200000012400101004002040201FDEC400304AC1E0009180A6152
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
