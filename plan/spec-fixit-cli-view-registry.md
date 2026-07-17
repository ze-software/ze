# Spec: fixit-cli-view-registry

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - "Registration over hardcoding (the CLI client too)"
4. `ai/patterns/registration.md` - the init + registry + longest-prefix-match pattern
5. Source files in Current Behavior below (model.go, model_keys.go, model_render.go, model_ping/traceroute/dashboard.go)

## Task

**[MEDIUM]** The CLI `Model` embodies the exact per-feature anti-pattern the project's own rule
(`ai/rules/plugin-self-containment.md`) names and bans, in the exact files it names. Both audit
architecture passes (2026-07-16) flagged it. The rule and the code currently teach opposite
patterns, which trains contributors to copy the wrong one.

Each rich live view (dashboard, traceroute, ping) adds its own field, factory setter, command
matcher, start method, poll handler, render method, and key handler to the core `cli.Model`,
wired one-by-one into the hub session factory and the client main. A new view edits the core
struct and its five per-feature switch sites -- the opposite of "the core discovers features
through a registry."

Implement a client-side view registry modeled on the daemon-side monitor-provider registry,
migrate dashboard/ping/traceroute onto it, and add a review guard against new per-feature fields
in `cli.Model`.

- **Fields:** `internal/component/cli/model.go:157-172` -- `monitorFactory` (157),
  `dashboardFactory`/`dashboard` (161-162), `tracerouteFactory`/`traceroute`/`traceroutePiped`
  (165-167), `pingFactory`/`pingMonitor`/`pingMonitorPiped` (170-172).
- **Update message switch:** `internal/component/cli/model.go:524-547` -- `monitorPollMsg` (524),
  `dashboardTickMsg` (527), `dashboardDataMsg` (534), `traceroutePollMsg` (537),
  `traceroutePipedPollMsg` (540), `pingPollMsg` (543), `pingPipedPollMsg` (546).
- **Command dispatch (two sites):** `internal/component/cli/model_keys.go:389-406` (editor mode)
  and `:536-553` (command-only mode) -- `isDashboardCommand`/`startDashboard`,
  `isPingMonitorCommand`/`startPingMonitor` (+ piped), `isTracerouteMonitorCommand`/`startTraceroute`
  (+ piped).
- **Key handling (Esc/stop):** `internal/component/cli/model_keys.go:24-63` -- per-active-view arms
  for dashboard, traceroute, ping, and the two piped variants.
- **Render dispatch:** `internal/component/cli/model_render.go:318-339` -- per-active-view arms
  selecting `renderDashboard`/`renderTraceroute`(+piped)/`renderPingMonitor`(+piped).
- **Wiring:** `cmd/ze/hub/session_factory.go:95-97,122-124` (two setter blocks) and its factory
  funcs at `:182-187`; `internal/component/cli/client/main.go:28,37` (owner `cmd` imports) and its
  setter blocks at `:129-136,220-227` plus factory funcs at `:949-953`.
- **Contract seed:** `internal/component/cli/model_ping.go:31` inverts the factory contract (cli
  defines `PingFactory` and never imports the ping engine). No view registry exists: zero
  `RegisterView`/`viewRegistry` hits across `internal/component/cli`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `ai/rules/plugin-self-containment.md` - "Registration over hardcoding (the CLI client too)"
  → Constraint: new live views register a `{command-prefix, session-factory, renderer}` and the Model discovers them; no new per-feature field/switch in `Model`.
  → Constraint: the rule cites the daemon-side precedent by the name `RegisterStreamingHandler`; the actual symbol is `RegisterMonitorProvider` + the `streamingHandlers`/`monitorProviders` maps in `internal/component/plugin/server/handler.go`. The example in the rule doc must be repointed at the new client-side registry once it exists (Discovery updates).
- [ ] `ai/patterns/registration.md` - the init + registry + blank-import pattern
  → Constraint: follow the established registry shape used elsewhere in the core; for a longest-prefix command match, model on `matchesPrefix` in `internal/component/plugin/server/handler.go:70-71`.

