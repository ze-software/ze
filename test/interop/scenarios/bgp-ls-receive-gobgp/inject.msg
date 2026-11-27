# Raw BGP injector for the BGP-LS receive-path fault management scenario.
#
# RFC 9552 Section 8.2.2, judged by real GoBGP v4.11.0 downstream of Ze:
#   - Frame 1 (valid BGP-LS Node NLRI, 10.95.52.1) and Frame 2 (valid BGP-LS
#     Link NLRI, 10.95.52.11->10.95.52.12) prove zero false positives on valid
#     Link-State NLRIs.
#   - Frame 3 pairs a skipable malformed Link-State Node NLRI (10.95.52.9,
#     TLV 256 declares length 24 with only 20 octets left inside a valid
#     Total NLRI Length of 33) with a well-formed Link-State Node NLRI survivor
#     (10.95.52.3). Ze strips only the malformed NLRI before relaying to GoBGP
#     while keeping both sessions established.
#   - Frame 4 sends an IPv4 unicast processing fence (10.10.0.0/24) last on the
#     same TCP session so GoBGP holding 10.10.0.0/24 proves Ze has processed
#     and relayed all preceding BGP-LS frames.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=300s

# BARRIER 1 -- 10.20.0.0/24, which the checker adds to GoBGP and Ze relays here,
# proves the GoBGP session is up before any BGP-LS UPDATE is sent.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) Valid BGP-LS Node NLRI control (bgplsReceivedNodeControl):
#     ORIGIN IGP (4B) + AS_PATH [65004] (9B) + MP_REACH_NLRI AFI 16388 / SAFI 71
#     next-hop 172.30.0.9 (45B: value len 42 = 9 fixed + 33 NLRI).
#     NLRI: type=1 (Node), total_len=29 (0x001D), proto=3 (OSPFv2), id=0,
#           TLV 256 (Local Node Desc, len=16) -> sub-TLV 512 (AS 65004) +
#           sub-TLV 515 (IGP Router-ID 10.95.52.1 = 0A5F3401).
#     attr_len=58 (0x003A), total_len=81 (0x0051).
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0051020000003A4001010040020602010000FDEC800E2A40044704AC1E0009000001001D03000000000000000001000010020000040000FDEC020300040A5F3401

# (2) Valid BGP-LS Link NLRI control (bgplsReceivedLinkControl):
#     ORIGIN IGP (4B) + AS_PATH [65004] (9B) + MP_REACH_NLRI AFI 16388 / SAFI 71
#     next-hop 172.30.0.9 (81B: value len 78 = 9 fixed + 69 NLRI).
#     NLRI: type=2 (Link), total_len=65 (0x0041), proto=3 (OSPFv2), id=0,
#           TLV 256 (Local Node Desc, len=16: AS 65004, IGP Router-ID 10.95.52.11),
#           TLV 257 (Remote Node Desc, len=16: AS 65004, IGP Router-ID 10.95.52.12),
#           TLV 259 (IPv4 Interface Addr 10.95.52.11),
#           TLV 260 (IPv4 Neighbor Addr 10.95.52.12).
#     attr_len=94 (0x005E), total_len=117 (0x0075).
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0075020000005E4001010040020602010000FDEC800E4E40044704AC1E0009000002004103000000000000000001000010020000040000FDEC020300040A5F340B01010010020000040000FDEC020300040A5F340C010300040A5F340B010400040A5F340C

# (3) Combined BGP-LS UPDATE: skipable malformed Node NLRI + well-formed survivor:
#     NLRI 1 (malformed, 37B): type=1 (Node), total_len=33 (0x0021), proto=3, id=0,
#       TLV 256 declares len=24 (0x0018) with only 20 octets remaining in the NLRI
#       body (AS 65004, IGP Router-ID 10.95.52.9, 4 trailing zero octets).
#     NLRI 2 (survivor, 33B): type=1 (Node), total_len=29 (0x001D), proto=3, id=0,
#       TLV 256 (len=16: AS 65004, IGP Router-ID 10.95.52.3).
#     MP_REACH value len=79 (0x4F = 9+37+33), attr=82B; attr_len=95 (0x005F),
#     total_len=118 (0x0076).
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0076020000005F4001010040020602010000FDEC800E4F40044704AC1E0009000001002103000000000000000001000018020000040000FDEC020300040A5F3409000000000001001D03000000000000000001000010020000040000FDEC020300040A5F3403

# (4) Trailing IPv4 unicast processing fence (10.10.0.0/24 = injectPrefixFirst):
#     ORIGIN IGP (4B) + AS_PATH [65004] (9B) + NEXT_HOP 172.30.0.9 (7B) = 20 (0x0014);
#     NLRI 10.10.0.0/24 = 180A0A00 (4B); total_len=47 (0x002F).
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00

# Hold the session open while the checker reads GoBGP and Ze. A peer whose script
# finishes closes TCP and would withdraw the relayed routes for the wrong reason.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
