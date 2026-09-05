# Spec: dataplane-seams-3 -- Egress Interface on the Route Event (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/immediate/spec-dataplane-seams-0-umbrella.md` (finding F-3) |
| Phase | - |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`BestChangeEntry` and `ECMPPath` in
`internal/component/sysrib/events/events.go` carry no egress-interface field.
Routes cross that seam as a next-hop IP only. A route that points at an
interface rather than at a next-hop address cannot be expressed on it.

**Read the open question below before designing anything.** Whether this is a
gap depends on a fact this spec has not established.

### The open question, and why it comes first

`internal/plugins/static` does not import `sysrib`. It declares its own
`routeBackend` interface in `internal/plugins/static/backend.go` with two
implementations, netlink and VPP, and it programs the dataplane through them
directly. Separately, `internal/plugins/static/inject.go` emits `redistevents`
route changes, which is how sysrib and BGP redistribution learn about static
routes.

So there appear to be two route-programming paths:

| Path | Programs via | Can name an egress interface |
|------|--------------|------------------------------|
| static's own backend | `routeBackend` in `internal/plugins/static/` | Yes. `test/static/static-table-interface.ci` and `test/static/static-interface-nexthop-no-backend.ci` exercise interface-only next-hops |
| sysrib merged best path | `(system-rib, best-change)` → `fibkernel` / `fibvpp` | No. There is no field for it |

**None of the following is established, and each changes what this spec should
do.** Answer them in the research phase, by reading the producing functions,
before writing any acceptance criteria.

| Q | Question | Why it changes the design |
|---|----------|---------------------------|
| Q-1 | Does `fibkernel` also program static routes from the merged best path, or does it skip routes whose protocol is static? | If both program, routes are installed twice and the real defect is duplication, not a missing field |
| Q-2 | Which protocols actually need an interface-scoped route on the `BestChangeEntry` path? | If none do today, adding the field is speculative and the spec should be cancelled or narrowed |
| Q-3 | Is the two-path arrangement deliberate? | Check `plan/learned/` and `plan/immediate/spec-fib-depth.md` before treating it as a defect |
| Q-4 | Would a future subscriber or access feature need per-session routes on the merged path? | This is the strongest argument for the field, and it is a forward-looking one, not a present defect |

