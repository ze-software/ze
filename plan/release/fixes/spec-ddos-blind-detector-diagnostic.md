# Spec: ddos-blind-detector-diagnostic

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | An armed detector with no live input must not present a healthy ready state. |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** ze cannot tell an operator that its ddos detector is armed and
blind. The plugin registers exactly one doctor check, `checkFlowSource`
(`internal/plugins/ddos/detect/doctor.go`), and that check reads the config tree
and nothing else: it warns `doctor-ddos-detect-no-flow-source` when ddos is
enabled with characterization on and neither `traffic/usage` nor `flow-export`
is a present container. It never touches the detector instance, the baseline,
the tick count, or any runtime counter.

**The failure it cannot see.** A source that is configured but dead passes this
check silently. The detector then runs against zero traffic, and every part of
it reports healthy:

- `(*baseline).Ready` (`internal/plugins/ddos/detect/baseline.go`) is
  `len(b.samples) >= b.window`, a count with no condition on the values, so a
  window of zeros makes the baseline "ready".
- `(*baseline).Threshold` on that baseline returns `max(0*multiplier, floor)`,
  which is the configured absolute floor. `config.Validate` forces the floor to
  be at least 1, so the threshold is a plausible non-zero number.

The result is a detector that looks warmed and armed, publishes a ready
baseline, and can never trigger, because nothing is arriving. Nothing in the
product says so. The operator discovers it during an attack.

**The goal.** A runtime readiness diagnostic that distinguishes "no attacks" from
"no traffic". The spec must decide:

1. **The signal.** Sustained zero (or near-zero) observed rate across a window
   long enough not to fire on a genuinely idle lab box. The tick count and the
   observed rates are both already on the detector; nothing new needs measuring.
2. **The surface.** A doctor check is the obvious home and the plugin already
   owns one, but a doctor check is pull-based and an operator has to run it. The
   alternative or addition is a log warning at the moment the condition is first
   met, and a field on the show surface. Which of the three, and whether the
   diagnostic is one-shot or latching, is the design question.
3. **What it says.** The useful message names the configured source and the fact
   that nothing has arrived, because the overwhelming majority of real instances
   of this are a misconfigured source rather than a genuinely silent network.
4. **The idle-box false positive.** A lab or standby router legitimately carries
   no traffic. The diagnostic must be a warning rather than an error, and it
   should be suppressible, or conditioned on the detector having been enabled
   deliberately.

**Related, and deliberately separate.** The baseline-readiness half of this
(`Ready` returning true on a window of zeros) is not itself a defect: the floor
saves the threshold, so nothing behaves incorrectly. It only becomes visible as
a problem once a diagnostic wants to distinguish a warm baseline from an empty
one, which is why it is noted here rather than in a fixit shard. If
`plan/post-release/features/spec-ddos-baseline-warmup-maturity.md` lands first, its maturity
accessor is the natural input to this check.

**Premise corrected (research, 2026-11-09).** The paragraphs above say "a
source that is configured but dead". The detector does not read the flow
sources. Its input is the per-interface receive rate:
`trafficstat.Service.SubscribeRates` (`onRates`, `register.go` `runEngine`
`subscribe`). The `iface.SubscribeCollectNotify` fallback (`onRate`) the
research found was unreachable and is deleted. `traffic-usage` and `flow-export` only
feed characterization after a detection (`characterize.go`). "Blind" therefore
means the RATE FEED delivers no sample, reports no interface, or reports zero
traffic. `checkFlowSource` stays: it answers a different question.

**Owning gate.** `go test -race ./internal/plugins/ddos/detect` for the
check itself, then `./le test functional plugin`. `doctor_test.go` already exists in the
package and is the place the new check's unit coverage belongs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` - the detector design
  → Decision: the detector consumes the trafficstat rate feed. `trafficstat.EnsureGlobal` never answers nil, so the iface counter poll fallback in `subscribe` was unreachable and is deleted
  → Constraint: an external plugin process holds its own empty trafficstat service, so its samples carry no interface; the external cause is recorded at `arm` from `p.IsInternal()` and reported instead
  → Constraint: d.mu is never held across an Emit; the watch logs under d.mu and emits nothing
