# Spec: show-enricher

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 (Close) |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/patterns/registration.md` - registration pattern
4. `internal/core/cos/cos.go` - precedent leaf registry
5. `internal/component/subscriber/cmd/subscriber.go` - first consumer
6. `internal/component/command/registry/registry.go` - existing command registry pattern

## Task

Generic show-enricher registry: a leaf package in `internal/core/show/` where plugins register enrichers keyed by command name. Any show command handler (CLI, web, API) calls `Enrich()` to merge plugin-contributed data into its base output. Enrichers receive the base `map[string]any` and modify it in place. Supports brief (list) and detail (single-entity) variants. External plugins register enrichers via their in-process `register.go` init(), same pattern as doctor checks.

First consumer: `show subscriber detail` enriched by CoS plugin with profile name and maps. `show subscriber` (list) enriched with brief CoS info.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - registration architecture
  -> Constraint: all registries follow init() + registry + blank import pattern; read-only after init
  -> Constraint: leaf package guarantee -- registry imports only stdlib so any owner can import from init()
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  -> Constraint: removing a plugin removes ALL its features; enricher registrations must vanish with plugin removal
- [ ] `ai/rules/plugin-design.md` - proximity principle
  -> Constraint: enricher code belongs in the plugin that owns the data, not in the show command owner
- [ ] `ai/rules/json-format.md` - JSON key conventions
  -> Constraint: all JSON keys must be lowercase kebab-case
- [ ] `ai/rules/design-context.md` - check core/ first
  -> Decision: registry goes in `internal/core/show/`, not in `command/registry/` (broader than CLI)
- [ ] `docs/architecture/core-design.md` - core architecture
  -> Constraint: core packages are leaf packages with no component/plugin imports

### Learned Summaries
- [ ] `plan/learned/760-subscriber-session-model.md` - subscriber session model
  -> Decision: subscriber.DefaultRegistry is the singleton for all access types; show commands query it
  -> Constraint: Session struct has basic fields; extended data (CoS, RADIUS attrs) lives in plugin-owned stores
- [ ] `plan/learned/884-cos-plugin.md` - CoS plugin
  -> Decision: shared registry in `internal/core/cos/`; config verifier runs before iface verifier
  -> Constraint: CoS per-session state is in `sync.Map` in `internal/plugins/cos/session_state.go`
- [ ] `plan/learned/849-command-surface-ownership.md` - command ownership
  -> Decision: each command lives with the package that owns the behavior it exposes
  -> Constraint: handler in `<owner>/cmd` with init() calling pluginserver.RegisterRPCs
- [ ] `plan/learned/845-plugin-self-containment.md` - self-containment rule
  -> Constraint: shared dispatch carries selector scope, never command spelling

**Key insights:**
- Three surfaces serve operational data: CLI (RPC dispatch), web (dispatch or service locator), API/LG (dispatch + transform). CLI enrichment propagates to web dispatch and API for free.
- Dispatcher is single-handler per command; enrichment happens inside the handler, not at dispatch.
- `internal/core/cos/cos.go` is the exact precedent: shared leaf registry, Register/Lookup/Clear, plugins import from init().
- External plugins register enrichers via their in-process `register.go` init(), accessing shared in-process state (cos registry, session metadata store).
- Enrichers receive base `map[string]any` and mutate it in place; no separate args contract needed.
- Registration key convention: the CLI path with selectors stripped. For `show subscriber id <id> detail` (YANG path `show/subscriber/id/detail`, wire method `ze-subscriber-api:detail`), the registration key is `"show subscriber detail"`. Selectors are runtime values, not part of the command identity. This matches how handlers identify their own command conceptually.
- Enricher call ordering: alphabetical by key, consistent with `registry.All()` sort behavior elsewhere. Deterministic output for `| json` and debuggability.
- Package name `internal/core/show/` shares the name `show` with `internal/component/cmd/show/` (central show verb). No practical collision: enricher registry is imported by handler packages (subscriber/cmd, cos plugin), central verb code imports cmd/show. Different consumers, no aliasing needed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/cos/cos.go` - shared CoS profile registry (Register, Lookup, All, Clear, ResolverFunc)
  -> Constraint: leaf package, imports only stdlib + strings; precedent for new core registries
- [ ] `internal/component/subscriber/cmd/subscriber.go` - show subscriber handlers (handleSummary, handleDetail)
  -> Constraint: builds map[string]any from subscriber.Session; returns via plugin.Map() or plugin.RawJSON()
  -> Decision: handleSummary calls sessionBrief() per session; handleDetail calls sessionFull() for one session
