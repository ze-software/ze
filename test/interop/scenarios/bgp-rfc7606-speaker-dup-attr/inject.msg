# Raw BGP injector for the speaker duplicate-attribute scenario.
#
# It announces 10.0.0.0/24 (NEXT_HOP 172.30.0.9, this container) as soon as its session comes
# up, then End-of-RIB. The harness starts this sidecar and Ze before the speaker, so the route
# is in Ze's adj-rib-in before the speaker connects; when the speaker establishes, Ze's
# delta-replay re-announces it through the wire-mode announce builder
# (reactor_api_batch.go buildWireModeUpdate).
#
# That builder holds the stored attribute block (NEXT_HOP included) and is handed a fresh
# authoritative next-hop, so it used to emit a SECOND NEXT_HOP. RFC 7606 Section 3(g) makes a
# duplicated attribute a treat-as-withdraw; the speaker's no-duplicate-attribute plugin flags
# exactly that, so the scenario is RED when the buildWireModeUpdate de-duplication is reverted.
# Ze's own validator is lenient there (discard-and-continue), which is why an INDEPENDENT peer
# is needed to catch it. The speaker MUST stay connected (speaker-args --stop-after-updates 0):
# the FIRST UPDATE it receives is the verbatim initial-sync copy; the dup arrives on the
# delta-replay re-announce that follows.
#
# ASN is pinned via inject-args (--asn 65004).
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# Announce 10.0.0.0/24 immediately, then End-of-RIB (adj-rib-in stores the route on receipt;
# the EoR marks the end of this peer's initial routes).
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Hold the session open while the speaker connects, is replayed the route, and check.py reads
# its verdict. A peer whose script is exhausted exits, and its session dropping would make Ze
# withdraw the relayed route before the speaker sees it. Three KEEPALIVE waits keep it up for
# roughly 90s (a third of the 90s hold each). Checker.check silently accepts an unmatched
# KEEPALIVE or EOR (internal/test/peer/checker.go).
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
