# 1015 -- DDoS Characterization: fill target, classify family, split responders

> Scope: the full `spec-cp-survival-5-detect-5-characterization` spec. Phase 1
> ("Unblock", below) shipped first; Phases 2-5 (flowexport recent-flow tap,
> classifier + `AttackCharacterized` + `Severity`, responder split, ops) landed in a
> second pass and are captured under "Phases 2-5" further down. One learned number
> for the whole spec, per the Phase-1 note.

## Context

The two-stage DDoS detector (umbrella 1011) shipped with its responders wired but
inert: the detector emitted `AttackDetected` with an empty `Target.DstPrefix`, and
the `ddos-local` responder gates its nft drop on a valid prefix, so no mitigation
ever installed. The umbrella left Stage-2 characterization "designed but not yet
wired" (1011 Consequences). The NetHawk comparison confirmed Ze had already specced
that classifier and that Ze's adaptive p99 baseline is stronger than NetHawk's static
threshold. Phase 1's goal was the minimum to make the chain act: on the rate trigger,
resolve the victim prefix from on-box flow data and emit a populated `AttackDetected`,
with a graceful fallback to the prior generic-flood behavior when no source exists.

## Decisions

- Resolved the target via `Plugin.DispatchCommand` (engine-routed text RPC, `show traffic-usage name <iface>`) over the umbrella's planned DirectBridge, because an out-of-process plugin reaches a sibling plugin through the engine command router, not a direct bridge (`internal/plugins/ddos/detect/characterize.go,84`, wired at `register.go` via `p.DispatchCommand`)
- Ran characterization on its own goroutine with a 2s bounded timeout (`detector.go` `d.wg.Go`, `characterize.go,38`) over an inline call in `onRate`, so the rate tick and `d.mu` are never blocked by the engine round-trip
- Picked the highest-byte destination as a host prefix (/32 v4, /128 v6) over a heuristic aggregate, because `ddos-local` drops a single victim and trafficusage already ranks egress IPs by bytes (`characterize.go`)
- Kept the fallback identical to pre-Phase-1 behavior (empty target + `FamilyGenericFlood`) when dispatch is nil, the source errors, or returns no destination, so the change is never worse than before (`characterize.go,76-93`)
- Built the command with `textbuf.Buffer`, not string concat, per no-sprintf-alloc (`characterize.go`)

## Consequences

- The mitigation chain now acts: a volumetric trigger installs a targeted `ddos-local` drop. Responders still receive `FamilyGenericFlood`; proto/ports/flags/family and IPv6 targets arrive with the flowexport tap in a later phase (trafficusage is IPv4-only today, `characterize.go`)
- `AttackDetected` is now emitted ASYNCHRONOUSLY (from the goroutine), where `Ongoing`/`Cleared` are emitted synchronously from `onRate`. Any future subscriber ordering assumption must account for this split. Two guards exist: `Ongoing` is gated on `detectedEmitted` (`detector.go`) and stale emits are dropped by `attackGen` (below)
- `characterizeTarget` is the single funnel for "is a flow source present"; the Phase-5 `doctor-ddos-detect-no-flow-source` check (D-6) should hang off the same condition

## Gotchas

- `ddos-local` parses and range-checks `max-mitigation-duration` (`local/config.go,46,69`) but NEVER enforces it: the only drop-removal path is `onCleared`->`removeMitigation` (`local/responder.go`). So an async `Detected` arriving AFTER `Cleared` installs a drop with no matching `Cleared` to remove it -- a permanently stuck rule. Fixed with a generation guard: `attackGen` bumps on every activate (`detector.go`) and every clear (`detector.go`); `characterizeAndEmit` re-reads it under `d.mu` and skips the emit if it advanced (`characterize.go`)
- Same async split caused an ordering hazard: a slow flow query could let the first synchronous `Ongoing` overtake the attack's `Detected` (flowtriq subscribes both). Gated `Ongoing` behind `detectedEmitted atomic.Bool`, set only after `Detected` reaches the bus (`characterize.go`, read at `detector.go`, reset in `emitCleared` at `detector.go`)
- `statusDone` is duplicated as a local `"done"` constant rather than importing `plugin.StatusDone`, to keep the detector runtime path free of an `internal/component/plugin` import (`characterize.go`)
- ci-sleep ratchet: the test baseline had drifted stale (HEAD already 434 vs file 425) before this work; the Phase-1 tests pushed it to 436. A failing ratchet is not always your diff -- check the committed baseline against `make` output first
- Functional test (`test/plugin/ddos-detect-mitigate.ci`) is `needs-linux`/QEMU and was authored unverified-on-darwin by choice; it relies on the Linux CI to execute (D-4)

