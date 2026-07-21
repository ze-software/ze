### `ze-test bgp plugin 224 forward-overflow-two-tier` -- deterministic dest-peer teardown on darwin, pre-existing

Observed 2026-07-21 on darwin (this host): `bin/ze-test bgp plugin 224`
(forward-overflow-two-tier) fails deterministically -- 4/4 isolated runs, RC=1,
each ~12-13s (not a timeout). The dest peer (127.0.0.2) exchanges OPEN then
closes: `failed: connection closed before completion`; the source's 50-route
burst never reaches adj-rib-in (`overflow-test` relay logs `OK: 0 routes
accepted`); the EOR dispatch then fails with
`bgp.rs ... error="no established peers to send to"` (one run showed the variant
`error="invalid FSM state"`); step 2 `expect peer-exchange` fails while step 1
`expect exit-code` passes. The test forces overflow with `ZE_FWD_CHAN_SIZE=2`
plus a 50-UPDATE burst under a 20s timeout, so the path is timing-fragile (cf.
`ec81f5005 perf: reduce forwarding variance`).

PRE-EXISTING, NOT caused by `spec-fixit-bgp-concurrency-races`: a HEAD baseline
built from `git archive HEAD` (SHA `dfb8c01ac`, clean extract, `ze-test` built
with the correct functional tags, `ze` self-compiled by the runner) fails 3/3
with the identical symptom. The 39-file reactor changeset did not regress this
test -- the failure exists on the committed tree without any of those changes.

Environmental scope UNVERIFIED and NO root cause asserted. Reproduced only on
this darwin host; the dest-peer teardown mechanism was not traced. Linux/CI
status not checked -- main's CI is presumed green, which would make this a
darwin-timing determinism in the tiny-channel overflow path rather than a
cross-platform bug, but that is unconfirmed. Triage: run `ze-test bgp plugin 224`
under QEMU/Linux to confirm darwin-specificity before attributing a cause; if it
reproduces on Linux it is a real forwarding/establishment bug and belongs in a
spec, not here. Do not assert the "no established peers" / "invalid FSM state"
message as the cause -- both are downstream of the earlier dest-peer close.