- [ ] `internal/plugins/cos/register.go` - CoS plugin registration and show command
  -> Constraint: external plugin pattern: OnExecuteCommand callback, SDK protocol
  -> Decision: showProfiles() returns slice of profileView structs for show class-of-service
- [ ] `internal/plugins/cos/handler.go` - dynamic CoS handler (session-up/down/CoA events)
  -> Constraint: accesses l2tp.LoadSessionMetadata() for CoS profile name; accesses coreCos.Lookup() for maps
- [ ] `internal/plugins/cos/session_state.go` - per-session CoS state
  -> Constraint: sync.Map keyed by sessionKey{tunnelID, sessionID}; stores profileName + static maps
- [ ] `internal/component/command/registry/registry.go` - command registry (leaf package)
  -> Constraint: imports only stdlib + textbuf; leaf guarantee for all registries
- [ ] `internal/component/plugin/server/rpc_register.go` - RPC registration
  -> Constraint: RegisterRPCs() from init(); single handler per WireMethod
- [ ] `internal/component/plugin/server/command.go` - dispatcher
  -> Constraint: Dispatch() finds longest match, returns single plugin.Response; no chaining

**Behavior to preserve:**
- `show subscriber summary` and `show subscriber detail` existing output format unchanged (enrichers add new keys, never remove existing ones)
- `show class-of-service` continues to work independently
- Plugin self-containment: removing CoS plugin leaves subscriber show commands working (just without CoS data)
- All existing pipe operators (json, yaml, table, etc.) work on enriched output

**Behavior to change:**
- `show subscriber detail` gains CoS enrichment data when CoS plugin is loaded
- `show subscriber` (list) gains brief CoS data when CoS plugin is loaded
- Web handlers displaying subscriber data gain the same enrichments

## Data Flow (MANDATORY)

### Entry Point
- CLI user types `show subscriber id X detail`
- Web user navigates to subscriber detail page
- API client requests subscriber session via LG or API endpoint

### Transformation Path
1. CLI: dispatcher matches `show subscriber id X detail` to handleDetail handler
2. Handler builds base `map[string]any` from `subscriber.Session` via `sessionFull()`
3. Handler calls `show.Enrich("show subscriber detail", base)` -- NEW
4. Each registered enricher receives base map, reads session identifiers from it, writes enrichment data
5. Handler returns enriched map via `plugin.Response`
6. Client applies pipe formatting (json, yaml, table)

For web (command dispatch path):
- Steps 1-5 happen in the CLI handler; web dispatches the same command and gets enriched JSON

For web (service locator path):
- Web handler builds its own data map from service locator
- Web handler calls `show.Enrich("show subscriber detail", base)` explicitly
- Template renders enriched data

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Core registry <- Plugin | init() registration via blank import | [ ] |
| Handler -> Core registry | show.Enrich() call in handler | [ ] |
| Enricher -> Plugin state | enricher func accesses plugin's in-process stores (cos sync.Map, l2tp metadata) | [ ] |

### Integration Points
- `internal/component/subscriber/cmd/subscriber.go:handleDetail` - calls show.Enrich() after building base map
- `internal/component/subscriber/cmd/subscriber.go:handleSummary` - calls show.EnrichBrief() per session map
- `internal/plugins/cos/register.go:init()` - registers CoS enricher for "show subscriber detail" and "show subscriber"