**Key insights:**
- The daemon already registers TUI streaming views generically: `RegisterMonitorProvider(prefix, provider)` keeps a `monitorProviders`/`streamingHandlers map[prefix]handler` and resolves the input by longest-prefix match (`internal/component/plugin/server/handler.go:30-52,70-75`). The client `Model` must not regress that into per-feature hardcoding. This is the exact registration shape to mirror on the client side.
- The factory-contract inversion at `model_ping.go:31` (cli defines the contract type, never imports the engine) is the seed of the right shape; generalize the injection into a registry rather than inventing a new mechanism.

## Current Behavior (MANDATORY)

**Source files read (verified 2026-07-16 against the working tree):**
- [ ] `internal/component/cli/model.go` - per-feature view fields (`:157-172`) and the per-feature `Update` message switch (`:524-547`).
  → Constraint: fields are heterogeneous -- factory + one or two active-session pointers per view. Removing them is AC-3.
- [ ] `internal/component/cli/model_keys.go` - command dispatch in editor mode (`:389-406`) and command-only mode (`:536-553`), and per-active-view key handling (`:24-63`).
  → Constraint: the command-dispatch surface is DUPLICATED across two mode paths; both must resolve through the registry or the per-feature switch is only half-removed.
- [ ] `internal/component/cli/model_render.go` - per-active-view render dispatch (`:318-339`).
- [ ] `internal/component/cli/model_ping.go` - `PingFactory` contract (`:31`), command matcher `isPingMonitorCommand` / `isPipedPingMonitorCommand` (`:121-136`), `startPingMonitor`/`startPingMonitorPiped`, poll handlers, renderers.
  → Constraint: each view start method branches plain vs piped internally on the presence of `|`; one registry entry per view (3), not five.
- [ ] `internal/component/cli/model_traceroute.go` - `TracerouteFactory` (`:26`), matcher `"monitor traceroute "` (`:162`), plain + piped start/poll/render.
- [ ] `internal/component/cli/model_dashboard.go` - `DashboardFactory = contract.DashboardFactory` (`:27`), matcher `"monitor bgp"` (`:225-227`), pull-poll lifecycle (2s tick calling `commandExecutor("bgp summary")`).
- [ ] `internal/component/cli/contract/contract.go` - `MonitorFactory` (`:77`), `DashboardFactory` (`:80`); leaf package `cli` imports.
- [ ] `cmd/ze/hub/session_factory.go` and `internal/component/cli/client/main.go` - the two consumers that inject the factories.
  → Constraint: both import the owner `cmd` packages (`ping/cmd`, `traceroute/cmd`); `cli` imports neither (inversion preserved). This is what makes a cycle-free registry possible.

**Behavior to preserve:**
- All existing CLI view behavior: `monitor ping`, `monitor traceroute`, `monitor bgp` (dashboard), and their piped/log variants render and stream exactly as today, including the `| resolve`/`| origin`/`| log` legend enrichment path (`model_ping.go` / `model_traceroute.go`).
- The existing key bindings and mode handling in `Model`, including the duplicated editor-mode vs command-only-mode dispatch.
- Backward-compatible command grammar (`monitor <module>` verb-first prefixes).
- The `model_ping.go:31` inversion: the `cli` package must continue to define view contracts and never import the ping/traceroute engines.

**Behavior to change:**
- Replace the per-feature fields + five switch/dispatch sites with a view registry; migrate the three views; no user-visible change.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A user types a live-view command (`monitor bgp`, `monitor ping <t>`, `monitor traceroute <t>`, with optional `| ...`) at the CLI prompt. `Model` (bubbletea) receives it on Enter in either editor mode or command-only mode.