- [ ] `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` - the `// Design:` page of `doctor.go`
  → Decision: it documents the flow-source check, which this spec leaves unchanged; the rate-feed check is documented on the umbrella page, which `blind.go` names
- [ ] `docs/features/ai-first.md` - the `// Design:` page of `internal/core/diagnostic/codes.go`
  → Constraint: a code is explained by `ze show diagnostic <code>`; the page describes the registry, not each code, so it needs no edit
- [ ] `ai/doors/system-plugins.md`, `ai/doors/system.md`, `ai/doors/core.md` - placement and process boundary
  → Constraint: a plugin's check must work when the plugin runs external, so it is answered by the plugin process
  → Constraint: the code spelling lives once in `internal/core/diagnostic/codes.go`

**Key insights:**
- `(*baseline).Ready` counts samples, not values; the window is baseline-window evaluations of check-interval samples.
- OSPF's `ospf-state` (`internal/plugins/ospf/register.go`, `OnDoctorCheck` and `sdk.Registration.DoctorChecks`) is the runtime precedent.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/detect/register.go` - runEngine subscribes the detector to trafficstat; one registry doctor check
- [ ] `internal/plugins/ddos/detect/detector.go` - tickRates folds each sample into tick
- [ ] `internal/plugins/ddos/detect/baseline.go` - Ready is a sample count
- [ ] `internal/plugins/ddos/detect/doctor.go` - checkFlowSource reads the config tree only
- [ ] `internal/plugins/ospf/register.go` - runtime doctor check through OnDoctorCheck
- [ ] `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go` - DoctorCheckDecl, OnDoctorCheck

**Behavior to preserve:**
- `ddos-detect-flow-source` and `doctor-ddos-detect-no-flow-source` unchanged.
- Detection, the baseline and every emitted event unchanged.

