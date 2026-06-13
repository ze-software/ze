# Spec: show-enricher-v2

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-show-enricher (completed) |
| Phase | - |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/patterns/registration.md` - registration pattern + Show Enricher Registry section
4. `internal/core/show/show.go` - current enricher registry
5. `pkg/plugin/rpc/types.go` - RPC type definitions
6. `internal/component/plugin/ipc/rpc.go` - SendExecuteCommand pattern for callback RPCs
7. `internal/test/plugins/fakeredist/fakeredist.go` - test plugin precedent

## Task

Extend the show enricher registry (v1: in-process only) to three new surfaces:

1. **External plugin enrichment via SDK protocol**: external (out-of-process) plugins declare enrichers at registration (Stage 1). At show time, the server serializes the base map to JSON, calls `ze-plugin-callback:enrich-show` on the plugin with a 2s timeout, receives enrichment data as JSON, and merges it into the base map. The SDK gets `OnEnrichShow(fn)` callback.

2. **Web service-locator enrichment**: web handlers that build their own `map[string]any` data (bypassing CLI dispatch) call `show.Enrich()` explicitly. The L2TP session detail page is the first consumer.

3. **Test plugins (permanent infrastructure)**: (a) Go in-process test plugin (`internal/test/plugins/fakeenrich/`) that registers enrichers via `show.MustRegister()`, guarding against in-process enrichment regressions. (b) Python `.ci` test plugin that exercises the external SDK enrichment path end-to-end.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - registration architecture + Show Enricher Registry section
  -> Constraint: all registries follow init() + registry + blank import pattern; enrichers are read-only after init
  -> Decision: external enrichers follow doctor-check pattern (declare at Stage 1, callback at runtime)
- [ ] `ai/patterns/plugin.md` - plugin SDK protocol, external plugin lifecycle
  -> Constraint: plugin registration is DeclareRegistrationInput at Stage 1; callbacks are ze-plugin-callback:* RPCs
- [ ] `ai/patterns/web-endpoint.md` - web handler patterns, dispatch vs service-locator
  -> Constraint: dispatch path gets enrichment for free; service-locator path needs explicit show.Enrich() calls
- [ ] `ai/rules/plugin-design.md` - SDK Is Generic, proximity principle
  -> Constraint: SDK must not contain plugin-specific types; enricher RPC types go in pkg/plugin/rpc/
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  -> Constraint: removing a plugin removes its enricher registrations; proxy enrichers cleaned up on process exit
- [ ] `ai/rules/json-format.md` - JSON key conventions
  -> Constraint: all JSON keys kebab-case; enricher RPC fields follow same convention

### Learned Summaries
- [ ] `plan/learned/894-show-enricher.md` - v1 decisions and gotchas
  -> Decision: in-place map mutation over return-and-merge; explicit Enrich() call in handlers over middleware
  -> Constraint: core/show is stdlib-only leaf package; enrichers called in alphabetical key order
  -> Constraint: hook blocks fmt.Sprintf and string concat; use error-returning Register + MustRegister
- [ ] `plan/learned/826-ipc-dispatch-data-raw.md` - IPC dispatch data as RawMessage
  -> Constraint: use json.RawMessage for data payloads crossing IPC boundary; single-marshal pattern
- [ ] `plan/learned/849-command-surface-ownership.md` - command ownership
  -> Decision: each command lives with the package that owns the behavior it exposes

**Key insights:**
- External enrichment follows the doctor-check pattern: declare at registration (Stage 1), engine calls back at runtime via `ze-plugin-callback:enrich-show`.
- The server registers a proxy enricher for each external declaration. The proxy serializes base map, calls the external plugin via IPC, deserializes response, merges into base map.
- 2s timeout on external enricher callbacks prevents hung plugins from blocking show commands.
- Proxy enrichers are cleaned up when the plugin process exits (via existing `RegisterProcessCleanup` hook).
- Web dispatch path (page_logs, page_tools, handler_admin) already gets enrichment for free. Only service-locator pages need explicit calls.
- L2TP session detail page builds data from `l2tp.LookupService().Snapshot()` using `WorkbenchTableRow` structs, not `map[string]any`. Adding enrichment requires building a side map, enriching it, then including the enriched data in the template context.
- Test plugins: Go fakeenrich for in-process regression; Python .ci script for external SDK path.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/show/show.go` - enricher registry: Register, MustRegister, Enrich, EnrichBrief, ResetForTest
  -> Constraint: leaf package, imports only stdlib (errors, log/slog, sort, sync)
  -> Constraint: Enricher struct has Detail and Brief funcs; both optional (nil-checked)
  -> Constraint: panic recovery in callDetail and callBrief; alphabetical key ordering
