# Spec: dropped-error-1-bgp-engine

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | 57 errors in the BGP engine (reactor, `ze bgp decode --plugin`, the BGP server) are caught and logged only below WARN: a next hop that fails to resolve drops a static route, a failed collision handoff or teardown write leaves no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/bgp-engine.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/bgp/` outside
`internal/component/bgp/plugins/`, and under `internal/core/bgp/`. At HEAD on
2026-11-02: **57 instances in 21 files** (reactor 42, `bgp/cli` 14,
`bgp/server` 1). Files to Modify lists each file with its functions and lines.

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`); on a per-route or per-message path, count it in the reactor's existing metrics and WARN once per condition |
| 3 | Going on is right and nobody can act (a close at teardown, a write to a connection already lost) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

Known overlaps, resolved where they are owned:

| Instance | Owner of the defect | What this child does |
|----------|---------------------|----------------------|
| `peer_static_wire.go` `sendStaticRoutes`, `sendStaticRoutesGrouped` | `plan/release/fixes/spec-bgp-one-off-defects.md` (silent-fall-through row 2026-10-31, static-route send error at DEBUG) | If that spec has landed, the instance is gone; otherwise this child resolves the log level only and leaves the send-error classification to that spec |
| `bgp/cli/decode_plugin.go` (14) | spec-command-surface-parity, closed (unwired-feature row 2026-11-02, `--plugin` parsed and ignored) | That spec's change to `decode_plugin.go` landed in 442e4e810e; resolve what remains: an operator ran this command, so a failure is returned to them (answer 1) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/bgp-engine.md`
  → Constraint: the engine holds no policy; a failure that changes a route decision belongs to the plugin that owns it, the engine reports it
  → Constraint: received bytes are borrowed; a WARN that formats a message body copies nothing out of the read buffer it does not already own
