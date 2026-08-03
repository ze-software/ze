# Spec: improve-2 -- Operational-State Providers for gNMI Get

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `internal/component/gnmi/get.go` -- current Get implementation
5. `internal/component/plugin/registry/registry.go` -- where providers would register

## Task

Ze's gNMI Get is a serialization view of the running config tree: it walks the config
tree and returns leaves/containers, ignoring the request's DataType field entirely. A
gNMI client asking for STATE or OPERATIONAL data gets config or nothing. Yet Ze has
rich operational state (BGP peers, RIB, interfaces, health) reachable through plugin
RPCs and show commands; it is simply not addressable through gNMI paths.

Make operational state a first-class northbound contract: an operational-state
provider registry keyed by YANG path/module, where plugins register state providers
through the existing plugin registry. gNMI Get honors DataType: CONFIG serves the
config tree (today's behavior), STATE/OPERATIONAL fan out to matching providers, ALL
merges provider state with config. gNMI Subscribe machinery is kept and untouched; it
is a Ze strength to preserve.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - gNMI server design
  → Constraint: (fill during design)
- [ ] `ai/rules/plugins.md` - providers must self-register
  → Constraint: gnmi core discovers providers; no per-plugin switch in the gnmi package
- [ ] `ai/rules/plugins.md` - how gnmi may call plugin state producers
  → Constraint: (fill during design -- in-process callback vs RPC dispatch)

### RFC Summaries (MUST for protocol work)
- Not wire-protocol work; gNMI spec sections cited inline in code per existing style.

**Key insights:**
- The plugin Registration struct already carries in-process callbacks (decoder,
  verifier); a state-provider hook follows an established pattern.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/gnmi/get.go` - `Get` reads the running config tree only (:21-26); walks it via `walkTree` (:77) and encodes leaves as StringVal, containers as JSON (:104-120); `req.GetType()` never consulted
- [ ] `internal/component/gnmi/capabilities.go` - advertises JSON_IETF encoding only (:20-24)
- [ ] `internal/component/gnmi/subscribe.go` - `ChangeNotifier` streams config change events (:23-27); operational data not in scope of Subscribe today
- [ ] `internal/component/plugin/registry/registry.go` - Registration fields for in-process callbacks (:76-102) -- the pattern a state-provider hook would follow
- [ ] `internal/component/gnmi/show.go` - `handleShowGNMI` is server status only (:19-31), not a state provider

**Behavior to preserve:** (unless user explicitly said to change)
- Get with DataType CONFIG (and unset) returns exactly what it returns today.
- Subscribe machinery (config change notifications) untouched.
- Capabilities keeps JSON_IETF; new encodings are out of scope.

**Behavior to change:** (only if user explicitly requested)
- Get with STATE/OPERATIONAL/ALL stops silently serving config-only data.

## Data Flow (MANDATORY)

### Entry Point
- gNMI GetRequest over gRPC with `type` = CONFIG | STATE | OPERATIONAL | ALL and one or
  more YANG paths.

### Transformation Path
1. `Get` parses paths as today (`get.go`).
2. DataType CONFIG: existing config-tree walk, unchanged.
3. DataType STATE/OPERATIONAL: the path is matched against the provider registry; each matching provider returns its state subtree; results are encoded as TypedValue.
4. DataType ALL: provider state fetched, config subtree fetched, merged (state wins on leaf conflicts, per gNMI convention decided during design).
5. Providers are registered by plugins at init/startup with the YANG paths they own.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gnmi ↔ provider registry | path-prefix lookup, core discovers registered providers | [ ] |
| gnmi ↔ plugin state producers | in-process callback or RPC dispatch (design decision) | [ ] |
| config tree ↔ state merge | merge only for DataType ALL | [ ] |

### Integration Points
- `registry.Registration` - carries the state-provider registration.
- `Get` (`get.go`) - dispatch point on `req.GetType()`.
- Existing show/RPC state producers (BGP summary, interface state) - first providers.

### Architectural Verification
- [ ] No bypassed layers (gnmi calls providers through the registry, not plugin packages)
- [ ] No unintended coupling (gnmi package imports no plugin implementation)
- [ ] No duplicated functionality (providers wrap existing state producers)
- [ ] Registration over hardcoding -- providers register; no path switch inside gnmi (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Existing state producers can render YANG-pathed subtrees without new per-plugin YANG state modules | plugins already emit structured JSON for show commands | Need state YANG modules first; scope grows substantially | Design-phase survey of 2-3 candidate providers (BGP peers, interfaces) | unvalidated |
| A-2 | Path-prefix matching is enough to route a Get path to a provider | simplest routing model consistent with registry lookup patterns | Need full schema-aware routing | Prototype with two providers during design | unvalidated |
| A-3 | State fetch latency is acceptable synchronously in the Get handler | current show commands respond interactively | Need timeouts/partial results per provider | Measure during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | State paths drift from config paths (two naming schemes) | provider registration reviews | Anchor state paths to the same YANG modules as config; document convention |
| R-2 | A slow/hung provider stalls Get for all paths | latency in functional tests | per-provider deadline; return partial with error annotation (design decision) |
| R-3 | Scope creep toward full OpenConfig state modeling | design review | explicitly out of scope: Ze YANG paths only, no OpenConfig mapping |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GetRequest type=STATE for a registered path | → | provider registry lookup + provider fanout | TestGetStateProviderFanout |
| GetRequest type=ALL | → | config + state merge | test/plugin/gnmi-state.ci |
| Plugin startup | → | state-provider registration discovered by gnmi | TestStateProviderRegistration |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Get type=CONFIG | Identical output to today (regression-locked) |
| AC-2 | Get type=STATE on a provider-owned path | Provider state returned as TypedValue |
| AC-3 | Get type=STATE on a path with no provider | NotFound (not config data) |
| AC-4 | Get type=ALL | Config and state merged per documented rule |
| AC-5 | Two providers registered on disjoint paths | Each serves only its paths |
| AC-6 | Provider exceeds its deadline | Get fails (or partial per design) without hanging the server |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Monitoring polls BGP peer state via gNMI | GetRequest STATE -> registry -> BGP state provider -> TypedValue | test/plugin/gnmi-state.ci |
| 2 | Operator fetches config + state in one call | GetRequest ALL -> merge | test/plugin/gnmi-state.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestGetStateProviderFanout | `internal/component/gnmi/get_test.go` | STATE routes to providers | |
| TestGetAllMergesConfigAndState | `internal/component/gnmi/get_test.go` | ALL merge rule | |
| TestStateProviderRegistration | `internal/component/plugin/registry/registry_test.go` | registration + lookup | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| provider deadline | (fill during design) | (fill) | (fill) | (fill) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| gnmi-state | `test/plugin/gnmi-state.ci` | gNMI client gets BGP/interface state via STATE and ALL | |

### Interop Tests (MANDATORY for protocol features)
- N/A for wire protocols; gNMI client interop (gnmic) exercised in the functional test.

## Files to Modify
- `internal/component/gnmi/get.go` - DataType dispatch + merge
- `internal/component/plugin/registry/registry.go` - state-provider registration field + lookup
- First provider plugins (BGP peer state, interface state) - register providers

## Files to Create
- `internal/component/gnmi/state.go` - provider fanout + merge
- `test/plugin/gnmi-state.ci` - functional test

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - registration field + lookup + failing wiring tests
2. **Phase: Get DataType dispatch** - CONFIG regression-locked, STATE fanout
3. **Phase: ALL merge + deadlines**
4. **Phase: first two real providers** (BGP peers, interfaces)
5. Functional tests, `make ze-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 with file:line |
| Correctness | CONFIG output byte-identical to before; merge rule documented |
| Registration over hardcoding | no per-plugin path switch in gnmi (`ai/rules/plugins.md`) |
| Data flow | gnmi -> registry -> provider only; no plugin imports in gnmi |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | path parsing already hardened; provider inputs bounded |
| Resource exhaustion | provider deadlines; response size on wide paths |
| Information exposure | state visible through gNMI matches what show commands already expose under the same auth |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Provider registry keyed by YANG path | per-protocol state trees in gnmi package | registration over hardcoding; plugins own their state |

## Known Limitations
- No OpenConfig model mapping; Ze YANG paths only (revisit separately if needed).

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
