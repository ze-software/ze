# Raw BGP injector for the RFC 1997 well-known community egress scenario.
#
# It gives Ze two routes to relay, identical except for one community value:
#
#   10.10.0.0/24  COMMUNITY = NO_EXPORT (0xFFFFFF01). RFC 1997 forbids it outside
#                 the confederation boundary, so FRR (external) must never see
#                 it and BIRD (internal) must.
#   10.11.0.0/24  COMMUNITY = 64985:100 (0xFDE90064), an ordinary value RFC 1997
#                 says nothing about. Both observers must see it. This is the
#                 control that makes FRR's absence assertion meaningful.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9 (this
# container), which shares both observers' subnet so RFC 4271 Section 6.3's
# semantic NEXT_HOP check passes on each of them.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER -- both observers are established and the forward rail carries traffic.
# 10.20.0.0/24 (180A1400) is FRR's OWN prefix and 10.21.0.0/24 (180A1500) is
# BIRD's, each relayed here by Ze. Receiving them is proof rather than
# assumption; matching on a handshake KEEPALIVE would not be, because an
# unmatched KEEPALIVE is silently accepted while a rule waits
# (internal/test/peer/checker.go ExpectedOrKeepalive) and a KEEPALIVE rule
# releases milliseconds after the OPEN.
#
# ONE UNORDERED GROUP (both seq=1). Which observer establishes first is
# scheduling, not behavior, and ordering these two would assert on it: the first
# run lost that race, with BIRD's prefix arriving before FRR's.
expect=bgp:conn=1:seq=1:contains=180A1400
expect=bgp:conn=1:seq=1:contains=180A1500

# (1) 10.10.0.0/24 carrying NO_EXPORT.
# ORIGIN=IGP, AS_PATH=[65004], NEXT_HOP=172.30.0.9, COMMUNITY=0xFFFFFF01.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDEC400304AC1E0009C00804FFFFFF01180A0A00
# (2) 10.11.0.0/24 carrying an ordinary community, then End-of-RIB.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDEC400304AC1E0009C00804FDE90064180A0B00
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while check.py finishes. A peer whose script is exhausted
# exits, and its session dropping would make Ze withdraw both routes -- which is
# the other way an absence assertion passes for the wrong reason.
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