- [ ] `ai/digests/bgp-reactor.md` - peer lifecycle, session read loop, delivery to plugins
  → Constraint: a write error on a session is followed by its teardown; a WARN there must not duplicate the teardown's own report
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: a WARN names the peer and the operation in operator terms, then the fix
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer", "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- Most reactor instances are per-peer events (teardown, collision, BFD), where a WARN per event is right; `notifyMessageReceiver` and the static-route loop are per message or per route, where counting is right.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/bgp/reactor/peer_static_wire.go` - a next hop that fails to resolve is logged at Debug and the route is skipped
- [ ] `internal/component/bgp/cli/decode_plugin.go` - plugin start and write failures logged at Debug, the decode answers nothing

**Behavior to preserve:**
- Session behavior on the wire. No instance changes what is sent to a peer unless answer 1 finds the drop was a defect, which then owes its own test.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 57 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Reactor metrics | `internal/component/bgp/reactor` `prefixMetrics` and the reactor's Prometheus families (`docs/plugin-development/metrics.md`) | Per-peer counters | Extends: answer 2 on per-message paths counts here, no new family |
| Operator warnings | `internal/core/report` (`show warnings`, `docs/architecture/core-design.md` section 15) | Lasting conditions an operator lists | Applies where a failure is a state (BFD client lost), not an event |
| Subsystem logger | `routesLogger()` and the reactor loggers (`slogutil.Logger`) | Per-subsystem levels | Applies: the WARN goes through the same logger |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside the reactor, the decode CLI or the BGP server: socket write, BFD client, event emit, plugin process.

### Transformation Path
1. The branch catches the error.
2. It returns it, counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ operator | Log at WARN, metric, `show warnings` | No |
| CLI ↔ operator | Returned error printed by `ze bgp decode` | No |

### Integration Points
- The existing loggers and metric families named above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | No new path |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing loggers and metrics |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-message paths count, they do not format bodies |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 57 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | Teardown-path drops are answer 3 | A write error on a lost connection has nothing left to undo | A drop that hides a peer left half up | Read each teardown site's caller before annotating | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per UPDATE floods the log under a broken peer | A plugin `.ci` log grows by thousands of lines | Count and WARN once per peer and condition |
| R-2 | Returning an error where the caller treats any error as a teardown | A `.ci` peer session drops | Keep the caller's existing error handling in view; answer 1 only where the caller acts correctly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a session torn down by a newly returned error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-bgp-one-off-defects`, `spec-command-surface-parity` (overlaps above) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to `internal/component/bgp/` minus `plugins/`: zero findings of the shape |
| A next hop that fails to resolve for a static route | → | `Peer.sendStaticRoutes` | `TestSendStaticRoutesReportsUnresolvedNextHop` (reactor package) asserts the WARN or counter |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | Each instance resolved by answer 1 | A unit test where the caller observes the error |
| AC-3 | Each file with an answer-2 WARN | A unit test captures one WARN line at the default level and asserts it names what failed, why it matters and the fix |
| AC-4 | Each annotation | Names a guard the file can see; the report's `annotated` list holds it |
| AC-5 | BGP package and functional suites the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendStaticRoutesReportsUnresolvedNextHop` | `internal/component/bgp/reactor/peer_static_wire_test.go` | AC-3 for the per-route path | |
| One test per answer-1 instance, named for the failure the caller now sees | The package of the instance | AC-2 | |
| `TestRefuseByDefaultReportListsTree` | `internal/le/arch/refusebydefault/refusebydefault_test.go` | Detector unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| The `test/plugin`, `test/decode` and `test/encode` `.ci` that reach changed files | `test/` | Unchanged behavior, plus the WARN where a `.ci` drives the failure | |

## Files to Modify
- `internal/component/bgp/cli/decode_plugin.go` - 14: `invokePluginDecodeRequest` 243,246; `invokePluginPath` 349,352,362,369; `invokePluginSubprocess` 427,430; `invokePluginInternal.func` 497,502,505; `invokePluginInternal` 515,520,526
- `internal/component/bgp/reactor/aigp_readvertise.go` - 1: `Reactor.replayAIGP` 511
- `internal/component/bgp/reactor/filter_ordered.go` - 1: `Reactor.runEgressPolicyChainASN4` 393
- `internal/component/bgp/reactor/listener.go` - 3: `Listener.acceptLoop.func` 172; `Listener.acceptLoop` 198; `Listener.safeHandle.func` 211
- `internal/component/bgp/reactor/operation.go` - 1: `reactorAPIAdapter.emitOperationListenerReady` 200
- `internal/component/bgp/reactor/peer.go` - 1: `Peer.teardownOf` 2181
- `internal/component/bgp/reactor/peer_admin.go` - 1: `rejectAdminDownConnection` 116
- `internal/component/bgp/reactor/peer_bfd.go` - 3: `Peer.openBFDSession` 332; `Peer.deliverBFDEvent` 663; `Peer.releaseBFDClient` 728
- `internal/component/bgp/reactor/peer_connection.go` - 1: `Peer.finishCollisionHandoff` 195
- `internal/component/bgp/reactor/peer_initial_sync.go` - 9: `Peer.sendInitialRoutes` 259; `Peer.drainAndCloseQueueGate` 514,537,564; `Peer.runQueuedTeardown` 656; `Peer.sendPluginRoutesVia` 881; `Peer.sendPluginRouteGroup.func` 924; `Peer.sendPluginRouteGroup` 954; `Peer.sendDefaultOriginateRoutes` 1094
- `internal/component/bgp/reactor/peer_static_wire.go` - 2: `Peer.sendStaticRoutes` 66; `Peer.sendStaticRoutesGrouped` 143
- `internal/component/bgp/reactor/reactor.go` - 2: `Reactor.emitCongestionEventBus` 968; `Reactor.emitUpdateNotificationEvent` 988
- `internal/component/bgp/reactor/reactor_api_forward_batch.go` - 2: `reactorAPIAdapter.ForwardUpdatesDirect` 190; `reactorAPIAdapter.ackBatch` 219
- `internal/component/bgp/reactor/reactor_connection.go` - 2: `closeConnQuietly` 270; `Reactor.rejectConnectionCollisionWithSettings` 277
- `internal/component/bgp/reactor/reactor_iface.go` - 1: `Reactor.handleAddressAdded` 327
- `internal/component/bgp/reactor/reactor_notify.go` - 2: `Reactor.notifyMessageReceiver` 444,638
- `internal/component/bgp/reactor/session.go` - 2: `NewSession.func` 796,851
- `internal/component/bgp/reactor/session_bfd_strict.go` - 4: `Session.raiseBFDHoldTimerExpired` 106; `Session.releaseBFDHoldDown` 124,130; `Session.raiseBFDStrictConfigChanged` 223
- `internal/component/bgp/reactor/session_connection.go` - 3: `tuneTCPConnectionForSettings.func` 315,318; `Session.ceaseAfterPanic` 636
- `internal/component/bgp/reactor/session_validation.go` - 1: `Session.publishBase` 468
- `internal/component/bgp/server/events.go` - 1: `onEORReceived` 941

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- `internal/component/bgp/reactor/peer_static_wire_test.go` additions, or the package's existing test file for it.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command added; `ze bgp decode --plugin` errors reach the operator |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing reactor families; name each counter used here at implementation |
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
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/api/architecture.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/config/apply-ordering.md`, `docs/architecture/core-design.md`, `docs/architecture/route-selection.md`, `rfc/short/rfc5882.md`. A log level or a returned error changes none of them unless answer 1 changes behavior they describe; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-1-bgp-engine.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; record the population.
2. **Phase: Classify** -- for each instance, read its caller and write the answer (1, 2 or 3) with a one-line reason into Design Insights.
3. **Phase: Resolve** -- per file: tests first for answers 1 and 2, then the change; annotations last.
4. **Phase: Prove** -- AC-1 to AC-5.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No answer 3 where the caller would act; no WARN per message |
| Rule: user-facing-errors | Each WARN names the peer, the operation, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label bgp command go test -tags ze_bgp ./internal/component/bgp/reactor/ ./internal/component/bgp/cli/ ./internal/component/bgp/server/` |
| Functional | `./le test bgp plugin <name>` for each `.ci` the change reaches, from `./le repo changed scope print both` |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN that prints a TCP MD5 key or a peer password |

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
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct |
| Overlapping instances wait for their owning spec | Fix them here too | Two specs editing one line collide; the owner holds the behavior fix |

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A unless answer 1 changes wire behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-1-bgp-engine.md` only, in the same `./le commit create` script