- [ ] `pkg/plugin/rpc/types.go` - RPC wire types shared between engine and SDK
  -> Constraint: DeclareRegistrationInput has Commands, DoctorChecks, Filters, etc. -- enrichers would be a new field
  -> Constraint: callback RPCs follow ze-plugin-callback: prefix convention
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - SDK callback registration
  -> Constraint: OnExecuteCommand pattern: store handler func, wrap in JSON marshal/unmarshal, register in callbacks map
  -> Decision: OnEnrichShow follows same pattern
- [ ] `internal/component/plugin/ipc/rpc.go` - server-side RPC calls to plugins
  -> Constraint: SendExecuteCommand sends ze-plugin-callback:execute-command, unmarshals response
  -> Decision: SendEnrichShow follows same pattern with timeout via context.WithTimeout
- [ ] `internal/component/plugin/server/rpc_register.go` - server-side RPC registration
  -> Constraint: RegisterProcessCleanup for cleanup on plugin exit
- [ ] `internal/component/web/page_l2tp.go` - web service-locator example
  -> Constraint: builds WorkbenchTableRow from l2tp.Snapshot(), not map[string]any
  -> Decision: L2TP session detail page can include enriched data as a separate section/map in template context
- [ ] `internal/component/web/page_logs.go` - web dispatch example
  -> Constraint: dispatch path calls CommandDispatcher, gets enriched JSON back for free
- [ ] `internal/test/plugins/fakeredist/fakeredist.go` - test plugin precedent
  -> Constraint: registers via registry.Register() in init(); blank-imported from test/plugins/all/all.go
  -> Constraint: test plugins build with zetest tag; production daemon does not include them
- [ ] `internal/test/plugins/all/all.go` - test plugin blank imports
  -> Constraint: hand-maintained; add blank import for new test plugin here

**Behavior to preserve:**
- All existing in-process enrichment (v1) works unchanged
- `show subscriber detail` and `show subscriber` existing output format preserved
- External plugins not declaring enrichers see no change
- CLI pipe operators (json, yaml, table) work on enriched output

**Behavior to change:**
- External plugins can now declare enrichers at registration and handle enrich callbacks
- L2TP session detail web page gains enriched data section when enrichers are registered
- New test plugin fakeenrich provides permanent enrichment regression guard

## Data Flow (MANDATORY)

### Entry Point
- CLI user types `show subscriber id X detail` or web user navigates to `/l2tp/<session-id>`
- External plugin declares enrichers in DeclareRegistrationInput at startup
- Test plugin registers enrichers via show.MustRegister (in-process) or SDK declaration (external)