### Transformation Path
1. Today: the key handler runs an ordered chain of `isXCommand(input)` predicates; the first match calls the matching `startX(input) tea.Cmd`, which reads the per-feature factory field and stores a per-feature active-session pointer. Subsequent `tea.Msg` ticks are routed by a per-feature `case` in `Update`; `View()` and the Esc/stop key handler each branch on which per-feature pointer is non-nil.
2. Required: a `viewRegistry` (a `map[prefix]viewSpec`, longest-prefix match like `matchesPrefix`) resolves the input to one `viewSpec`. The Model holds one `activeView` handle plus a generic keyed factory store, no per-feature field. Command dispatch, tick routing, render, and key handling all consult the registry / the single active handle.
3. Each view registers its `viewSpec` from its own `model_*.go` `init()` (Design 1, below). The consumers (hub factory, client main) iterate `cli.RegisteredViews()` to inject each view's factory generically instead of calling three typed setters.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Command ↔ Model | command prefix resolves via the registry's longest-prefix match, not an `isXCommand` chain | [ ] |
| Model ↔ view | a `viewSpec` lifecycle interface (`match / start / update / render / key / stop`) + a per-view factory injected generically | [ ] |
| View ↔ registration | each view registers its `viewSpec` in its own file's `init()`; core discovers it via the map | [ ] |
| Consumer ↔ Model | hub factory and client main iterate `RegisteredViews()` and inject factories by name, not three typed setters | [ ] |

### Integration Points
- New `viewRegistry` + `RegisterView` in `internal/component/cli`; the ping/traceroute/dashboard views as registrants (in-package `init()`); `cmd/ze/hub/session_factory.go` and `internal/component/cli/client/main.go` as consumers that iterate the registry.

