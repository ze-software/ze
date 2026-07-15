# spec-fixit-static-interface-nexthops

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-15 |

**SKELETON.** This spec tracks known-but-unstarted work. It is NOT ready to implement.
Run `/ze-spec` first: the Open Questions below must be answered before any design. Test
names, files, and steps listed here are CANDIDATES from a read of the current code, not a
settled design.

## Task

A static route whose next-hop names an interface but carries no address
(`next { interface tun100 { } }`) fails to program on BOTH data-plane backends, for two
different reasons. On linux the next-hop interface is resolved through `iface.Resolve`,
which errors with "iface: no backend loaded" whenever no iface backend is active in the
calling process. On VPP the translator rejects interface-only next-hops outright because no
logical-name to `sw_if_index` resolver is threaded into the static backend. Address
next-hops, recursive next-hops, blackhole and reject all work on both backends, so this is a
bounded gap on one next-hop form. Goal: static interface-only next-hops work on the linux
and VPP backends, or the limitation is made explicit and diagnosable rather than failing an
entire static config section.

The linux root cause is CONFIRMED as a mechanism (the call chain is read end to end below)
but it is UNVERIFIED whether it bites in production or only in the test config that
surfaced it.

## Origin

Problem A (linux): found during `spec-fixit-migrate-sleeps-infra` work, 2026-07-15, while
looking at `test/static/005-table-interface.ci`.
Problem B (VPP): `plan/deferrals.md` row dated 2026-07-10,
"spec-test-coverage-gaps W-3 static/vpp wiring" (deferred).

## Required Reading

### Source (read before designing)

- [ ] `internal/component/iface/dispatch.go:12` - `errIfaceNoBackendLoaded`, the actual
      producer of "iface: no backend loaded"; returned by `backendOrErr()` (`:15-21`)
  → Constraint: the error text lives here, NOT in `resolve.go`; `resolve.go:61-64` only
    documents it in a doc comment. Cite this file when describing the failure.
- [ ] `internal/component/iface/resolve.go:65` - `Resolve()`, the entry point the static
      backend calls
  → Constraint: `Resolve` caches per logical name and is backend-agnostic; the failure is
    inherited from `GetInterface`, not produced here.
- [ ] `internal/component/iface/backend.go:248-250`, `:266-301` - `activeBackend`
      process-global, nil until `LoadBackend`; `GetBackend()` at `:301`
  → Constraint: backend activation is per-PROCESS global state, not passed as a parameter.
    Whether a consumer sees a backend depends on which process it runs in.
- [ ] `internal/component/iface/register.go:406-418` - the only production `LoadBackend`
      call path (also `:575`)
  → Constraint: `LoadBackend` runs only from the iface component's config path and only
    when `interface { backend ... }` is non-empty (`:406-408` returns early otherwise).
- [ ] `internal/plugins/static/backend_linux.go:97-103` - `resolveNexthopIndex` calls
      `iface.Resolve`; called from `buildRoute` at `:141` and `:159`
  → Decision: the static backend deliberately routes through the shared resolver rather
    than its own `LinkByName`, to honor os-name / mac-match selectors. Do not undo this.
