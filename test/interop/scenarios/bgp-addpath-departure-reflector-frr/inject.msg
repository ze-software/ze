# Raw BGP source for the reflector departure scenario. It announces two paths of
# 10.0.0.0/8 under ADD-PATH, then stays established until the checker stops its
# container, which is the departure under test.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=400s
option=linger:value=true
# Pinned so it can never collide with FRR's own BGP Identifier: RFC 4456
# Section 8 tells a receiver to ignore a route whose ORIGINATOR_ID is its own.
option=open:value=router-id:id=172.30.0.9

# (1) READINESS BARRIER, not an assertion. FRR originates 10.20.0.0/24 and Ze
# reflects it here, so FRR's session is up and a forward reaches it before the
# two paths are sent. Ze sends this peer no Path Identifier (it negotiated
# ADD-PATH receive only), so the NLRI is plain.
expect=bgp:conn=1:seq=1:contains=180A1400

# (2) ORIGIN IGP, empty AS_PATH, NEXT_HOP 172.30.0.9, LOCAL_PREF 100, NLRI
# [0000000B][8][10] [0000000C][8][10]: two paths of 10.0.0.0/8 under Path
# Identifiers 11 and 12. The NEXT_HOP octets are hex, which the lab does not
# renumber, so they name the injector on the base network.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0038020000001540010100400200400304AC1E0009400504000000640000000B080A0000000C080A