## Files

### Created
- `internal/plugins/ddos/detect/characterize.go` -- characterizeAndEmit, characterizeTarget, parseTopDestination, dispatchFunc
- `internal/plugins/ddos/detect/characterize_test.go` -- parse cases + AC-1/AC-10 + ordering/stale-emit guards
- `test/plugin/ddos-detect-mitigate.ci` -- QEMU functional: flood loopback, assert nft drop on victim

### Modified
- `internal/plugins/ddos/detect/detector.go` -- dispatch/wg/sourceAbsentOnce/detectedEmitted/attackGen fields; onAttackStart goroutine; Ongoing gate; emitCleared generation bump
- `internal/plugins/ddos/detect/register.go` -- inject `p.DispatchCommand` into newDetector
- `internal/plugins/ddos/detect/detector_test.go` -- newDetector(cfg,bus,nil) + wg.Wait()
- `test/.ci-sleep-baseline` -- 425 -> 436
- `docs/guide/ddos-mitigation.md` -- corrected match-vector overclaim; trafficusage track-ip prerequisite; source anchor
- `spec-cp-survival-5-detect-5-characterization` -- the spec itself, in-progress 1/5 when written; deviations D-1..D-8; review gate runs

---

## Phases 2-5 -- recent-flow tap, classifier, responder split, ops

### Context

Phase 1 left responders acting on a coarse `FamilyGenericFlood` with only a
`DstPrefix`. Phases 2-5 fill the vector: a bounded recent-flow ring on flowexport,
a heuristic classifier producing `AttackCharacterized`, a responder split (local
narrows in place + TCP flags; flowspec announces only on the characterized signal),
and the ops surface (tuning YANG leaves, doctor check, metrics, lifecycle await).

### Decisions

- **The recent-flow RPC filters by destination prefix, not interface.** `ConntrackFlow`
  carries no ingress interface (conntrack is host-global), so the spec's literal
  `flow-recent name <iface>` is not derivable. `show flow-recent [dst <prefix>]`
  filters by victim destination instead -- exactly what the characterizer needs
  (`internal/plugins/flowexport/cmd_show.go:handleShowFlowRecent`, `recent.go:snapshot`).
- **SYN-flood is detected from conntrack TCP *state*, not header flags.** `ConntrackFlow`
  has no TCP flags, but the netlink lib exposes `ProtoInfoTCP.State`; a SYN flood is a
  dominance of half-open states (SYN_SENT/SYN_RECV/SYN_SENT2). Captured a new
  `TCPState` field through reader->FlowEntry->delta->ConntrackFlow and classify on it,
  setting `VectorTuple.TCPFlags = SYN` for responders (`conntrack/reader_linux.go`,
  `detect/characterize.go:classifyFlows`).
- **Fragment-flood classifies as generic (user-approved).** Conntrack runs after IP
  defrag, so it never sees fragments, and the sampling path retains nothing on-box. No
  AC requires fragment classification (AC-6 routes non-{reflection,syn,icmp} to generic);
  the `FamilyFragFlood` enum stays for a future sampling-based classifier.
- **flowspec announces on `AttackCharacterized`, not `AttackDetected` (behavior change).**
  Announcing upstream blinds the box behind the filter, so the rule must be precise
  first ("get it right once"). `onDetected` now only engages the RTBH `blackhole-fallback`
  on a `critical` severity when the policy leaf is set (AC-8/AC-14). Existing flowspec
  responder tests moved their trigger from `onDetected` to `onCharacterized`.
- **local narrows in place** via a shared `applyMitigation(target)` used by both
  `onDetected` (coarse) and `onCharacterized` (narrowed + TCP flags), re-registering the
  same nft table (`local/responder.go`, `local/match.go:buildDropTerm` gained a
  `MatchTCPFlags` term for AC-9).
- **Severity is derived, not stored.** `ddosevent.GradeSeverity(peak, threshold)` grades
  1x/2x/5x -> medium/high/critical from the ratio the detector already holds; a value
  field on both events, no wire change.

### Consequences

