# Raw BGP injector for draft-simpson-idr-flowspec-redirect-02 Section 3:
# "If the flow-spec route has an IP next-hop X and includes a redirect-to-IP
# extended community, then the BGP speaker SHOULD discard the redirect-to-ip
# extended community (and not propagate it further with the flow-spec route)
# if the last AS in the AS_PATH or AS4_PATH attribute of the longest prefix
# match for X does not match the AS of the EBGP peer."
#
# The injector is the EBGP peer, AS 65004 (inject-args). It sends the cover
# for X first, originated in AS 65009 (AS_PATH 65004 65009), then a FlowSpec
# rule for the same prefix carrying the 0x0800 redirect-to-IP community with
# next hop X = 192.0.2.77. The longest prefix match for X is 192.0.2.0/24,
# whose origin AS 65009 is not 65004, so Ze must relay the rule to FRR without
# the community. RFC 8955 Section 6 authorizes the rule itself: the cover's
# best path is from this same peer.
#
# No conforming daemon in the lab originates a draft-simpson 0x0800 rule (FRR
# receives FlowSpec only; GoBGP's redirect actions are the 0x8008 forms), so
# the frames are raw. Both are sent before FRR connects, so FRR receives them
# through Ze's replay on peer up.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=300s
option=open:value=add-capability:code=1:hex=00010001
option=open:value=add-capability:code=1:hex=00010085
option=linger:value=true

# (1) Cover 192.0.2.0/24: ORIGIN IGP | AS_SEQUENCE[65004 65009] |
# NEXT_HOP 172.30.0.9 | /24 NLRI.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003302000000184001010040020A02020000FDEC0000FDF1400304AC1E000918C00002
# (2) Rule destination 192.0.2.0/24: ORIGIN IGP | AS_SEQUENCE[65004] |
# EXT_COMMUNITIES[0x0800 redirect-to-IP] | MP_REACH AFI 1 SAFI 133,
# next hop length 4, 192.0.2.77, reserved 0, NLRI length 5, component 1, /24.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0041020000002A4001010040020602010000FDECC010080800000000000000800E0F00018504C000024D00050118C00002
# (3) End-of-RIB for IPv4 unicast, then for IPv4 FlowSpec (MP_UNREACH, no NLRI).
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000185