### Architectural Verification
- [ ] No bypassed layers (all three per-feature surfaces -- dispatch, tick, render, key -- resolve through the registry / single active handle)
- [ ] No unintended coupling (`cli` still imports no engine package; the `model_ping.go:31` inversion is preserved)
- [ ] No duplicated functionality (generalize the existing injection; do not add a parallel mechanism; reuse a `matchesPrefix`-style longest-prefix match, do not reinvent it)
- [ ] Registration over hardcoding -- the entire point: no new per-feature field/switch/factory in `Model` (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three views share a common *lifecycle* interface (match → start(input) → poll-tick → drain → render → key/stop), even though their factory *signatures* differ | read each view: `model_dashboard.go` (pull poller `func()(func()(string,error),error)`), `model_traceroute.go:26` (`ctx,target,maxHops → chan`), `model_ping.go:31` (`ctx,target,interval,timeout,count,size → chan`) | Registry contract must abstract the lifecycle, not the factory signature; a `{prefix,factory,renderer}` 3-tuple is too narrow | reading each view's start/poll/render path | confirmed (lifecycle common; factory signature heterogeneous) |
| A-2 | The client main and hub factory can consume the registry without an import cycle | `cli` imports no engine (`model_ping.go` comments); `ping/cmd`,`traceroute/cmd` import no `cli`; both consumers already import both sides | Import cycle | traced imports (grep both directions) | confirmed (no cycle for Design 1; Design 2 keeps direction owner→cli only) |
| A-3 | The command-dispatch surface is duplicated across editor mode and command-only mode and both must be migrated | `model_keys.go:389-406` and `:536-553` | A half-migration leaves a per-feature chain in one mode; AC-3 fails | grep both dispatch sites | confirmed |
| A-4 | Longest-prefix match is the right resolution rule (prefixes `monitor bgp`, `monitor ping`, `monitor traceroute` are siblings under `monitor`) | `handler.go:70-75` daemon precedent; `isXCommand` all use `monitor <module>` | Ambiguous resolution between sibling prefixes | mirror `matchesPrefix` (word-boundary aware) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A view's piped/log variant does not fit the common lifecycle interface (ping/traceroute piped bypass `ApplyPipes` and enrich the legend inline) | migration of that variant stalls | Keep plain+piped inside one `viewSpec.start(input)` that branches on `|`; the interface exposes start/update/render/stop, not the pipe internals |
| R-2 | Regression in an existing view's rendering during migration | existing view `.ci`/`.et` tests fail | Migrate one view at a time behind the passing regression tests (Implementation Phases) |
| R-3 | Generic factory injection (`map[string]any` + per-view type assertion) loses the compile-time type safety of the current typed setters | a view start panics on a bad assertion | Keep each factory's typed setter as a thin wrapper that stores into the keyed map, or store a typed accessor closure in the `viewSpec`; assert once at start with a clear status message on mismatch (fail-closed, `ai/rules/fail-closed-guards.md`) |
| R-4 | Scope creep to the generic `monitor` view (`monitorFactory`/`monitorSession`, `model_monitor.go`) or a future `traffic` view | migration touches monitor | Keep migration to the three named views; note monitor/traffic as the next registrants (its `*MonitorSession` session object is the closest precedent for the contract) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor ping <t>` command | → | resolved via `viewRegistry` longest-prefix match, not a `Model` `isXCommand` chain | `TestViewRegistryResolvesPing` |
| a newly registered `viewSpec` | → | reachable (dispatch + render + key) without editing `Model` | `TestViewRegistryDiscoversRegisteredView` |
| `cli.Model` after migration | → | carries no per-feature view field/switch for the migrated views | `TestModelHasNoPerFeatureViewField` |
| existing `monitor ping` view | → | unchanged rendering under migration | `test/ui/monitor-ping-pipe-resolve-log.ci` (existing) |
| existing `monitor traceroute` view | → | unchanged data/render path | `test/plugin/monitor-traceroute.ci` (existing) |
| existing `monitor bgp` dashboard | → | unchanged data reachability | `test/plugin/bgp-monitor-dashboard.ci` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `monitor bgp` / `monitor ping <t>` / `monitor traceroute <t>` (plain and piped) | resolve via the registry; byte-identical output to today, in both editor and command-only mode |
| AC-2 | A new view registered in its own file via `RegisterView` | dispatch, tick routing, render, and key handling all work without editing `Model`, the hub factory, or the client main |
| AC-3 | `cli.Model` after migration | carries no per-feature view field and no per-feature `case`/branch for the migrated views across all five sites (fields, Update switch, both dispatch sites, key handling, render) |
| AC-4 | Review guard | a test flags any new per-feature view field added to `cli.Model` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `monitor ping 127.0.0.1 \| resolve \| log` in the TUI | key handler → registry longest-prefix match → ping `viewSpec.start` → poll/drain → legend enrich render | `test/ui/monitor-ping-pipe-resolve-log.ci` |
| 2 | runs `monitor traceroute <t>` | registry → traceroute `viewSpec.start` → probe-round channel drain → hop-table render | `test/plugin/monitor-traceroute.ci` |
| 3 | runs `monitor bgp` | registry → dashboard `viewSpec.start` → 2s pull poll of `bgp summary` → peer-table render | `test/plugin/bgp-monitor-dashboard.ci` |
| 4 | adds a brand-new live view in one file | `RegisterView(viewSpec{...})` in the owner file `init()`; no edit to `Model`/hub/client | `TestViewRegistryDiscoversRegisteredView` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestViewRegistryResolvesPing` | `internal/component/cli/view_registry_test.go` | AC-1 (prefix resolution) | |
| `TestViewRegistryLongestPrefixMatch` | `internal/component/cli/view_registry_test.go` | AC-1/A-4 (sibling `monitor *` prefixes resolve unambiguously) | |
| `TestViewRegistryDiscoversRegisteredView` | `internal/component/cli/view_registry_test.go` | AC-2 | |
| `TestModelHasNoPerFeatureViewField` | `internal/component/cli/model_test.go` | AC-3/AC-4 (reflect over `Model` fields; ban per-view names) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| registered views | 0..N | N | N/A | N/A |
| prefix match on empty input | "" → no match | one-word prefix | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `monitor-ping-pipe-resolve-log` | `test/ui/monitor-ping-pipe-resolve-log.ci` | `monitor ping \| resolve \| log` renders unchanged after migration (drives the real TUI model with a fake ping factory) | |
| `monitor-traceroute` | `test/plugin/monitor-traceroute.ci` | `monitor traceroute` hop data path unchanged after migration | |
| `bgp-monitor-dashboard` | `test/plugin/bgp-monitor-dashboard.ci` | `monitor bgp` dashboard summary reachable/unchanged after migration | |

### Interop Tests (MANDATORY for protocol features)
- N/A: this is a client-side TUI refactor with no wire-protocol behavior change.

## Files to Modify
- `internal/component/cli/model.go` - remove per-feature view fields (`:157-172`); collapse the Update message switch (`:524-547`) to a single "route tick to active view" arm
- `internal/component/cli/model_keys.go` - replace both command-dispatch chains (`:389-406`, `:536-553`) with a single registry resolution; replace the per-view key-handling chain (`:24-63`) with a single active-view delegate
- `internal/component/cli/model_render.go` - replace the per-view render chain (`:318-339`) with a single active-view render
- `internal/component/cli/model_ping.go`, `model_traceroute.go`, `model_dashboard.go` - register each view's `viewSpec` in `init()`; keep render/update/state in-package (Design 1)
- `cmd/ze/hub/session_factory.go` - iterate `cli.RegisteredViews()` to inject factories instead of the three typed setters at `:95-97,122-124`
- `internal/component/cli/client/main.go` - same, at `:129-136,220-227`
- `ai/rules/plugin-self-containment.md` - repoint the "Registration over hardcoding (the CLI client too)" example at the now-correct code; fix the `RegisterStreamingHandler` → `RegisterMonitorProvider` name (Discovery updates)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | none -- no new command grammar, no new RPC |
| CLI commands/flags | [ ] No | commands unchanged; only their in-model dispatch is refactored |
| Functional test for new RPC/API | [ ] No | no new RPC; existing view `.ci`/`.et` tests are the regression guard |
| Pipe completeness | [ ] Preserve | the `| resolve`/`| origin`/`| log` legend path in ping/traceroute must survive migration (`ai/rules/pipe-completeness.md`) |
| Registration guard test | [ ] Yes | `internal/component/cli/model_test.go` -- `TestModelHasNoPerFeatureViewField` (AC-4) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 5 | Plugin added/changed? | [ ] No | client-internal refactor, no plugin surface change |
| 12 | Internal architecture changed? | [ ] Yes | `ai/rules/plugin-self-containment.md` (example repoint); consider a note in `docs/architecture/cli/` if a view-registry doc exists |
| 15 | Registered command/view inventory changed? | [ ] No (mechanism only) | user-visible commands and their prefixes are unchanged |
| 16 | Changed source referenced by doc source anchors? | [ ] Verify | grep `docs/` and `ai/` for `model_ping.go`/`model_traceroute.go`/`model_dashboard.go`/`session_factory.go` anchors and update stale line refs |

## Files to Create
- `internal/component/cli/view_registry.go` - `RegisterView`, `RegisteredViews`, the `viewSpec` lifecycle contract, and longest-prefix resolution (mirrors `handler.go:matchesPrefix`)
- `internal/component/cli/view_registry_test.go` - registry resolution + discovery unit tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Close | two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding. Migrate ONE view at a time; keep the existing `.ci`/`.et` regression tests green after every phase (R-2).

1. **Phase: Wiring (MANDATORY FIRST)** — add `view_registry.go` with `viewSpec`, `RegisterView`, `RegisteredViews`, and longest-prefix `resolve(input)`; add `Model.activeView` handle + generic keyed factory store; write failing `TestViewRegistryResolvesPing` / `TestViewRegistryDiscoversRegisteredView`.
   - Verify: a test-only registered `viewSpec` is reachable through the registry with no `Model` edit; tests fail because no real view is registered yet.
2. **Phase: migrate ping** — register the ping `viewSpec` in `model_ping.go` `init()`; route its command dispatch (both mode paths), tick, render, and key handling through the registry/active handle; delete `pingFactory`/`pingMonitor`/`pingMonitorPiped` fields and their five switch arms.
   - Verify: `test/ui/monitor-ping-pipe-resolve-log.ci` stays green; `| resolve`/`| origin`/`| log` legend unchanged.
3. **Phase: migrate traceroute** — same, `model_traceroute.go`.
   - Verify: `test/plugin/monitor-traceroute.ci` green.
4. **Phase: migrate dashboard** — same, `model_dashboard.go` (pull-poll lifecycle; no target args -- proves the lifecycle interface spans the heterogeneous factory signature, A-1).
   - Verify: `test/plugin/bgp-monitor-dashboard.ci` green.
5. **Phase: consumers** — replace the typed setter blocks in `session_factory.go` and `client/main.go` with iteration over `RegisteredViews()`; keep the owner-`cmd` imports (they build the concrete factories -- Design 1).
6. **Phase: review guard** — add `TestModelHasNoPerFeatureViewField` (reflect over `Model`, ban per-view field-name patterns) to enforce AC-4.
7. **Full verification** → `make ze-verify`.
8. **Discovery update** → repoint the `ai/rules/plugin-self-containment.md` example and fix the `RegisterStreamingHandler`→`RegisterMonitorProvider` misnomer.
9. **Complete spec** → audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three views migrated; `Model` free of their per-feature fields/cases across ALL FIVE sites (fields, Update switch, both dispatch chains, key handling, render) |
| Registration over hardcoding | Views register and are discovered; no new per-feature field/switch in `Model` (`ai/rules/plugin-self-containment.md`); resolution reuses a `matchesPrefix`-style match |
| Correctness | Identical rendering; existing view `.ci`/`.et` tests pass; both editor-mode and command-only-mode dispatch migrated (A-3) |
| No cycle / inversion preserved | `cli` still imports no engine package; owner `cmd` imports stay in the consumer layer only |
| Fail-closed | Generic factory type-assertion fails with a clear status message, never a nil-driven silent no-op (R-3, `ai/rules/fail-closed-guards.md`) |
| Discovery updates | `ai/rules/plugin-self-containment.md` example updated to point at the now-correct code and corrected symbol name (`ai/rules/discovery-updates.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `view_registry.go` exists with `RegisterView`/`RegisteredViews`/`viewSpec` | `ls` + `grep -n 'func RegisterView' internal/component/cli/view_registry.go` |
| Zero per-feature view fields remain | `grep -n 'pingFactory\|tracerouteFactory\|dashboardFactory\|pingMonitor\|traceroutePiped' internal/component/cli/model.go` → no field declarations |
| Consumers iterate the registry | `grep -n 'RegisteredViews' cmd/ze/hub/session_factory.go internal/component/cli/client/main.go` |
| Review guard runs in verification | `go test` name `TestModelHasNoPerFeatureViewField` present and passing |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | prefix resolution must not match on partial-word boundaries (`monitor bgpx` must not resolve to `monitor bgp`); mirror `matchesPrefix` word-boundary rule |
| Resource lifecycle | migrated views must still cancel their context/`CancelFunc` on stop (no goroutine/channel leak when switching views) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Existing `.ci` regression | Re-read the migrated view's start/poll/render; a behavior change is a bug, not a test to weaken (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Registry contract is a **lifecycle interface** (`Matches`/`Start(input) tea.Cmd`/`Update(msg)`/`Render()`/`Key(k)`/`Stop`), NOT a `{prefix, factory, renderer}` 3-tuple | the skeleton's literal 3-tuple | A-1: factory signatures are heterogeneous (dashboard is a pull poller with no target; ping/traceroute are streaming channels with different arg lists). Only the lifecycle is common. The `viewSpec` carries the prefix + a set of lifecycle funcs; the factory stays per-view |
| **Design 1: registrants live in-package** (`model_ping.go`/`model_traceroute.go`/`model_dashboard.go` `init()` call `RegisterView`); render/update/state stay in `cli` | Design 2: move each registrant into its owner `cmd` package so it self-contains fully | A-2: Design 1 is cycle-free with no code move (render touches `Model` internals). Design 2 (owner→`cli`, `cli` imports no engine) is the fuller self-containment the rule ideal describes but requires exposing a stable exported view interface and moving render out of `cli` — larger, riskier. **Recommend Design 1** for a behavior-preserving refactor; note Design 2 as the follow-up ideal. ~~**OPEN for Thomas.**~~ → AUTONOMOUS DEFAULT (2026-07-17): RESOLVED to Design 1 (registrants live in-package: `model_ping.go`/`model_traceroute.go`/`model_dashboard.go` `init()` call `RegisterView`; render/update/state stay in `cli`). Rationale: cycle-free, behavior-preserving, and requires no code move. Verified against source: `internal/component/cli/*.go` imports no ping/traceroute engine (grep empty), so an in-package `RegisterView` adds zero import edge; the mirror registry `RegisterMonitorProvider` plus word-boundary `matchesPrefix` is real at `internal/component/plugin/server/handler.go:31,70-72`; the `PingFactory` inversion comment (cli defines the contract and never imports the ping engine) is at `model_ping.go:31`. Design 2 (full owner-package self-containment) is recorded as the noted follow-up ideal, NOT this spec's scope. Thomas: override if wrong. |
| `RegisterView` + `viewRegistry` live in **`internal/component/cli`** | `internal/core/*` shared registry | the views are cli-internal (Design 1); the registry is not cross-plugin. If Design 2 is chosen, the `viewSpec` interface + registry move to `internal/component/cli/contract` (leaf) so owner packages can register without importing `cli` |
| Longest-prefix resolution mirrors **`handler.go:matchesPrefix`** (`internal/component/plugin/server/handler.go:70-75`) | first-match ordered `isXCommand` chain (status quo); exact-match map | the daemon-side monitor-provider registry already solved sibling `monitor *` prefixes this way; reuse the pattern rather than reinvent (design-context "check `internal/core`/existing first") |
| Generic factory injection: consumers iterate `RegisteredViews()` and inject each factory by name into a keyed store; typed setters become thin wrappers | keep three typed `Set*Factory` methods | removes the per-feature wiring from the consumers (AC-2) while preserving type safety at the wrapper boundary (R-3) |
| One `viewSpec` per view (3), plain+piped branched inside `Start(input)` | five entries (plain + piped per view) | the `|` split is a rendering-mode detail internal to the view, not a separate command |
| Migration order ping → traceroute → dashboard | all-at-once | dashboard's no-target pull-poll factory is the hardest fit for the common interface; migrate it last, after the streaming pair proves the interface, so a mismatch surfaces against a working baseline (R-1) |
| Review guard reflects over `Model` fields and bans per-view name patterns | doc-only rule | AC-4 needs an executable ratchet; a reflection test is the `TestShowSchemaHasNoBGPPluginCommands`-style mechanical backstop the rule's Mechanical Check demands |

## Known Limitations
- Migration scope is the three named views. The generic `monitor` view (`monitorFactory`/`monitorSession`, `model_monitor.go`) and any future `traffic` view are NOT migrated here; they are the natural next registrants (monitor's `*MonitorSession` session object is the closest precedent for the contract). Scope reduction below these three requires user approval.
- Design 1 leaves each view's render/update code physically in the `cli` package; full owner-package self-containment (Design 2) is ~~deferred pending Thomas's decision~~ the noted follow-up ideal, out of scope for this spec (Design 1 adopted 2026-07-17; see the Key Design Decisions resolution).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Existing view `.ci`/`.et` regression tests still pass (no user-visible change)
- [ ] Registration over hardcoding respected (the point of the spec) — no per-feature field/switch across all five sites
- [ ] Rule doc `ai/rules/plugin-self-containment.md` example repointed at the corrected code

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for registry size and empty-input prefix match
- [ ] Existing view `.ci`/`.et` regression tests pass

## Notes
- Skeleton captured from the 2026-07-16 repository audit (both architecture passes). Deepened to `design` on 2026-07-16: every citation re-verified against the working tree (fields `157-172`, Update switch `524-547`, dispatch `model_keys.go:389-406`/`536-553`, key handling `24-63`, render `model_render.go:318-339`); the per-feature surface is five sites, not the two the skeleton listed; the three factory signatures are heterogeneous (A-1 re-scoped to a lifecycle interface); no import cycle (A-2 confirmed both directions); the daemon-side `RegisterMonitorProvider` registry is the model to mirror; real regression tests are `test/ui/monitor-ping-pipe-resolve-log.ci`, `test/plugin/monitor-traceroute.ci`, `test/plugin/bgp-monitor-dashboard.ci` (the skeleton's `test/cli/*.ci` / `test-cli-ping-view` names do not exist).