**Behavior to change:**
- An armed detector whose rate feed is blind for one window is reported by `ze doctor` and logged.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin runtime doctor check | `pkg/plugin/sdk` `OnDoctorCheck`, `rpc.DoctorCheckDecl`; OSPF `ospf-state` | The engine calls the plugin's callback for a declared check and shows its diagnostics in `ze doctor` | Extends: declares `ddos-detect-rate-feed` and answers it from the running detector |
| Registry doctor check | `registry.Registration.DoctorChecks`, `checkFlowSource` | Config-tree check run in the daemon | Does not apply to runtime state: the daemon process holds no detector when the plugin runs external. Kept for its own question |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` `builtinCodes` | `ze show diagnostic <code>` explains a code | Extends: new exported `CodeDoctorDDoSDetectBlind` and its entry |
| Detector feed tick | `detector.go` `tickRates` (`tickInfos` deleted with its unreachable fallback) | Folds each one-second sample under d.mu | Extends: reports each sample to `observeFeed` |
| Baseline maturity | `plan/post-release/features/spec-ddos-baseline-warmup-maturity.md` | Not landed | Does not apply: the zero-run count answers this check without it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Rate feed sample (`[]trafficstat.InterfaceEntry`), once a second.
- `ze doctor` run by an operator, reaching the plugin through the doctor-check callback.

### Transformation Path
1. `tickRates` computes peak pps/bps and the interface count, call `observeFeed` under d.mu.
2. `observeFeed` updates the zero run, logs WARN at onset and INFO on recovery.
3. The callback loads the armed detector and `checkRateFeed` maps `rateFeedFinding` to a diagnostic.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | `ze-plugin-callback:doctor-check` with `DoctorCheckInput`/`DoctorCheckOutput` | Yes: `test/plugin/api-doctor-check.ci` |

### Changed-Wire Census

**Search evidence:** no wire format changes. The doctor-check callback and its JSON types (`pkg/plugin/rpc/types.go` `DoctorCheckDecl`, `DoctorCheckOutput`) are used unchanged; the Stage 1 declaration gains one entry of the existing shape. Positive control: OSPF declares `ospf-state` through the same field.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Stage 1 `doctor-checks` | writer | `internal/plugins/ddos/detect/register.go` `runEngine` | `sdk.Registration.DoctorChecks` | Preserve format; one new entry |
| Stage 1 `doctor-checks` | reader | engine plugin registration (as for OSPF) | `rpc.DoctorCheckDecl` | Preserve |
| doctor-check callback output | writer | `checkRateFeed` via `OnDoctorCheck` | `rpc.DoctorCheckDiagnostic` | Preserve |
| doctor-check callback output | reader | engine doctor runner that shows plugin diagnostics in `show doctor` | `rpc.DoctorCheckOutput`, proven by `test/plugin/api-doctor-check.ci` | Preserve |

### Integration Points
- `subscribe` in `runEngine` arms the watch and publishes the detector to an `atomic.Pointer`; `unsubscribe` clears it.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The check is answered through the SDK callback |
| No unintended coupling (components stay isolated) | Yes | Only `internal/core/diagnostic` added as an import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses OnDoctorCheck and the code registry |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Not a wire path |
| Registration over hardcoding, outbound | Yes | The check is declared by the plugin |
| Registration over hardcoding, inbound | Yes | No central doctor list names the check; searched `internal/component/doctor` |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The trafficstat feed delivers one sample a second | `detector.go` `tick` comment, trafficstat tick | Window in seconds wrong | Code read of `tick` | confirmed |
| A-2 | The engine dispatches a declared SDK doctor check to the callback | OSPF precedent, `test/plugin/api-doctor-check.ci` | Check never runs | the existing .ci | confirmed |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Idle lab box warns | warning on an idle router | Warning severity, message names the idle case, clears on traffic, `enabled false` silences |
| R-2 | Doctor callback races the tick | race detector | All watch fields under d.mu; pointer via atomic |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false or missing doctor warning; detection itself unchanged |
| How is it reverted? | Single commit revert |
| Who else touches this path? | ddos specs editing detector.go |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| rate feed sample | → | `tickRates` → `observeFeed` | `TestRateFeedZeroTrafficReportedAtWindow`; end to end `test/plugin/ddos-detect-internal-blind.ci` (promoted after its red proof) |
| doctor callback | → | `checkRateFeed` | `TestRateFeedCheckDeclaration`, transport `test/plugin/api-doctor-check.ci` |
| external ddos-detect, doctor callback | → | `arm(feed, p.IsInternal())` → `rateFeedFinding` → `checkRateFeed` | `TestRateFeedExternalReportsCause`; end to end `test/draft/plugin/ddos-detect-external-blind.ci`, promoted when `spec-plugin-event-bus-across-the-process-boundary` lands |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Armed detector, one window of all-zero samples with interfaces | `ze doctor` shows warning `doctor-ddos-detect-blind` naming the feed, the window and `ze show interface rate` | One sample short of the window: no warning, the baseline is not yet all zeros |
| AC-2 | One window of samples with no interface | Warning names the missing interfaces and `ze show interface` | None |
| AC-3 | No sample for one window since arm or since the last sample, internal plugin | Warning says no sample arrived and names `ze show interface rate` | Less than a window: no warning |
| AC-9 | An armed detector that runs as an external plugin process | From arm, the doctor check warns `doctor-ddos-detect-blind` naming the external-process cause and the `plugin { external ddos-detect { ... } }` block to remove; its empty samples never turn it into "no interface", and log no zero-run WARN. Proven by `TestRateFeedExternalReportsCause` (5559f4a2fd). Today an external ddos-detect never arms: `loadBus` refuses `OnConfigure` with "ddos-detect: event bus not configured" (`plan/journal/process-global-read-across-the-plugin-boundary.md`, 2026-11-10, owned by `spec-plugin-event-bus-across-the-process-boundary`). ze logs that line at ERROR, removes the plugin and keeps running, because ddos-detect is not a fatal plugin; `show doctor` then lists no ddos-detect check and is clean (`plan/journal/failed-start-reads-as-absent.md`, 2026-11-10, owned by `spec-doctor-reports-failed-plugin-startup`). The finding becomes reachable end to end when the event-bus spec lands | `warnIfExternal` already logs the cause at start |
| AC-4 | First sample with pps or bps above zero after a blind run | Finding clears; one INFO log line | None |
| AC-5 | Zero run reaches the window | Exactly one WARN log line for the run | Later idle samples log nothing, so the log is not flooded |
| AC-6 | ddos detect disabled | Check answers with an empty diagnostic list | Disabled detector subscribes to no feed |
| AC-7 | Unknown check name, or unarmed detector | Callback returns an error | None: an empty answer would read as healthy |
| AC-8 | `ze show diagnostic doctor-ddos-detect-blind` | Explains the code and the fix commands | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze doctor` on a box whose detector sees no traffic | engine → doctor-check callback → checkRateFeed | `TestRateFeedZeroTrafficReportedAtWindow` plus `test/plugin/api-doctor-check.ci` transport |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRateFeedZeroTrafficReportedAtWindow` | `internal/plugins/ddos/detect/doctor_test.go` | AC-1 | |
| `TestRateFeedWindowFollowsCheckInterval` | same | AC-1 window | |
| `TestRateFeedClearsOnTraffic` | same | AC-4 | |
| `TestRateFeedNoInterface` | same | AC-2 | |
| `TestRateFeedNoSample` | same | AC-3 | |
| `TestRateFeedExternalReportsCause` | same | AC-9 | |
| `TestRateFeedLogsEachEdgeOnce` | same | AC-4, AC-5 | |
| `TestRateFeedDisabledDetectorReportsNothing` | same | AC-6 | |
| `TestRateFeedRefusesUnknownCheckAndUnarmedDetector` | same | AC-7 | |
| `TestRateFeedCheckDeclaration` | same | AC-8 | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| zero-run length vs window | 0 to window | window-1 silent, window warns | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-doctor-check` | `test/plugin/api-doctor-check.ci` | Declared plugin doctor check reaches `show doctor` | existing |
| `ddos-detect-external-blind` | `test/draft/plugin/ddos-detect-external-blind.ci`, fixture `internal/test/fixture/ddos_detect_blind.go` | An external ddos-detect is reported blind with its cause by `show doctor` (AC-9) | draft: red until `spec-plugin-event-bus-across-the-process-boundary` lands |
| `ddos-detect-internal-blind` | `test/plugin/ddos-detect-internal-blind.ci`, fixture `doctorDDoSDetectBlindZero` in `internal/test/fixture/ddos_detect_blind.go` | An internal ddos-detect on a box that receives nothing is reported blind by `show doctor` (AC-1, AC-5) | live: passed in a Ze guest, red proof recorded, promoted and named in `netnsSelections[netnsPlugin]` |

