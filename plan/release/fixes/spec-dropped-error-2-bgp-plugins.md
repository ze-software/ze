# Spec: dropped-error-2-bgp-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 63 errors in the BGP plugins (BMP, graceful restart, the NLRI decoders, route refresh, IRR, adj-rib-in) are caught and logged only below WARN: a malformed GR capability is ignored, a BMP collector write fails, a decode request answers nothing, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/bgp-plugins.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/bgp/plugins/`.
At HEAD on 2026-11-02: **63 instances in 23 files** (BMP 21, GR 12, route
refresh 7, NLRI decoders 20, IRR 2, adj-rib-in 2).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. A decode request (`handleDecodeNLRI`, `RunDecode`, `RunCLIDecodeWithCode`) answers its caller with the error rather than an empty result |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`); on a per-route or per-message path, count it and WARN once per peer and condition |
| 3 | Going on is right and nobody can act (closing a collector connection already lost) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/bgp-plugins.md`
  → Constraint: every plugin can run forked; a WARN goes to the plugin's stderr logger, never to `internal/core/report`, which is process-local
  → Constraint: stdout carries the protocol; a plugin logs to stderr only
  → Constraint: a forwarder never drops a cached UPDATE (cumulative ack); no answer here may skip an ack to report an error
- [ ] `docs/architecture/plugin/plugin-system.md` - the process boundary
  → Constraint: an error returned to the engine travels as an RPC error; its text reaches the operator through the dispatcher
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- The NLRI decode instances answer an operator's or a plugin's decode request; an empty answer there is a wrong answer, so they are answer 1.
- The GR OPEN-capability instances are per OPEN, a peer event: WARN with the peer and the capability code.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/bgp/plugins/gr/gr.go` - an undecodable capability 64 hex string is logged at Debug and the capability skipped

**Behavior to preserve:**
- Route selection, forwarding, ack order and GR state, except where answer 1 finds the drop hid a defect, which then owes its own test.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 63 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Plugin metrics | `docs/plugin-development/metrics.md`; BMP's counters (`docs/guide/bmp.md`) | Per-plugin counters | Extends: answer 2 on per-message paths counts here |
| Plugin stderr logger | `slogutil.Logger` per plugin | The plugin's log, internal or forked | Applies: the WARN goes here |
| Decode RPC error answer | `OnDecodeNLRI` / the decode RPC's error reply (`docs/architecture/api/process-protocol.md`) | Carries a failure back to the caller | Extends: answer 1 for the decoders |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside a BGP plugin: a collector write, a capability decode, an NLRI decode, a route-refresh decode, a state write.

### Transformation Path
1. The branch catches the error.
2. It returns it (decode answers carry it to the caller), counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ engine | RPC error answer | No |
| Plugin ↔ operator | Plugin stderr log at WARN, metrics | No |

### Integration Points
- The plugin loggers, metric families and decode RPC answers named above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors leave through the RPC answer or the plugin's log |
| No unintended coupling (components stay isolated) | Yes | No import of `internal/core/report` from a forkable plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing paths |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-message paths count |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 63 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to `internal/component/bgp/plugins/` before the first edit | unvalidated |
| A-2 | Every decode caller handles an error answer | The decode RPC carries errors | A caller that reads any error as fatal | Read each decode caller before answer 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per BMP route-monitoring message floods under a lost collector | BMP `.ci` log grows per UPDATE | WARN once per collector and condition, count per message |
| R-2 | A decode that returned an empty result now returns an error a `.ci` did not expect | `test/decode` red | That `.ci` asserted the wrong answer; the change goes through the `ze-test-judge` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a decode caller that now sees an error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Any spec on BMP, GR or the NLRI codecs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to `internal/component/bgp/plugins/`: zero findings of the shape |
| An OPEN whose GR capability hex does not decode | → | `grPlugin.handleOpenEvent` | `TestGROpenUndecodableCapabilityIsWarned` (gr package) |
| A decode request with a malformed VPN NLRI | → | `handleDecodeNLRI` (vpn) | `TestVPNDecodeAnswersTheError` (vpn package) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to `internal/component/bgp/plugins/` | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | Each decoder's malformed input | The decode answer carries the error to its caller |
| AC-3 | Each answer-1 instance | A unit test where the caller observes the error |
| AC-4 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-5 | Each annotation | Names a guard the file can see |
| AC-6 | Plugin package tests and the `test/plugin` and `test/decode` `.ci` the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGROpenUndecodableCapabilityIsWarned` | `internal/component/bgp/plugins/gr/gr_test.go` | AC-4 | |
| `TestVPNDecodeAnswersTheError` | `internal/component/bgp/plugins/nlri/vpn/vpn_test.go` | AC-2; the same shape for evpn, ls, flowspec, labeled, mup, mvpn, rtc, vpls | |
| One test per other answer-1 instance | The package of the instance | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/plugin` and `test/decode` `.ci` the change reaches | `test/` | Unchanged behavior; the error reaches the operator where a `.ci` drives it | |

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib_complex.go` - 2: `AdjRIBInManager.installComplexNLRIs.func` 52; `AdjRIBInManager.removeComplexNLRIs.func` 106
- `internal/component/bgp/plugins/bmp/bmp.go` - 6: `closeLog` 668; `BMPPlugin.handleSession.func` 709; `BMPPlugin.handleSession` 752; `BMPPlugin.processTermination` 865; `BMPPlugin.processPeerDown` 1014; `BMPPlugin.processRouteMonitoring` 1032
- `internal/component/bgp/plugins/bmp/bmp_events.go` - 6: `BMPPlugin.bounceMonitoredPeers` 454; `BMPPlugin.handleSenderState` 498,522; `BMPPlugin.handleSenderMirror` 548,555; `BMPPlugin.handleSenderUpdate` 619
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - 5: `BMPPlugin.handleBestChange` 761; `BMPPlugin.ensureLocRIBPeerUp` 831; `BMPPlugin.primeLocRIBPeerUp` 884; `BMPPlugin.sendLocRIBEndOfRIB` 902; `BMPPlugin.sendLocRIBPeerDown` 999
- `internal/component/bgp/plugins/bmp/receiver.go` - 1: `BMPPlugin.stopListeners` 124
- `internal/component/bgp/plugins/bmp/sender.go` - 1: `senderSession.writeTerminationLocked` 466
- `internal/component/bgp/plugins/bmp/sender_drain.go` - 1: `senderSession.drainLoop` 135
- `internal/component/bgp/plugins/bmp/statistics.go` - 1: `BMPPlugin.sendStatisticsReports` 145
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - 1: `irrPlugin.trackVerdict` 567
- `internal/component/bgp/plugins/filter_irr/readmit.go` - 1: `readmitter.readmit` 354
- `internal/component/bgp/plugins/gr/gr.go` - 9: `grPlugin.handleStructuredOpen` 335,349; `grPlugin.extractGRCaps` 400,416; `grPlugin.handleOpenEvent` 591,596,612,617; `writeOut` 967
- `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` - 3: `grPlugin.handleStructuredSentOpen` 83; `grPlugin.handleSentOpenEvent` 143; `addLLGRFamilies` 155
- `internal/component/bgp/plugins/nlri/evpn/plugin.go` - 4: `runEVPNDecode.func` 404; `handleDecodeNLRI` 489,500; `decodeEVPNNLRI` 558
- `internal/component/bgp/plugins/nlri/flowspec/plugin_decode.go` - 2: `decodeFlowSpecNLRI` 50,60
- `internal/component/bgp/plugins/nlri/labeled/labeled.go` - 1: `RunDecode.func` 149
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - 4: `runBGPLSDecode.func` 189; `handleDecodeNLRI` 270,287; `decodeBGPLSNLRI` 333
- `internal/component/bgp/plugins/nlri/mup/mup.go` - 1: `RunDecode.func` 174
- `internal/component/bgp/plugins/nlri/mvpn/mvpn.go` - 1: `RunDecode.func` 176
- `internal/component/bgp/plugins/nlri/rtc/rtc.go` - 1: `RunDecode.func` 150
- `internal/component/bgp/plugins/nlri/vpls/vpls.go` - 1: `RunDecode.func` 140
- `internal/component/bgp/plugins/nlri/vpn/vpn.go` - 4: `runVPNDecode.func` 275; `handleDecodeNLRI` 361,379; `decodeVPNNLRI` 422
- `internal/component/bgp/plugins/route_refresh/register.go` - 1: `init.func.func` 50
- `internal/component/bgp/plugins/route_refresh/route_refresh.go` - 6: `RunRouteRefreshPlugin.func` 68; `writeOut` 98; `RunCLIDecodeWithCode` 193,200,210,215

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC added; the decode RPC's error answer already exists |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing plugin families (BMP); name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/guide/bmp.md` or `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/bgp/filter-irr.md`, `docs/architecture/core-design.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/wire/nlri-evpn.md`, `docs/architecture/wire/nlri-flowspec.md`, `docs/architecture/wire/nlri.md`, `docs/guide/bmp.md`, `docs/guide/graceful-restart.md`, `rfc/short/rfc9069.md`. A log level changes none of them; a decode error answer is described where the page states what the decoder returns, checked with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to `internal/component/bgp/plugins/`; write the GR and VPN tests red.
2. **Phase: Decoders** -- answer 1 for the NLRI and route-refresh decode paths.
3. **Phase: GR, BMP, IRR, adj-rib-in** -- classify each instance (reason in Design Insights), tests first, then the change; annotations last.
4. **Phase: Prove** -- AC-1 to AC-6.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No ack skipped, no cached UPDATE dropped to report an error |
| Rule: user-facing-errors | Each WARN names the peer or collector, the operation, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label bgp-plugins command go test -tags ze_bgp,ze_bmp ./internal/component/bgp/plugins/...` |
| Functional | `./le test bgp plugin <name>` and `./le test bgp decode <name>` for each `.ci` the change reaches |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A decode error that echoes unbounded peer bytes into a log line |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decode paths return the error in their answer | WARN and answer empty | An empty decode is a wrong answer the caller cannot tell from "nothing there" |
| The umbrella's three answers, in order, elsewhere | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` only, in the same `./le commit create` script
