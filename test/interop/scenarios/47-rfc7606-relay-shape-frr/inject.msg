# Raw BGP injector for the RFC 7606 Section 5.1 relay scenario.
#
# It drives Ze (which is CONFIGURED as a route server here -- Ze is a general BGP speaker,
# route-server is one mode) through two distinct relay paths, so the scenario is not
# vacuous:
#
#   1. REPLAY path (relay-liveness). The injector announces 10.0.0.0/24 as soon as its
#      session comes up, before FRR connects, so FRR receives it via Ze's replay-on-peer-up.
#      This replay is forwarded verbatim (buildFwdBody), so FRR keeps 10.0.0.0/24 even with
#      the NEXT_HOP dedup reverted (empirically verified) -- so check.py's 10.0.0.0/24
#      assertion is a relay-liveness check, NOT the discriminator.
#
#   2. LIVE relay + split path (the discriminator). After FRR is up, the injector sends ONE
#      UPDATE mixing Withdrawn Routes with NLRI. Ze forwards it live and splits it per RFC
#      7606 Section 5.1; the announced half (203.0.113.0/24) is re-encoded through the
#      wire-mode builder (reactor_api_batch.go buildWireModeUpdate), which used to add a
#      second NEXT_HOP -- Section 3(g) makes a duplicated attribute a treat-as-withdraw, so
#      FRR would discard it. check.py asserts FRR has 203.0.113.0/24: THIS goes RED when the
#      de-duplication is reverted (verified). It does NOT prove FRR rejects the UNSPLIT form
#      (Section 5.1's third bullet obliges receivers to accept any combination); split-vs-
#      unsplit discrimination lives in the unit tests and the .ci. The independent-speaker
#      scenario 48 exercises the NEXT_HOP dedup directly via the adj-rib-in delta-replay.
#
# ASN is pinned via inject-args (--asn 65004); NEXT_HOP is 172.30.0.9 (this container). No
# LOCAL_PREF: from an eBGP peer that is an attribute-discard (RFC 7606 Section 7.5) which Ze
# marks on the wire as attribute type 253, a separate question this scenario does not probe.
option=tcp_connections:value=1
# Bind all interfaces: the default is loopback only, which Ze cannot dial from another container.
option=bind:value=0.0.0.0
option=timeout:value=180s

# (1) Announce 10.0.0.0/24 immediately, then End-of-RIB. The route server gates release of a
# client's routes on its EoR (see bgp-rs-replaying-gate.ci).
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0000
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000

# Wait for Ze's first KEEPALIVE (~30s, a third of the 90s hold): by now FRR is established
# and has been replayed 10.0.0.0/24, so the next UPDATE is forwarded LIVE, not replayed.
expect=bgp:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304

# (2) A mixed UPDATE: withdraw 198.51.100.0/24 (never announced -- a no-op at FRR, present
# only to make the shape mixed) and announce 203.0.113.0/24. Ze must split this before
# relaying it.
action=send:conn=1:seq=4:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003302000418C6336400144001010040020602010000FDEC400304AC1E000918CB0071

# Hold the session open while check.py polls FRR. A peer whose script is exhausted exits, and
# its session dropping would make Ze withdraw the relayed routes before FRR is asked. Two more
# KEEPALIVE waits keep it up for roughly a minute. Ze's EoR needs no rule: Checker.check
# silently accepts an unmatched KEEPALIVE or EOR (internal/test/peer/checker.go:414-418).
expect=bgp:conn=1:seq=5:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=6:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