### Transformation Path
1. **Registration (startup)**: external plugin sends DeclareRegistrationInput with Enrichers field. Server iterates declarations, calls `show.Register()` with a proxy enricher for each.
2. **Show command (runtime, in-process path)**: handler builds base `map[string]any`, calls `show.Enrich(command, base)`. Registry calls each enricher in order. In-process enrichers run directly. Proxy enrichers serialize base to JSON, call external plugin via IPC, deserialize response, merge into base.
3. **Show command (runtime, external path)**: proxy enricher serializes base map to JSON. Server sends `ze-plugin-callback:enrich-show` RPC to plugin with 2s timeout. Plugin's `OnEnrichShow` handler receives base map, returns enrichment `map[string]any`. Server merges enrichment into base map.
4. **Web (service-locator path)**: web handler builds data from service snapshot, constructs `map[string]any` for enrichable fields, calls `show.Enrich()`, includes enriched data in template context.
5. **Cleanup**: when external plugin process exits, `RegisterProcessCleanup` hook removes proxy enrichers for that plugin.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server -> External plugin | `ze-plugin-callback:enrich-show` RPC with JSON base map | [ ] |
| External plugin -> Server | JSON response with enrichment map | [ ] |
| Core registry <- Server | `show.Register()` proxy enricher at startup | [ ] |
| Web handler -> Core registry | `show.Enrich()` call with constructed map | [ ] |

### Integration Points
- `pkg/plugin/rpc/types.go` - new `EnricherDecl`, `EnrichShowInput`, `EnrichShowOutput` types
- `pkg/plugin/sdk/sdk_callbacks.go` - new `OnEnrichShow` callback registration
- `internal/component/plugin/ipc/rpc.go` - new `SendEnrichShow` method
- `internal/component/plugin/server/startup.go` - proxy enricher registration during plugin startup
- `internal/component/plugin/server/rpc_register.go` - cleanup hook for proxy enrichers
- `internal/component/web/page_l2tp.go` - `show.Enrich()` call in session detail builder
- `internal/core/show/show.go` - new `Unregister(command, key)` for cleanup on plugin exit

