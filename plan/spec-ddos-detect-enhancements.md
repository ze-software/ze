# Spec: ddos-detect Enhancements (bandwidth trigger, baseline persistence, incident confidence)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-07-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/1011-cp-survival-5-detect-0-umbrella.md`, `plan/learned/1015-cp-survival-5-detect-5-characterization.md`
4. Source: `internal/plugins/ddos/detect/{baseline.go,detector.go,characterize.go,config.go,register.go,metrics.go}`, `internal/core/ddosevent/event.go`
5. Consumers: `internal/plugins/ddos/{observe,flowtriq,local,flowspec}/`
6. Per-session digests: `tmp/session/session-state-94016.md`

## Task

Port three DDoS-detection improvements observed in Flowtriq ftagent (v1.9.29-1.9.31,
local copy `~/Code/github.com/Flowtriq/ftagent`) into ze's `internal/plugins/ddos/detect`
engine. Scope is user-approved as "the 3 that fit cleanly"; the ftagent items that do not
map onto ze's conntrack-flow + bounded-ring architecture (payload-byte signatures,
HyperLogLog, velocity trigger, classification vote-locking, NetFlow sampling-rate, Agones,
ring-buffer threading) are explicitly excluded.

Structure: ONE spec, THREE phases (user-approved).

1. **Bandwidth (BPS) baseline + trigger for amplification.** The rate trigger is PPS-only
   (`detector.go:181-182`) and the baseline tracks only PPS (`baseline.go`). `maxBps` is
   already threaded through `applyTick` and tracked as `peakRxBps` but never used as a
   trigger, so a low-PPS / high-Gbps amplification flood (NTP/memcached/CLDAP) is missed.
   Add a parallel BPS p99 baseline with a multiplier (default 3x p99) and a minimum
   bandwidth floor (default 50 Mbps) below which the BPS trigger is inert, and OR it into
   the trigger.

2. **Baseline persistence across restart.** The baseline is in-memory only, so a detector
   restart re-enters `StartupGrace` (`detector.go:175`) and re-warms over `BaselineWindow`
   before it can detect again. Persist the rolling baseline state (PPS and BPS samples /
   p99 / threshold / count) and restore it on startup, with a version + minimum-sample guard
   before trusting a restore.

3. **Multi-signal incident confidence (0-100) on the ddos path.** ze emits only `Severity`
   (`ddosevent.GradeSeverity`, `characterize.go:101`). Add a composite `Confidence` computed
   from signals ze already gathers in `classifyFlows` (source entropy, top-source
   concentration, protocol dominance, family specificity, peak/threshold ratio). Wire it
   (user-approved: all three consumers) to `ddos/observe` (stored on the incident, shown in
   `show ddos incidents`), the `ddos/flowtriq` dashboard reporter, and as a gate on the
   `ddos/local` + `ddos/flowspec` responders.

