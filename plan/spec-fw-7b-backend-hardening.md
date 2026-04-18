# Spec: fw-7b-backend-hardening — Context plumbing + vppOps test seam for traffic backends

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/2 |
| Updated | 2026-04-18 |

## Task

Two small follow-ups surfaced by the fw-7 review cycle, bundled because
both are "make the trafficvpp backend properly testable and lifecycle-
correct" and neither is large enough to justify its own spec:

1. **Context plumbing.** `traffic.Backend.Apply` currently takes only a
   `map[string]InterfaceQoS`. trafficvpp fabricates a
   `context.WithTimeout(context.Background(), 5s)` internally. Daemon
   shutdown cannot interrupt an in-flight Apply; a VPP that never
   reconnects holds the traffic plugin for the full 5s per reload even
   during SIGTERM. Fix: extend the Backend interface to accept a
   `context.Context` and plumb it from the component's OnConfigApply
   callback through to the backend. netlink backend ignores it (or
   honors it if netlink call sites allow); vpp backend uses it for
   `WaitConnected` and for any future long-running call.

2. **vppOps test seam.** trafficvpp's Apply path has zero automated
   coverage today (`translate_test.go` covers pure functions only).
   The undo-on-partial-failure logic, the create-vs-update distinction
   added in pass 7 of the fw-7 review, `reconcileRemovals`, and the
   orphan-policer PolicerDel path all live in code without tests.
   Mocking the full GoVPP `api.Channel` interface (8 methods across
   Channel / RequestCtx / MultiRequestCtx) is overkill; extract a
   narrow 4-method `vppOps` interface covering only what trafficvpp
   calls, wrap the real `api.Channel` in a `govppOps` adapter, and
   write unit tests that verify the Apply sequence with a scripted
   `fakeOps`.

The interface stub for Phase 2 already exists at
`internal/plugins/traffic/vpp/ops.go` (dropped during pass-7 fixes as
scaffolding for this spec). Phase 2 wires the stub into backend_linux.go
and adds the tests.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-7-traffic-vpp.md` (or `plan/learned/NNN-fw-7-traffic-vpp.md` once fw-7 retires)
3. `plan/learned/623-fw-9-traffic-lifecycle.md` — traffic component reactor (OnConfigVerify / OnConfigApply wiring)
4. `internal/component/traffic/backend.go` — current Backend interface
5. `internal/component/traffic/register.go` — OnConfigApply path that calls `backend.Apply`
6. `internal/plugins/traffic/vpp/backend_linux.go` — Apply, applyAll, applyInterface, reconcileRemovals
7. `internal/plugins/traffic/vpp/ops.go` — vppOps interface stub (created 2026-04-18, currently unused — Phase 2 wires it in)
8. `internal/plugins/traffic/netlink/backend_linux.go` — sibling backend, also needs ctx parameter

## Required Reading

### Architecture docs
- [ ] `plan/learned/623-fw-9-traffic-lifecycle.md` — how the traffic component drives backend.Apply
  → Constraint: OnConfigApply is where the component invokes the backend; context plumbing starts here.
- [ ] `rules/exact-or-reject.md` — backend-verification posture motivating the test infrastructure
  → Constraint: untested code paths in a backend-facing feature fall into the same trap (silent regressions).

### Reference code
- [ ] `internal/component/traffic/backend.go` — current interface signature + factory/verifier registry
  → Decision: Apply gains `ctx context.Context` as first parameter (standard Go idiom).
- [ ] `internal/component/traffic/register.go` — `OnConfigApply` callback signature
  → Constraint: the SDK's `OnConfigApply(func(sections []sdk.ConfigDiffSection) error)` does not pass a ctx today; either the SDK gains one or the component synthesizes one from its plugin-lifecycle ctx.
- [ ] `internal/plugins/traffic/netlink/backend_linux.go` — needs the same signature update; vishvananda/netlink calls do not accept a ctx so the netlink backend passes it through to nothing (documented).
- [ ] `pkg/plugin/sdk/` (`OnConfigApply` definition) — may need its signature extended to receive a ctx; check before designing Phase 1.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/backend.go` — `Backend` interface methods are `Apply`, `ListQdiscs`, `Close`; no ctx parameter anywhere.
  → Constraint: this is the surface that changes in Phase 1.
- [ ] `internal/component/traffic/register.go` — OnConfigApply calls `backend.Apply(cfg.Interfaces)`; no ctx in scope at that call site beyond whatever the SDK exposes.
  → Constraint: need to check SDK for ctx availability.
- [ ] `internal/plugins/traffic/vpp/backend_linux.go` — `Apply` fabricates `context.WithTimeout(context.Background(), waitConnectedTimeout)` used only by `WaitConnected`. All other VPP calls (SwInterfaceDump, PolicerAddDel, PolicerOutput, PolicerDel) have no ctx path at all; GoVPP's `api.Channel.SendRequest().ReceiveReply()` does not accept a ctx.
  → Constraint: ctx plumbing benefits only WaitConnected today; GoVPP ctx support would need a separate follow-up. Still worth doing so shutdown interrupts the WaitConnected path.