- [ ] `internal/plugins/static/backend_vpp_linux.go:73-102` - `toVPPRoute`; `:91-95` is the
      interface-only rejection
  → Decision: the current rejection is deliberate ("failing loudly beats programming a
    wrong (index-0) path silently", `:69-72`). Any fix must keep index-0 unreachable.
- [ ] `internal/plugins/static/vpp/backend.go:24-30`, `:111-115` - `Path.SwIfIndex` already
      exists and `toFibPath` already carries it into `fib_types.FibPath`
  → Constraint: the VPP encode path is ALREADY capable. Only parent-side name resolution
    is missing, which makes the VPP leg much smaller than the deferrals row implies.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go:225-238` - `resolveIndex`, an existing name to
      `SwIfIndex` lookup backed by `b.names.LookupIndex`
  → Constraint: it is an unexported method on `vppBackendImpl` and is channel-gated by
    `ensureChannel()`. Reaching it from the static plugin needs a boundary decision.
- [ ] `internal/plugins/static/inject.go:62-93`, `:138-171` - error join / propagation path
  → Constraint: per-route errors are joined and returned to `OnConfigure`, so ONE bad
    next-hop fails the whole static section. Any fix must decide whether that stays.
- [ ] `test/static/005-table-interface.ci:53-73` - the config that surfaced Problem A
  → Constraint: the config has NO `interface { backend ... }` stanza and never creates
    `tun100`. Both facts matter before "fixing" the test.
- [ ] `internal/component/plugin/inprocess.go:93-124` - in-process runner
  → Decision: plugins have BOTH an in-process runner and a subprocess runner
    (`internal/component/plugin/cli/cli.go:164`). Which one `static` uses decides whether
    Problem A is a product bug or a test-config bug.

### Architecture Docs

- [ ] `plan/learned/650-static-routes.md` - cited as the design doc by both static backends
      (`backend_linux.go:1`, `backend_vpp_linux.go:1`)
  → Constraint: read before changing backend selection or route programming.
- [ ] `plan/learned/950-iface-resolve-2-resolver.md` - cited as the design doc by the shared
      resolver (`resolve.go:1`)
  → Constraint: the resolver is the single owner of logical-name to device resolution for
    external consumers. Do not add a second resolution path.
- [ ] `plan/learned/1107-test-coverage-gaps.md` - Gotchas section, static/vpp interface-only
      nh (named by the deferrals row as the destination)
  → Decision: this is where the VPP half was recorded when deferred.
- [ ] `ai/rules/plugin-self-containment.md` - constrains cross-plugin spelling
  → Constraint: the static plugin must not spell `ifacevpp` internals directly.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - applies directly to R-1
  → Constraint: do not make the `.ci` green by removing the interface-only next-hop.

**Key insights:**
- The "no backend loaded" error is produced in `dispatch.go:12`, not `resolve.go`.
- iface backend activation is process-global state, so the fix depends on the static
  plugin's process model, which is not yet established.
- The VPP encode path already supports `SwIfIndex`; only name resolution is missing.
- One unresolvable next-hop currently fails the ENTIRE static section.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/plugins/static/backend_linux.go` - lines 135-147: single next-hop sets
      `route.Gw` when the address is valid, and sets `route.LinkIndex` from
      `resolveNexthopIndex` when `nh.Interface != ""`. Multipath does the same per next-hop
      at lines 150-167. Lines 97-103: `resolveNexthopIndex(name)` calls `iface.Resolve(name)`
      and wraps any error as `interface %q: %w`.
- [ ] `internal/component/iface/resolve.go` - lines 82-100 and 117-142: `Resolve` to
      `resolver.resolve` to `osDeviceFor` to `GetInterface` (line 136).
- [ ] `internal/component/iface/dispatch.go` - lines 274-278: `GetInterface` calls
      `backendOrErr()`, which returns `errIfaceNoBackendLoaded` (line 12) when `GetBackend()`
      is nil.
- [ ] `internal/component/iface/backend.go` - lines 266-296: `LoadBackend` is the only writer
      of `activeBackend` (lines 248-250); nothing else sets it.
- [ ] `internal/plugins/static/backend_vpp_linux.go` - lines 91-95: `toVPPRoute` returns
      `static/vpp: interface-only next-hop %q needs a VPP sw_if_index (not yet supported)`
      for any next-hop whose `Address.IsValid()` is false. Lines 78-89: blackhole and reject
      return early before the next-hop loop, so they are unaffected.
- [ ] `internal/plugins/static/inject.go` - lines 87-92: `applyRoutes` collects per-route
      errors and joins them.
- [ ] `internal/plugins/static/register.go` - lines 138-141: a non-nil `applyRoutes` result
      becomes an `OnConfigure` error, so `"static routes loaded"` is never logged.

**Behavior to preserve:**

- Address next-hops (`next { hop 192.168.1.1 { } }`) on both backends, single and ECMP.
- Blackhole and reject actions on both backends (`backend_vpp_linux.go:78-89`,
  `backend_linux.go:122-133`).
- The os-name / mac-match selector remapping the linux backend gets by going through
  `iface.Resolve` rather than a raw `LinkByName`
  (`internal/plugins/static/resolve_integration_linux_test.go:26-59` pins this).
- Failing loudly rather than programming an index-0 path silently
  (`backend_vpp_linux.go:69-72`).
- VPP backend selection when a connector is active (`backend_linux.go:25-37`).

**Behavior to change:**

- None yet, research first. The shape depends on the Open Questions below, in particular
  whether Problem A is a production gap or a test-config gap.

## Data Flow

### Entry Point

- Config: a `static { table <t> { route <prefix> { next { interface <name> { } } } } }`
  stanza, parsed into `staticRoute.NextHops[i].Interface` with an invalid
  (unset) `Address`. Entry via the plugin SDK `OnConfigure` / `OnConfigApply`
  (`internal/plugins/static/register.go:126-144`, `:148-189`).
- Format at entry: `[]sdk.ConfigSection` with `Root == "static"`.

### Transformation Path

1. `parseStaticConfig` builds `[]staticRoute` from the section
   (`internal/plugins/static/register.go:131`).
2. `routeManager.applyRoutes` diffs against the installed set and calls
   `applyRouteLocked` per changed route (`internal/plugins/static/inject.go:62-93`).
3. `programRouteLocked` selects active next-hops and calls `backend.applyRoute`
   (`internal/plugins/static/inject.go:138-171`).
4. Backend split (`internal/plugins/static/backend_linux.go:25-37`): VPP backend when a
   connector is active, else the netlink backend.
5a. Linux: `buildRoute` calls `resolveNexthopIndex` per interface next-hop
    (`backend_linux.go:141`, `:159`), which calls `iface.Resolve` (`:98`), which reaches
    `GetInterface` (`resolve.go:136`) and fails at `backendOrErr` (`dispatch.go:15-21`)
    when `activeBackend` is nil.
5b. VPP: `toVPPRoute` (`backend_vpp_linux.go:73-102`) rejects the interface-only next-hop
    at `:91-95` before any `staticvpp.Path` is built.
6. Error joins back up through `applyRoutes` (`inject.go:87-92`) into the `OnConfigure`
   return (`register.go:138-140`), failing the whole static section.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config to static plugin | `[]sdk.ConfigSection`, root `static`, over the plugin SDK RPC conn | [ ] |
| Static plugin to iface component | `iface.Resolve(name)` package call reading the `activeBackend` process-global (`backend.go:248`) | [ ] |
| Static plugin to kernel | netlink `RouteReplace` via `netlink.Handle` (`backend_linux.go:51`) | [ ] |
| Static plugin to VPP | GoVPP channel, `staticvpp.Backend.ApplyRoute` (`backend_vpp_linux.go:49`) | [ ] |
| static/vpp to VPP binapi | `fib_types.FibPath` with `SwIfIndex` (`static/vpp/backend.go:111-115`) | [ ] |
| Process boundary (UNRESOLVED) | in-process runner (`plugin/inprocess.go:93-124`) vs subprocess runner (`plugin/cli/cli.go:164`); decides whether `activeBackend` is shared | [ ] |

### Integration Points

- `iface.Resolve` (`internal/component/iface/resolve.go:65`) - the existing shared resolver
  the linux static backend already consumes; the fix must extend usage, not duplicate it.
- `staticvpp.Path.SwIfIndex` (`internal/plugins/static/vpp/backend.go:28`) - the existing
  field the VPP fix must populate; already carried to `fib_types.FibPath` by `toFibPath`.
- `resolveIndex` (`internal/plugins/iface/vpp/ifacevpp.go:229`) - the existing VPP name to
  index lookup; currently unexported, so the integration needs a boundary decision.
- `iface.Binding.Ifindex` (`internal/component/iface/resolve.go:214-223`) - the value type
  already returned across the boundary.

### Architectural Verification

- [ ] No bypassed layers (static keeps resolving through `iface.Resolve`, not raw netlink)
- [ ] No unintended coupling (static does not spell `ifacevpp` internals)
- [ ] No duplicated functionality (no second name-to-index resolver is introduced)
- [ ] Zero-copy preserved where applicable (value `Binding`, no cross-boundary pointers)
- [ ] Registration over hardcoding: any new resolver reaches the static plugin through an
      existing registry / component boundary; no per-backend switch case or factory is added
      to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The linux failure is reachable in production, not only in a test config missing an `interface` stanza | Call chain read (`dispatch.go:12` to `register.go:414`); `activeBackend` is a process-global (`backend.go:248`) | Problem A shrinks to a test-config fix and the spec is mostly Problem B | Determine whether `static` runs in-process or as a subprocess in a normal `ze` run | unvalidated |
| A-2 | `staticvpp.Path.SwIfIndex` plus `toFibPath` is sufficient to program an interface-scoped VPP path with no further encode work | `static/vpp/backend.go:24-30`, `:111-115`, `static/vpp/translate_test.go:54-62` | The VPP leg grows to include FibPath flags / proto work for the no-address case | A real VPP apply with an index-only path | unvalidated |
| A-3 | The iface/vpp name to index map is reachable from the static plugin's process and is populated when static applies | `resolveIndex` exists (`ifacevpp.go:229`) but is unexported and channel-gated (`ensureChannel`) | The resolver must be exported AND its lifetime coordinated with static's apply ordering | Trace who owns `b.names` and when it is filled (`ifacevpp/query.go:194` mentions a construction-time dump) | unvalidated |
| A-4 | Interface-only next-hops are a form operators actually use, so this is worth fixing rather than rejecting at config-verify | The YANG accepts it and `test/static/005-table-interface.ci:68-72` was written for it | The cheaper correct answer is a config-verify rejection with a clear message | User / operator input | unvalidated |
| A-5 | `test/static/005-table-interface.ci` currently fails | Mechanism read above predicts it; NOT run (research-only spec) | The mechanism analysis is wrong somewhere and needs re-reading before design | Run the `.ci` and read the actual error | unvalidated |
| A-6 | The two problems share enough design to stay one spec | Both are the same user-visible symptom on one next-hop form | Split into two specs at DESIGN | DESIGN review | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing the test by adding an `interface { backend netlink }` stanza masks a real production gap instead of fixing it | The stanza makes the test green with no product change | Settle A-1 BEFORE touching the `.ci`; see `ai/rules/no-workarounds-for-missing-behavior.md` |
| R-2 | The device (`tun100`) does not exist in the test environment, so a backend alone will not make the test pass | Resolve fails with a device-absent error rather than a no-backend error | Create the device in the test, or pick an interface that exists |
| R-3 | Whole-section failure on one bad next-hop is a bigger operational problem than the next-hop form itself | An operator loses all static routes because one interface is absent | Consider per-route error isolation as a separate concern; do not silently widen scope |
| R-4 | Exporting a VPP name to index resolver leaks iface/vpp internals into the static plugin | The change requires the static plugin to spell `ifacevpp` | Route through an existing component boundary; read `ai/rules/plugin-self-containment.md` before designing |
| R-5 | The two problems are only superficially related and want separate designs | The linux fix is config-shaped, the VPP fix is resolver-shaped | Split the spec at DESIGN; the grouping is for tracking, not a claim of shared root cause |
| R-6 | A VPP fix cannot be proven without a live VPP, so it lands unverified | No QEMU/VPP path exercises interface-only static next-hops | Identify the VPP test rail during research; see `ai/rules/qemu-testing.md` |

## Wiring Test (MANDATORY)

Candidate rows. Every row is a proposal from a code read, to be confirmed by `/ze-spec`.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `static { table lns { route 0.0.0.0/0 { next { interface tun100 } } } }` config, linux backend | -> | `resolveNexthopIndex` (`backend_linux.go:97`) then `route.LinkIndex` (`:145`) | `test/static/005-table-interface.ci` |
| Same config, iface backend absent from the process | -> | `backendOrErr` (`dispatch.go:15`) error surfaced to the operator | `test/static/006-interface-nexthop-no-backend.ci` (new) |
| Same config, VPP data plane active | -> | `toVPPRoute` (`backend_vpp_linux.go:73`) emitting `Path.SwIfIndex` | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` |
| Interface-only next-hop, interface unknown to VPP | -> | `toVPPRoute` rejection path (`backend_vpp_linux.go:91`) | `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Static route with an interface-only next-hop, linux backend, iface backend loaded, device present | Route programs with `route.LinkIndex` set to the resolved ifindex |
| AC-2 | Static route with an interface-only next-hop, linux backend, no iface backend loaded | Diagnosable failure naming the missing iface backend; DESIGN decides whether this stays a runtime error or the config is made to load a backend |
| AC-3 | Static section mixing a good address-next-hop route and an unresolvable interface next-hop | Blast radius is a deliberate, documented choice; today the whole section fails (`inject.go:87-92`) |
| AC-4 | Static route with an interface-only next-hop, VPP backend, interface known to VPP | `toVPPRoute` emits a `staticvpp.Path` with the resolved `SwIfIndex` and the route programs into the VPP FIB |
| AC-5 | Static route with an interface-only next-hop, VPP backend, interface unknown to VPP | Clear error naming the interface; never an index-0 path |
| AC-6 | Address next-hops, ECMP, blackhole, reject, on both backends | Unchanged |
| AC-7 | `test/static/005-table-interface.ci` | Passes, exercising the interface-only next-hop for real rather than working around it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Configures a default route out of a tunnel with no gateway address, linux data plane | config -> parseStaticConfig -> applyRoutes -> buildRoute -> resolveNexthopIndex -> iface.Resolve -> netlink RouteReplace | `test/static/005-table-interface.ci` |
| 2 | Same config on a VPP data plane | config -> applyRoutes -> toVPPRoute -> staticvpp.Path.SwIfIndex -> toFibPath -> VPP FIB | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` |
| 3 | Configures an interface next-hop for an interface that does not exist | config -> resolveNexthopIndex -> iface.Resolve error -> operator-visible message | `test/static/006-interface-nexthop-no-backend.ci` (new) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` | `internal/plugins/static/backend_vpp_linux_test.go` | Interface-only next-hop produces a `Path` with the resolved `SwIfIndex`, replacing the `:113` rejection assertion | proposed |
| `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors` | `internal/plugins/static/backend_vpp_linux_test.go` | An interface unknown to VPP errors and never yields index 0 | proposed |
| `TestToVPPRouteMixedAddressAndInterfaceNextHops` | `internal/plugins/static/backend_vpp_linux_test.go` | ECMP mixing an address next-hop and an interface next-hop | proposed |
| `TestResolveNexthopIndexNoBackendErrorIsDiagnosable` | `internal/plugins/static/backend_linux_test.go` | The no-backend error names the interface and the missing backend | proposed |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Path.SwIfIndex` | 0 to 2^32-1 (`static/vpp/backend.go:28`) | index of a real interface | 0 is the trap value: it must never be emitted for an unresolved name | N/A |
| `Path.Weight` | 0 to 255 after `capWeight` (`backend_vpp_linux.go:104-111`) | 255 | N/A | uint16 inputs above 255 cap to 255 (existing behavior, preserve) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `005-table-interface` | `test/static/005-table-interface.ci` | Named-table route with an interface-only next-hop loads and programs | exists, expected failing (A-5) |
| `006-interface-nexthop-no-backend` | `test/static/006-interface-nexthop-no-backend.ci` | An interface next-hop with no iface backend gives a clear, diagnosable error | proposed |

### Interop Tests

Not applicable. This spec changes no wire protocol behavior: it is data-plane route
programming (netlink and VPP binapi), not a BGP/IPsec/L2TP wire format change.

### Future

None deferred. This is a skeleton; scope is set at DESIGN.

## Files to Modify

Candidates from a code read; confirm during research.

- `internal/plugins/static/backend_vpp_linux.go` - `toVPPRoute` resolves a logical interface
  name to a `SwIfIndex` instead of rejecting (`:73-102`)
- `internal/plugins/static/backend_linux.go` - possibly nothing; depends on A-1. If the
  no-backend case stays an error, improve the message at `:97-103`
- `internal/plugins/iface/vpp/ifacevpp.go` - expose name to `SwIfIndex` resolution across the
  plugin boundary (`resolveIndex`, `:225-238`), subject to R-4
- `internal/plugins/static/inject.go` - only if AC-3 changes the per-route error isolation
  (`:62-93`)
- `test/static/005-table-interface.ci` - make the existing test exercise the path for real
- `plan/learned/650-static-routes.md` - the `// Design:` anchor on both static backends; update
  if backend behavior changes

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No new leaf expected; `next { interface }` already parses | `internal/plugins/static/yang/` |
| Doctor check for runtime dependencies | [ ] Decide at DESIGN: an interface next-hop depends on a loaded iface backend, which is a runtime dependency per `ai/rules/doctor-checks.md` | owning package + `internal/core/diagnostic/codes.go` |
| Functional test | [ ] Yes | `test/static/*.ci` |
| CLI commands/flags | [ ] No | - |
| Prometheus counters | [ ] No | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Decide at DESIGN (a next-hop form starts working) | `docs/features.md` |
| 2 | Config syntax changed? | [ ] Expected No (syntax already exists) | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | [ ] Check static routes guide coverage | `docs/guide/<topic>.md` |
| 12 | Internal architecture changed? | [ ] If a resolver crosses a new boundary | `plan/learned/650-static-routes.md`, `docs/architecture/core-design.md` |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Grep `docs/` for the changed files | per grep |

## Files to Create

- `test/static/006-interface-nexthop-no-backend.ci` - proposed; only if AC-2 keeps the
  no-backend case as a diagnosable runtime error

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

**BLOCKING: this spec is a skeleton. Phase 0 is research via `/ze-spec`; the phases below
are placeholders whose shape depends on the Open Questions.**

1. **Phase 0: Research (`/ze-spec`)**: answer the Open Questions, especially the static
   plugin's process model (A-1) and the real failure of `005-table-interface.ci` (A-5).
   Then rewrite the phases below and move Status to `design`.
2. **Phase: Wiring**: establish the entry point and a failing wiring test per the Wiring
   Test table.
   - Tests: `test/static/005-table-interface.ci`
   - Verify: it fails for the RIGHT reason, and record which error it actually reports.
3. **Phase: Linux leg**: resolve or diagnose the no-backend case per the A-1 answer.
   - Tests: `TestResolveNexthopIndexNoBackendErrorIsDiagnosable`
   - Files: `internal/plugins/static/backend_linux.go`
4. **Phase: VPP leg**: thread name to `sw_if_index` resolution into `toVPPRoute`.
   - Tests: `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex`,
     `TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors`
   - Files: `internal/plugins/static/backend_vpp_linux.go`, `internal/plugins/iface/vpp/ifacevpp.go`
5. **Functional tests**: make `005-table-interface.ci` pass by fixing the product.
6. **Full verification**: `make ze-verify`
7. **Complete spec**: learned summary, two-commit closure.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | An unresolved name NEVER produces `SwIfIndex` 0 (`backend_vpp_linux.go:69-72` rationale) |
| Data flow | Static still resolves via `iface.Resolve` only; no second resolver introduced |
| Registration over hardcoding | Any new resolver crosses the boundary via an existing registry / component seam; no per-backend switch case or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) |
| Doctor checks | If the iface-backend dependency is made explicit, a doctor check is registered per `ai/rules/doctor-checks.md` |
| Rule: no-workarounds | `005-table-interface.ci` passes because the product works, not because the test was weakened |
| Blast radius | The whole-section-failure decision (AC-3) is deliberate and documented |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Interface-only next-hop programs on linux | `test/static/005-table-interface.ci` passes |
| Interface-only next-hop programs on VPP | `TestToVPPRouteInterfaceOnlyNextHopResolvesIndex` passes |
| No index-0 path is reachable | `grep -n "SwIfIndex" internal/plugins/static/backend_vpp_linux.go` plus the unknown-interface test |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A config-supplied interface name reaches a kernel/VPP index lookup; an unknown or malformed name must error, never resolve to index 0 (which would program a wrong path) |
| Error leakage | Error messages name the interface; confirm they carry no unexpected host detail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| `005-table-interface.ci` fails for a third reason not analyzed here | Back to RESEARCH; re-read the producer, do not patch the test |
| VPP leg cannot be proven without live VPP | Identify the test rail per `ai/rules/qemu-testing.md`; do not claim done |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- The deferrals row for the VPP half (2026-07-10, W-3) says the work "needs the iface/vpp
  name to sw_if_index resolver threaded into the static backend". A code read narrows this:
  `staticvpp.Path.SwIfIndex` and `toFibPath` already exist and are tested
  (`static/vpp/translate_test.go:54-62`), so only the parent-side lookup is missing.