## Files to Modify
- `internal/plugins/ddos/detect/detector.go` - feed watch field, observeFeed calls
- `internal/plugins/ddos/detect/register.go` - arm, atomic pointer, OnDoctorCheck, declaration
- `internal/plugins/ddos/detect/doctor.go` - checkRateFeed and its declaration
- `internal/core/diagnostic/codes.go` - code constant and entry
- `docs/guide/ddos-mitigation.md`, `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md`

## Files to Create
- `internal/plugins/ddos/detect/blind.go` - the feed watch

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | Uses `ze doctor` |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No new RPC; transport covered by `api-doctor-check.ci` |
| Pipe completeness | N-A | No command |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | Yes | `doctor.go` checkRateFeed, `codes.go` |
| Prometheus counters/metrics | No | Doctor and log are the surfaces |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Diagnostic only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Plugin surface unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | guide and umbrella anchors updated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Guide examples unchanged and still valid |

## Implementation Steps
1. **Phase: Wiring** -- declaration, callback, arm in subscribe
   - Tests: `TestRateFeedCheckDeclaration`
   - Files: register.go, doctor.go, codes.go
   - Verify: callback answers the declared name
2. **Phase: Feed watch** -- blind.go, detector.go hooks, edge logs
   - Tests: the rest of the TDD table
   - Files: blind.go, detector.go
   - Verify: tests fail without observeFeed, pass with it

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Window = BaselineWindow * evalTicks samples; zero means pps and bps both zero |
| Data flow | No emit under d.mu; doctor callback reads only the atomic pointer |
| Rule: user-facing-errors | Each message says what, why, and the command |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Runtime check declared and answered | `grep -n rateFeedCheckDecl internal/plugins/ddos/detect/register.go` |
| Code registered | `grep -n CodeDoctorDDoSDetectBlind internal/core/diagnostic/codes.go` |
| Tests | `go test -race ./internal/plugins/ddos/detect` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Check name compared exactly; unknown name refused |
| Resource exhaustion | Log edges once per run; no per-sample allocation besides existing |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights
- The detector's input and the characterization sources are different things; the spec skeleton conflated them.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Runtime check via SDK OnDoctorCheck | Registry check, `RegisterDaemonDoctorCheck` | Those run in the daemon process and see no detector when the plugin runs external |
| Window = one baseline window | Fixed time, separate leaf | The moment the baseline is all zeros; no new config needed |
| Doctor plus edge-triggered log; not latching | Doctor only; show field | Log covers the operator who never runs doctor; latching would keep a cleared box warned. No show field: `show ddos` belongs to ddos-observe in another plugin |
| Exact zero, not near-zero | A small threshold | Any keepalive is traffic; near-zero would need a tuning leaf |