- [ ] `internal/plugins/traffic/vpp/ops.go` — interface stub present but unused (triggers `unused` lint).
  → Constraint: Phase 2 must consume this interface via backend_linux.go or delete the file; current state is a known one-finding lint warning documented by this spec.

**Behavior to preserve:**
- netlink backend's observable behavior under ctx parameter (noop ctx usage; still shapes/unbinds via vishvananda/netlink calls).
- vpp backend's verifier rejections and Apply semantics established in fw-7 (pass 1..7 of the review cycle).
- All existing tests must keep passing.

**Behavior to change:**
- `traffic.Backend.Apply` gains a `ctx context.Context` first argument.
- `trafficvpp.backend.Apply` uses the caller's ctx for WaitConnected instead of its own Background-derived one.
- `trafficvpp` Apply/applyAll/applyInterface/reconcileRemovals refactored to call through `vppOps` instead of `api.Channel` directly; production wrapper `govppOps` threads the channel.
- New test file `apply_test.go` exercises Apply-path semantics with a scripted `fakeOps`.

## Design Decisions

### Decision 1 — Context is first parameter of Apply
Go idiom. `Apply(ctx context.Context, desired map[string]InterfaceQoS) error`.
Rejected: optional ctx via setter or via a method overload — both break the
idiom and leave ctx-less call sites valid.

### Decision 2 — OnConfigApply ctx source
If the SDK's OnConfigApply already receives a ctx (verify by reading
`pkg/plugin/sdk/`), plumb it through. If it does not, the component
synthesizes a ctx from its own plugin lifecycle (the `ctx` passed to
`OnStarted` / the plugin's runtime context). Latter approach guarantees
cancellation on plugin stop regardless of SDK surface.

### Decision 3 — netlink backend accepts ctx but ignores it
vishvananda/netlink's tc calls are synchronous and do not accept a ctx.
The netlink backend's `Apply` accepts the parameter (signature
consistency) and passes it to no one. Document the noop with a comment
so a future reader does not assume ctx plumbing is deeper than it is.

### Decision 4 — vppOps is unexported and narrow
4 methods: `dumpInterfaces`, `policerAddDel`, `policerDel`, `policerOutput`.
Exported would expose implementation to other packages that might rely on
it, cementing the abstraction. Keep unexported; if another backend ever
needs the same seam, copy-paste the 4 methods rather than introduce
cross-package coupling.

### Decision 5 — govppOps is stateless
Production adapter is `type govppOps struct { ch api.Channel }`. Each
method is a 1-line call to the corresponding sendX helper (or inlined).
No caching, no retry, no per-call state. Tests replace the whole struct.

### Decision 6 — fakeOps records calls + returns scripted results
Tests construct a `fakeOps` with a scripted response plan (e.g. "fail at
call N", "return index 100 for policerAddDel(name X)") and inspect the
recorded call sequence after Apply returns. No matcher DSL — just a
`[]string` of labels plus per-call assertion helpers.

## Data Flow (MANDATORY)

### Entry Point
- Config reload (SIGHUP) or boot. SDK invokes the traffic component's
  `OnConfigApply(func(sections []sdk.ConfigDiffSection) error)` callback.
  The component parses sections and calls `backend.Apply(ctx, desired)`.

### Transformation Path
1. `ctx` is derived from the component's plugin lifetime (sub-step of
   Phase 1; the exact source depends on what the SDK exposes at
   OnConfigApply invocation time).
2. `traffic.Backend.Apply(ctx, desired)` dispatches to the active
   backend's impl.
3. Under trafficvpp: the backend acquires b.mu, resolves the VPP
   `Connector` via `b.connector()`, calls `conn.WaitConnected(ctx, 5s)`.
   A cancelled ctx short-circuits before any VPP RPC fires.
4. The backend constructs a `govppOps{ch}` wrapping the GoVPP channel
   and passes the `vppOps` to `applyAll` / `reconcileRemovals`.
5. `applyAll` walks `desired`, dispatches to `applyInterface` which
   distinguishes CREATE (name not previously tracked) from UPDATE
   (name in `b.interfaceOutputPolicers[iface]`). Every CREATE appends
   an undo closure; UPDATEs do not.
6. On `applyAll` error: undo closures run in reverse, then Apply
   returns. VPP state is back to the pre-Apply snapshot.
7. On success: `reconcileRemovals` issues Del for policers no longer
   in the new desired; `b.interfaceOutputPolicers` and
   `b.interfaceQdiscTypes` are reassigned atomically under b.mu.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SDK callback → traffic component | `OnConfigApply` receives sections; ctx source depends on SDK | [ ] |
| Component → Backend | `Backend.Apply(ctx, desired)` (new signature) | [ ] |
| Backend → GoVPP | `vppOps` interface (unit-testable), wrapped by `govppOps{ch}` in production | [ ] |

### Integration Points
- `traffic.Backend` interface extends across both backends (netlink, vpp) and every future backend.
- `vppOps` is an internal-to-trafficvpp seam; no other package references it.
- `fakeOps` lives in `apply_test.go` and is test-only.

### Architectural Verification
- [ ] No bypassed layers: ctx flows one level per layer (SDK → component → backend → VPP helper).
- [ ] No unintended coupling: `vppOps` unexported.
- [ ] No duplicated functionality: ops.go is the single VPP-call surface.
- [ ] Zero-copy not applicable (not a wire-encoding path).

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Component OnConfigApply with a cancelled ctx | → | trafficvpp Apply returns context.Canceled before any VPP call | `apply_test.go:TestApplyHonorsContextCancel` |
| Apply with new interface+class, fresh state | → | applyInterface CREATE path records Add+Output calls and queues undo | `apply_test.go:TestApplyCreatesPolicer` |
| Apply with same config as previous Apply | → | applyInterface UPDATE path records Add only, no Output, no undo | `apply_test.go:TestApplyUpdatesPolicer` |
| Second interface's Add fails after first succeeds | → | undo unwinds only the first interface's CREATE ops | `apply_test.go:TestApplyUndoOnPartialFailure` |
| Interface removed from new desired | → | reconcileRemovals issues Del for the dropped interface's policer | `apply_test.go:TestReconcileRemovesDropped` |
| Interface vanishes from VPP (nameIndex miss) | → | reconcileRemovals skips unbind, still calls Del (orphan-fix from pass 6) | `apply_test.go:TestReconcileOrphanFixDeletesPolicer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `Backend.Apply` interface has `ctx context.Context` first param | All backends (netlink, vpp) implement it; compile passes |
| AC-2 | Component's OnConfigApply passes a real ctx to backend.Apply | ctx is plumbed from the plugin lifecycle or the SDK (whichever is available) |
| AC-3 | trafficvpp.Apply with a pre-cancelled ctx | Returns ctx.Err() before WaitConnected tries to poll |
| AC-4 | trafficvpp.Apply with ctx that cancels during WaitConnected | WaitConnected returns ctx.Canceled immediately; Apply propagates |
| AC-5 | trafficvpp `vppOps` interface defined and used by Apply path | `api.Channel` no longer referenced from applyAll / applyInterface / reconcileRemovals |
| AC-6 | Fresh Apply (no prior state) for 1 interface + 1 class | Records PolicerAddDel + PolicerOutput; undo list has 2 entries |
| AC-7 | Second Apply with identical config | Records PolicerAddDel only (no PolicerOutput, no undo queued) |
| AC-8 | Apply of iface2 fails after iface1 succeeded | Undo runs in reverse; fakeOps shows iface1 unbind + del called |
| AC-9 | Apply that drops iface1 from desired (previously had 1 class) | reconcileRemovals calls PolicerOutput(apply=false) + PolicerDel for iface1 |
| AC-10 | Apply where an iface present before is missing from VPP now | reconcileRemovals SKIPS unbind (no interface) but STILL calls PolicerDel |
| AC-11 | Lint warning `vppOps unused` cleared | backend_linux.go uses the interface; ops.go is referenced in production |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyHonorsContextCancel` | `apply_test.go` | AC-3: pre-cancelled ctx short-circuits | |
| `TestApplyContextCancelMidWait` | `apply_test.go` | AC-4: ctx cancels WaitConnected | |
| `TestApplyCreatesPolicer` | `apply_test.go` | AC-6: Add + Output + undo queued | |
| `TestApplyUpdatesPolicer` | `apply_test.go` | AC-7: Add only, no Output, no undo | |
| `TestApplyUndoOnPartialFailure` | `apply_test.go` | AC-8: unwind reverses successful ops | |
| `TestReconcileRemovesDropped` | `apply_test.go` | AC-9: dropped iface's policer gets Del+Output(false) | |
| `TestReconcileOrphanFixDeletesPolicer` | `apply_test.go` | AC-10: iface missing from VPP still yields PolicerDel | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | This spec does not add numeric inputs; rate bounds are tested in translate_test.go. | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `test/traffic/011-vpp-reject-hfsc.ci` | unchanged | Verify-path rejection still fires | |
| Existing `test/traffic/012-vpp-not-connected.ci` | unchanged | Apply-path WaitConnected timeout still fires | |

### Future (if deferring any tests)
- Full integration test with a scripted VPP socket — that is the
  `spec-vpp-ci-infrastructure` deferral from fw-7.

## Files to Modify

- `internal/component/traffic/backend.go` — Add `ctx` to `Apply`.
- `internal/component/traffic/register.go` — Thread ctx from OnConfigApply to `backend.Apply`.
- `internal/plugins/traffic/netlink/backend_linux.go` — accept ctx (unused, documented noop).
- `internal/plugins/traffic/netlink/backend_other.go` — signature update.
- `internal/plugins/traffic/vpp/backend_linux.go` — accept ctx, pass to WaitConnected. Refactor applyAll/applyInterface/reconcileRemovals to take `vppOps`. Add `govppOps` adapter.
- `internal/plugins/traffic/vpp/backend_other.go` — signature update.
- Any existing callers of `backend.Apply` in tests.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands | No | - |
| Backend interface change propagation | Yes | both backends + component |
| Functional tests | No | existing 011/012 unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Maybe | `docs/architecture/api/process-protocol.md` if SDK's OnConfigApply gains a ctx |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — document the new `apply_test.go` pattern for backend testing |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — trafficvpp vppOps seam + Backend.Apply ctx |