### Architectural Verification
- [ ] No bypassed layers (enrichers registered at init, called at show time)
- [ ] No unintended coupling (enricher package is leaf; plugins import it, not the reverse)
- [ ] No duplicated functionality (extends existing show pattern, doesn't recreate)
- [ ] Zero-copy preserved where applicable (in-place map mutation, no serialization)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | CoS per-session state is accessible from in-process code via sessionStore sync.Map | `internal/plugins/cos/session_state.go` uses package-level sync.Map | Enricher can't access CoS state; need exported accessor | grep for sessionStore usage; check exports | confirmed: sessionStore is package-level; enricher.go in same package accesses it directly |
| A-2 | subscriber.Session contains tunnel-id and session-id fields that CoS enricher can use to look up state | `internal/component/subscriber/session.go` Session struct | Enricher can't correlate subscriber session to CoS state | read Session struct fields | confirmed: Session has TunnelID/SessionID uint16; sessionFull() writes "tunnel-id"/"session-id" |
| A-3 | Enricher registry is write-once-read-many (all registrations complete before any Enrich() call) | `ai/patterns/registration.md` "read-only after init" | Need runtime locking for concurrent registration | verified by startup ordering | unvalidated |
| A-4 | Web handlers that dispatch CLI commands get enriched output for free | web handlers parse dispatch JSON response | Web handler might not parse/display new enricher keys | read web handler code for subscriber page | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Enricher key collision (two plugins write same key) | Test with multiple enrichers registered | Registry rejects duplicate keys per command at registration time |
| R-2 | Enricher panics or corrupts base map | Test with nil map, empty map, enricher that writes wrong types | Document contract: enricher must not delete base keys; add tests |
| R-3 | Performance: many enrichers on a list view with hundreds of sessions | Benchmark with 500 sessions, 3 enrichers | Brief enrichers should be O(1) lookups; document performance expectation |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show subscriber id X detail` CLI command | -> | `show.Enrich()` in handleDetail | `TestSubscriberDetailCallsEnrich` |
| `show subscriber` CLI command (list) | -> | `show.EnrichBrief()` in handleSummary | `TestSubscriberSummaryCallsEnrichBrief` |
| CoS plugin init() | -> | `show.Register()` enricher registration | `TestCoSRegistersSubscriberEnricher` |
| Plugin removal | -> | no enricher present, base map unchanged | `TestEnrichNoEnrichersIsNoop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show.Register("show subscriber detail", ...)` called from plugin init() | Enricher is stored in registry, retrievable by command name |
| AC-2 | `show.Enrich("show subscriber detail", base)` with registered enricher | Enricher function called with base map; base map contains enricher's additions |
| AC-3 | `show.Enrich("show subscriber detail", base)` with no enrichers registered | base map unchanged (noop) |
| AC-4 | Two enrichers registered for same command with different keys | Both enrichers called; base map contains both additions |
| AC-5 | Two enrichers registered for same command with same key | Registration rejected (error or panic at init time) |
| AC-6 | `show subscriber id X detail` with CoS plugin loaded, session has CoS profile | Output includes `"cos"` key with profile name and maps |
| AC-7 | `show subscriber id X detail` with CoS plugin NOT loaded | Output is unchanged from current behavior (no `"cos"` key) |
| AC-8 | `show subscriber` (list) with CoS plugin loaded | Each session entry includes brief CoS data (profile name only) |
| AC-9 | Enricher registry package imports only stdlib | Leaf package guarantee; any owner can import from init() |
| AC-10 | Remove CoS plugin dir + blank import | Build succeeds; show subscriber commands work without CoS enrichment |
| AC-11 | `show.ResetForTest()` clears all registrations | Test isolation |
| AC-12 | Enricher registered with nil Brief function | `EnrichBrief()` skips it; no panic, no data added |
| AC-13 | Enricher panics during `Enrich()` call | Panic recovered, enricher skipped, warning logged, remaining enrichers still called |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterAndEnrich` | `internal/core/show/show_test.go` | Basic registration + enrichment cycle | |
| `TestEnrichNoRegistrations` | `internal/core/show/show_test.go` | Noop when no enrichers registered | |
| `TestEnrichMultipleEnrichers` | `internal/core/show/show_test.go` | Multiple enrichers for same command all called | |
| `TestRegisterDuplicateKeyRejects` | `internal/core/show/show_test.go` | Same command + same key rejected | |
| `TestEnrichBrief` | `internal/core/show/show_test.go` | Brief enricher called for list views | |
| `TestEnrichModifiesBaseInPlace` | `internal/core/show/show_test.go` | Enricher writes into the passed map | |
| `TestResetForTest` | `internal/core/show/show_test.go` | ResetForTest clears all registrations | |
| `TestEnrichBriefSkipsNilBrief` | `internal/core/show/show_test.go` | EnrichBrief skips enrichers with nil Brief function | |
| `TestEnrichRecoversPanic` | `internal/core/show/show_test.go` | Enrich recovers from panicking enricher, calls remaining enrichers | |
| `TestEnrichOrderAlphabetical` | `internal/core/show/show_test.go` | Multiple enrichers called in alphabetical key order | |
| `TestSubscriberDetailCallsEnrich` | `internal/component/subscriber/cmd/subscriber_test.go` | handleDetail calls show.Enrich with correct command key | |
| `TestSubscriberSummaryCallsEnrichBrief` | `internal/component/subscriber/cmd/subscriber_test.go` | handleSummary calls show.EnrichBrief per session | |
| `TestCoSSubscriberEnricherDetail` | `internal/plugins/cos/enricher_test.go` | CoS enricher adds profile + maps for known session | |
| `TestCoSSubscriberEnricherBrief` | `internal/plugins/cos/enricher_test.go` | CoS brief enricher adds only profile name | |
| `TestCoSSubscriberEnricherNoSession` | `internal/plugins/cos/enricher_test.go` | CoS enricher is noop when session has no CoS state | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature

### Functional Tests

Creating a subscriber session with a CoS profile requires L2TP kernel support (QEMU integration tests). Existing `cos-dynamic-session.ci` explicitly notes this same constraint. The functional test strategy splits into two parts:

**Part 1: Wiring regression test (.ci, runs in CI)**

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-subscriber-enricher-wiring` | `test/plugin/show-subscriber-enricher-wiring.ci` | show subscriber on idle daemon still works with enrichers registered; CoS profiles load; no regression in existing output | |

This test follows the pattern of `show-subscriber-summary.ci` (idle daemon, no sessions) plus `cos-dynamic-session.ci` (CoS profiles loaded). It proves the enricher registry is wired and the show command doesn't break when enrichers are present but no session state exists.

Setup requirements (all exist in current test infra):
- Python plugin script dispatching `show subscriber` and `show class-of-service` via `ze_api.API`
- Config with `class-of-service` profiles defined
- BGP peer with test plugin process attached
- Assertions: show subscriber returns `total: 0` (no sessions); show class-of-service returns loaded profiles; both commands return `status: done`

**Part 2: Full enrichment test (QEMU integration, requires L2TP kernel)**

Full validation of enriched output (CoS data visible in `show subscriber detail`) runs in QEMU integration tests where L2TP sessions can be established with RADIUS auth returning `Filter-Id: cos:residential`. This follows the same deferred pattern as `cos-dynamic-session.ci` ("full session lifecycle requires L2TP kernel support and runs in QEMU integration tests").

### Interop Tests
N/A - no protocol features

### Future (if deferring any tests)
- QEMU integration test for full L2TP+CoS enriched subscriber output (deferred: requires L2TP kernel support, same constraint as cos-dynamic-session.ci)
- Web handler integration test (deferred: web test infrastructure not yet established for subscriber pages)

## Files to Modify
- `internal/component/subscriber/cmd/subscriber.go` - add show.Enrich()/show.EnrichBrief() calls in handleDetail/handleSummary
- `internal/plugins/cos/register.go` - add show.Register() calls in init() for subscriber enrichment
- `ai/patterns/registration.md` - add Show Enricher Registry section (discovery-updates rule)
- `ai/INDEX.md` - add "enricher" / "show enricher" keywords (discovery-updates rule)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new commands or config |
| CLI commands/flags | No | Existing commands gain richer output |
| Functional test for new RPC/API | Yes | `test/plugin/show-subscriber-enricher-wiring.ci` |
| Pipe completeness | No | Output is still map[string]any, pipes work unchanged |
| Doctor check for runtime dependencies | No | No new runtime dependencies |
| Prometheus counters/metrics | No | No observable state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - mention show enrichment |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | Existing commands, richer output |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | CoS plugin gains enricher, not a new plugin |
| 6 | Has a user guide page? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or new doc for show enricher pattern |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `ai/patterns/registration.md` (new Show Enricher Registry section), `ai/INDEX.md` (enricher keywords) |

## Files to Create
- `internal/core/show/show.go` - enricher registry (Register, Enrich, EnrichBrief, ResetForTest)
- `internal/core/show/show_test.go` - registry unit tests
- `internal/component/subscriber/cmd/subscriber_test.go` - wiring tests for enricher calls in subscriber handlers
- `internal/plugins/cos/enricher.go` - CoS enricher functions for subscriber show commands
- `internal/plugins/cos/enricher_test.go` - CoS enricher unit tests
- `test/plugin/show-subscriber-enricher-wiring.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint && make ze-unit-test && make ze-functional-test |

### Implementation Phases

1. **Phase: Registry (core infrastructure)** -- create `internal/core/show/show.go` with Register, Enrich, EnrichBrief, ResetForTest
   - Tests: TestRegisterAndEnrich, TestEnrichNoRegistrations, TestEnrichMultipleEnrichers, TestRegisterDuplicateKeyRejects, TestEnrichBrief, TestResetForTest
   - Files: `internal/core/show/show.go`, `internal/core/show/show_test.go`
   - Verify: all registry tests pass

2. **Phase: Wiring** -- wire show.Enrich() into subscriber show handlers
   - Tests: TestSubscriberDetailCallsEnrich, TestSubscriberSummaryCallsEnrichBrief
   - Files: `internal/component/subscriber/cmd/subscriber.go`
   - Verify: wiring tests pass with no enrichers (noop)

3. **Phase: CoS enricher** -- implement CoS enricher for subscriber show commands
   - Tests: TestCoSSubscriberEnricherDetail, TestCoSSubscriberEnricherBrief, TestCoSSubscriberEnricherNoSession, TestCoSRegistersSubscriberEnricher
   - Files: `internal/plugins/cos/enricher.go`, `internal/plugins/cos/enricher_test.go`, `internal/plugins/cos/register.go`
   - Verify: CoS enricher tests pass; show subscriber detail includes CoS data

4. **Functional tests** -- create .ci test for enriched subscriber show
5. **Full verification** -- make ze-verify

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Enricher key collision rejected; enricher noop when no state; base map unmodified keys preserved |
| Naming | JSON keys from enrichers are kebab-case |
| Data flow | Enrichment happens in handler, not at dispatch |
| Self-containment | Removing CoS plugin removes enrichers; build stays green |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/show/show.go` exists | `ls internal/core/show/show.go` |
| CoS enricher registered for subscriber | `grep -r 'show.Register' internal/plugins/cos/` |
| Subscriber handler calls Enrich | `grep -r 'show.Enrich' internal/component/subscriber/cmd/` |
| No stdlib-only violation in core/show | `head -20 internal/core/show/show.go` shows only stdlib imports |
| Functional test exists | `ls test/plugin/show-subscriber-enricher-wiring.ci` |
| Subscriber wiring tests exist | `ls internal/component/subscriber/cmd/subscriber_test.go` |
| Registration pattern documented | `grep -c 'Show Enricher' ai/patterns/registration.md` |
| INDEX keywords added | `grep 'enricher' ai/INDEX.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Enrichers receive handler-built maps, not user input directly; no injection risk |
| Resource exhaustion | Enricher count bounded by plugin count (finite at startup); no runtime registration |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Import cycle from core/show | Verify leaf package constraint; move offending import |
| CoS enricher can't access session state | Export accessor from cos package (A-1 validation) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Core leaf registry (`internal/core/show/`) with explicit Enrich() call in handlers | Dispatcher middleware (post-dispatch enrichment) | Middleware breaks single-response dispatcher model, requires JSON parse/merge at wire layer, doesn't work for service-locator web pattern. One-line call per handler is minimal and explicit. |
| Enrichers mutate base `map[string]any` in place | Enrichers return separate map, caller merges; enrichers receive typed args map | In-place mutation avoids copy overhead, lets enrichers read any base field without a separate contract. User preference. |
| Registry in `internal/core/show/` (new core leaf) | In `command/registry/` (CLI-specific); in subscriber package (component-specific) | Broader than CLI (serves web, API too). Must be importable by any plugin from init(). Follows `internal/core/cos/` precedent. |
| External plugins register enrichers via in-process `register.go` init() | SDK callback for external process enrichment | In-process registration accesses shared state directly (cos sync.Map, l2tp metadata). Same pattern as doctor checks. External process enrichment adds IPC round-trip for no gain since data is already in-process. |
| Brief + Detail as two functions on Enricher struct | Single function with detail bool; single function that infers from base map contents | Explicit separate functions are clearer and map directly to summary vs detail show commands. |
| Duplicate key rejection at registration time (panic) | Last-writer-wins; runtime error | Init-time panic catches the bug immediately during development. Same pattern as command/registry duplicate detection. |
| Registration key is CLI path with selectors stripped (e.g., `"show subscriber detail"`) | Wire method (`ze-subscriber-api:detail`); full YANG path with selector nodes | CLI path is human-readable, matches how handlers conceptually identify their command. Wire methods are opaque and could change during YANG restructuring. Stripping selectors makes the key stable across different selector values. |
| Enricher call ordering is alphabetical by key | Registration order; random (map iteration) | Deterministic output for `| json` and debugging. Consistent with `registry.All()` sort convention elsewhere in Ze. |
| `Enrich()` recovers from enricher panics, logs, and continues | Let panics propagate; return error | Production network OS must not crash a show command because one enricher has a bug. Silent skip with log warning lets the operator see partial data while the bug is diagnosed. |
| Brief function is optional (nil means no brief enrichment) | Brief required on all enrichers | Some enrichers only make sense in detail view (e.g., full RADIUS attribute dump). Requiring a brief function forces authors to write no-op stubs. |

## Known Limitations
- Web handlers using service locator pattern need explicit show.Enrich() calls (not automatic)
- External plugin commands (out-of-process handlers) require post-dispatch enrichment at the server layer (not in scope for v1)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Critical Review passes

### Design
- [ ] No premature abstraction (registry pattern has 40+ precedents in codebase)
- [ ] No speculative features (brief + full are the two known use cases)
- [ ] Single responsibility per component (registry in core, enrichers in plugins, wiring in handlers)
- [ ] Explicit > implicit behavior (handler explicitly calls Enrich)
- [ ] Minimal coupling (string-keyed, leaf package)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior
