# Learned: one RTR cache at a time, two clocks to fail back, and a lab peer that drops its own refusal

`spec-rpki-cache-preference-and-source-marking` finished the RPKI multi-cache
model the owner fixed on 2026-10-27: Ze loads from the most preferred cache that
answers and never holds a union (the FRR/rtrlib model). `cacheGroup` in
`internal/component/bgp/plugins/rpki/rtr_session.go` now sends a Cache Reset's
Reset Query to the same cache (RFC 8210 Section 8.3), asks a failed
more-preferred cache again at its own Retry Interval without polling the holder
early (Sections 6 and 10), and passes over any cache still inside its Retry
Interval. Equal preferences keep the operator's order because
`parseRPKIConfig` reads the list with `configorder.Entries`. The interop
scenario `rtr-stayrtr-preference` proves it against two StayRTR caches, observed
at BIRD's export. The reference pages are `docs/guide/rpki.md` ("RTR Poll
Timing") and `docs/architecture/testing/interop.md` ("RTR cache preference,
failover and fail-back").

## What outlives the spec

1. **A stable sort keeps nothing when its input came from a map.** The decision
   "equal preferences keep config order" rested on `slices.SortStableFunc`, but
   the list was built by ranging over the delivered JSON object. A red walk that
   made the sort a no-op passed whenever the map happened to put the preferred
   cache first. An ordered YANG list is read with `configorder.Entries`, never
   ranged as a map.
2. **A lab peer's silent failure reads as Ze's defect.** StayRTR v0.6.4 queues
   the Error Report that refuses Ze's version 2 query and cancels the client in
   the same breath, and its `sendLoop` picks the cancel at random. Ze then reads
   a bare close and, as RFC 8210 Section 6, BIRD and rtrlib all do, waits the
   600-second default Retry Interval, far past a checker's wait. The harness's
   Ze log tail could not show it; a full log capture of every container across
   parallel runs did. The fix is a patch to the upstream release applied at
   build time, as `ai/META-DECISIONS.md` requires.
3. **Changing how a build installs a third party can hide it from the pin
   gate.** Moving StayRTR from `go install` to `go mod download` plus a patch
   took it out of `./le lab pin check` with no message. The scanner now reads
   both forms (`internal/le/lab/pin/scan.go`, `goInstall`).
4. **A functional runner that starts a helper in the wrong directory makes a
   test assert on a scenario that never happened.** `le mock rtr` ran in the
   repository root and never saw the observer's trigger files, so the preferred
   cache never went down and the failover `.ci` looked like a product defect
   (fixed in `internal/test/runner/runner.go`, `repositoryAnchoredBinary`).