- The linux half was recorded as a static problem, but the mechanism is entirely in the
  iface component's process-global backend activation. The static plugin is the victim, not
  the cause.

## Known Limitations

- To be set at DESIGN. Candidate: recursive next-hop resolution is out of scope; this spec
  covers interface-only next-hops.

## Open Questions (research before design)

- Does the `static` plugin run in-process (sharing `activeBackend` with the iface component)
  or as a subprocess (where a loaded iface backend is invisible to it) in a normal `ze` run?
  This single answer decides whether Problem A is a product bug or a test-config bug.
- Does `test/static/005-table-interface.ci` fail today, and with WHICH error: "iface: no
  backend loaded" or a device-absent error for `tun100`? Run it before designing.
- Is `tun100` expected to exist in that test, and who is supposed to create it?
- Should an interface-only next-hop with no loadable iface backend be a config-verify
  rejection (fail early, clearly) or a runtime error? Compare with how the VPP side already
  chose "fail loudly".
- Should one unresolvable next-hop fail the whole static section (today's behavior,
  `inject.go:87-92`) or only its own route? Is that in scope here or a separate spec?
- What is the right boundary for exposing VPP name to `sw_if_index` resolution to the static
  plugin, given `resolveIndex` is an unexported method on `vppBackendImpl` and
  `ai/rules/plugin-self-containment.md` constrains cross-plugin spelling?
- Is `b.names` (the iface/vpp index map) populated at the time static applies its routes, or
  is there an ordering dependency between the iface backend's interface dump
  (`ifacevpp/query.go:194`) and static's first apply?
- Does an interface-only next-hop on VPP need any FibPath flag beyond `SwIfIndex` (for
  example an attached / no-next-hop-address path type), or does a zero `Nh.Address` with a
  set `SwIfIndex` do the right thing?
- Is there an equivalent interface-only next-hop gap in the other route-programming
  consumers of `iface.Resolve` (for example `internal/plugins/ldp/`, which has a sibling
  resolve integration test), and should they be fixed together?
- How is a VPP-backed static route proven at all today, and does a test rail exist for the
  VPP leg?

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A: no wire protocol change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