## Implementation Summary

### What Was Implemented
- `blind.go`: the feed watch. `arm` records the feed, whether the plugin runs internal, and the arm time; `observeFeed` (called from `tickRates` under d.mu) counts the zero run and logs one WARN at the window and one INFO when traffic returns; `rateFeedFinding` judges the last window; `blindMessage` is the operator text for both surfaces (3c7aef2403, 5559f4a2fd).
- `doctor.go` `checkRateFeed` and `rateFeedCheckDecl`: the `ddos-detect-rate-feed` plugin doctor check, declared in the SDK registration and answered by `OnDoctorCheck` from the `armed` atomic pointer in `register.go` `runEngine`.
- `internal/core/diagnostic/codes.go` `CodeDoctorDDoSDetectBlind` and its entry.
- The unreachable `iface.SubscribeCollectNotify` fallback, `onRate` and `tickInfos` deleted; their tests drive `onRates` (e4529a2ef1).
- Fixture `plugin/doctor-ddos-detect-blind-zero` (`internal/test/fixture/ddos_detect_blind.go` `doctorDDoSDetectBlindZero`) and the live `test/plugin/ddos-detect-internal-blind.ci`, named in `netnsSelections[netnsPlugin]` (`internal/le/test/qemu/netns_linux.go`) (f9c8beeec7, closure commit A).

### Bugs Found/Fixed
- An external ddos-detect was reported "no interface" (round 1 BLOCKER): `arm` records `p.IsInternal()`, covered by `TestRateFeedExternalReportsCause`.

### Documentation Updates
- `docs/guide/ddos-mitigation.md` (blind-detector section, check-interval paragraph), `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md`, `docs/architecture/traffic/traffic-usage-monitor.md` Consequences (one feed, anchor `register.go -- runEngine, EnsureGlobal, SubscribeRates`): landed in 3c7aef2403, 5559f4a2fd, e4529a2ef1, f9c8beeec7.
- Closure: `docs/architecture/testing/qemu-integration.md` (command table `plugin` row; plugin-selection paragraph with anchor `netns_linux.go -- netnsSelections`) and `docs/architecture/testing/ci-workflows.md` (plugin subset list) name the new case. `internal/le/test/qemu/guestlabs.go` `netnsSuiteHelp` for `plugin` names it, and redistribution, which it had omitted.
- `./le doc check all`: owed to the main thread (not run in the closure agent).

