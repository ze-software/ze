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
# for ipv4/mpls-label alone, the family under test.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=400s
option=open:value=add-capability:code=1:hex=00010004
# Hold the session open after the last directive. A peer whose script is
# exhausted exits, and its session dropping would make Ze withdraw the relayed
# routes before the checker has counted the withdrawal under test. The checker
# requires Ze to still hold this session Established after FRR lost
# 10.10.0.0/24, then stops this container itself.
option=linger:value=true

# BARRIER 1 -- give FRR time to come up before anything is announced. FRR
# shares no family with the injector, since FRR refuses to activate one
# neighbor for both ipv4 unicast and ipv4 labeled-unicast, so no route of FRR's
# can tell the injector that FRR is up, and the wait is a count of Ze's
# KEEPALIVEs: one every 30s (a third of the 90s hold), so three are 60-90s.
# The announcements must reach FRR live. The route server's peer-up replay
# (adj-rib-in, ze.conf) cannot stand in for that, because adj-rib-in keys every
# ipv4/mpls-label route alike and replays only the last one (plan/journal/
# declared-format-contradicts-payload.md): FRR would then never hold
# 10.10.0.0/24, and the checker's first assertion goes red rather than passing.
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

# BARRIER 2 -- give the checker a window to see FRR hold both routes before
# one is withdrawn: two KEEPALIVEs, 30-60s.
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) Withdraw 10.10.0.0/24 with the Compatibility field in the label position.
#   0024            length 36
#   0000 000D       no withdrawn routes, 13 octets of attributes
#   80 0F 0A        MP_UNREACH_NLRI, 10 octets
#      0001 04      AFI 1, SAFI 4
#      30 800000 0A0A00   Length 48, Compatibility 0x800000, 10.10.0.0/24
# The announcement framing reads 0x800000 as a label entry with the S bit
# clear, walks on into the prefix, and finds no withdrawal at all.
action=send:conn=1:seq=9:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0024020000000D800F0A000104308000000A0A00