### Architectural Verification
- [ ] No bypassed layers (external enrichers go through same show.Enrich() as in-process)
- [ ] No unintended coupling (proxy enricher in server, not in core/show)
- [ ] No duplicated functionality (extends existing enricher pattern)
- [ ] Zero-copy preserved where applicable (enrichers mutate base map in place)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | External plugin process is alive when show.Enrich() runs | Plugin lifecycle managed by server | Proxy enricher call times out (2s); show command returns partial data | Test with plugin stopped mid-session | unvalidated |
| A-2 | json.Marshal/Unmarshal round-trip preserves all map[string]any value types used by handlers | Go JSON handles string, int, float64, bool, nil, nested maps/slices | uint16 values marshal as float64; enricher must handle number types | Unit test with uint16/int/float64 values | unvalidated |
| A-3 | show.Register supports an Unregister operation for cleanup | v1 has ResetForTest but no per-key Unregister | Need to add Unregister(command, key) to core/show | Read show.go | unvalidated |
| A-4 | L2TP session detail page has a template context where enriched data can be displayed | page_l2tp.go builds WorkbenchTableData | May need a new template section for enriched data | Read page_l2tp.go detail builder | unvalidated |
| A-5 | DeclareRegistrationInput can be extended with a new Enrichers field without breaking existing plugins | JSON unmarshaling ignores unknown fields | Old plugins sending without Enrichers field work fine | Existing .ci tests still pass | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | External enricher timeout (2s) too short for complex enrichers | Enricher data missing in show output intermittently | Make timeout configurable via env var; start with 2s |
| R-2 | JSON round-trip changes numeric types (uint16 becomes float64) | Type assertion failures in enricher functions | Enrichers must accept float64 for numeric fields from external path |
| R-3 | Plugin crash during enrich callback leaves show command hanging | Show command blocked until timeout | 2s context timeout; proxy enricher logs warning and continues |
| R-4 | Race between plugin exit cleanup and concurrent Enrich() call | Panic or stale data | Unregister acquires write lock; Enrich snapshot under read lock is safe |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| External plugin declares enricher in registration | -> | Server registers proxy enricher via show.Register() | `TestExternalEnricherRegisteredAtStartup` |
| `show subscriber detail` with external enricher registered | -> | Proxy enricher calls external plugin, merges result | `TestExternalEnricherCalledOnShowCommand` |
| External plugin exits | -> | Proxy enricher unregistered via show.Unregister() | `TestExternalEnricherCleanedUpOnExit` |
| L2TP session detail web page | -> | show.Enrich() called on session data | `TestL2TPDetailPageCallsEnrich` |
| fakeenrich test plugin init() | -> | show.MustRegister() enricher for test command | `TestFakeEnrichRegistered` |
| .ci test with Python enricher plugin | -> | Full external enrichment lifecycle | `show-enricher-external.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | External plugin declares `enrichers: [{command: "show subscriber detail", key: "ext-test"}]` at registration | Server registers proxy enricher; `show.Enrich("show subscriber detail", ...)` calls the proxy |
| AC-2 | Proxy enricher calls external plugin via `ze-plugin-callback:enrich-show` | Plugin receives base map as JSON, returns enrichment map; server merges into base |
| AC-3 | External plugin does not respond within 2s | Proxy enricher times out, logs warning, show command returns without external enrichment data |
| AC-4 | External plugin process exits | Proxy enricher is unregistered; subsequent show commands skip it |
| AC-5 | `show.Unregister(command, key)` called for existing enricher | Enricher removed from registry; Enrich() no longer calls it |
| AC-6 | `show.Unregister(command, key)` called for non-existent enricher | No-op, no error |
| AC-7 | L2TP session detail web page with enrichers registered | Page includes enriched data section from show.Enrich() |
| AC-8 | L2TP session detail web page with no enrichers registered | Page renders normally without enriched section (noop) |
| AC-9 | fakeenrich test plugin loaded (zetest build) | Enricher registered for `"show test enrich"` command |
| AC-10 | Python .ci test plugin declares enricher, dispatches show command | Show command output includes enrichment data from Python plugin |
| AC-11 | Old plugin without enricher declarations | Registration succeeds; no enrichers registered (backward compatible) |
| AC-12 | JSON round-trip preserves enrichment data types | uint16 values from base map arrive as float64 in external enricher; enricher output merges correctly |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnregister` | `internal/core/show/show_test.go` | Unregister removes enricher; Enrich no longer calls it | |
| `TestUnregisterNonExistent` | `internal/core/show/show_test.go` | Unregister on missing key is noop | |
| `TestUnregisterLastForCommand` | `internal/core/show/show_test.go` | Unregister last enricher for command; command entry cleaned up | |
| `TestEnrichShowInput` | `pkg/plugin/rpc/types_test.go` | EnrichShowInput JSON round-trip preserves base map | |
| `TestEnrichShowOutput` | `pkg/plugin/rpc/types_test.go` | EnrichShowOutput JSON round-trip preserves enrichment map | |
| `TestOnEnrichShowCallback` | `pkg/plugin/sdk/sdk_callbacks_test.go` | SDK OnEnrichShow registers callback, handles RPC correctly | |
| `TestProxyEnricherCallsPlugin` | `internal/component/plugin/server/enricher_test.go` | Proxy enricher serializes base, calls plugin, merges result | |
| `TestProxyEnricherTimeout` | `internal/component/plugin/server/enricher_test.go` | Proxy enricher times out after 2s, returns without enrichment | |
| `TestProxyEnricherCleanup` | `internal/component/plugin/server/enricher_test.go` | Plugin exit triggers show.Unregister for all proxy enrichers | |
| `TestFakeEnrichRegistered` | `internal/test/plugins/fakeenrich/fakeenrich_test.go` | Test plugin enricher is registered after init() | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Timeout | 100ms - 30s | 30s | N/A (clamped) | N/A (clamped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-enricher-external` | `test/plugin/show-enricher-external.ci` | Python plugin declares enricher, dispatches show subscriber, verifies enrichment data present | |
| `show-enricher-fakeenrich` | `test/plugin/show-enricher-fakeenrich.ci` | fakeenrich test plugin enricher adds data to show test enrich command | |

### Interop Tests
N/A -- no protocol features

### Future (if deferring any tests)
- Full L2TP+subscriber enrichment test (deferred: requires L2TP kernel support, same constraint as v1)

## Files to Modify
- `internal/core/show/show.go` - add Unregister(command, key) function
- `pkg/plugin/rpc/types.go` - add EnricherDecl, EnrichShowInput, EnrichShowOutput types; add Enrichers field to DeclareRegistrationInput
- `pkg/plugin/sdk/sdk_callbacks.go` - add OnEnrichShow callback registration
- `internal/component/plugin/ipc/rpc.go` - add SendEnrichShow method
- `internal/component/plugin/server/startup.go` - register proxy enrichers during plugin startup from declared enrichers
- `internal/component/plugin/server/rpc_register.go` - add process cleanup hook for proxy enrichers
- `internal/component/web/page_l2tp.go` - add show.Enrich() call in session detail builder
- `internal/test/plugins/all/all.go` - add blank import for fakeenrich
- `ai/patterns/registration.md` - update Show Enricher Registry section with external path
- `ai/INDEX.md` - add external enricher keywords
- `docs/features.md` - update enricher mention with external + web support

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new config |
| CLI commands/flags | No | Existing commands gain richer output |
| Functional test for new RPC/API | Yes | `test/plugin/show-enricher-external.ci` |
| Pipe completeness | No | Output is still map[string]any |
| Doctor check for runtime dependencies | No | No new runtime dependencies |
| Prometheus counters/metrics | No | No observable state |
| Plugin SDK/protocol changed | Yes | New RPC pair: enrich-show declaration + callback |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - update enricher section with external + web |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` - new enrich-show callback |
| 5 | Plugin added/changed? | Yes | fakeenrich test plugin |
| 6 | Has a user guide page? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugin-design.md` - mention enricher declaration |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - mention fakeenrich test plugin |
| 12 | Internal architecture changed? | Yes | `ai/patterns/registration.md` - update with external enricher path |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | fakeenrich adds a test command |

## Files to Create
- `internal/core/show/show.go` (modify) - add Unregister
- `internal/component/plugin/server/enricher.go` - proxy enricher registration + cleanup
- `internal/component/plugin/server/enricher_test.go` - proxy enricher tests
- `internal/test/plugins/fakeenrich/fakeenrich.go` - test plugin (in-process enricher)
- `internal/test/plugins/fakeenrich/fakeenrich_test.go` - test plugin tests
- `test/plugin/show-enricher-external.ci` - external enrichment .ci test
- `test/plugin/show-enricher-fakeenrich.ci` - fakeenrich .ci test

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

1. **Phase: Unregister (core infrastructure)** -- add Unregister(command, key) to core/show
   - Tests: TestUnregister, TestUnregisterNonExistent, TestUnregisterLastForCommand
   - Files: `internal/core/show/show.go`, `internal/core/show/show_test.go`
   - Verify: all registry tests pass

2. **Phase: RPC types** -- add enricher declaration and callback types to pkg/plugin/rpc
   - Tests: TestEnrichShowInput, TestEnrichShowOutput
   - Files: `pkg/plugin/rpc/types.go`
   - Verify: JSON round-trip tests pass

3. **Phase: SDK callback** -- add OnEnrichShow to SDK
   - Tests: TestOnEnrichShowCallback
   - Files: `pkg/plugin/sdk/sdk_callbacks.go`
   - Verify: SDK callback tests pass

4. **Phase: Server-side proxy enricher** -- register proxy enrichers, IPC call, cleanup
   - Tests: TestProxyEnricherCallsPlugin, TestProxyEnricherTimeout, TestProxyEnricherCleanup
   - Files: `internal/component/plugin/server/enricher.go`, `internal/component/plugin/ipc/rpc.go`
   - Verify: proxy enricher tests pass

5. **Phase: fakeenrich test plugin** -- in-process test plugin
   - Tests: TestFakeEnrichRegistered
   - Files: `internal/test/plugins/fakeenrich/fakeenrich.go`, `internal/test/plugins/all/all.go`
   - Verify: test plugin loads and enricher is registered

6. **Phase: Web wiring** -- L2TP session detail page enrichment
   - Tests: TestL2TPDetailPageCallsEnrich
   - Files: `internal/component/web/page_l2tp.go`
   - Verify: web enrichment works

7. **Phase: Functional tests** -- .ci tests for external and fakeenrich paths
   - Files: `test/plugin/show-enricher-external.ci`, `test/plugin/show-enricher-fakeenrich.ci`
   - Verify: both .ci tests pass

8. **Full verification** -- make ze-verify

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON round-trip preserves types; timeout works; cleanup removes all proxy enrichers |
| Naming | JSON keys kebab-case; RPC names follow ze-plugin-callback: convention |
| Data flow | External enrichment goes through same show.Enrich() as in-process |
| Self-containment | Removing external plugin removes its proxy enrichers |
| Backward compatibility | Old plugins without enricher declarations still work |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `show.Unregister` exists | `grep -r 'func Unregister' internal/core/show/` |
| EnricherDecl type in rpc/types.go | `grep 'EnricherDecl' pkg/plugin/rpc/types.go` |
| OnEnrichShow in SDK | `grep 'OnEnrichShow' pkg/plugin/sdk/sdk_callbacks.go` |
| SendEnrichShow in IPC | `grep 'SendEnrichShow' internal/component/plugin/ipc/rpc.go` |
| Proxy enricher registration | `grep 'show.Register' internal/component/plugin/server/enricher.go` |
| fakeenrich plugin | `ls internal/test/plugins/fakeenrich/fakeenrich.go` |
| External .ci test | `ls test/plugin/show-enricher-external.ci` |
| L2TP web enrichment | `grep 'show.Enrich' internal/component/web/page_l2tp.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | External enricher receives base map as JSON; enrichment response is merged -- verify no key overwrite of critical fields |
| Resource exhaustion | External enricher response size unbounded; add size limit on response |
| Timeout enforcement | 2s timeout on external callback prevents DoS from hung plugins |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| JSON round-trip type mismatch | Fix type handling in proxy enricher |
| Timeout test flaky | Increase timeout margin in test |
| Plugin exit cleanup race | Verify lock ordering in Unregister |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Declare enrichers at registration (Stage 1) | Plugin-initiated registration after ready | Follows doctor-check pattern; avoids race window where show commands run without external enrichers |
| 2s timeout on external enricher callback | No timeout; configurable timeout | Prevents hung plugins from blocking show commands; 2s is generous for a show command enricher; can be made configurable later via env var |
| Proxy enricher in server package | Proxy enricher in core/show | core/show is a leaf package (stdlib only); proxy needs IPC access which requires server imports |
| Unregister function in core/show | ResetForTest only (wipe all) | Need per-key removal for plugin exit cleanup; ResetForTest is too coarse |
| Merge semantics: external enricher response keys added to base map | Replace entire base map; nested merge | Simple top-level key merge matches in-process enricher behavior (mutate base in place) |
| Go fakeenrich + Python .ci for testing | Python only; Go only | Go plugin guards in-process regression permanently; Python .ci exercises external SDK path end-to-end |

## Known Limitations
- External enricher IPC round-trip adds latency to show commands (~1-10ms per enricher)
- External enricher response size is unbounded (mitigated by 2s timeout)
- Web enrichment requires handler modification per page (not automatic)
- L2TP session detail page enrichment is limited to data that can be represented as map[string]any

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Critical Review passes

### Design
- [ ] No premature abstraction (extends existing enricher pattern with 1 new path)
- [ ] No speculative features (external enrichment + web + test plugin are concrete use cases)
- [ ] Single responsibility per component (proxy in server, registry in core, types in rpc)
- [ ] Explicit > implicit behavior (handler calls Enrich explicitly)
- [ ] Minimal coupling (proxy enricher uses existing IPC layer)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior
