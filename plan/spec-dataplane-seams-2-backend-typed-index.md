# Spec: dataplane-seams-2 -- The Resolved Interface Index Carries Its Dataplane (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | `plan/immediate/spec-dataplane-seams-0-umbrella.md` (finding F-2) |
| Phase | - |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`iface.Resolve` returns a `Binding` whose `Ifindex` field is an `int`. When the
netlink backend is active that int is a Linux kernel interface index. When the
VPP backend is active the same field carries a VPP `sw_if_index`. The two are
not interchangeable, and programming one where the other is expected is a bug.

This is by design. The static-interface-nexthops ruling records
the ruling: one resolver serves both dataplanes, the VPP iface backend publishes
its `sw_if_index` through `iface.InterfaceInfo.Index` into `Binding.Ifindex`, and
no second resolution path may be introduced. `iface.ActiveBackendName()` was
added as the guard, and chosen over a config-time pairing check specifically
because `LoadBackend` swaps the backend live at runtime.

**What this spec changes is the enforcement, not the design.** The guard is
currently a function each caller must remember to call. Make the raw index
reachable only through an accessor that names the dataplane the caller is about
to program, and that errors when the active backend is not that dataplane. A
caller then cannot skip the check by forgetting it.

The check stays a runtime check. Which backend is loaded is a runtime fact, so
compile-time safety is not available here and must not be claimed.

**Scope-gate decision (D-2, 2026-08-07, user):** keep one resolver, keep
`ActiveBackendName`, move the check onto the extraction path. Reopening the
one-resolver design was offered and rejected.

### Why this is medium, not small

About thirteen non-test call sites read `Binding.Ifindex`, and exactly one calls
`iface.ActiveBackendName()` before using it. Every other site appears to assume a
kernel index. Enumerating what each site actually wants is the bulk of the work,
and it comes before any type change.

**The table below was built from grep output, not by reading each producing
function. Every row marked unverified must be confirmed against its function
before the design gate (`ai/rules/evidence.md`).**

| Call site | Appears to want | Guards today | Verified |
|-----------|-----------------|--------------|----------|
| `internal/plugins/static/backend_vpp_linux.go` | VPP `sw_if_index` | Yes, `ActiveBackendName() != "vpp"` rejects, and a non-positive index is rejected | Read |
| `internal/plugins/static/backend_linux.go` | kernel ifindex | No | Unverified |
| `internal/plugins/vrrp/register.go` | kernel ifindex | No | Unverified |
| `internal/plugins/vrrp/transport/backend_linux.go` | kernel ifindex | No | Unverified |
| `internal/plugins/iface/ra/sender_linux.go` | kernel ifindex (passed to `net.Interface` and an IPv6 control message) | No | Unverified |
| `internal/plugins/trafficusage/monitor.go` | kernel ifindex | Rejects non-positive only | Unverified |
| `internal/plugins/flowexport/sampling_worker.go` | kernel ifindex (index to name map) | No | Unverified |
| `internal/plugins/flowexport/sampling/tc_linux.go` | kernel ifindex (tc) | No | Unverified |
| `internal/plugins/ospf/transport/backend_linux.go` | kernel ifindex | No | Unverified |
| `internal/plugins/ospf/v3/transport/backend_linux.go` | kernel ifindex | No | Unverified |
| `internal/plugins/isis/transport/backend_linux.go` | kernel ifindex | No | Unverified |
| `internal/component/ike/dataplane/xfrm_linux.go` | kernel ifindex (XFRM policy) | No | Unverified |
| `internal/component/l2tp/pppoe/resolve.go` | kernel ifindex | No | Unverified |

**The open question the enumeration answers.** Every unguarded site above is
Linux-tagged, but a Linux host can load the VPP iface backend. Whether these
sites are reachable at all under a VPP iface backend, and what the correct index
would be there (plausibly the LCP TAP's kernel index rather than the VPP
`sw_if_index`), is not established. Answer it per site before designing the
accessor. If some sites are genuinely unreachable under VPP, say so with
evidence rather than assuming it.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - named by the `// Design:` annotation on `internal/component/iface/backend.go`
- [ ] `ai/rules/evidence.md` - guards fail closed; a zero value must never be a valid-looking answer

### Learned Summaries
- [ ] The static-interface-nexthops ruling (record retired with the learned corpus) - the ruling this spec strengthens
  → Decision: one resolver, two dataplanes, on purpose. No second resolution path.
  → Decision: a runtime accessor was chosen over a config-verify pairing check because `LoadBackend` swaps the backend live. A config-time check cannot replace it.
  → Constraint: a zero or invalid resolved index must be rejected, never emitted. Index 0 is VPP `local0`.
- [ ] `docs/architecture/iface/logical-name-resolution.md` - `Binding` is a pure value type over the backend; no second resolver

### Related Specs
- [ ] `plan/immediate/spec-dataplane-seams-0-umbrella.md` - the parent, finding F-2
- [ ] `plan/spec-finish-vpp-stub.md` - VPP test coverage this spec's tests may depend on

**Key insights:** (minimal context to resume after compaction)
- The design is settled and is not being revisited. Only the enforcement moves.
- The enumeration is the work. Do it before designing the accessor.
- Index 0 is VPP `local0`, so returning zero on failure is a valid-looking wrong answer.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-07)
- [ ] `internal/component/iface/iface.go` - `Binding` is a value type carrying `Ifindex int`, `OsName`, `OperMAC`, `PermMAC`, `MTU`, `State`
- [ ] `internal/component/iface/backend.go` - `RegisterBackend`, `LoadBackend`, `GetBackend`, `ActiveBackendName`, `CloseBackend`. One backend is active at a time and `LoadBackend` swaps it at runtime
- [ ] `internal/plugins/iface/vpp/query.go` - fills `InterfaceInfo.Index` from the VPP `sw_if_index`

**Source files to read before design:**
- [ ] every call site in the enumeration table above
- [ ] `internal/component/iface/resolve.go` - `bindingFromInfo`, the producer of `Binding`

**Behavior to preserve:**
- One resolver. No second resolution path (learned 1185, 950).
- `LoadBackend` swapping the active backend at runtime.
- The existing rejection in `static/backend_vpp_linux.go`: a non-positive index is refused with an actionable error naming the interface.
- Every current caller's resolution semantics. This spec changes how the index is reached, not what it resolves to.

**Behavior to change:**
- How the raw index is obtained. After this spec the check happens on the extraction path rather than in each caller.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin or component holds a logical interface name from config and calls `iface.Resolve`.

### Transformation Path
1. `iface.Resolve` reaches the single active backend.
2. The backend returns an `InterfaceInfo` whose `Index` means whatever that backend's index namespace means.
3. `bindingFromInfo` copies it into `Binding.Ifindex`.
4. The caller uses the int, today with no obligation to state which dataplane it is addressing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| caller ↔ iface component | `iface.Resolve`, returning a value type | No |
| iface component ↔ active backend | `iface.Backend`, one implementation at a time | No |
| caller ↔ kernel | raw socket bind, netlink, tc, XFRM, all expecting a kernel index | No |
| caller ↔ VPP | binary API, expecting a `sw_if_index` | No |

### Integration Points
- `internal/component/iface` - where `Binding` and the accessor live
- every call site in the enumeration table

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every unguarded call site wants a kernel index | Read from grep output only, not from the producing functions | The accessor's per-site migration is wrong and mis-programs an index | Read each function named in the enumeration table | unvalidated |
| A-2 | Some unguarded sites are unreachable under a VPP iface backend, which is why they have never been wrong in practice | Not established | Those sites are live bugs today, and this spec is a fix rather than a hardening | Determine per site whether a VPP iface backend can reach it, with evidence | unvalidated |
| A-3 | An accessor can be introduced without changing `iface.Resolve`'s signature or adding a second resolver | Learned 1185 and 950 forbid a second resolver; the accessor sits on the returned value | The change conflicts with a standing decision and needs the user | Design review against both learned summaries | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The migration touches thirteen call sites across nine plugins and grows past its scope | The change list exceeds the enumeration table | Land the accessor first with the bare field retained, then migrate sites one at a time, each with its own test |
| R-2 | The accessor returns zero on the error path and a caller ignores the error | A VPP route programs against `local0` | The accessor must return an error and a zero value together, and the existing non-positive rejection must be kept as a second line |
| R-3 | The enumeration finds a genuine live bug, which is then out of this spec's stated scope | A site provably mis-programs an index under a supported configuration | Fix it. A known wrong index is a defect this spec is the entry point for (`ai/rules/completion.md`), not a deferral |
| R-4 | Removing the bare field breaks out-of-tree plugin code using the SDK | `pkg/ze` or `pkg/plugin` exposes `Binding` | Check whether `Binding` is part of the public SDK before removing the field |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An interface index is programmed into the wrong dataplane. On VPP, index 0 is `local0`, so a wrong index can look valid and silently attach state to the wrong interface |
| How is it reverted? | Single-commit revert while the bare field is retained. Harder once call sites are migrated and the field is removed |
| Who else touches this path? | `spec-finish-vpp-stub` (VPP test coverage), and any spec touching a plugin in the enumeration table |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill at design) a static route with an interface-only next-hop, VPP backend active | → | (fill at design) the VPP accessor | (fill at design) |
| (fill at design) the same config with the netlink backend active | → | (fill at design) the kernel accessor | (fill at design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A caller asks a resolved binding for a VPP index while the netlink backend is active | It receives an error naming the mismatch, and no index it could use by accident |
| AC-2 | A caller asks for a kernel index while the VPP backend is active | It receives an error naming the mismatch |
| AC-3 | The resolved index is zero or negative | The accessor errors regardless of which dataplane was asked for. Index 0 is VPP `local0` |
| AC-4 | The repository is searched for direct reads of the bare index field outside the iface component | None remain, or each remaining one is listed here with a reason |
| AC-5 | An interface-only next-hop is configured on each backend in turn | Routes program exactly as they did before this spec. No resolution semantics change |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a static route whose next-hop names an interface, on a VPP dataplane | config → static → `iface.Resolve` → VPP accessor → VPP binary API | (fill at design) |
| 2 | Does the same on a kernel dataplane | config → static → `iface.Resolve` → kernel accessor → netlink | (fill at design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill at design) | `internal/component/iface/*_test.go` | The accessor errors on backend mismatch | |
| (fill at design) | `internal/component/iface/*_test.go` | The accessor errors on a zero or negative index for both dataplanes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| resolved index | (fill at design; note index 0 is VPP `local0`) | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `static-table-interface` (existing, must not regress) | `test/static/static-table-interface.ci` | An interface-named next-hop still programs | |
| `static-interface-nexthop-no-backend` (existing, must not regress) | `test/static/static-interface-nexthop-no-backend.ci` | The no-backend case still fails with an actionable error | |
| `vpp-iface-create`, `vpp-fib-route` (existing, must not regress) | `test/vpp/vpp-iface-create.ci`, `test/vpp/vpp-fib-route.ci` | Interface and route programming still work on VPP | |
| new: backend mismatch is refused | `test/static/*.ci` | Asking for a VPP index under a kernel backend fails with a message naming both, and programs nothing | |

## Files to Modify
- `internal/component/iface/iface.go` - `Binding` and the accessor
- `internal/component/iface/resolve.go` - `bindingFromInfo`, to record which backend produced the index
- every call site in the enumeration table, one at a time
- `docs/features/interfaces.md` - the resolution contract

## Files to Create
- `test/static/*.ci` - the backend-mismatch functional test (name at design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf. The check is runtime by design (learned 1185) |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/static/*.ci`, named above |
| Pipe completeness | N-A | No new CLI output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency. `doctor-static-interface-nexthop-no-backend` already exists from learned 1185 and must keep passing |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Operator-visible behavior is unchanged |
| 2 | Config syntax changed? | N-A | No syntax change |
| 3 | CLI command added/changed? | N-A | No command change |
| 4 | API/RPC added/changed? | N-A | No API change |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it documents interface resolution for plugin authors |
| 6 | Has a user guide page? | N-A | Not user-facing |
| 7 | Wire format changed? | N-A | No wire format change |
| 8 | Plugin SDK/protocol changed? | Yes | Check whether `Binding` is exposed via `pkg/ze` or `pkg/plugin` before changing it (see R-4) |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation touched |
| 10 | Test infrastructure changed? | N-A | Uses existing `.ci` infrastructure |
| 11 | Affects daemon comparison? | N-A | No externally comparable behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md` |
| 13 | Route metadata keys added/changed? | N-A | No route payload change |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `iface/iface.go`, `iface/resolve.go`, and each migrated call site |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check interface-resolution examples in `docs/features/interfaces.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the failing functional test that asks for the wrong dataplane's index and expects a refusal.
2. **Phase: enumerate** -- read each call site in the table and record what it wants and whether it is reachable under a VPP iface backend. Replace every `Unverified` with evidence. This phase gates the rest.
3. **Phase: accessor** -- record the producing backend on the binding and add the per-dataplane accessors, failing closed on mismatch and on a non-positive index. Keep the bare field for now.
4. **Phase: migrate** -- move call sites to the accessor, one commit per site or per plugin, each with a test.
5. **Phase: remove the bare field** -- only after every site is migrated, and only if R-4 clears.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every row of the enumeration table is verified, and every site is migrated or listed as a deliberate exception with a reason |
| Correctness | The accessor errors on mismatch and on a non-positive index, and never returns a usable value alongside an error |
| Data flow | No second resolution path was introduced (learned 1185, 950) |
| Naming | The accessor names the dataplane the caller is addressing, so the call site reads as a statement of intent |
| Rule: `ai/rules/evidence.md` | The guard fails closed. Index 0 is VPP `local0`, so zero is never a valid-looking answer |
| Rule: `ai/rules/completion.md` | Any live bug the enumeration finds is fixed, not recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No direct read of the bare index outside the iface component | `grep -rn "\.Ifindex" internal/ --include=*.go` |
| Every enumeration row verified | No `Unverified` remains in the table |
| The learned-1185 doctor check still passes | Run the check named in that summary |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed guard | The accessor must not return a usable index alongside an error. A caller that ignores the error must get something unusable, not `local0` |
| Authorization that could fail open | None; this guard is about correctness, not access control |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Move the guard onto the extraction path, keep one resolver | Reopen "one resolver, two dataplanes"; accept the risk and change nothing | The design is sound and the runtime backend swap that motivated it still happens. What is weak is that the guard must be remembered. Recorded at the scope gate as D-2, 2026-08-07 |

## Known Limitations
- The check remains a runtime check. Which backend is loaded is a runtime fact, so compile-time safety is not available and must not be claimed for this change.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
