# DDoS Characterization and the Responder Split

Stage 2 of the detector: resolve the victim prefix, classify the attack family,
and split what the local responder and the upstream responder act on. The
subsystem overview is in [the umbrella page](cp-survival-5-detect-0-umbrella.md).

## Why stage 2 exists

The detector once emitted `AttackDetected` with an empty `Target.DstPrefix`. The
`ddos-local` responder gates its nft drop on a valid prefix, so the whole chain
was wired and inert: no mitigation ever installed.

## Resolving the victim

<!-- source: internal/plugins/ddos/detect/characterize.go -- characterizeTarget, parseTopDestination, dominantDestination -->

The detector reaches trafficusage through `Plugin.DispatchCommand`, the
engine-routed text RPC, with `show traffic-usage name <iface>`. A DirectBridge
was planned and does not work: an out-of-process plugin reaches a sibling plugin
through the engine command router.

The victim is the highest-byte destination, taken as a host prefix (/32 for IPv4,
/128 for IPv6). trafficusage already ranks egress IPs by bytes, and `ddos-local`
drops a single victim, so a heuristic aggregate buys nothing.

trafficusage is IPv4-only. An IPv6 victim therefore resolves from the flow ring
through `dominantDestination`.

The fallback is the pre-stage-2 behavior: an empty target and
`FamilyGenericFlood`, used when the dispatch is nil, the source errors, or the
source returns no destination. The change is never worse than no stage 2.

## Characterization runs off the rate tick

<!-- source: internal/plugins/ddos/detect/detector.go -- onAttackStart, wg, ctx, stopped -->

The engine round trip runs on its own goroutine with a bounded timeout, so
neither the rate tick nor `d.mu` waits for it. `AttackDetected` is therefore
emitted ASYNCHRONOUSLY, while `Ongoing` and `Cleared` are emitted synchronously
from `onRate`. A subscriber that assumes an order must account for that split.

`detector.Stop()` cancels `d.ctx` and waits on `d.wg`, at shutdown and before a
reconfigure replaces the detector.

## The two locks, and why the second one exists

<!-- source: internal/plugins/ddos/detect/detector.go -- emitMu, d.mu, attackGen -->
<!-- source: internal/plugins/ddos/detect/characterize.go -- markDetectedEmitted -->

Lock order is `emitMu` then `d.mu`. Never take `emitMu` while holding `d.mu`.

The EventBus fan-out is synchronous and sequential on the emitting goroutine, and
it reaches responders that reconcile the kernel inside the handler
(`ddos-local onDetected` calls `firewall.ApplyAll` which calls netlink). Holding
`d.mu` across that parks the detector's whole state behind a netlink round trip,
because the rate tick, `Stop` and the baseline save all take `d.mu`.

`attackGen` is the generation guard. It bumps on every activate and every clear.
`emitDetected` and `emitCharacterized` compare it and emit under `emitMu`. The
tick stages its `Cleared` under `d.mu`, which advances the generation, and
publishes it under `emitMu` as well. A `Cleared` can therefore never interleave
between a stale check and its emit: either it wins the generation race and the
check drops the emit, or it waits for `emitMu` and lands after.

What `emitMu` does not give back is atomicity between a state write and the
generation it belongs to, because `attackGen` moves under `d.mu`. Any state an
emitter records after its fan-out re-checks the generation under `d.mu`.
`markDetectedEmitted` is the only such write.

The reason this matters: `ddos-local` parses and range-checks
`max-mitigation-duration` and never enforces it. The only drop-removal path is
`onCleared` calling `removeMitigation`. A stale `Detected` that lands after a
`Cleared` therefore installs a drop that nothing removes. There is no timer
backstop.

The first version of this guard checked the generation under `d.mu`, released it,
then emitted. That is a TOCTOU hole, and adversarial review found it, not a test.

## Classification

<!-- source: internal/plugins/ddos/detect/characterize.go -- classifyFlows, filterByWindow, sourceEntropy -->
<!-- source: internal/plugins/flowexport/recent.go -- bounded recent-flow ring, snapshot -->

The classifier reads a bounded recent-flow ring that flowexport keeps, over the
`show flow-recent [dst <prefix>]` RPC.

- **The RPC filters by destination prefix, not by interface.** `ConntrackFlow`
  carries no ingress interface, because conntrack is host-global. The victim
  destination is what the characterizer needs anyway.
- **SYN flood is detected from the conntrack TCP STATE, not from header flags.**
  `ConntrackFlow` has no TCP flags. The netlink library exposes
  `ProtoInfoTCP.State`, and a SYN flood is a dominance of the half-open states
  SYN_SENT, SYN_RECV and SYN_SENT2. The `TCPState` field is carried through
  reader, `FlowEntry`, delta and `ConntrackFlow`, and the classifier sets
  `VectorTuple.TCPFlags = SYN` for the responders.
- **A fragment flood classifies as generic.** Conntrack runs after IP defrag, so
  it never sees a fragment, and the sampling path retains nothing on-box. The
  `FamilyFragFlood` enum stays for a future sampling-based classifier.
- **`filterByWindow` keeps a flow with no timestamp** (`LastMs == 0`). Absence of
  a timestamp is not evidence of staleness, and a window filter using
  `time.Now()` would otherwise drop every synthetic flow.
- **`classifyFlows` counts each flow at least once.** A SYN flood's half-open
  conntrack entries often report zero cumulative packets, and a raw packet-share
  ranking hides them.

Families and their vectors: reflection (protocol plus source port), syn-flood
(protocol plus SYN), icmp-flood (protocol), udp-flood (protocol plus destination
port), generic (best effort).

## The responder split

<!-- source: internal/plugins/ddos/local/responder.go -- applyMitigation, onDetected, onCharacterized -->
<!-- source: internal/plugins/ddos/local/match.go -- buildDropTerm -->
<!-- source: internal/component/firewall/model.go -- MatchTCPFlags -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onDetected, onCharacterized -->

`ddos-local` narrows in place. `applyMitigation(target)` is shared by
`onDetected`, which installs the coarse drop, and `onCharacterized`, which
re-registers the same nft table narrowed and with TCP flags.

`ddos-flowspec` announces on `AttackCharacterized`, not on `AttackDetected`.
Announcing upstream blinds the box behind the filter, so the rule has to be right
the first time. `onDetected` only engages the RTBH `blackhole-fallback`, and only
on `critical` severity with the policy leaf set.

Severity is derived, not stored: `ddosevent.GradeSeverity(peak, threshold)` grades
1x, 2x and 5x to medium, high and critical from a ratio the detector already
holds.

## Traps

<!-- source: internal/plugins/ddos/detect/doctor.go -- doctor-ddos-detect-no-flow-source -->

- `sync.WaitGroup.Go` can race `Wait` at shutdown. trafficstat invokes its
  subscribers OUTSIDE its mutex, so a rate tick can reach `onAttackStart` after
  `UnsubscribeRates` returns. An Add during a `Wait` panics with "WaitGroup
  reused". A `stopped` flag under `d.mu` fences `onAttackStart`, and the
  unsubscribe happens before `Stop`, not after.
- The recent-flow ring is allocated only when conntrack export is enabled,
  because nothing else feeds it. `NewExporter` gates on `cfg.Conntrack.Enabled`,
  and a nil or zero-size ring is inert, so `RecentFlows` returns empty off Linux.
- A doctor check needs a non-empty `Dependencies` or registration fails at init
  with "invalid doctor check: missing dependencies". A config-reading check uses
  `Dependencies: []string{"config-loaded"}`.
