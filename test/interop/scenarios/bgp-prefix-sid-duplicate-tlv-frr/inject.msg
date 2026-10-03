# Raw BGP injector for the RFC 8669 Section 6 repeated-TLV scenario.
#
# No conforming speaker sends a Prefix-SID that repeats a single-occurrence
# TLV, so the raw injector writes the bytes. The UPDATE carries one SRv6 L3VPN
# route whose Prefix-SID holds the SRv6 L3 Service TLV (type 5) TWICE, with
# different SIDs.
#
# The UPDATE, field by field:
#
#   FF x16          marker
#   008D            length 141
#   02              type UPDATE
#   0000            withdrawn routes length 0
#   0076            total path attribute length 118
#   40 01 01 00     ORIGIN, IGP
#   40 02 06        AS_PATH, 6 octets: one AS_SEQUENCE of one four-octet AS,
#      02 01 0000FDEC   65004 (ASN4 is negotiated with ze)
#   80 0E 20        MP_REACH_NLRI, 32 octets
#      0001 80          AFI IPv4, SAFI 128 (MPLS-labeled VPN)
#      0C 0000000000000000 AC1E0009   next hop: RD 0, 172.30.0.9 (this container)
#      00               reserved
#      70 000101 0000FDE900000001 0A6200
#                       112 bits: label 16 (BoS), RD 65001:1, 10.98.0.0/24
#   C0 10 08        EXTENDED_COMMUNITIES, route target 100:1
#      0002 0064 00000001
#   C0 28 38        BGP Prefix-SID, 56 octets: two type-5 TLVs
#      05 0019 00             SRv6 L3 Service TLV, length 25, reserved
#         01 0015 00          SRv6 SID Information Sub-TLV, length 21, reserved1
#         FC000000000100010000000000000000   SID fc00:0:1:1::  (the FIRST)
#         00 0013 00          flags, endpoint behavior End.DT4, reserved2
#      05 0019 00
#         01 0015 00
#         FC000000000200020000000000000000   SID fc00:0:2:2::  (the REPEAT)
#         00 0013 00
#
# RFC 8669 Section 6 obliges ze to keep the first type-5 TLV and discard the
# second, so FRR must see fc00:0:1:1:: and never fc00:0:2:2::. Relayed as
# received, FRR's own parser rejects the attribute ("Prefix SID SRv6 L3VPN
# field repeated") and the route never reaches its table.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s
# Hold the session open after the last directive, so ze does not withdraw the
# relayed route before FRR is asked about it, and discard whatever arrives.
option=linger:value=true

action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF008D02000000764001010040020602010000FDEC800E200001800C0000000000000000AC1E000900700001010000FDE9000000010A6200C010080002006400000001C028380500190001001500FC000000000100010000000000000000000013000500190001001500FC00000000020002000000000000000000001300
