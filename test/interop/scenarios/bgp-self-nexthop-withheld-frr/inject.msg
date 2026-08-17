# Raw BGP injector for the RFC 4271 Section 5.1.3 egress scenario.
#
# Two announcements, identical in every byte except the NEXT_HOP and the prefix:
#
#   (1) 10.11.0.0/24 with NEXT_HOP 172.30.0.3, which is FRR's OWN address.
#       Ze must withhold this from FRR.
#   (2) 10.12.0.0/24 with NEXT_HOP 172.30.0.9, this container. Ze must relay it.
#
# ORDER IS LOAD-BEARING. The withheld prefix is sent FIRST, so by the time FRR
# logs its decode of the second one, the first has already been through the same
# rail and the same fan-out. check.py's absence assertion is then about a
# decision Ze has made, not about a message still in flight.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from
# another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# BARRIER 1 -- FRR is established and the forward rail carries traffic. Ze dials
# this injector immediately, so everything below would otherwise run before FRR
# had a session at all, and FRR would see neither message.
#
# 10.20.0.0/24 is FRR's OWN prefix (frr.conf `network 10.20.0.0/24`), relayed
# here by Ze. Receiving it is proof rather than assumption. Matching on Ze's
# handshake KEEPALIVE would not be: an unmatched KEEPALIVE or EOR is silently
# accepted while this rule waits (internal/test/peer/checker.go
# ExpectedOrKeepalive), so a KEEPALIVE rule releases milliseconds after the OPEN
# and long before FRR is up.
expect=bgp:conn=1:seq=1:contains=180A1400

# (1) THE WITHHELD ROUTE. ORIGIN=IGP, AS_PATH=[65004], NEXT_HOP=172.30.0.3
# (AC1E0003, FRR's own address), NLRI=10.11.0.0/24.
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0003180A0B00

# (2) THE CONTROL ROUTE. The same bytes with NEXT_HOP=172.30.0.9 (AC1E0009, this
# container) and NLRI=10.12.0.0/24. Ze has no reason to withhold this one, and
# FRR receiving it is what turns the absence of (1) into evidence.
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0C00

# End-of-RIB, so Ze's initial-sync bookkeeping toward this peer completes.
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while check.py finishes. A peer whose script is exhausted
# exits, and its session dropping would make Ze withdraw both routes -- which is
# exactly how an absence assertion passes vacuously. Ze sends a KEEPALIVE every
# 30s (a third of the 90s hold), so four of them hold the session for 2-4 minutes.
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=7:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=8:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