### Deviations from Plan
- The internal `.ci` has no stress run under load: no QEMU form of the stress-repro exists, and on the host the test is skipped (Known Limitations).
- The no-interface case has no `.ci`: a namespace always holds lo.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The promotion plan named only `qemu-integration.md` as the page listing the `plugin` netns selection | `ci-workflows.md` and the `netnsSuiteHelp` help text list it too | Closure grep for the selection's members (`redistribute-chain-static-ospf`) | All three updated in closure commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The signal: sustained zero over a window | Done | `blind.go` `observeFeed`, `blindSamples` | Window = baseline-window x check-interval samples |
| The surface: doctor plus edge log | Done | `doctor.go` `checkRateFeed`; `blind.go` `observeFeed` | No show field (Key Design Decisions) |
| What it says: names the feed and the fix command | Done | `blind.go` `blindMessage` | |
| Idle-box false positive: warning, clears, suppressible | Done | `checkRateFeed` severity `warning`; `observeFeed` resets on traffic; disabled publishes nil | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRateFeedZeroTrafficReportedAtWindow`, `TestRateFeedWindowFollowsCheckInterval`, `test/plugin/ddos-detect-internal-blind.ci` | |
| AC-2 | Done | `TestRateFeedNoInterface` | No `.ci` (Deviations) |
| AC-3 | Done | `TestRateFeedNoSample` | |
| AC-4 | Done | `TestRateFeedClearsOnTraffic`, `TestRateFeedLogsEachEdgeOnce` | |
| AC-5 | Done | `TestRateFeedLogsEachEdgeOnce`; `.ci` asserts the WARN text on stderr | |
| AC-6 | Done | `TestRateFeedDisabledDetectorReportsNothing` | |
| AC-7 | Done | `TestRateFeedRefusesUnknownCheckAndUnarmedDetector` | |
| AC-8 | Done | `TestRateFeedCheckDeclaration`; `codes.go` entry | |
| AC-9 | Done | `TestRateFeedExternalReportsCause` | Not reachable end to end until the event bus crosses the process boundary (Work Not Done) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The ten `TestRateFeed*` units | Done | `internal/plugins/ddos/detect/doctor_test.go` | |
| `ddos-detect-internal-blind` | Done | `test/plugin/ddos-detect-internal-blind.ci` | Passed in a Ze guest, red recorded |
| `ddos-detect-external-blind` | Changed | `test/draft/plugin/ddos-detect-external-blind.ci` (untracked draft) | Red until the event-bus spec lands |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `detector.go`, `register.go`, `doctor.go`, `blind.go`, `codes.go` | Done | 3c7aef2403, 5559f4a2fd, e4529a2ef1, f9c8beeec7 |
| `docs/guide/ddos-mitigation.md`, `cp-survival-5-detect-0-umbrella.md` | Done | same commits |

### Audit Summary
- **Total items:** 4 requirements, 9 ACs, 12 tests, 7 files
- **Done:** all but one test
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1, the external `.ci` stays a draft (Deviations, Work Not Done)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An armed detector whose rate feed shows no traffic is reported by `ze doctor`, distinguishing "no attack" from "no traffic" | Functional test end to end on the real rate feed, in a Ze guest, with a recorded red | `test/plugin/ddos-detect-internal-blind.ci` (fixture `doctorDDoSDetectBlindZero`), run on its draft path by the main thread: `./le test qemu docker-lab kernel ~/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-a9caebdd-0d1d6496/vmlinuz harness timeout 1800s command "ZE_BIN=tmp/qemu/linux-amd64/bin/ze ... tmp/qemu/linux-amd64/le test qemu all test test/draft/plugin/ddos-detect-internal-blind.ci"` gave `16.0s 1/1 PASS 18 ddos-detect-internal-blind ... ALL PHASES PASSED`. Red: `traffic := maxPps > 0 \|\| maxBps > 0` replaced by `traffic := true` in `blind.go` `observeFeed`, guest binaries rebuilt: FAIL (41.8s) `ZE-OBSERVER-FAIL: show doctor lacks doctor-ddos-detect-blind naming "has reported zero received packets and bytes" after 3 reads, each after 13s of silence`; file restored byte-exact and binaries rebuilt. Units `TestRateFeedZeroTrafficReportedAtWindow`, `TestRateFeedNoInterface`, `TestRateFeedNoSample` drive `tickRates` and `checkRateFeed` |
| The operator who never runs doctor sees it in the log, once per edge | Unit test, plus the onset WARN asserted by the `.ci` | `TestRateFeedLogsEachEdgeOnce`; `ddos-detect-internal-blind.ci` `expect=stderr:contains=has reported zero received packets and bytes on every interface for 10s`, a text the fixture never prints, so only the daemon's WARN line (`observeFeed`) satisfies it |
| An armed external detector is reported blind with its real cause (AC-9) | Unit test | `TestRateFeedExternalReportsCause` (5559f4a2fd, red proof done). No external detector arms today: `loadBus` refuses Config, ze logs ERROR and keeps running without the plugin, and `show doctor` is clean (`plan/journal/failed-start-reads-as-absent.md`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| End-to-end proof of AC-9: promote `ddos-detect-external-blind.ci` | A forked plugin is handed no event bus, so an external ddos-detect fails Config and never arms | `plan/release/fixes/spec-plugin-event-bus-across-the-process-boundary.md` |
| `show doctor` names a non-fatal plugin that failed startup | The failed plugin is removed and its checks are no longer listed, so doctor reads clean | `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ddos-blind-detector-diagnostic-f90e7822-ecbf-462e-ac37-c494b4c2c988.md` (8 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | Round 1: logic and wiring. Round 2: docs against producers, startup path, `.ci` coverage. Round 3 (closure agent, which authored none of the six landed commits; the promotion edits are its own and are the smallest part of the diff): f9c8beeec7, d293a82e4a and the promotion; wiring of the new selection entry against `validateNetnsSelection`, fixture timing against the 60s timeout, comment accuracy, style pass over the changed Go |

| Round | Reviewer | BLOCKER | ISSUE | Resolution |
|-------|----------|---------|-------|------------|
| 1 | independent reviewer | 1: an external ddos-detect was reported "no interface" | 1: no ddos-specific `.ci` | BLOCKER fixed in 5559f4a2fd. ISSUE answered by the external draft and the Known Limitations entry |
| 2 | second independent reviewer | 0 | 4: "fails loudly" half true; guide and `codes.go` claimed the external case "is reported"; stale two-feed docs after e4529a2ef1; no `.ci` for the internal blind finding. 3 NOTE | Fixed in d293a82e4a (journal rows, owning spec, reworded AC-9) and f9c8beeec7 (docs, comment, fixture); the `.ci` promoted at closure |
| 3 | closure agent | 0 | 0 | 3 NOTE, below |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | External detector reported "no interface" | `blind.go` `arm`, `rateFeedFinding` | 5559f4a2fd |
| 2 | ISSUE | Doctor reads clean after a failed plugin start; the claim said it failed loudly | spec AC-9, guide | d293a82e4a; owned by `spec-doctor-reports-failed-plugin-startup` |
| 3 | ISSUE | Guide and `codes.go` said the external case is reported | `docs/guide/ddos-mitigation.md`, `codes.go` | f9c8beeec7 |
| 4 | ISSUE | Two-feed docs stale after the fallback's deletion | guide, `traffic-usage-monitor.md` | f9c8beeec7 |
| 5 | ISSUE | No `.ci` for the internal finding | `test/plugin/ddos-detect-internal-blind.ci` | f9c8beeec7 fixture, closure promotion |

Round 3 NOTEs: (1) `register_ddos_detect_blind.go` named the external test at the live path `test/plugin/ddos-detect-external-blind.ci`, which does not exist; it now names the draft path. (2) The external draft lives only in the gitignored `test/draft/plugin/`, and `test/draft/README.md` admits no tracked exception for a test that is "not ready", so the event-bus spec rebuilds it from the committed fixture `plugin/doctor-ddos-detect-blind` if the file is lost. (3) The `.ci` header said the test runs only under `./le test qemu netns`; a one-test `./le test qemu all test` also prepares its namespace (`allTestsRun.executeOne`), and the header now says so.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ddos/detect/blind.go` | Yes | 207 lines, holds `observeFeed` |
| `test/plugin/ddos-detect-internal-blind.ci` | Yes | moved from `test/draft/plugin/` at closure |
| `test/plugin/api-doctor-check.ci` | Yes | `ls` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-9 | Each test exists | `grep -n '^func Test' doctor_test.go` lists the ten `TestRateFeed*` |
| AC-1, AC-5 | End to end | the guest PASS and the red in Goal Validation |
| AC-8 | Code registered | `codes.go` constant `CodeDoctorDDoSDetectBlind` and its `builtinCodes` entry |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| rate feed sample → `tickRates` → `observeFeed` | `test/plugin/ddos-detect-internal-blind.ci` | `detector.go` `tickRates` calls `observeFeed`; the `.ci` arms the detector with `baseline-window 10`, `check-interval 1`, and the red with `observeFeed` broken failed |
| doctor callback → `checkRateFeed` | `test/plugin/api-doctor-check.ci` | `register.go` `runEngine`: `OnDoctorCheck` calls `checkRateFeed`, and the `sdk.Registration` declares `rateFeedCheckDecl` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `detector.go` `tick` comment; the guest run formed the 10-sample finding inside the fixture's 13s silence (`ddosBlindQuiet`), which a slower feed could not have done |
| A-2 | confirmed | `test/plugin/api-doctor-check.ci`; the guest run reached `checkRateFeed` through `show doctor` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide blind-detector table and check-interval paragraph | `blind.go` `blindMessage`; `register.go` `subscribe` | Yes |
| QEMU and CI pages name the case in the `plugin` selection | `netns_linux.go` `netnsSelections[netnsPlugin]` | Yes |
| No other page lists the selection | `git grep -ln "redistribute-chain-static-ospf\|seven ASPA" -- docs ai .github website`: only those two pages | Yes |