**Cancelling this spec is a legitimate outcome.** If the answer to Q-2 is "none",
say so with evidence and close it rather than adding an unused field to a payload
external plugin processes decode.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/meta/README.md` - route metadata keys and their documented values
- [ ] `docs/architecture/api/process-protocol.md` - what external FIB plugin processes are promised
- [ ] `ai/rules/architecture.md` - data flow tracing, and not duplicating an existing path

### Related Specs
- [ ] `plan/immediate/spec-dataplane-seams-0-umbrella.md` - the parent, finding F-3
- [ ] `plan/immediate/spec-fib-depth.md` - **in-progress, owns FIB programming depth.** Read its current state before starting. Its own header warns that its Current Behavior table is stale
- [ ] `plan/spec-dataplane-seams-2-backend-typed-index.md` - if the field carries an interface index rather than a name, it inherits that spec's problem

**Key insights:** (minimal context to resume after compaction)
- There appear to be two route-programming paths. Establish their relationship before treating the missing field as a defect.
- This changes a payload external plugin processes decode, so compatibility is part of the design, not an afterthought.
- Prefer a logical interface name over an index. An index on this seam would carry the backend-namespace problem child 2 exists to fix.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-07)
- [ ] `internal/component/sysrib/events/events.go` - `BestChangeEntry` carries action, prefix, next-hop, protocol, labels, route type, metric, table id, SRv6 SID, ECMP paths and backup paths. `ECMPPath` carries next-hop, weight and labels. Neither has an interface field
- [ ] `internal/plugins/static/backend.go` - declares `routeBackend` with apply, remove, list and close; no sysrib import in the package
- [ ] `internal/plugins/static/inject.go` - emits `redistevents` route changes

**Source files to read before design:**
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - what it programs, and whether it filters by protocol
- [ ] `internal/plugins/fib/vpp/fibvpp.go` - the same question for VPP
- [ ] `internal/component/sysrib/` - the merge and best-path selection, and what it does with a static-sourced route
- [ ] `internal/plugins/static/backend_linux.go`, `backend_vpp_linux.go` - how an interface-only next-hop is programmed today

**Behavior to preserve:**
- Interface-only next-hops on static routes keep working on both dataplanes. `test/static/static-table-interface.ci` and `test/static/static-interface-nexthop-no-backend.ci` must not regress.
- The JSON shape external FIB plugin processes already decode, unless the compatibility decision explicitly versions it.

**Behavior to change:**
- To be determined by the open questions above. Possibly nothing.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A route source publishes a route: config for static, the wire for BGP, an SPF result for an IGP.

### Transformation Path
1. Sources emit `redistevents` route changes.
2. `sysrib` merges them, applies administrative distance and selects a best path.
3. `sysrib` emits `(system-rib, best-change)` carrying `BestChangeBatch`, per family.
4. `fibkernel`, `fibvpp` and `fibp4` consume it.
5. Separately, static programs its own dataplane backend directly.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| route source ↔ sysrib | `redistevents` | No |
| sysrib ↔ in-process FIB plugin | Go typed payload | No |
| sysrib ↔ external FIB plugin process | JSON | No |
| static ↔ its own backend | package-private `routeBackend` | No |

### Integration Points
- `internal/component/sysrib/events` - where the field would be added
- `internal/plugins/fib/kernel`, `internal/plugins/fib/vpp` - where it would be consumed
- `internal/plugins/static` - the parallel path the open questions are about

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
| A-1 | The absence of an interface field is a gap rather than a deliberate boundary | Not established. It may be deliberate, given static programs its own backend | The spec should be cancelled instead of implemented | Answer Q-1 through Q-4 by reading the producing functions | unvalidated |
| A-2 | Some protocol on the merged path needs an interface-scoped route | Not established | The field would be added unused, on a payload external consumers decode | Answer Q-2 | unvalidated |
| A-3 | A logical interface name is the right carrier, not an index | Child 2 shows an index on a shared seam carries a backend-namespace problem | An index reintroduces that problem on a second seam | Design review against child 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The field is added, external plugins ignore it, and interface-scoped routes silently program as next-hop routes | An external FIB plugin installs the route without the interface | Decide the compatibility story first: additive and optional, or versioned |
| R-2 | The work collides with `spec-fib-depth`, which is in-progress on the same surface | Both specs edit `sysrib` | Read that spec's current state first and agree the split before starting |
| R-3 | The field is added speculatively for future subscriber work that never arrives | No consumer exists at closure | Q-2 gates the work. No consumer means no field |
| R-4 | Adding the field papers over a duplicate-programming defect found by Q-1 | Routes appear twice in the kernel table | If Q-1 finds duplication, fix that first. It is the larger defect (`ai/rules/completion.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes program against the wrong interface, or program twice. External FIB plugin processes can silently drop the interface |
| How is it reverted? | Not cleanly, once an external plugin has seen the new field. That is why the compatibility decision comes before the code |
| Who else touches this path? | `spec-fib-depth` (in-progress), `spec-dataplane-seams-1` (same payload) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill after the open questions are answered) | → | (fill at design) | (fill at design) |

## Acceptance Criteria

<!-- Deliberately not drafted. The open questions decide whether there is a
     change to make at all, and drafting ACs first would presume the answer. -->

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-0 | The research phase completes | Q-1 through Q-4 are each answered with the producing function named, and the spec either proceeds with drafted ACs or is cancelled with the reason recorded in the umbrella's findings table |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (fill after the open questions are answered) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill at design) | `internal/component/sysrib/events/*_test.go` | The JSON encoding of the new field, and its absence on routes that do not use it | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (N/A if the field carries a name rather than an index) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `static-table-interface` (existing, must not regress) | `test/static/static-table-interface.ci` | An interface-only next-hop still programs | |
| `static-interface-nexthop-no-backend` (existing, must not regress) | `test/static/static-interface-nexthop-no-backend.ci` | The no-backend case still fails with an actionable error | |
| `vpp-fib-route`, `vpp-fib-route-lookup` (existing, must not regress) | `test/vpp/vpp-fib-route.ci`, `test/vpp/vpp-fib-route-lookup.ci` | Routes still program and resolve on VPP after the payload changes | |
| `isis-route-install`, `ospf-route-install` (existing, must not regress) | `test/isis/isis-route-install.ci`, `test/ospf/ospf-route-install.ci` | IGP routes still reach the FIB | |
| new: interface-scoped route on the merged path | `test/static/*.ci` (only if Q-2 finds a consumer) | A route naming an egress interface reaches the FIB with that interface attached | |

## Files to Modify
- `internal/component/sysrib/events/events.go` - the field, if the open questions justify it
- `internal/plugins/fib/kernel/`, `internal/plugins/fib/vpp/` - consumption
- `docs/architecture/meta/README.md` - published payload
- `docs/architecture/api/process-protocol.md` - the external contract

## Files to Create
- `test/static/*.ci` - only if a consumer exists (name at design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The config surface for interface-scoped routes already exists in static |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/static/*.ci`, if a consumer exists |
| Pipe completeness | N-A | No new CLI output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | No new operator-facing capability; static already supports interface next-hops |
| 2 | Config syntax changed? | N-A | No syntax change |
| 3 | CLI command added/changed? | N-A | No command change |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if the route payload is documented there |
| 5 | Plugin added/changed? | N-A | No plugin added |
| 6 | Has a user guide page? | N-A | Not user-facing |
| 7 | Wire format changed? | N-A | No protocol wire format change; this is the plugin payload, row 8 |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation touched |
| 10 | Test infrastructure changed? | N-A | Uses existing `.ci` infrastructure |
| 11 | Affects daemon comparison? | N-A | No externally comparable behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` if it describes the route-programming paths |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | The event type is unchanged; only its payload gains a field |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `sysrib/events/events.go` and the fib plugins |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check documented route-payload examples |

## Implementation Steps

1. **Phase: research (gates everything)** -- answer Q-1 through Q-4 against the producing functions. Record each answer with the file and symbol. If Q-2 finds no consumer, cancel the spec and record the reason in the umbrella's findings table.
2. **Phase: coordinate** -- read `spec-fib-depth`'s current state and agree the split before touching `sysrib`.
3. **Phase: Wiring (MANDATORY FIRST for the code work)** -- write the failing functional test for an interface-scoped route on the merged path.
4. **Phase: compatibility** -- decide additive-optional or versioned, before adding the field.
5. **Phase: field and consumption** -- add it, consume it in both FIB backends, update the published payload docs in the same commit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every open question answered with a named producing function, not an inference |
| Correctness | A route with no interface behaves exactly as before |
| Data flow | The field does not create a second way to say what a next-hop already says |
| Naming | The field carries a logical interface name, not a backend index (see child 2) |
| Rule: `ai/rules/completion.md` | If Q-1 finds duplicate programming, that is fixed rather than recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Open questions answered | Each Q row names a file and symbol |
| Published payload matches code | Compare `docs/architecture/meta/README.md` against the struct |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An interface name arriving on the event must be validated before it reaches a backend, exactly as `iface.Backend` requires of its own callers |
| Fail-closed | A route naming an interface that cannot be resolved must be refused, not programmed without the interface |

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

## Known Limitations
- This spec may correctly close as cancelled. A field added to a published payload with no consumer is a cost with no benefit.

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
