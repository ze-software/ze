# Raw BGP injector for the RFC 8277 Section 2.4 labelled withdrawal scenario.
#
# The injector announces 10.10.0.0/24 under label 100 and 10.11.0.0/24 under
# label 101 in ipv4/mpls-label (AFI 1, SAFI 4), waits while the checker sees FRR
# hold both routes Ze relayed, then withdraws 10.10.0.0/24 alone with an
# MP_UNREACH_NLRI whose label field is the Compatibility value 0x800000.
# RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
# MUST be ignored." Ze must read the withdrawal as the removal of 10.10.0.0/24,
# so that bgp-rs drops it from its route inventory. The checker then stops this
# container, and requires the route server's peer-down withdrawals to remove
# 10.11.0.0/24 from FRR without withdrawing 10.10.0.0/24 a second time.
#
# ASN is pinned via inject-args (--asn 65004). The OPEN offers Multiprotocol
# for ipv4/mpls-label, ipv4/mpls-vpn and ipv6/mpls-vpn.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=400s
option=open:value=add-capability:code=1:hex=00010004
option=open:value=add-capability:code=1:hex=00010080
option=open:value=add-capability:code=1:hex=00020080
# Hold the session open after the last directive. A peer whose script is
# exhausted exits, and its session dropping would make Ze withdraw the relayed
# routes before the checker has counted the withdrawal under test. The checker
# requires Ze to still hold this session Established after FRR lost
# 10.10.0.0/24, then stops this container itself.
option=linger:value=true

# BARRIER 1 -- give FRR time to come up before anything is announced. The injector
# waits for three of Ze's KEEPALIVEs: one every 30s (a third of the 90s hold),
# so three are 60-90s. The checker requires every initial route at FRR before
# any withdrawal; a delayed FRR session therefore fails rather than skipping
# the positive half of the proof.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (1) Announce 10.10.0.0/24 under label 100, then 10.11.0.0/24 under label 101,
# then End-of-RIB for ipv4/mpls-label.
#   FF x16          marker
#   0037            length 55
#   02              type UPDATE
#   0000            withdrawn routes length 0
#   0020            total path attribute length 32
#   40 01 01 00     ORIGIN IGP
#   40 02 06 02 01 0000FDEC   AS_PATH, one AS_SEQUENCE [65004], four-octet
#   80 0E 10        MP_REACH_NLRI, 16 octets
#      0001 04      AFI 1, SAFI 4
#      04 AC1E0009  next hop 172.30.0.9, this container
#      00           reserved
#      30 000641 0A0A00   Length 48, label 100 with S set, 10.10.0.0/24
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000204001010040020602010000FDEC800E1000010404AC1E000900300006410A0A00
# The same UPDATE for 10.11.0.0/24 under label 101 (30 000651 0A0B00). It is
# never withdrawn by an UPDATE: it leaves FRR only when the checker stops this
# injector, which proves the route server's peer-down withdrawal ran.
action=send:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003702000000204001010040020602010000FDEC800E1000010404AC1E000900300006510A0B00
action=send:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000104

# (1b) VPN routes distinguish the same IP prefix by RD, not by label.
# RFC 4364 Section 4.1: "A VPN-IPv4 address is a 12-byte quantity, beginning
# with an 8-byte Route Distinguisher (RD) and ending with a 4-byte IPv4 address."
# VPNv4 NLRI: [length 0x70][label:3][RD type0/AS65004:number:8][10.12.0:3].
# VPNv6 NLRI: [length 0x88][label:3][RD type0/AS65004:number:8][2001:db8:12:6].
# The lengths are hex: 0x70=112 (24+64+24), 0x88=136 (24+64+48).
# MP_REACH: [AFI:2][SAFI128:1][NH length:1][zero RD:8][IP:4 or16][reserved:1][NLRI].
# RDs10 and20 are targets, RD30 is the equal-prefix control. VPNv4 labels
# are200/201/202; VPNv6 labels are300/301/302. Controls are announced last.
action=send:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004702000000304001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070000C810000FDEC0000000A0A0C00
action=send:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004702000000304001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070000C910000FDEC000000140A0C00
action=send:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF004702000000304001010040020602010000FDEC800E200001800C0000000000000000AC1E00090070000CA10000FDEC0000001E0A0C00
action=send:conn=1:seq=10:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000180
action=send:conn=1:seq=11:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0056020000003F4001010040020602010000FDEC800E2F00028018000000000000000020010DB800090000000000000000000900880012C10000FDEC0000000A20010DB80012
action=send:conn=1:seq=12:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0056020000003F4001010040020602010000FDEC800E2F00028018000000000000000020010DB800090000000000000000000900880012D10000FDEC0000001420010DB80012
action=send:conn=1:seq=13:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0056020000003F4001010040020602010000FDEC800E2F00028018000000000000000020010DB800090000000000000000000900880012E10000FDEC0000001E20010DB80012
action=send:conn=1:seq=14:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000280

# BARRIER 2 -- give the checker a window to see FRR hold all eight routes
# before targets are withdrawn: two KEEPALIVEs, 30-60s.
expect=bgp:conn=1:seq=15:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=16:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Withdraw 10.10.0.0/24 with the Compatibility field in the label position.
#   0024            length 36
#   0000 000D       no withdrawn routes, 13 octets of attributes
#   80 0F 0A        MP_UNREACH_NLRI, 10 octets
#      0001 04      AFI 1, SAFI 4
#      30 800000 0A0A00   Length 48, Compatibility 0x800000, 10.10.0.0/24
# The announcement framing reads 0x800000 as a label entry with the S bit
# clear, walks on into the prefix, and finds no withdrawal at all.
action=send:conn=1:seq=17:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0024020000000D800F0A000104308000000A0A00

# (2b) RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility
# field MUST be ignored." Exactly three octets replace the announcement label,
# even when their low bit is clear; the following RD and prefix do not move.
# [MP_UNREACH AFI:2 SAFI128:1][length:1][Compatibility:3][RD:8][prefix].
# Both VPN families withdraw RD10 with800000 and RD20 with123456, neither
# equal to its announcement label. RD30 survives until the checker sends TERM.
action=send:conn=1:seq=18:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002C0200000015800F12000180708000000000FDEC0000000A0A0C00
action=send:conn=1:seq=19:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002C0200000015800F12000180701234560000FDEC000000140A0C00
action=send:conn=1:seq=20:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F0200000018800F15000280888000000000FDEC0000000A20010DB80012
action=send:conn=1:seq=21:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F0200000018800F15000280881234560000FDEC0000001420010DB80012
