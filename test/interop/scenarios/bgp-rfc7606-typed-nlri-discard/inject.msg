# Raw BGP injector for the RFC 7606 Section 5.4 typed-NLRI discard scenario.
#
# It announces ONE l2vpn/evpn MP_REACH carrying TWO NLRIs:
#   route type 2  (MAC/IP Advertisement, RFC 7432 Section 7.2) -- ze implements it
#   route type 99 (unassigned)                                 -- ze does not
# Both sit in the same attribute, so the scenario proves ze splits the section and removes
# one route rather than dropping or keeping the whole attribute.
#
# RFC 7606 Section 5.4 requires the second one to be discarded. Ze is a route server here,
# so what the speaker receives is what ze relayed.
#
# ASN is pinned via inject-args (--asn 65004). The injector mirrors ze's OPEN, so the
# l2vpn/evpn capability comes from ze.conf's family block on this peer.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which ze cannot dial from another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# Wait for TWO KEEPALIVEs before announcing anything. The first arrives with the OPEN
# handshake; the second is ze's periodic one, a third of the 90-second hold later. By then
# the speaker has dialled ze and reached Established, so the route-server fan-out has a
# second client to reach.
#
# This ordering is load-bearing, not caution. Announcing on session-up put the UPDATE on the
# wire while ze knew one peer, and ze logged "forward matched no target ... peers-known=1":
# the speaker received nothing and the scenario proved nothing.
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# The EVPN announcement. Body: withdrawn-len 0, attr-len 0x0046, ORIGIN(IGP), AS_PATH(65004),
# MP_REACH_NLRI(AFI 25 / SAFI 70, next-hop 172.30.0.9, the two NLRIs above).
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF005D02000000464001010040020602010000FDEC800E3600194604AC1E00090002210001AC1E000900010000000000000000000000000000300011223344550000002163080001AC1E00090002
# End-of-RIB: this peer's initial routes are complete.
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the internal/le/interoplab/bgp checker reads the speaker's verdict. A
# peer whose script is
# exhausted exits, and its session dropping would make ze-build withdraw the relayed route before
# the speaker sees it. Two more KEEPALIVE waits keep it up for roughly 60s.
# Checker.check silently accepts an unmatched KEEPALIVE or EOR (internal/test/peer/checker.go).
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
