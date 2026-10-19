# Raw BGP injector for the RFC 9774 Section 3 receive half.
#
# RFC 9774 Section 3: "MUST use the "treat-as-withdraw" error handling behavior
# per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or
# AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793]."
#
# ze holds this session to two octets (ze.conf, capability asn4 disable), so the
# AS numbers below are two octets wide whatever the injector offers.
#
# UPDATE 1, the set route, field by field:
#
#   FF x16          marker
#   0033            length 51
#   02              type UPDATE
#   0000            withdrawn routes length 0
#   0018            total path attribute length 24
#   40 01 01 00     ORIGIN, well-known transitive, IGP
#   40 02 0A        AS_PATH, well-known transitive, 10 octets
#      02 01 FDEC        AS_SEQUENCE of one: 65004, this injector
#      01 02 FDED FDEE   AS_SET of two: {65005, 65006}
#   40 03 04 AC1E0009   NEXT_HOP 172.30.0.9, this container
#   18 0A 61 4A     NLRI 10.97.74.0/24
#
# UPDATE 2, the control route: the same attributes with the AS_SET segment
# removed (AS_PATH 40 02 04 0201FDEC), total path attribute length 18, length
# 45, NLRI 10.97.75.0/24. It is sent AFTER the set route on the same session,
# so once a peer holds it, ze has already judged the set route.
#
# Then End-of-RIB. ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive, so ze does not withdraw the
# control route before FRR is asked about it; linger also discards what ze sends.
option=linger:value=true

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003302000000184001010040020A0201FDEC0102FDEDFDEE400304AC1E0009180A614A
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0200000012400101004002040201FDEC400304AC1E0009180A614B
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
