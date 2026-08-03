# 1101 -- followup-test-infra

## Context

Consolidation spec from the 2026-07-06 deferral triage: four classes of deferred
tests blocked on "missing infrastructure" (property testing, privileged kernel-state
CI, two-peer wire-forwarding proofs, stress/chaos harnesses). Design-time
verification showed two of the four were wiring/authoring on EXISTING infra
(QEMU `option=needs-linux` root execution; conn_map two-peer `.ci`s), not new
frameworks. Delivered: 4 stdlib `testing/quick` property tests (fixed seeds),
QEMU traffic-suite enrollment + tc-qdisc kernel-state tests (022/023), the
MP_REACH next-hop-self two-peer `.ci`, an additive chaos iface fault family
(iface-link-flap, iface-addr-remove; netns-scoped), web concurrent-edit stress
and fleet 128-client perf (evidence tier). Commits cad47fcc0 + a35f6c355 +
e6c52a94f + closure. AC-5 (multi-peer LLGR egress `.ci`) deferred with explicit
user approval; AC-2/AC-3 execution legs env-blocked with recorded runbooks.

## Decisions

- **stdlib `testing/quick` over `pgregory.net/rapid`**: go-standards forbids new
  third-party deps without user approval; quick with custom `Generate` methods and
  fixed seeds proved sufficient for all four targets (A-2 confirmed). No dep ask.
- **L92 property set excludes transitivity**: `conflicts()` is symmetric by
  construction but provably NOT transitive (wildcard `0.0.0.0:80` ~ two distinct
  IPs that do not conflict with each other). The test documents the counterexample
  so nobody "fixes" it back in (R-2).
- **Privileged runs via QEMU suite enrollment, not a new runner**: root `.ci`
  execution already existed (`option=needs-linux` + `make ze-qemu-needs-linux-test`);
  the whole L200 item reduced to one `fsuite traffic` line + two gated tests.
- **Chaos iface faults are netns-scoped and excluded from the dashboard trigger
  UI by design** (R-5): a live-dashboard trigger would manipulate real host
  interfaces. CLI/scenario/integration-test only.
- **AC-5 deferred as a FEATURE, not authored as a fixture**: root cause found with
  a full producer chain (below); wiring the egress pipeline into a hot path is not
  closure-session work. Destination `spec-rib-arch-7-llgr-multipeer-ci.md` updated
  with the finding; its "just a test-coverage gap" premise was corrected (A-2 there
  marked broken).

## Consequences

- Property, stress (web/fleet), and chaos-iface tiers now exist and are
  discoverable (`docs/functional-tests.md`, reachable from `ai/INDEX.md`).
  Evidence-tier targets: `make ze-stress-web-test`, `make ze-stress-fleet-test`
  (both need `CGO_ENABLED=1` on the command line for `-race`, see Gotchas).
- The traffic suite runs as root under QEMU serially (`-p 1`: qdisc state on eth0
  is shared kernel state; parallel runs would race).
- `ze-chaos --help` v2 action list is now DERIVED from the engine action tables
  (`engine.V2ActionNames()`, guarded by `TestV2ActionNamesComplete`), eliminating
  the hardcoded-help drift class.

## Gotchas

- **THE LLGR RAIL GAP (the big one)**: RFC 9494 per-peer egress divergence never
  fires end-to-end in production. `LLGREgressFilter` (gr_egress.go) runs ONLY on
  the ForwardUpdate rail (forward_rs.go, reactor_api_forward.go); the only
  producer of `meta["stale"]` is the RIB readvertise path (rib_replay.go), which
  flows `updateRouteWithMeta` -> `MethodUpdateRoute` -> `DispatchNLRIGroups` ->
  `AnnounceNLRIBatch` (reactor_api_batch.go) -- a rail that DROPS `ctx.Meta` and
  calls NO egress filter (documented at sdk_engine.go). Unit tests pass by
  calling the filter directly; the single-peer `.ci` passes because reconnect
  replay needs no stamping. Moral: green unit tests + a green single-peer `.ci`
  can coexist with a feature that is entirely unwired on its production path.
  Owner: `spec-rib-arch-7-llgr-multipeer-ci.md`.
- **`Makefile:16 export CGO_ENABLED := 0` breaks every `-race` target** on
  go1.26 linux/amd64 ("go: -race requires cgo"). Workaround: command-line
  override (`make <target> CGO_ENABLED=1`) since command-line beats `:=`.
  Pre-existing (10708d7dc), affects ze-unit-test too.
- **`ze-validate` unwired-export check cannot see registry closures**: `CLIRun`
  is production-reachable via `MustRegisterRootHandler` in the same package, but
  the grep-based checker wants a cross-package non-test textual reference, so it
  false-positives whenever cli.go is in the changed set. Unexporting breaks the
  ze_chaos-tagged cmd/ze tests -- don't.
- A comment claiming a test file exists is a review-catchable lie: the prior
  session's llgr-readvertise.ci comment cited the two multi-peer `.ci`s as
  existing before they landed; AC-5 was then deferred, leaving a false claim in a
  shipped test. Never reference deliverables-to-be as delivered.
- The `.ci` two-peer LLGR reproduction ALSO needs a forwarding mechanism
  (`rs`/`redistribute`) in the config; `gr` + `bgp-rib` alone never re-forwards a
  source's route to other peers (the WIP timed out with zero deliveries).

## Files

None recorded.
