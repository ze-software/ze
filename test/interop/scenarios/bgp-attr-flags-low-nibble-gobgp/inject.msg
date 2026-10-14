# Raw BGP injector for the RFC 4271 Section 4.3 low-nibble scenario.
#
# RFC 4271 Section 4.3: "The lower-order four bits of the Attribute Flags octet
# are unused. They MUST be zero when sent and MUST be ignored when received."
# No conforming daemon sends those bits, so this injector does: ORIGIN flagged
# 0x4F and an unrecognized optional transitive attribute (code 241) flagged 0xCF.
# Ze relays the route to GoBGP. The checker in internal/le/interoplab/bgp
# (checkers.go, bgp-attr-flags-low-nibble-gobgp) reads GoBGP's JSON for the
# relayed path and requires code 241 at flags 224 (0xE0: Optional, Transitive,
# and the Partial bit RFC 4271 Section 5 has Ze stamp on an unrecognized
# transitive attribute it passes on), never 239 (0xEF, the nibble left set).
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9 (this
# container).
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER -- GoBGP is established and Ze forwards between the two peers. The
# checker adds 10.20.0.0/24 to GoBGP's RIB; Ze relays it here. Receiving it is
# proof that the GoBGP session is up, so the route below is relayed live.
expect=bgp:conn=1:seq=1:contains=180A1400

# 10.10.0.0/24: ORIGIN 4F 01 01 00, AS_PATH 4F 02 06 02 01 0000FDEC, NEXT_HOP
# 4F 03 04 AC1E0009, attribute 241 CF F1 03 AABBCC. Every flags octet carries
# the low nibble, so a writer that clears it on one attribute only stays red.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0035020000001A4F0101004F020602010000FDEC4F0304AC1E0009CFF103AABBCC180A0A00
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the checker reads GoBGP. A peer whose script is
# exhausted exits, and Ze would then withdraw the route for the wrong reason.
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
