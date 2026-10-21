# Raw BGP injector for the withheld labelled withdrawal scenario (AC-27).
#
# The injector announces, each under ONE label, 10.10.0.0/24 and 10.11.0.0/24
# at RD 65004:40 in ipv4/mpls-vpn, and 10.20.0.0/24 and 10.21.0.0/24 in
# ipv4/mpls-label. It waits while the checker sees FRR hold all four routes ze
# relayed, then re-announces 10.10.0.0/24 and 10.20.0.0/24 under a three-label
# stack with the community NO_EXPORT. ze withholds those revisions from FRR, an
# external peer (RFC 1997), and sends FRR the withdrawal of the routes it
# holds, which RFC 8277 Section 2.4 frames with one Compatibility field.
#
# ASN is pinned via inject-args (--asn 65004). The OPEN offers Multiprotocol
# for ipv4/mpls-vpn and ipv4/mpls-label.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=400s
option=open:value=add-capability:code=1:hex=00010080
option=open:value=add-capability:code=1:hex=00010004
# Hold the session open after the last directive: a peer whose script is
# exhausted exits, and its session dropping would make ze withdraw the routes
# for a reason other than the one under test.
option=linger:value=true

# BARRIER 1 -- give FRR time to come up before anything is announced: three of
# ze's KEEPALIVEs, 60-90s.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) ipv4/mpls-vpn: 10.10.0.0/24 under label 200 and 10.11.0.0/24 under label
# 201, both at RD 65004:40, then End-of-RIB.
#   0047            length 71
#   0000 0030       no withdrawn routes, 48 octets of attributes
#   40 01 01 00     ORIGIN IGP
#   40 02 06 02 01 0000FDEC   AS_PATH [65004], four-octet
#   80 0E 20        MP_REACH_NLRI, 32 octets
#      0001 80      AFI 1, SAFI 128
#      0C 0000000000000000 AC1E0009   next hop, zero RD and 172.30.0.9
#      00           reserved
#      70 000C81 0000FDEC00000028 0A0A00
#                   Length 112, label 200 with S set, RD 65004:40, 10.10.0.0/24
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004702000000304001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070000C810000FDEC000000280A0A00
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004702000000304001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070000C910000FDEC000000280A0B00
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000180

# (1b) ipv4/mpls-label: 10.20.0.0/24 under label 300 and 10.21.0.0/24 under
# label 301, then End-of-RIB.
#   80 0E 10        MP_REACH_NLRI, 16 octets: AFI 1, SAFI 4, next hop 172.30.0.9
#      30 0012C1 0A1400   Length 48, label 300 with S set, 10.20.0.0/24
action=send:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000204001010040020602010000FDEC800E1000010404AC1E000900300012C10A1400
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000204001010040020602010000FDEC800E1000010404AC1E000900300012D10A1500
action=send:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000104

# BARRIER 2 -- give the checker a window to see FRR hold the four routes: two
# KEEPALIVEs, 30-60s.
expect=bgp:conn=1:seq=10:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=11:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Re-announce 10.10.0.0/24 at RD 65004:40 under labels 200, 210, 220 with
# NO_EXPORT.
#   0054            length 84
#   0000 003D       no withdrawn routes, 61 octets of attributes
#   C0 08 04 FFFFFF01   COMMUNITIES, NO_EXPORT
#   80 0E 26        MP_REACH_NLRI, 38 octets
#      A0 000C80 000D20 000DC1 0000FDEC00000028 0A0A00
#                   Length 160, labels 200 and 210 without S, 220 with S
action=send:conn=1:seq=12:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0054020000003D4001010040020602010000FDECC00804FFFFFF01800E260001800C0000000000000000AC1E000900A0000C80000D20000DC10000FDEC000000280A0A00

# (2b) Re-announce 10.20.0.0/24 under labels 300, 310, 320 with NO_EXPORT.
#   80 0E 16        MP_REACH_NLRI, 22 octets
#      60 0012C0 001360 001401 0A1400   Length 96, three labels, 10.20.0.0/24
action=send:conn=1:seq=13:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0044020000002D4001010040020602010000FDECC00804FFFFFF01800E1600010404AC1E000900600012C00013600014010A1400