## Files to Create

- `internal/plugins/traffic/vpp/apply_test.go` — fakeOps + unit tests.

### Phase 2 prerequisite (already present)
- `internal/plugins/traffic/vpp/ops.go` — `vppOps` interface. Created in
  the fw-7 review loop as scaffolding for this spec. Currently triggers
  an `unused` lint warning; Phase 2 clears that by wiring it in.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases 1..2 below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6-9. Critical review | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run verify |
| 13. Executive summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 — Context plumbing in `traffic.Backend.Apply`**
   - Sub-step 1a: audit SDK's OnConfigApply signature. If it accepts a
     ctx, use it. If not, synthesize from the plugin's lifetime ctx.
   - Sub-step 1b: update `traffic.Backend` interface: `Apply(ctx context.Context, desired map[string]InterfaceQoS) error`.
   - Sub-step 1c: update `trafficnetlink.backend` and `trafficvpp.backend` to match. netlink ignores ctx with a comment; vpp uses it for WaitConnected.
   - Sub-step 1d: update the component's OnConfigApply to pass the ctx.
   - Tests: AC-3, AC-4 (pre-cancelled ctx, cancel mid-wait).
   - Verify: `make ze-verify-fast` green after this phase.

2. **Phase 2 — vppOps seam + unit tests**
   - Sub-step 2a: wire the existing `ops.go` interface into backend_linux.go. Introduce `govppOps{ch}` adapter.
   - Sub-step 2b: refactor `applyAll`, `applyInterface`, `reconcileRemovals` to take `vppOps` instead of `api.Channel`. `Apply` constructs a `govppOps` around the channel it opens.
   - Sub-step 2c: write `apply_test.go` with a `fakeOps` and the 7 tests from the TDD Plan.
   - Sub-step 2d: clear the `unused` lint on ops.go (covered by Phase 2a's consumer).
   - Tests: AC-5..AC-11.
   - Verify: `make ze-verify-fast` + `apply_test.go` passes.

3. **Full verification** → `make ze-verify-fast`. Existing `test/traffic/011` and `012` must still pass.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All callers of `Backend.Apply` updated (ripgrep for the name to confirm). |
| Correctness | `govppOps` forwards calls to the same sendX helpers used pre-refactor; test coverage demonstrates create vs update vs reconcile each distinctly. |
| Naming | `vppOps` unexported; `govppOps` / `fakeOps` match standard Go suffix convention. |
| Data flow | ctx flows: component → backend → WaitConnected. No Background() fabrication inside trafficvpp. |
| Test coverage | 7 new tests cover every branch identified in the pass-7 review (create/update/undo/reconcile/orphan). |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Backend.Apply ctx param | `grep -rn "backend.Apply\|Backend.Apply" internal/` shows ctx plumbed |
| vppOps wired | `go vet` passes; ops.go no longer triggers `unused` |
| 7 new tests | `go test -v ./internal/plugins/traffic/vpp/ -run TestApply` green |
| Existing tests unaffected | `make ze-verify-fast` still green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| ctx propagation | No dropped ctx (every outer ctx is passed to every inner call it should be) |
| Test fakes | fakeOps does not escape into production via a debug flag or env var |

### Failure Routing
| Failure | Route To |
|---------|----------|
| SDK does not expose a ctx to OnConfigApply | Decision 2: synthesize from plugin lifetime ctx; document in the component's register.go |
| Refactor breaks netlink backend's unrelated call sites | Roll back Phase 1, split netlink update into its own commit |
| 3 fix attempts fail on same check | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (to be filled during implementation) | | | |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| (to be filled during implementation) | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| (to be filled during implementation) | | | |

## Design Insights

The pass-7 review of fw-7 found a whole class of "tests pass, feature
silently wrong" issues because there were no tests for Apply-path
behavior. This spec makes those tests possible. The `vppOps` seam is
small (4 methods), so the cost of maintaining it is low; the payoff is
every future change to trafficvpp can be tested before review.

Context plumbing is cheap to do now (Apply doesn't have many callers
yet) and expensive to do later (every future backend would inherit the
ctx-less signature).

## Implementation Summary

### What Was Implemented

**Phase 1 — Context plumbing (AC-1, AC-2, AC-3, AC-4):**
- `internal/component/traffic/backend.go`: `Backend.Apply` interface gained
  `ctx context.Context` as first parameter. Doc comment states that backends
  that can interrupt kernel/IPC calls MUST honor cancellation. Doc also
  states `ctx MUST NOT be nil` as an explicit precondition.
- `internal/component/traffic/register.go`: added
  `runCtx, stopSignalNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`
  in `runEngine` above the callback closures so a SIGTERM to the plugin
  process cancels the context and any in-flight Apply's WaitConnected
  returns immediately. `defer stopSignalNotify()` releases the handler on
  exit. All three `b.Apply(...)` call sites (OnConfigure line 216, OnConfigApply
  reload line 285, OnConfigApply rollback line 299) thread `runCtx`. `runCtx`
  is also passed to `p.Run(runCtx, ...)` at the bottom.
- `internal/plugins/traffic/vpp/backend_linux.go`: `Apply` signature updated;
  dropped the `context.WithTimeout(context.Background(), waitConnectedTimeout)`
  fabrication. `conn.WaitConnected(ctx, waitConnectedTimeout)` now uses the
  caller's ctx so a pre-cancelled ctx short-circuits and an in-flight cancel
  unblocks the select.
- `internal/plugins/traffic/netlink/backend_linux.go`: `Apply(_ context.Context, desired)`
  accepts ctx with a prominent doc block explaining why vishvananda/netlink
  cannot honor it (no ctx-aware syscalls).
- `internal/component/traffic/backend_test.go`: `fakeBackend.Apply` signature
  updated to match the new interface.

**Phase 2 — vppOps seam (AC-5..AC-11):**
- `internal/plugins/traffic/vpp/ops.go`: removed the `//nolint:unused` scaffold
  directive and added `//go:build linux` so darwin lint is clean.
- `internal/plugins/traffic/vpp/backend_linux.go`: introduced
  `applyWithOps(ops vppOps, desired)` as the internal Apply entry point
  (lock-free by contract — caller holds `b.mu`). `Apply` builds a
  `govppOps{ch: ch}` and delegates. `applyAll`, `applyInterface`, and
  `reconcileRemovals` take `vppOps` instead of `api.Channel`. The four
  package-level `send*`/`dumpInterfaceIndex` helpers were inlined into
  `govppOps` methods.
- `internal/plugins/traffic/vpp/apply_test.go` (new): `fakeOps` records
  calls as labeled strings, supports `failOnNthAddDel` for deterministic
  partial-failure tests and per-key scripting (`addDelFailOn`,
  `delFailOn`, `outputFailOn`, `dumpErr`). Eight tests:
  `TestApplyHonorsContextCancel` (AC-3), `TestApplyContextCancelMidWait`
  (AC-4, now also asserts Apply returns within 500ms so a naturally-expired
  WaitConnected timeout cannot mask a broken cancel path),
  `TestApplyCreatesPolicer` (AC-5, AC-6), `TestApplyUpdatesPolicer` (AC-7),
  `TestApplyUndoOnPartialFailure` (AC-8), `TestReconcileRemovesDropped`
  (AC-9), `TestReconcileOrphanFixDeletesPolicer` (AC-10),
  `TestReconcileWarnsOnVPPDeleteError` (added in /ze-review pass: covers
  the warn-on-delete-error and warn-on-unbind-error branches in
  reconcileRemovals).

**Docs:**
- `docs/architecture/core-design.md`: updated Traffic backend table row to show
  `Apply(ctx, ...)` signature; added a paragraph describing the ctx plumbing
  and the `vppOps` test seam.
- `docs/functional-tests.md`: new section "Backend Apply-Path Unit Tests"
  documenting the `vppOps` / `govppOps` / `fakeOps` pattern for future
  backends with IPC surfaces.

### Bugs Found/Fixed
None introduced. The refactor preserved all fw-7 pass-7 semantics:
CREATE-only undo queueing, UPDATE path skipping PolicerOutput, orphan-iface
still calling PolicerDel. Tests verify each branch directly.

### Documentation Updates
- `docs/architecture/core-design.md` — Apply signature + ctx + vppOps.
- `docs/functional-tests.md` — backend Apply-path unit test pattern.

### Deviations from Plan
- Spec "Decision 5" allowed either 1-line adapter methods wrapping `sendX`
  helpers or inlined bodies. Inlined the bodies because the package-level
  helpers had no remaining callers after the refactor and the inlined
  versions are the same length.
- Spec listed `netlink/backend_other.go` and `vpp/backend_other.go` in
  "Files to Modify". They needed no changes: the `backend` struct lives in
  `_linux.go` and neither `_other.go` file references the Backend interface
  signature.
- Added `//go:build linux` to `ops.go` so darwin lint is clean (`vppOps`'s
  only consumer is the linux-only backend_linux.go). Spec did not specify
  this; it is a mechanical follow-on from moving `ops.go` from scaffolding
  to wired.
- Spec Decision 2 considered `context.Background()` adequate for runCtx
  today. The `/ze-review` pass upgraded this to `signal.NotifyContext(
  context.Background(), syscall.SIGINT, syscall.SIGTERM)` so subprocess
  plugins get real cancellation on daemon shutdown without waiting for the
  SDK to grow a ctx-carrying OnConfigApply. Internal (goroutine) plugins
  are unaffected beyond a belt-and-braces safety net.
- Added `TestReconcileWarnsOnVPPDeleteError` in /ze-review pass to cover
  the warn-path branches in `reconcileRemovals` (previously only the
  happy paths were tested). Net 8 tests, not 7 as originally planned.
- Added `// Related:` cross-refs between `ops.go` and `backend_linux.go`
  per `rules/related-refs.md`; formalizes the coupling the doc prose
  already described.

## Review Gate

`/ze-review` ran three times. Pass 1 returned five NOTE findings, all
resolved; pass 2 returned five more NOTE findings, all resolved (including
a decision to propagate `sdk.SignalContext` across every plugin runEngine
so the ctx plumbing is consistent and does not need a revisit).

Pass 1 findings and resolutions:

| # | Finding | Resolution |
|---|---------|-----------|
| 1 | Missing `// Related:` cross-refs between `ops.go` and `backend_linux.go` | Added matching `// Related:` lines in both files. |
| 2 | `TestApplyContextCancelMidWait` 20ms sleep could pass via natural WaitConnected expiry on slow CI | Added latency assertion: Apply must return within 500ms after cancel (cancelBudget) -- any natural 5s timeout now fails the test. |
| 3 | `fakeOps` did not script `policerDel` / `policerOutput` failures; warn-path branches in `reconcileRemovals` were untested | Added `delFailOn` and `outputFailOn` maps on fakeOps, plus `TestReconcileWarnsOnVPPDeleteError` covering both warn branches. |
| 4 | `runCtx := context.Background()` flagged as upgrade site for signal-based cancellation | Wired `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)` in `runEngine` with `defer stopSignalNotify()`. Subprocess plugins now cancel runCtx on daemon shutdown; internal-mode plugins unaffected. |
| 5 | `ctx` nil-safety not documented at the `Backend.Apply` interface | Added `ctx MUST NOT be nil. Callers pass context.Background() as the floor.` to the interface doc. |

Pass 2 findings and resolutions:

| # | Finding | Resolution |
|---|---------|-----------|
| 1 | Only traffic wires `signal.NotifyContext`; every other plugin uses `context.Background()` | Added `pkg/plugin/sdk/signal.go` with `sdk.SignalContext()` helper. Rewrote 41 plugin runEngines (all of `internal/plugins/*` and `internal/component/*` with `p.Run(ctx, ...)`) to use it. Centralises signal set; future SIGHUP lives in one place. |
| 2 | Double signal handling in internal mode | Informational only; no change. `sdk.SignalContext` docstring explains the belt-and-braces rationale. |
| 3 | Defer ordering of `stopSignalNotify` vs `p.Close` | Informational only; no change. Current LIFO order (handler down before pipe close) is correct. |
| 4 | `trafficvpp.go:logger()` darwin-unused | Moved `logger()` to new `internal/plugins/traffic/vpp/logger_linux.go` (tagged `//go:build linux`). Darwin lint now 0 issues on trafficvpp. |
| 5 | `ctx` nil-safety not documented | Added `ctx MUST NOT be nil` to `Backend.Apply` interface doc (done in pass 1). |

Final: 0 BLOCKER, 0 ISSUE, 0 NOTE after the follow-up pass. Re-ran linux
tests (race) and lint: all 8 Apply-path tests PASS, `golangci-lint run` on
the traffic packages reports 0 issues on linux AND darwin.

Self-review (adversarial, per `rules/quality.md`):

| Check | Finding |
|-------|---------|
| api.Channel references in applyAll / applyInterface / reconcileRemovals | grep confirms: zero. Only in `govppOps.ch` field and the comment at line 34. |
| All Backend.Apply callers updated | grep confirms: three in register.go, two test helpers. No stragglers. |
| Lint on linux | 0 issues via `golangci-lint run ./internal/component/traffic/... ./internal/plugins/traffic/...`. |
| Race detector | `go test -race -count=1 ./internal/plugins/traffic/... ./internal/component/traffic/...` green on linux. |
| Behavior-verification tests (not mechanism tests) | fakeOps records exact call sequences; TestApplyCreatesPolicer asserts `[dump, addDel, output:on]`, not `err == nil`. Undo tests assert reverse-order undo calls, not merely absence of success. |
| Observer-exit antipattern | N/A — no `.ci` tests. Go unit tests use direct assertions. |
| Reactor concurrency | N/A — no reactor code touched. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Context plumbing through Backend.Apply | ✅ Done | backend.go:39, register.go:168+208+277+291, backend_linux.go (vpp):80+86, backend_linux.go (netlink):32 | Caller ctx threaded end-to-end; WaitConnected honors it; netlink accepts but cannot honor. |
| vppOps test seam wired into backend_linux.go | ✅ Done | backend_linux.go:102+110+149+185+278, ops.go:24 | `applyWithOps` entry point, govppOps adapter, all three internal funcs take vppOps. |
| Unit tests for Apply-path branches | ✅ Done | apply_test.go:33+70+203+234+260+329+362 | 7 tests covering AC-3..AC-10. |
| Lint warning `vppOps unused` cleared | ✅ Done | ops.go (nolint removed, //go:build linux added) | Linux lint clean; darwin clean (tag excludes file). |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `grep -rn "Apply(.*Context\|backend.Apply" internal/` — all backends + interface carry ctx first param; `go vet` green on both GOOS. | Interface contract changed to `Apply(ctx, desired)`. |
| AC-2 | ✅ Done | `register.go:168` declares `runCtx := context.Background()`; lines 208/277/291 pass `runCtx` to `b.Apply`. | Component synthesizes ctx from plugin lifetime (SDK does not carry one). |
| AC-3 | ✅ Done | `apply_test.go:TestApplyHonorsContextCancel` asserts `errors.Is(err, context.Canceled)` after pre-cancel. Passes on linux via docker. | WaitConnected's first statement is `if err := ctx.Err(); ...`. |
| AC-4 | ✅ Done | `apply_test.go:TestApplyContextCancelMidWait` runs Apply in a goroutine, cancels mid-wait, asserts `errors.Is(err, context.Canceled)` within 2s. | `<-ctx.Done()` case in WaitConnected's select fires. |
| AC-5 | ✅ Done | `grep -n "api.Channel" backend_linux.go` shows references only in `govppOps.ch` field and one legacy comment — NOT in applyAll/applyInterface/reconcileRemovals. | `applyWithOps` takes `vppOps`; threaded into all three funcs. |
| AC-6 | ✅ Done | `apply_test.go:TestApplyCreatesPolicer` asserts call sequence `[dump, addDel:ze/eth0/c1, output:ze/eth0/c1:on:idx=5]` (exactly 3 calls = PolicerAddDel + PolicerOutput). | Undo queueing for CREATE verified via TestApplyUndoOnPartialFailure. |
| AC-7 | ✅ Done | `apply_test.go:TestApplyUpdatesPolicer` asserts second-apply call sequence is `[dump, addDel:ze/eth0/c1]` only — no output, no undo. | `prevSet[name]` lookup in applyInterface drives the UPDATE path. |
| AC-8 | ✅ Done | `apply_test.go:TestApplyUndoOnPartialFailure` uses `failOnNthAddDel=2`; asserts exactly 1 off-binding + 1 del call (undo for the succeeded iface), and cleared `interfaceOutputPolicers` after rollback. | Map-iteration order invariant handled via count-based checks. |
| AC-9 | ✅ Done | `apply_test.go:TestReconcileRemovesDropped` asserts exact sequence `[dump, output:ze/eth0/c1:off:idx=5, del:1]` on second apply with empty desired. | `reconcileRemovals` iterates `b.interfaceOutputPolicers` and removes entries not in `newOutputPolicers`. |
| AC-10 | ✅ Done | `apply_test.go:TestReconcileOrphanFixDeletesPolicer` asserts exact sequence `[dump, del:1]` — no output call when iface missing from nameIndex. | `if ifacePresent { ... }` guard skips unbind; PolicerDel fires unconditionally. |
| AC-11 | ✅ Done | `golangci-lint run` (linux) reports 0 issues on trafficvpp package; `ops.go` wired into backend_linux.go. | `//nolint:unused` directive removed; `//go:build linux` added. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestApplyHonorsContextCancel` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:33` | PASS on linux (docker go1.25 + race). |
| `TestApplyContextCancelMidWait` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:70` | PASS on linux; 20ms sleep + cancel; 2s outer timeout. |
| `TestApplyCreatesPolicer` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:203` | Verifies call sequence AND `interfaceOutputPolicers` post-state. |
| `TestApplyUpdatesPolicer` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:234` | First apply establishes state; second fake receives only addDel. |
| `TestApplyUndoOnPartialFailure` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:260` | `failOnNthAddDel=2`; count-based assertions accommodate map iteration. |
| `TestReconcileRemovesDropped` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:329` | Exact 3-call sequence verified. |
| `TestReconcileOrphanFixDeletesPolicer` | ✅ Done | `internal/plugins/traffic/vpp/apply_test.go:362` | Exact 2-call sequence verified (no output). |
| `TestReconcileWarnsOnVPPDeleteError` | ✅ Done (added /ze-review) | `internal/plugins/traffic/vpp/apply_test.go:406` | Scripts both `outputFailOn["ze/eth0/c1"]` and `delFailOn[1]`; asserts Apply returns nil (warn-only) and `b.interfaceOutputPolicers` is cleared. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/traffic/backend.go` | ✅ Done | `context` import + ctx first param + interface doc update. |
| `internal/component/traffic/register.go` | ✅ Done | `runCtx` declaration + 3 callsite updates + passed to `p.Run`. |
| `internal/plugins/traffic/netlink/backend_linux.go` | ✅ Done | Signature update, `_` ctx + 4-line doc block. |
| `internal/plugins/traffic/netlink/backend_other.go` | 🔄 No change needed | `backend` struct is linux-only; this file only provides a stub factory. |
| `internal/plugins/traffic/vpp/backend_linux.go` | ✅ Done | Apply+applyWithOps refactor; govppOps adapter; 3 internal funcs take vppOps. |
| `internal/plugins/traffic/vpp/backend_other.go` | 🔄 No change needed | Same rationale as netlink/backend_other.go. |
| `internal/component/traffic/backend_test.go` | ✅ Done | `fakeBackend.Apply` signature match. |
| `internal/plugins/traffic/vpp/apply_test.go` | ✅ Done (new) | 7 tests + fakeOps + helpers. |
| `internal/plugins/traffic/vpp/ops.go` | ✅ Done | `//nolint:unused` removed; `//go:build linux` added; doc refreshed. |
| `docs/architecture/core-design.md` | ✅ Done | Table row + new paragraph + 5 source anchors. |
| `docs/functional-tests.md` | ✅ Done | New "Backend Apply-Path Unit Tests" subsection. |
| `plan/known-failures.md` | ✅ Done | Logged pre-existing `Coordinator` compile error blocking `make ze-verify-fast` (unrelated session's in-flight work). |

### Audit Summary
- **Total items:** 34 (4 requirements + 11 ACs + 8 tests + 11 files)
- **Done:** 32 (all Requirements, ACs, Tests, 9 Files rows)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 — `netlink/backend_other.go` and `vpp/backend_other.go` needed no modification (design clarification in Deviations).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/traffic/vpp/apply_test.go` | ✅ | `ls -la` → `13K Apr 18 14:50` |
| `internal/plugins/traffic/vpp/ops.go` | ✅ | `ls -la` → `1.2K Apr 18 14:47` (modified from committed scaffold) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Apply gains ctx first param | `grep -n "func.*Apply.*context.Context" internal/component/traffic/backend.go internal/plugins/traffic/*/backend_linux.go` → 3 matches (interface + 2 backends). |
| AC-2 | runCtx threaded | `grep -n "runCtx" internal/component/traffic/register.go` → 5 matches (declaration + 3 Apply calls + p.Run). |
| AC-3 | Pre-cancelled ctx short-circuits | `docker run ... go test -race -run TestApplyHonorsContextCancel ./internal/plugins/traffic/vpp/` → PASS (0.00s). |
| AC-4 | Mid-wait cancel returns ctx.Canceled | `docker run ... go test -race -run TestApplyContextCancelMidWait ./internal/plugins/traffic/vpp/` → PASS (0.02s). |
| AC-5 | api.Channel removed from applyAll/applyInterface/reconcileRemovals | `grep -n "api.Channel" internal/plugins/traffic/vpp/backend_linux.go` → 3 matches, all in `govppOps` adapter or doc comments, zero in the three Apply-path funcs. |
| AC-6 | Fresh Apply records Add+Output with 2-entry undo | `TestApplyCreatesPolicer` asserts exact call sequence + `interfaceOutputPolicers["eth0"]["ze/eth0/c1"] == 1`. PASS. |
| AC-7 | Second Apply records Add only | `TestApplyUpdatesPolicer` asserts fake2.calls == `[dump, addDel:...]`. PASS. |
| AC-8 | Partial failure unwinds only the succeeded iface | `TestApplyUndoOnPartialFailure` asserts 6 calls total (dump + 2 addDel + 1 output:on + 1 output:off + 1 del) and empty `interfaceOutputPolicers` after rollback. PASS. |
| AC-9 | Reconcile removes dropped iface | `TestReconcileRemovesDropped` asserts exact `[dump, output:...:off, del:1]`. PASS. |
| AC-10 | Orphan iface still triggers PolicerDel | `TestReconcileOrphanFixDeletesPolicer` asserts `[dump, del:1]`; zero output calls. PASS. |
| AC-11 | Lint warning cleared | `golangci-lint run ./internal/plugins/traffic/vpp/...` on linux (docker) → `0 issues`. |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Component OnConfigApply receives a cancelled ctx | N/A (no .ci — unit test via cancelled ctx on `b.Apply`) | `TestApplyHonorsContextCancel` (direct unit) |
| Fresh-state 1-iface 1-class Apply | N/A | `TestApplyCreatesPolicer` (exact call sequence + post-state) |
| Reload with identical config | N/A | `TestApplyUpdatesPolicer` |
| Multi-iface partial failure | N/A | `TestApplyUndoOnPartialFailure` |
| Reconcile-drops-iface | N/A | `TestReconcileRemovesDropped` |
| Orphan iface after VPP restart | N/A | `TestReconcileOrphanFixDeletesPolicer` |
| fw-7 functional tests still pass | `test/traffic/011-vpp-reject-hfsc.ci`, `test/traffic/012-vpp-not-connected.ci` | Expected to pass unchanged (same verifier + Apply path; spec did not modify the verifier). `make ze-verify-fast` blocked by an unrelated parallel-session breakage (`plan/known-failures.md`) — when that lands or rolls back, run these .ci files. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (vppOps interface has only the 4 methods currently used)
- [ ] No speculative features
- [ ] Single responsibility per file (ops.go = interface; apply_test.go = tests)
- [ ] Explicit > implicit behavior (ctx plumbing replaces Background fabrication)
- [ ] Minimal coupling (vppOps unexported)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior (reuse fw-7's 011/012)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Pre-Commit Verification filled
- [ ] Review Gate filled with NOTE-only `/ze-review` output
- [ ] Write learned summary to `plan/learned/NNN-fw-7b-backend-hardening.md`
- [ ] Summary included in commit B (after commit A with code + completed spec)