- Responders now receive a real family + vector: reflection (proto+src-port), syn-flood
  (proto+SYN), icmp-flood (proto), udp-flood (proto+dst-port), generic (best-effort). IPv6
  victims resolve from the flow ring (trafficusage is IPv4-only): when trafficusage gives
  no victim, `dominantDestination` derives it from the flows.
- The characterization goroutine is now bounded to the detector lifetime: `detector.Stop()`
  cancels `d.ctx` and waits `d.wg`, called at shutdown and before a reconfigure replaces
  the detector (closes Phase-1 D-8).
- New observability: doctor `doctor-ddos-detect-no-flow-source` (warns when characterization
  is on with no flow source), metrics `ze_ddos_detect_characterize_total{family}`,
  `ze_ddos_detect_characterize_fallback_total`, `ze_flowexport_recent_ring_drops`.

### Gotchas

- **A generation guard is only sound if the check and the emit are atomic.** Phase 1's
  `attackStale(gen)` locked `d.mu`, checked, then *released* it before `Emit` -- a TOCTOU
  hole. Because `emitCleared` runs its `Cleared` emit under `d.mu` on the rate tick, a clear
  could slip between the check and a late `AttackCharacterized`/`AttackDetected` emit,
  installing a ddos-local drop with no matching `Cleared` (permanent -- ddos-local has no
  timer backstop). Fix: `emitDetected`/`emitCharacterized` hold `d.mu` across the check AND
  the synchronous emit. Safe because the responders take their own `r.mu`, never `d.mu`, and
  the slow source queries already ran off the lock. Found by adversarial review, not tests.
- **`sync.WaitGroup.Go` can race `Wait` at shutdown.** trafficstat invokes subscribers
  *outside* its mutex, so a rate tick can still reach `onAttackStart` -> `wg.Go` after
  `UnsubscribeRates` returns. If `Stop()` is in `wg.Wait()`, that Add-during-Wait panics
  ("WaitGroup reused"). Fix: a `stopped` flag set under `d.mu` in `Stop()` fences
  `onAttackStart` (also under `d.mu`); and unsubscribe before `Stop`, not after.
- **The recent-flow ring is only allocated when conntrack export is enabled** (nothing else
  feeds it). `NewExporter` gates on `cfg.Conntrack.Enabled`; a nil/zero-size ring is inert,
  so `RecentFlows` returns empty off-Linux. The functional test (`ddos-flow-recent.ci`) is
  therefore `needs-linux`: flowexport hard-depends on the `interface` plugin whose backend is
  Linux-only, so the daemon cannot even start off-Linux.
- **filterByWindow keeps timestamp-less flows** (`LastMs == 0`): unit-test flows carry no
  timestamp, and absence of a timestamp is not evidence of staleness. Without this rule a
  time-window filter using `time.Now()` would drop every synthetic flow.
- **classifyFlows counts each flow at least once** (`p := f.Packets; if p == 0 { p = 1 }`):
  a SYN flood's half-open conntrack entries often report zero cumulative packets, so raw
  packet-share would hide them.
- **Doctor checks require a non-empty `Dependencies`** or registration fails at init with
  "invalid doctor check: missing dependencies". A config-reading check uses
  `Dependencies: []string{"config-loaded"}`.
- **A cross-reference hook** blocks editing a file that another file `// Related:`-points to
  unless the back-reference exists; `characterize.go` had to add `// Related: doctor.go` /
  `// Related: metrics.go` before an unrelated edit would apply.

### Files (Phases 2-5)

Created: `flowexport/recent.go`(+test), `detect/metrics.go`, `detect/doctor.go`(+test),
`detect/config_test.go`, `flowspec/config_test.go`, `test/plugin/ddos-flow-recent.ci`.
Modified: `ddosevent/event.go` (AttackCharacterized, Severity, GradeSeverity);
`detect/characterize.go` (classifier, queries, window, entropy, ctx); `detect/config.go`
+ yang (5 tuning leaves); `detect/register.go` (metrics + doctor + Stop wiring);
flowexport `exporter.go`/`config.go`/`conntrack_worker.go`/`metrics.go`/`cmd_show.go` +
`conntrack/{flow,reader_linux}.go` (TCPState + ring + flow-recent RPC);
`flowexport-cmd` + flowexport conf/cmd yang; local + flowspec responder/register/match/config
+ yang; `diagnostic/codes.go`; docs (`features.md`, `ddos-mitigation.md`, `ai/INDEX.md`).