## Known Limitations
- The internal zero-traffic case is proven end to end only in a fresh network namespace (`ddos-detect-internal-blind.ci`, `option=netns-link`): a host or the shared guest root always receives traffic. lo still carries the fixture's TLS connection, so the fixture reads `show doctor` only after a silence longer than one window. The no-interface case has no `.ci`: a namespace always holds lo. Unit tests cover it over the real tick path.
- No stress run under load proves the internal `.ci`. The `ze-test` stress-repro runs on the host, where this `needs-linux` and `netns-link` test is skipped, and no QEMU form of the stress run exists. Its proof is one passing Ze-guest run and one recorded red.
- The external case has no passing `.ci` yet. An external ddos-detect fails at Config with "ddos-detect: event bus not configured", because `ConfigureEventBus` runs only in-process (`plan/journal/process-global-read-across-the-plugin-boundary.md`, 2026-11-10). ze logs it at ERROR and keeps running without the plugin, and `show doctor` is then clean, because a removed plugin's checks are no longer listed (`plan/journal/failed-start-reads-as-absent.md`, owned by `spec-doctor-reports-failed-plugin-startup`). `test/draft/plugin/ddos-detect-external-blind.ci` stays a draft and goes green when `spec-plugin-event-bus-across-the-process-boundary` lands. `test/plugin/ddos-detect-external-warns.ci` masks the Config failure: it tears down after the start warning; that spec owns it too.
- The no-sample case has no log line: with no sample there is no tick to drive one. `ze doctor` reports it.
- `onRate` and `tickInfos` (the cumulative-counter feed) are deleted with the fallback that called them (no-layering). The detector and characterization tests that drove them now drive `onRates`; `TestRateFeedNoInterfaceOnCounterPoll` existed only for the deleted path and is deleted. A `ze-test-judge` approved the change for all 9 units.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (`test/plugin/ddos-detect-internal-blind.ci`, run by `./le test qemu all test` on its draft path, then promoted; `test/draft/plugin/ddos-detect-external-blind.ci`, a draft until `spec-plugin-event-bus-across-the-process-boundary` lands)
- [ ] Interop tests for protocol features (N-A: no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove` the spec only