Reference precedent: `anomaly/detect` builds a bounded composite score
(`internal/plugins/anomaly/detect/score.go` `combineScore`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/1011-cp-survival-5-detect-0-umbrella.md` - detector + event contract origin
  → Constraint: `ddosevent` is a value-type contract on the typed EventBus (`events.Register`); adding a field is additive, no wire/schema break, but every consumer must opt in to `Characterized` to see it.
  → Constraint: each `ddos/*` sub-plugin owns its own CLI/config surface (plugin-self-containment); confidence config for a responder lives in that responder's YANG, not centrally.
- [ ] `plan/learned/1015-cp-survival-5-detect-5-characterization.md` - Stage-2 characterization
  → Constraint: characterization runs once per attack off the onRate hot path, guarded by `attackGen`; confidence is computed there, at attack *start*, so attack *duration* is NOT an available signal (unlike ftagent, which scores at attack end).
- [ ] `ai/rules/config-surface.md` - YANG-vs-env decision
  → Decision: BPS multiplier/floor/enable and responder confidence-min are operator capacity tunables → YANG config leaves, no env vars (ddos-detect has none today, and config-surface reserves env for debug/bootstrap/safety-cap knobs).
- [ ] `ai/rules/config-naming.md` - leaf/env naming
  → Constraint: kebab-case YANG leaf, PascalCase Go field with identical word boundaries, JSON tag = exact leaf; booleans positive-form.
- [ ] `ai/patterns/config-option.md` - config leaf template
  → Constraint: every numeric leaf carries `type` + `range` + `default` + `description`; Go `DefaultConfig()` and `Validate()` mirror the YANG default and range (this plugin duplicates defaults in Go rather than reading `SchemaDefault*`).

**Key insights:** BPS baseline is a parallel of the existing PPS baseline (same recalc cadence, same p99 index math); persistence copies the traffic tc-snapshot pattern (versioned JSON + tmp/rename atomic write to `<config-dir>/state/`); confidence is an additive `AttackCharacterized` field computed at characterize time, and `observe`+`flowtriq` must each ADD a `Characterized` subscription (they ignore it today) while the two responders already consume it.

## Current Behavior (MANDATORY)

**Source files read:** (digests in `tmp/session/session-state-94016.md`)
- [ ] `internal/plugins/ddos/detect/baseline.go` - rolling PPS baseline; `Add(pps,attacking)` skips samples when attacking-or-above; `recalc()` every 10 samples; `Threshold()=max(p99*mult,floor)` PPS-only; `Ready()` at window full; no BPS, no persistence.
  → Constraint: `Add` already excludes above-threshold samples (poisoning guard) — the BPS baseline must mirror this so amplification probes don't inflate the BPS p99.
- [ ] `internal/plugins/ddos/detect/detector.go` - `applyTick(maxPps,maxBps,maxIface)` (:174); trigger `above:=maxPps>threshold` (:181-182); `maxBps` carried but only sets `peakRxBps` (:188-192); `baseline.Add(maxPps, attacking||above)` (:186); StartupGrace (:175); state machine `d.sm`; `Stop()` cancels ctx + `wg.Wait` (:89).
  → Constraint: `applyTick` holds `d.mu`; any BPS-baseline mutation and persistence save must respect that lock and not block the tick on I/O.
- [ ] `internal/plugins/ddos/detect/characterize.go` - `classifyFlows` returns `(family,vec,topSources,entropy)` (:372-444); `GradeSeverity` at :101; builds `AttackCharacterized` literal at :204-214.
  → Constraint: confidence must be derived only from data already in scope at :196-214 (family, vec, topSources, entropy, peakPps, threshold) — no new source query.
- [ ] `internal/plugins/ddos/detect/config.go` - `Config`, `DefaultConfig`, `ParseConfig` (map[string]any, unwraps `{"ddos":{"detect":{}}}`), `Validate` ranges; no env vars.
- [ ] `internal/core/ddosevent/event.go` - `AttackDetected` (:73-82), `AttackCharacterized` (:90-103), `Severity` (:24-30), `GradeSeverity` (:36-48).

**Behavior to preserve:**
- PPS trigger semantics and the existing baseline poisoning guard (`Add` skips attacking/above samples).
- `AttackDetected`/`AttackCharacterized`/`AttackOngoing`/`AttackCleared` existing fields and JSON tags (additive `confidence` only).
- Characterization runs once per attack off the hot path, `attackGen`-guarded; the confidence computation must not add a source query or block the tick.
- `show ddos` / `show ddos incidents` existing output fields (additive `confidence` only).
- Responder default behavior when the new confidence-min is 0 (disabled) — mitigation unchanged.

**Behavior to change:** the three phases above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
Per-interface rate samples arrive from the trafficstat service as `[]trafficstat.InterfaceEntry`
into `detector.onRates` (`detector.go:101`), or as `[]iface.InterfaceInfo` into `detector.onRate`
(`detector.go:127`); both reduce to the busiest interface and call `applyTick(maxPps, maxBps, maxIface)`
(`detector.go:174`). Config arrives as a JSON subtree `{"ddos":{"detect":{...}}}` via the plugin's
`OnConfigApply` into `ParseConfig` (`config.go:55`). Persisted baseline state is read at detector
construction from `<config-dir>/state/ddos-detect-baseline.json`.

### Transformation Path
1. trafficstat tick → `onRates`/`onRate` computes per-tick `maxPps`/`maxBps` for the busiest interface → `applyTick` (`detector.go:174`).
2. `applyTick`: `baseline.Add(maxPps, maxBps, attacking||above)` records both series with the poisoning guard; PPS threshold `= max(p99Pps*mult, pps_floor)`; NEW BPS threshold `= max(p99Bps*bpsMult, bps_floor)` computed only once the baseline is ready and above the 50 Mbps floor.
3. Trigger `above = maxPps > ppsThreshold OR (bps_ready AND maxBps > bpsThreshold)` (NEW clause). State machine `Tick(above)`; sustained-confirmation and startup-grace unchanged.
4. On idle→active transition `onAttackStart` snapshots context and spawns `characterizeAndEmit` (`detector.go:215`, off the hot path).
5. `characterizeFromFlows` → `classifyFlows` yields family/vector/topSources/entropy → NEW `GradeConfidence(...)` → `AttackCharacterized{..., Confidence}` emitted (`characterize.go:204-214`).
6. Consumers: `observe` (NEW `Characterized` subscription → writes confidence onto the open incident by DstPrefix → surfaced in `show ddos incidents`); `flowtriq` (NEW `Characterized` subscription → carries confidence into the next `updateIncident`/`resolveIncident` POST); `local` + `flowspec` responders (`onCharacterized` → NEW `confidence < confidence-min` gate before mitigating).
7. Persistence: baseline saved (atomic tmp+rename) on detector `Stop()` and periodically; restored on construction so a restart skips the `BaselineWindow` re-warm.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| trafficstat → detect | `[]InterfaceEntry` via rate subscription → `applyTick` | [ ] |
| detect → observe/flowtriq/responders | `ddosevent.AttackCharacterized` (additive `Confidence`) on typed EventBus | [ ] |
| detect → disk | versioned JSON baseline, tmp-file + `os.Rename` atomic write under `<config-dir>/state/` | [ ] |
| config tree → detect/local/flowspec | `OnConfigApply` → `ParseConfig` → `Config` (new leaves) | [ ] |

### Integration Points
- `baseline.go` - `Add`/`Threshold`/`Ready` extended for a parallel BPS series; new `Snapshot`/`Restore` for persistence.
- `ddosevent/event.go` - `AttackCharacterized.Confidence` + `GradeConfidence` helper alongside `GradeSeverity`.
- `detect/register.go` - config parse for new leaves; baseline restore on start, save on stop/periodic.
- `observe/{store.go,register.go,show.go}` - new `Characterized` subscription + confidence on incident record.
- `flowtriq/{register.go,client.go}` - new `Characterized` subscription + `confidence` in the reported payload.
- `local/{responder.go,config.go,yang}` and `flowspec/{responder.go,config.go,yang}` - `confidence-min` gate + leaf.

### Architectural Verification
- [ ] No bypassed layers (rates → applyTick → state machine → characterize → event, unchanged)
- [ ] No unintended coupling (BPS baseline stays inside detect; confidence stays on the event contract)
- [ ] No duplicated functionality (BPS baseline reuses the PPS recalc cadence; persistence reuses the tc-snapshot idiom)
- [ ] Zero-copy preserved where applicable (no per-tick allocation added; persistence I/O off the tick)
- [ ] Registration over hardcoding — each responder's confidence-min is its own YANG leaf; no central switch

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The detector is constructed on config apply and torn down via `Stop()` on reconfigure/shutdown, giving a place to restore-on-start and save-on-stop. | `detector.go:66,89`; NTP lifecycle precedent | Persistence has no clean save hook; may need a periodic-only save | Read `detect/register.go` lifecycle | **confirmed**: `newDetector` at `register.go:162,192` (restore hook); `det.Stop()` at `register.go:160,190,213,220` fires on both reconfigure and shutdown (save hook). Reconfigure Stop→newDetector even preserves the baseline across config changes. |
| A-2 | Confidence can be computed purely from `classifyFlows` outputs already in scope at characterize.go:196-214 (no new query, no packet payload). | `characterize.go:196-214` | Confidence needs data not present → scope grows | Grep the emit site fields | **confirmed**: family/vec/topSources/entropy/peakPps/threshold all in scope at the emit site (agent research + `characterize.go:196-214`). |
| A-3 | `observe` and `flowtriq` do NOT subscribe to `Characterized` today, so both need a new subscription to see confidence. | Agent research: `observe/register.go:89-97`, `flowtriq/register.go:86-129` | If they already subscribe, less work | Re-grep both register.go | **confirmed** by agent research; both subscribe only Detected/Ongoing/Cleared. |
| A-4 | Adding a YANG leaf needs no codegen-glue edit (embed.go/yang register.go are generated but the module is already registered). | Agent research: `detect/yang/{embed.go,register.go}` are codegen | `make generate` needed / build breaks | Add leaf, run `make generate` + build in Phase 1 | **confirmed**: leaf loads via `go:embed` (embed.go embeds the whole .yang), no glue edit; config/yang + composition tests pass in Phase 1 |
| A-5 | The internal BPS value and the config floor share a unit. | `detector.go` uses `RxBps` | Threshold off by 8x | Read the `RxBps` producer | **confirmed WITH FINDING**: `iface/rate.go:218` `rate.RxBps = rateDelta(RxBytes,prev,elapsed)` = **bytes/sec** (name "Bps" is a misnomer; `rate_test.go:61` confirms). So the operator-facing floor (bits/sec) needs an ×8 conversion — see Key Design Decision "BPS unit". |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | BPS trigger raises false positives on legitimate high-bandwidth bursts (backups, video). | FP incidents in `show ddos incidents` with low confidence | 50 Mbps floor + 3x p99 multiplier + BPS baseline poisoning guard; BPS trigger only after baseline ready; `bps-trigger-enable` (default true) lets an FP-prone site disable just the bandwidth path. |
| R-2 | Restoring a stale baseline after a long downtime detects against outdated traffic. | Threshold far from current traffic after restart | Version + min-sample guard; consider a max-age on the persisted file; fall back to fresh warm-up if guard fails. |
| R-3 | Confidence gate on responders suppresses a real mitigation (confidence-min too high). | Attack proceeds with no rule installed | Default confidence-min = 0 (gate disabled); document tuning; gate only the `Characterized` precise path, never the `Detected` critical RTBH fast path. |
| R-4 | Persistence I/O on the detector lock stalls the 1s tick. | Tick timing drift under slow disk | Save off the lock (snapshot under lock, write outside); periodic + on-stop only, never per-tick. |
| R-5 | Confidence computed at attack start lacks duration (ftagent scores at end), so it may over/under-state vs ftagent. | Confidence values cluster oddly | Document the semantic difference; base the formula on start-time signals only. |

## Wiring Test (MANDATORY — NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `applyTick` with high maxBps, sub-threshold maxPps | → | BPS trigger in `applyTick` | `TestApplyTick_BpsTriggerFiresBelowPpsThreshold` |
| detector construct with a saved baseline file | → | baseline restore in `newDetector`/register | `TestBaselineRestore_SkipsWarmup` |
| `AttackCharacterized` emitted | → | `observe` stores + `show ddos incidents` renders confidence | `TestObserveIncident_CarriesConfidence` |
| `AttackCharacterized` with confidence below min | → | `flowspec.onCharacterized` gate | `TestFlowspecResponder_ConfidenceGateSuppresses` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Baseline ready; maxBps > 3x BPS-p99 and > 50 Mbps floor; maxPps below PPS threshold | Trigger fires (bandwidth path); state machine sees `above=true` |
| AC-2 | Baseline ready; maxBps above floor but below 3x BPS-p99 | No BPS trigger; PPS path governs unchanged |
| AC-3 | BPS baseline below the 50 Mbps floor | BPS trigger inert; only PPS trigger can fire |
| AC-3b | `bps-trigger-enable` = false | BPS trigger never fires regardless of BPS baseline; PPS path unchanged |
| AC-4 | BPS-baseline samples arrive while `attacking||above` | Sample excluded from BPS p99 (poisoning guard mirrors PPS) |
| AC-5 | Detector restarts with a valid persisted baseline (>= min samples, matching version) | Baseline restored; `Ready()` true; no `BaselineWindow` re-warm |
| AC-6 | Persisted file missing/corrupt/version-mismatch/too-few-samples | Restore rejected; detector warms fresh; no crash |
| AC-7 | `AttackCharacterized` emitted for a single-source, single-proto, high-ratio flood | `Confidence` high (near 100); bounded to [0,100] |
| AC-8 | `AttackCharacterized` emitted for a low-ratio, high-entropy, generic-flood | `Confidence` lower; still in [0,100] |
| AC-9 | Incident stored by `observe` | `show ddos incidents` includes a `confidence` field for the incident |
| AC-10 | `flowtriq` reporting enabled, attack characterized | Reported payload includes `confidence` |
| AC-11 | Responder `confidence-min` = N; `Characterized.Confidence` < N | Responder does NOT mitigate on the characterized path; logs the suppression |
| AC-12 | Responder `confidence-min` = 0 (default) | Responder behavior identical to today (no gate) |
| AC-13 | `flowspec` critical-severity RTBH fast path (`onDetected`) | Never gated by confidence (AttackDetected carries none) |

### Confidence Formula (proposed — pinned in Phase 3, tuned to satisfy AC-7/AC-8 + bounds)

`GradeConfidence` is a pure helper beside `GradeSeverity`, taking only signals in scope at
the emit site (`characterize.go:196-214`): `peakPps`, `threshold`, `family`, `entropy`,
`entropyThreshold`. No new source query, no packet payload, no duration (computed at attack
start). Result clamped to [0,100].

| Signal | Source | Contribution |
|--------|--------|--------------|
| Base | — | 25 |
| Peak/threshold ratio `r = peakPps/threshold` (threshold>0 else r=1) | `peakPps`,`threshold` | `+ min(30, int(r*6))` (max at r≥5) |
| Classified family (≠ `generic-flood`) — a dominant proto + discriminator matched | `family` | `+25` |
| Highest-specificity family (`reflection` or `syn-flood`) | `family` | `+10` (additional) |
| Distributed source spread (`entropy ≥ entropyThreshold`) | `entropy` | `+10` |
| Clamp | — | bounded to [0,100] |

Worked bounds: clear attack (r≥5, reflection, distributed) → 25+30+25+10+10 = 100 (AC-7);
ambiguous (r≈1, generic-flood, low entropy) → 25+6 = 31 (AC-8). Unit test asserts [0,100]
and monotonic non-decrease in `r`.

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Suffers a 22 Gbps / low-PPS NTP amplification | trafficstat → applyTick BPS trigger → state machine → characterize → AttackDetected/Characterized → responder | `TestApplyTick_BpsTriggerFiresBelowPpsThreshold` + functional `.ci` |
| 2 | Restarts the router mid-baseline | persisted baseline restore → detector Ready without re-warm | `TestBaselineRestore_SkipsWarmup` |
| 3 | Runs `show ddos incidents` after an attack | observe store (Characterized sub) → confidence field | `TestObserveIncident_CarriesConfidence` + `.ci` |
| 4 | Sets `confidence-min` to avoid low-confidence upstream announces | flowspec onCharacterized gate | `TestFlowspecResponder_ConfidenceGateSuppresses` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBaseline_BpsThreshold` | `internal/plugins/ddos/detect/baseline_test.go` | BPS p99 + multiplier + floor | |
| `TestBaseline_BpsPoisoningGuard` | `internal/plugins/ddos/detect/baseline_test.go` | attacking/above samples excluded from BPS series | |
| `TestApplyTick_BpsTriggerFiresBelowPpsThreshold` | `internal/plugins/ddos/detect/detector_test.go` | BPS-only trigger | |
| `TestBaseline_SnapshotRestoreRoundTrip` | `internal/plugins/ddos/detect/baseline_test.go` | serialize/restore fidelity | |
| `TestBaselineRestore_RejectsBadState` | `internal/plugins/ddos/detect/state_test.go` | version/min-sample/corrupt guards | |
| `TestGradeConfidence_Bounds` | `internal/core/ddosevent/event_test.go` | [0,100] clamp; monotonic in ratio | |
| `TestObserveIncident_CarriesConfidence` | `internal/plugins/ddos/observe/store_test.go` | confidence stored + rendered | |
| `TestFlowspecResponder_ConfidenceGateSuppresses` | `internal/plugins/ddos/flowspec/responder_test.go` | gate below min; pass at/above | |
| `TestLocalResponder_ConfidenceGate` | `internal/plugins/ddos/local/responder_test.go` | gate below min; default 0 unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| bps-threshold-multiplier | 1.00-100.00 | 100.00 | 0.99 | 100.01 |
| bps-floor (bits/sec) | 1-max | max | 0 | N/A |
| confidence-min (local/flowspec) | 0-100 | 100 | N/A (uint) | 101 |
| Confidence (computed) | 0-100 | 100 | clamps to 0 | clamps to 100 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ddos-bps-amplification` | `test/plugin/*.ci` | Low-PPS/high-BPS flow triggers a detection | |
| `test-ddos-incident-confidence` | `test/plugin/*.ci` | `show ddos incidents` shows a confidence value | |

## Files to Modify
- `internal/plugins/ddos/detect/baseline.go` - parallel BPS series; Snapshot/Restore
- `internal/plugins/ddos/detect/detector.go` - BPS trigger clause in `applyTick`; restore-on-construct, save-on-Stop hook
- `internal/plugins/ddos/detect/config.go` - `BpsThresholdMultiplier`, `BpsFloor`, `BpsTriggerEnable` fields + parse + validate
- `internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang` - new leaves + revision
- `internal/plugins/ddos/detect/register.go` - wire persistence lifecycle
- `internal/plugins/ddos/detect/metrics.go` - counter for BPS-triggered detections (if observable)
- `internal/core/ddosevent/event.go` - `AttackCharacterized.Confidence` + `GradeConfidence`
- `internal/plugins/ddos/detect/characterize.go` - compute + set `Confidence`
- `internal/plugins/ddos/observe/{store.go,register.go,show.go}` - Characterized subscription + confidence
- `internal/plugins/ddos/flowtriq/{register.go,client.go}` - Characterized subscription + confidence payload
- `internal/plugins/ddos/local/{responder.go,config.go,yang/ze-ddos-local-conf.yang}` - confidence-min gate + leaf
- `internal/plugins/ddos/flowspec/{responder.go,config.go,yang/ze-ddos-flowspec-conf.yang}` - confidence-min gate + leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `detect/yang/ze-ddos-detect-conf.yang`, `local/yang/...`, `flowspec/yang/...` |
| YANG validation constraints | Yes | range on every new numeric leaf |
| YANG custom validators | No | native `range` suffices |
| CLI commands/flags | No | `show ddos incidents` auto-renders the new field |
| Functional test for new behavior | Yes | `test/plugin/*.ci` |
| Env var registration | No | ddos uses YANG-only config (config-surface: operator tunables) |
| Doctor check for runtime dependencies | Yes/verify | persisted baseline file path under `<config-dir>/state/` — assess whether a doctor check is warranted |
| Prometheus counters/metrics | Yes | BPS-trigger counter in `detect/metrics.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, `docs/guide/ddos-mitigation.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (new leaves) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show ddos incidents` confidence) |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (ddos-detect/observe/flowtriq/local/flowspec) |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` |
| (7-17 answered at Completion) | | | |

## Files to Create
- `internal/plugins/ddos/detect/persist.go` - baseline Snapshot/Restore + atomic file I/O (tc-snapshot idiom)
- `internal/plugins/ddos/detect/persist_test.go` - round-trip + guard tests
- `test/plugin/<ddos-bps>.ci`, `test/plugin/<ddos-confidence>.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary + two-commit closure |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `Confidence` field, new config fields (parsed but unused), failing wiring tests.
   - Tests: the four Wiring Test rows (failing)
   - Files: `event.go`, `detect/config.go`, `detect/baseline.go` (stubs)
   - Verify: entry points exist; wiring tests fail because logic is a stub
2. **Phase 1: BPS baseline + trigger** — parallel BPS series in `baseline.go`, trigger clause in `applyTick`, YANG leaves + parse + validate.
   - Tests: `TestBaseline_BpsThreshold`, `TestBaseline_BpsPoisoningGuard`, `TestApplyTick_BpsTriggerFiresBelowPpsThreshold`, boundary tests
   - Files: `baseline.go`, `detector.go`, `config.go`, `detect/yang`, `metrics.go`
   - Verify: A-5 unit check; tests fail → implement → pass
3. **Phase 2: Baseline persistence** — `persist.go` Snapshot/Restore, restore-on-construct, save-on-Stop + periodic; guards.
   - Tests: `TestBaseline_SnapshotRestoreRoundTrip`, `TestBaselineRestore_RejectsBadState`, `TestBaselineRestore_SkipsWarmup`
   - Files: `persist.go`, `detector.go`, `register.go`
   - Verify: A-1 lifecycle check first; R-4 (I/O off lock)
4. **Phase 3: Incident confidence** — `GradeConfidence`, compute+set in `characterize.go`, wire observe + flowtriq (new Characterized subs) + responder gates.
   - Tests: `TestGradeConfidence_Bounds`, `TestObserveIncident_CarriesConfidence`, `TestFlowspecResponder_ConfidenceGateSuppresses`, `TestLocalResponder_ConfidenceGate`
   - Files: `event.go`, `characterize.go`, `observe/*`, `flowtriq/*`, `local/*`, `flowspec/*`
   - Verify: A-2/A-3 checks; R-3 (default 0 unchanged)
5. **Functional tests** → `test/plugin/*.ci`
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | BPS unit correct (A-5); poisoning guard mirrors PPS; confidence clamped [0,100] |
| Data flow | Persistence I/O never on the tick lock; confidence uses only in-scope signals |
| Naming | kebab-case leaves; `confidence`, `bps-threshold-multiplier`, `bps-floor`, `confidence-min` |
| Registration over hardcoding | responder confidence-min is per-plugin YANG; no central switch |
| Doctor checks | assess baseline-file-path doctor check |
| YANG validation | every new leaf has range + default + description |
| Prometheus counters | BPS-trigger counter defined + registered |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| BPS trigger | `go test ./internal/plugins/ddos/detect/ -run Bps` |
| Persistence | `go test ./internal/plugins/ddos/detect/ -run Restore` |
| Confidence wired to 3 consumers | grep Characterized subscriptions in observe+flowtriq; responder gate tests |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | persisted JSON is untrusted on disk — validate version, bounds, reject NaN/negative before use |
| Resource exhaustion | persisted file size bounded (fixed window); no unbounded growth |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural → DESIGN |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- **Phase 1 (done):** BPS baseline is a SECOND `baseline` instance (`d.baselineBps`), so `baseline.go` needed ZERO changes — the poisoning guard, p99 recalc, and `Ready()` are reused as-is. The ×8 unit conversion is a single `cfg.BpsFloor/8.0` at `newBaseline` (bits/sec leaf → bytes/sec floor, since `iface/rate.go:218` `RxBps` is bytes/sec). Trigger clause in `applyTick`: `bpsAbove = enable && baselineBps.Ready() && maxBps > baselineBps.Threshold()`; both baselines fed with the same `attacking||above` guard. Metric `ze_ddos_detect_bps_trigger_total` incremented in `onAttackStart` when the tick was bandwidth-attributed (`bpsAbove && !ppsAbove`). All 47 detect tests pass.
- **Deviation from TDD plan (Phase 1):** dropped separate `TestBaseline_BpsThreshold`/`TestBaseline_BpsPoisoningGuard` unit tests — because BPS reuses the `baseline` type, those would duplicate the existing baseline tests. Coverage is instead at the detector level (`TestApplyTick_BpsTriggerFiresBelowPpsThreshold`, `_BpsTriggerDisabled`, `_BpsBelowFloorInert`) plus config tests (`TestParseBpsLeaves`, `TestBpsDefaults`, `TestBpsBoundaries`). This exercises the actual new logic (trigger + unit conversion + config) rather than re-testing the reused type.
- **Phase 2 (done):** persistence copies the tc-snapshot idiom in a new `persist.go` (versioned JSON, atomic temp+rename via `textbuf` for the `.tmp` suffix, `//nolint:gosec` on the state-dir read like `ntp/persist.go`). `baseline.snapshot()`/`restore()` (same-package) added to `baseline.go`; restore validates version (in `loadBaselines`) + min(50,window) samples + no NaN/Inf/negative (in `restore`). Lifecycle: `newDetector` stays pure and only sets `statePath`; explicit `d.restore()` is called by `register.go` after each `newDetector` (both OnConfigure and OnConfigApply), and `d.saveBaseline()` runs in `Stop()` (after `wg.Wait`, off the lock) plus periodically every `baselineSaveInterval`=300 ticks via `wg.Go` guarded by `d.stopped` (mirrors `onAttackStart`). Refactoring restore out of the constructor made it unit-testable without `env.Get`'s init-time cache (which defeats `t.Setenv`). 54 detect tests pass, race-clean.
- **Deviation (Phase 2):** `env.Get` caches os.Environ at first call, so the originally-planned `t.Setenv`-driven restore test was unreliable. Resolved by injecting `d.statePath` directly in tests (in-package) after keeping the constructor I/O-free.
- **Pre-existing unrelated failures:** `internal/component/config/cli` has 3 failing tests (`TestValidateListenerConflictRelated`, `TestConfigFixPlanRepairIDs`, `TestConfigFixPlanRepairIDsFromFix`) about web-listener conflict detection. Confirmed NOT caused by this spec: my diff is isolated to `internal/plugins/ddos/detect/`, `config/cli` is byte-identical to `origin/main` (`git diff origin/main...HEAD -- internal/component/config/cli/` empty), the test configs contain no ddos, and all `config/yang`+`config` schema-loading tests pass with the new leaves. Full-verify at closure will be scoped to changed packages per `ai/rules/git-safety.md` (Known-Red Full Verify).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| BPS unit: operator leaf in bits/sec, convert internally | Express the leaf in bytes/sec to match ze's native `RxBps` | **(user-approved)** Operators think in Mbps/Gbps (bits); ze's `RxBps` is bytes/sec (`iface/rate.go:218`). Leaf `bps-floor` in bits/sec (default 50000000) with an ×8 conversion at compare time keeps the config human-standard and unambiguous, at the cost of one multiply. (Pre-existing `PeakRxBps` stays bytes/sec, unchanged.) |
| BPS trigger behind `bps-trigger-enable` (default true) | Always-on guarded by floor+multiplier only; or default-off | **(user-approved)** BPS detection active by default (catches amplification out of the box) but with a dedicated boolean escape hatch so an FP-prone site can disable just the bandwidth path without turning off ddos-detect. |
| Confidence-only into observe/flowtriq (strict scope) | Also carry refined Family/Severity through the new Characterized subscription | **(user-approved)** Keep the diff minimal and within the ported ftagent scope; the pre-existing coarse-Family gap in observe/flowtriq is left as a documented Known Limitation, not fixed here. |
| BPS baseline as a parallel series inside `baseline` | Generalise `baseline` to a metric-agnostic type | Parallel series is the minimum change, mirrors ftagent, reuses recalc cadence; generalisation is speculative for two series |
| Confidence on `AttackCharacterized` only | Also on `AttackDetected` | Detected has no classification signals; confidence is a characterization output |
| observe/flowtriq add a `Characterized` subscription | Move confidence onto Detected/Cleared | Confidence exists only at characterize time; matches where the signals are |

## Known Limitations
- Confidence is computed at attack start, so attack duration (a ftagent signal) is not included.
- BPS trigger uses the busiest-interface aggregate, not per-victim BPS (matches the existing PPS trigger granularity).
- Persisting a single rolling window; no per-hour/time-of-day BPS baseline (ftagent has hourly PPS; ze does not, and this spec does not add it).
- Strict confidence-only scope (user-approved): `observe`/`flowtriq` gain a `Characterized` subscription solely to carry `confidence`; they continue to report the coarse generic-flood `Family` from `AttackDetected`. Fixing that pre-existing coarse-Family gap is deliberately excluded here.
- **local confidence-min gates only narrowing (review #3):** `ddos-local` installs the coarse drop on `AttackDetected`, which carries no confidence, so `confidence-min` cannot suppress the initial on-host drop; it only gates the in-place narrowing on `AttackCharacterized`. Operators wanting confidence to suppress local mitigation entirely cannot get that today (the fast signal has no confidence). `ddos-flowspec` is unaffected: its precise announce is the `onCharacterized` path, so its `confidence-min` genuinely gates upstream announcements (the blackhole-fallback fast path stays ungated by design).
- **observe/flowtriq confidence attaches only when the victim prefix matches (review #4):** confidence is matched onto the incident by `Target.DstPrefix`. When `trafficusage` resolved no victim at `AttackDetected` (empty prefix) and the victim was derived from flows at `AttackCharacterized`, the prefixes differ and confidence is not recorded (incident keeps 0). Graceful (no crash), same match semantics as the existing `finalize`; fixing it is excluded from this spec.

## RFC Documentation
N/A - no wire protocol change (ddosevent is an internal value contract).

## Implementation Summary
### What Was Implemented
- **Phase 1:** parallel BPS baseline (second `baseline` instance), BPS trigger clause in `applyTick`, `bps-trigger-enable`/`bps-threshold-multiplier`/`bps-floor` config + YANG, ×8 bits→bytes conversion at `newBaseline`, `ze_ddos_detect_bps_trigger_total` metric.
- **Phase 2:** `persist.go` (versioned JSON, atomic write serialized by `saveMu`), `baseline.snapshot()/restore()` with version + min-sample + sanity guards, restore in `register.go`, save in `Stop()` + periodic (300 ticks).
- **Phase 3:** `GradeConfidence` + `AttackCharacterized.Confidence`; computed in `characterize.go`; observe stores + `show ddos incidents`; flowtriq reports; `local`/`flowspec` `confidence-min` gate + YANG.
- 70 tests across the touched packages; all ddos + ddosevent pass with `-race`; `make ze-lint-changed` 0 issues; `make ze-doc-test` PASSED.
### Bugs Found/Fixed
- Review #1: concurrent `saveBaselines` temp-file collision → serialized with `saveMu`.
- Review #2: `newDetector` writing to the real config dir from unit tests → constructor made I/O-free, `statePath` set in `register.go`.
### Documentation Updates
- `docs/guide/ddos-mitigation.md`: BPS-trigger + `confidence-min` config rows and BPS/persistence/confidence behavior notes with `<!-- source: -->` anchors. `ai/DOCS-TO-CODE.md` regenerated. `make ze-doc-test` PASSED.
### Deviations from Plan
- BPS reuses the `baseline` type (second instance) instead of extending it, so the planned `TestBaseline_Bps*` unit tests folded into the existing baseline tests + detector-level trigger tests.
- Peak-bps tracking made independent of peak-pps (improvement for BPS attacks).
- Persistence restore refactored out of `newDetector` into an explicit `restore()` (testability + review-#2 fix).

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Amplification (low-PPS/high-BPS) is detected | functional test | `test-ddos-bps-amplification` |
| Restart does not blind the detector | unit test | `TestBaselineRestore_SkipsWarmup` |
| Confidence distinguishes real attacks | unit + functional | `TestGradeConfidence_Bounds`, `test-ddos-incident-confidence` |

## Review Gate

### Run 1 (initial) — /code-review high (8 angles)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Concurrent saveBaseline calls collide on a shared `path+".tmp"` temp file (periodic save vs on-stop under slow disk) | persist.go:59 | fixed: `saveMu` serializes saves in `saveBaseline` (detector.go) |
| 2 | ISSUE | `newDetector` set `statePath` to the real config dir, so unit tests calling `Stop()` wrote the baseline outside their tempdir | detector.go newDetector | fixed: `newDetector` leaves `statePath` empty; `register.go` sets it before `restore()`; tests inject it. `TestStopFencesCharacterization` now no-ops the save |
| 3 | NOTE | local `confidence-min` gates only narrowing; the coarse drop already installed by `onDetected` (no confidence) is not gated | local/responder.go | acknowledged: recorded in Known Limitations |
| 4 | NOTE | observe.characterize misses the flow-derived-victim case (Detected had empty target) | observe/store.go | acknowledged: recorded in Known Limitations (same match semantics as `finalize`) |
| 5 | NOTE | `baselineState.P99Cache` is persisted but recomputed by `restore()`'s `recalc()` | baseline.go | acknowledged: harmless redundancy; kept for a self-describing snapshot |

### Fixes applied
- **#1:** added `saveMu sync.Mutex` to the detector; `saveBaseline` snapshots under `d.mu`, then holds `saveMu` around the file I/O only (never with `d.mu`), so the periodic save and the on-stop save never overlap on the temp file.
- **#2:** `newDetector` is now I/O-free (no `statePath`); `register.go` sets `det.statePath = baselineStatePath()` before `det.restore()` at both config hooks; persistence tests set `d.statePath` explicitly. Eliminates real-filesystem writes from unit tests.

### Run 2 (closure re-review) — independent adversarial subagent (correctness / concurrency / security)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | BPS trigger only saw the bandwidth of the top-PPS interface, so an amplification flood (high-BPS/low-PPS) on any non-top-PPS interface never reached the BPS baseline/trigger — defeats the feature on multi-interface boxes | detector.go `onRates`/`onRate` | fixed: track peak PPS and peak BPS (+ their interfaces) independently; attribute `attackIface` to the BPS interface on BPS-only detections |
| 2 | ISSUE | Confidence exact-match could attach to a stale resolved-target incident (AttackCleared's empty target never finalizes a resolved-target incident, so it lingers Active) instead of the current attack | observe/store.go `characterize` | fixed: single newest-first scan scoped to the interface, matching exact-or-empty target |

### Fixes applied (Run 2)
- **#1:** `onRates`/`onRate` compute `maxBps`/`maxBpsIface` in an independent branch (no longer bound to the max-PPS interface); `applyTick` attributes `attackIface` to `maxBpsIface` when only the BPS path fired. Regression test `TestApplyTick_BpsTriggerFiresOnNonTopPpsInterface`.
- **#2:** `characterize` replaced with a newest-first, interface-scoped scan (exact-target OR empty). Regression test `TestStoreCharacterizePrefersCurrentIncidentOverStale`. Residual (documented in store.go, observability-only): two concurrent same-interface attacks, one resolved + one empty, resolve newest-wins.

### Run 3 (verification) — independent adversarial subagent over the two Run 2 fixes
Result: **RUN3 CLEAN: 0 BLOCKER, 0 ISSUE.** Both fixes CONFIRMED CORRECT — attribution correct for PPS / BPS-only / mixed; `peakRxBps` now a genuine running max (strict improvement); `attackIface` per-tick reassignment proven observability-neutral against every `Ongoing`/`Cleared` consumer; interface-scoping safe because `AttackDetected` and `AttackCharacterized` always share the emit-site `ifaceName` snapshot. Passed under `-race`.

### Final status
- Run 1: 2 ISSUEs fixed, 3 NOTEs documented as Known Limitations.
- Run 2 (closure): 2 ISSUEs found and fixed, each with a regression test.
- Run 3: 0 BLOCKER, 0 ISSUE. **Review Gate satisfied.**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ddos/detect/persist.go` | Yes | git diff shows new file (78L) |
| `internal/plugins/ddos/detect/persist_test.go` | Yes | git diff shows new file |
| all modified files | Yes | `git diff --stat` lists 26 modified + 2 new under `internal/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/2/3/3b | BPS trigger fires/inert/gated | `TestApplyTick_BpsTriggerFiresBelowPpsThreshold`, `_BpsTriggerDisabled`, `_BpsBelowFloorInert` PASS |
| AC-4 | BPS poisoning guard | `baselineBps.Add(maxBps, attacking||above)` at detector.go; reuses `baseline` (TestBaselineExcludesAttackSamples) |
| AC-5/6 | restore/reject | `TestDetectorRestoresBaselineFromDisk`, `TestBaselineRestore_RejectsBadState`, `TestLoadBaselines_RejectsVersion/MissingAndCorrupt` PASS |
| AC-7/8 | confidence bounds/spread | `TestGradeConfidence` PASS (100 clear / 31 ambiguous / clamp) |
| AC-9 | observe stores confidence | `TestStoreCharacterizeSetsConfidence` PASS |
| AC-10 | flowtriq reports confidence | `TestClientResolveIncident` asserts `confidence:88` PASS |
| AC-11/12/13 | responder gate + default 0 + ungated fast path | `TestLocalConfidenceGate`, `TestFlowspecConfidenceGate` PASS; flowspec fast path gate at `onDetected` unchanged |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| unit/integration only (no new .ci this pass) | `detector_test.go`, `characterize_test.go`, `store_test.go`, `responder_test.go`, `client_test.go` | Yes — each drives the entry point (applyTick / event emit / subscription / responder handler) to the feature code |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `register.go:162/192` newDetector, `:160/190/213/220` Stop; restore/save wired |
| A-2 | confirmed | confidence computed from emit-site signals (characterize.go), no new query |
| A-3 | confirmed | observe+flowtriq each gained a `Characterized` subscription |
| A-4 | confirmed | YANG leaves load via `go:embed`; config/yang tests PASS; no glue edit |
| A-5 | confirmed | `RxBps` is bytes/s (`iface/rate.go:218`); ×8 handled by `BpsFloor/8.0` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| ddos-mitigation.md config rows + behavior notes | `<!-- source: -->` anchors resolve | `make ze-doc-test` PASSED (all references valid) |
| Discovery index | `ai/DOCS-TO-CODE.md` regenerated | `make ze-discovery-index` (only that file changed) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ddos-detect-enhancements.md` only
