# 1015 -- DDoS Characterization Phase 1: fill attack target from trafficusage

> Scope: Phase 1 ("Unblock") of `spec-cp-survival-5-detect-5-characterization`, which
> remains OPEN at `in-progress 1/5`. This entry was written immediately after the
> Phase-1 commit while context was fresh. At spec closure, EXPAND this same file
> (do not mint a new number) with Phases 2-5 (flowexport recent-flow tap, classifier
> + `AttackCharacterized`, `Severity`, doctor checks).

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

- Resolved the target via `Plugin.DispatchCommand` (engine-routed text RPC, `show traffic-usage name <iface>`) over the umbrella's planned DirectBridge, because an out-of-process plugin reaches a sibling plugin through the engine command router, not a direct bridge (`internal/plugins/ddos/detect/characterize.go:31,84`, wired at `register.go` via `p.DispatchCommand`)
- Ran characterization on its own goroutine with a 2s bounded timeout (`detector.go:163` `d.wg.Go`, `characterize.go:22,38`) over an inline call in `onRate`, so the rate tick and `d.mu` are never blocked by the engine round-trip
- Picked the highest-byte destination as a host prefix (/32 v4, /128 v6) over a heuristic aggregate, because `ddos-local` drops a single victim and trafficusage already ranks egress IPs by bytes (`characterize.go:100-133`)
- Kept the fallback identical to pre-Phase-1 behavior (empty target + `FamilyGenericFlood`) when dispatch is nil, the source errors, or returns no destination, so the change is never worse than before (`characterize.go:42-65,76-93`)
- Built the command with `textbuf.Buffer`, not string concat, per no-sprintf-alloc (`characterize.go:81-82`)

## Consequences

- The mitigation chain now acts: a volumetric trigger installs a targeted `ddos-local` drop. Responders still receive `FamilyGenericFlood`; proto/ports/flags/family and IPv6 targets arrive with the flowexport tap in a later phase (trafficusage is IPv4-only today, `characterize.go:96-99`)
- `AttackDetected` is now emitted ASYNCHRONOUSLY (from the goroutine), where `Ongoing`/`Cleared` are emitted synchronously from `onRate`. Any future subscriber ordering assumption must account for this split. Two guards exist: `Ongoing` is gated on `detectedEmitted` (`detector.go:138`) and stale emits are dropped by `attackGen` (below)
- `characterizeTarget` is the single funnel for "is a flow source present"; the Phase-5 `doctor-ddos-detect-no-flow-source` check (D-6) should hang off the same condition

## Gotchas

- `ddos-local` parses and range-checks `max-mitigation-duration` (`local/config.go:19,46,69`) but NEVER enforces it: the only drop-removal path is `onCleared`->`removeMitigation` (`local/responder.go:93-110`). So an async `Detected` arriving AFTER `Cleared` installs a drop with no matching `Cleared` to remove it -- a permanently stuck rule. Fixed with a generation guard: `attackGen` bumps on every activate (`detector.go:158`) and every clear (`detector.go:169`); `characterizeAndEmit` re-reads it under `d.mu` and skips the emit if it advanced (`characterize.go:51-56`)
- Same async split caused an ordering hazard: a slow flow query could let the first synchronous `Ongoing` overtake the attack's `Detected` (flowtriq subscribes both). Gated `Ongoing` behind `detectedEmitted atomic.Bool`, set only after `Detected` reaches the bus (`characterize.go:69`, read at `detector.go:138`, reset in `emitCleared` at `detector.go:170`)
- `statusDone` is duplicated as a local `"done"` constant rather than importing `plugin.StatusDone`, to keep the detector runtime path free of an `internal/component/plugin` import (`characterize.go:24-26`)
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
- `plan/spec-cp-survival-5-detect-5-characterization.md` -- in-progress 1/5; deviations D-1..D-8; review gate runs
