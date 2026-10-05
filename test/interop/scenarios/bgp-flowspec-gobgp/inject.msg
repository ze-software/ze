# One TCP session, one FlowSpec announcement. GoBGP marker routes release each
# stage only after the checker has observed the preceding foreign RIB state.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=300s
option=open:value=add-capability:code=1:hex=00010001
option=open:value=add-capability:code=1:hex=00010085
option=linger:value=true

# Stage 1: same-originator cover, 10.99.77.0/24, AS65004, NEXT_HOP .9.
# Frame: 19-byte header | withdrawn length 0 | attributes length 20 |
# ORIGIN IGP | AS_SEQUENCE[65004] | NEXT_HOP | legacy /24 NLRI.
expect=bgp:conn=1:seq=1:announced=20C6120001
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A634D

# Stage 2: RFC 8955 Section 4: "The Network Address of the Next-Hop field
# MUST be ignored." Inject a 16-octet IPv6 NH although no IPv6 family or
# extended-next-hop capability was negotiated. Egress uses next-hop unchanged.
# Attributes: ORIGIN[4] AS_PATH[9] EXT_COMMUNITIES[11] MP_REACH[30].
# MP_REACH: AFI1 SAFI133 NH-length16 | 2001:db8::dead | reserved0 |
# NLRI length5, component1, prefix24, 10.99.77.0.
# RFC 8955 Section 7.1: traffic-rate-bytes 9600 = IEEE754 0x46160000.
expect=bgp:conn=1:seq=3:announced=20C6120002
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004D02000000364001010040020602010000FDECC010088006000046160000800E1B0001851020010DB800000000000000000000DEAD000501180A634D

# Stage 3 removes only the cover. The retained rule becomes unauthorized.
expect=bgp:conn=1:seq=5:announced=20C6120003
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A634D0000

# Stage 4 restores only the cover. No FlowSpec UPDATE is sent again: the
# registered RIB/RS must reconstruct its retained rule and traffic-rate action.
expect=bgp:conn=1:seq=7:announced=20C6120004
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A634D
