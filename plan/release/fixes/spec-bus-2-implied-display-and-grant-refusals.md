# Spec: bus-2-implied-display-and-grant-refusals

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Configuration correctness and transparency of shipped behavior: a config that grants a feature less than it needs silently loads and moves nothing, AIGP over-triggers Loc-RIB on every iBGP peer by default, and implied endpoints/flows are invisible on the CLI, editor, web UI, and API |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Split from `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` (Child 2 of 3, per R-8) so the `bus { endpoint, flow, hub }` grammar cutover lands first without holding up narrow-grant refusals or the implied-config display.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deliver three capabilities on top of the `bus { endpoint, flow, hub }` model landed by `plan/release/fixes/spec-explicit-plugin-and-attach-config.md`:

1. **Implied configuration display (`| implied`, owner decisions D-10, Q4, Q5):**
   - Nothing Ze implies on an operator's behalf is hidden. One shared pure resolver over `*config.Tree` (with optional live-daemon overrides for Stage-1 claims) computes every implied `endpoint` and implied `flow` together with its provenance (`implied-by`), feeding runtime auto-loading, delivery resolution, and every display surface from a single source of truth so runtime and display cannot drift.
   - **Pipe (`| implied`, bare, no sub-argument):** `show config running | implied`, `show config source <file> | implied`, `show config dump <file> | implied`, and interactive editor `show | implied` (and `show | implied | format config`) show written and implied items together; operators filter with standard pipes such as `| match`.
   - **Text rendering:** Single-hash `# implied by: <reason>` comments (never `##` at column 0, which `config.DetectFormat` classifies as `FormatSetMeta`). Add a hierarchical config `TextRenderer` to `show config running` so human text renders Ze config syntax instead of a generic table.
   - **JSON / YAML rendering:** Structured envelope `{"config": <explicit>, "implied": {"endpoints": [...], "flows": [...]}}` so machine consumers keep explicit config separate from implied items and provenance.
   - **Plain-display notice (R-6):** When implied items exist, plain human displays state the count and name `| implied` — strictly in human channels (editor status bar, `| text` footer on `show config running`/`dump`, `stderr` on `ze config show`), keeping `| json`, `| yaml`, REST/gRPC running endpoints, and `ze config show` stdout byte-pure.
   - **Web & gNMI surfaces:** Web CLI and Web Graphical Inspection expose `show | implied`; Web Workbench tree views (`/show/bus/...`, `/show/bgp/...`) render read-only implied rows/badges inline (Q5 Option B); gNMI `Get` stays explicit-config-only to preserve `Get` -> `Set` round-trips.

2. **Generic plugin-declared implied flows, narrow-grant refusals, and AIGP default fix (D-2 remainder, W-2, R-4, W-6):**
   - Replace hardcoded Loc-RIB and redistribution-orchestrator plugin names in `internal/component/bgp/config/redistribute_binding.go` and `flowspec_binding.go` with generic plugin declarations on `registry.Registration` (generalizing `DisseminationDecl` so feed-only and egress-only features declare their trigger condition and required event/command sets).
   - Where the operator wrote an explicit `bus { flow }` narrower than a configured feature requires (redistribution import/export, FlowSpec authorization, explicit AIGP selection), `ze config validate`, editor commit, and `ze start` refuse with one message shape carrying a typed `FixCommand` naming the feature, the endpoint/flow, and the line that completes it.
   - A default-on capability asks for nothing: AIGP left at its RFC 7311 iBGP default implies no `bgp-rib` flow; only explicit `aigp` configuration triggers it.
   - Filter-owner implicit `state` grants (`filter_state_binding.go`) become implied flows surfaced by `| implied`; fix the `loader_create.go` gap where family/event/send auto-load collectors skip dynamic groups.

3. **Declared intrinsic flows visible on `show event delivery` (D-6, W-5, W-6):**
   - Extend `DeliveryGraph` and `show event delivery` to be multi-namespace and report `origin` (`declared`, `implied`, `intrinsic`), showing intrinsic plugin-declared flows (protocol -> Loc-RIB, Loc-RIB -> `sysrib`, `sysrib` -> FIB, `WantsValidateOpen` removal cleanup observers such as `bgp-role`) beside operator-written and config-implied flows.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Owner ruling: explicit, or refused at commit", "Owner ruling: endpoints and flows on the bus"
  -> Decision: D-10 (implied items run and are shown; an explicit narrower grant is refused at commit) and D-6 (intrinsic flows are declared and shown)
  -> Constraint: Ze never widens an operator-written grant; default-on capabilities imply nothing
- [ ] `docs/architecture/api/commands.md` - pipe operations, `AnswerShape`, `TextRenderer`, command-declared pipe filters
  -> Constraint: `| display` selects row fields (`ArgFields` on `rowShapes`) and cannot be overloaded for config documents; `| implied` is a distinct command-declared pipe operation
- [ ] `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs"
  -> Constraint: refusals that must hold across validate, commit, and start live where all three run them
- [ ] `docs/architecture/web-interface.md`, `docs/architecture/web-workbench-pages.md` - Web Workbench and Graphical Inspection
  -> Constraint: markup is `.templ` with no inline script or style

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe_catalog.go` - defines 17 generic pipe operators including `display` (`ArgFields`, `rowShapes`)
- [ ] `internal/component/cli/model_load.go` - `ClassifyShowPipes` handles editor `show` pipes separately from operational pipes
- [ ] `internal/component/config/cli/running_rpc.go` - `handleShowConfigRunning` returns raw map with no `AnswerShape`, `TextRenderer`, or `PipeFilters`
- [ ] `internal/component/config/serialize_set.go` - `DetectFormat` classifies `#<non-space>` (including `##`) at column 0 as `FormatSetMeta`
- [ ] `internal/component/plugin/server/startup_autoload.go` - computes the 6 auto-load mechanisms at boot and reload
- [ ] `internal/component/bgp/config/redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go` - hardcode Loc-RIB/orchestrator derivations and narrow-grant refusals for FlowSpec/AIGP only
- [ ] `internal/component/bgp/reactor/config_aigp.go` - `AIGPEnabled` defaults true for all iBGP peers, over-triggering `bgp-rib`
- [ ] `internal/component/plugin/server/delivery_graph.go`, `internal/component/cmd/show/show.go` - BGP-only `DeliveryGraph` and `show event delivery`

**Behavior to preserve:**
- An operator-written flow grant is never silently widened.
- Machine outputs (`| json`, `| yaml`, REST/gRPC running config, `ze config show` stdout, gNMI `Get`) remain pure config without prose footers.
- FlowSpec and explicit AIGP continue refusing narrower explicit `bgp-rib` grants.

**Behavior to change:**
- Single pure `ImpliedPlan` resolver over `*config.Tree` feeds runtime auto-load, delivery resolution, and all `| implied` views.
- `| implied` added to `show config running|source|dump`, editor `show`, Web Graphical Inspection, and read-only inline badges on Web Workbench.
- Generic plugin-declared flow requirements replace hardcoded `bgp-rib` and `redistribute-orchestrator` names in BGP config code; redistribution gains narrow-grant refusals; default-on AIGP stops implying `bgp-rib`.
- `show event delivery` shows multi-namespace `declared`, `implied`, and `intrinsic` flows.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Command-declared pipe filters | `internal/component/command/pipe_filter.go` `RegisterPipeFilters` | Declares domain-specific leading pipe ops on operational commands across CLI, REST, gRPC, MCP, Web | Extends: registers `implied` (`ArgNone`) on `show config running`, `source`, and `dump` |
| Editor show pipes | `internal/component/cli/model_load.go` `ClassifyShowPipes` | Parses editor-mode `show \| ...` modifiers (`blame`, `active`, `changes`, `format config`) | Extends: adds `implied` to `ShowPipeOpts` and surfaces R-6 notice in `statusMessage` |
| Plugin auto-load | `internal/component/plugin/server/startup_autoload.go` `getConfigPathPlugins` and siblings | Resolves implied plugins by config root, data plane, dependencies/`ConfigNeeds`, families, custom events, custom sends | Extends: shares one pure `*config.Tree` `ImpliedPlan` resolver with `\| implied` so runtime and display never drift |
| Declared grant needs | `pkg/plugin/rpc/dissemination.go` `DisseminationDecl`; `internal/component/plugin/registry/registry.go` `Registration.Dissemination` | Declares two-sided transit receive/send sets for `bgp-rib`, `bgp-rs`, `bgp-rr` | Extends: generalizes with trigger predicates and optional one-sided receive/command sets to replace hardcoded `redistribute_binding.go` and `flowspec_binding.go` |
| Typed fix of a refusal | `internal/component/cli/fix_frame.go` `FixCommand`, `FrameFix` | Formats refusal fixes as exact editor commands on every surface | Extends: narrow-grant refusals emit a typed `FixCommand` naming the flow line that completes the grant |
| Delivery graph | `internal/component/plugin/server/delivery_graph.go` `DeliveryGraph`; `internal/component/cmd/show/show.go` `handleShowEventDelivery` | Inspects BGP peer delivery edges | Extends: adds multi-namespace edges and `origin` (`declared`, `implied`, `intrinsic`) for D-6 |
| Web Graphical Inspection & Workbench | `internal/component/web/graphical_inspection.go` `graphicalInspectionWords`; `handler_workbench.go` | Renders inspection `show \| ...` views and tree forms/tables | Extends: adds `implied` to Inspection and read-only implied rows/badges to Workbench tree pages |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config validation / commit / daemon start, operational `show config running|source|dump | implied`, editor `show | implied`, Web Workbench pages, and `show event delivery`.

### Transformation Path
1. [owed at design: pure `*config.Tree` `ImpliedPlan` resolution with subsystem hooks registered via `init()`]
2. [owed at design: narrow-grant check shared by validate, commit, and start]
3. [owed at design: rendering `ImpliedPlan` to hierarchical text, structured JSON/YAML, editor status bar, and Web Workbench `.templ` badges]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registration <-> config resolver | Pure registered declarations + subsystem hooks (no `config` -> `bgp` or `iface` imports) | No |

### Changed-Wire Census

**Search evidence:** [owed at design: `show event delivery` RPC additive fields and `show config running | implied` JSON envelope]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [owed at design] | [role] | [carrier] | [evidence] | [disposition] |

### Integration Points
- [owed at design]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every compiled-in feature's minimum required event/command grants are statically known at config time | Phase 1 census of all compiled-in `SetStartupSubscriptions` and `DisseminationDecl` calls | Refusal would defer to runtime for a dynamic feature | Design pass over every compiled-in plugin | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Runtime auto-load / delivery and `\| implied` drift apart | `show event delivery` holds an edge `\| implied` lacks or vice versa | Both consume the same pure `ImpliedPlan` resolver; parity test compares them |
| R-2 | Plain-display R-6 notice breaks JSON/YAML or `ze config show` round-trips | `show config running \| json` or `ze config show` tests fail | Emit R-6 notice only in editor status bar, human `\| text` footer, and CLI `stderr` |
| R-3 | `##` comment marker at column 0 misdetected as `FormatSetMeta` by `config.DetectFormat` | Round-trip parse failure on `\| implied` output | Use single `# implied by: <reason>` uniformly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Valid configs refused on default AIGP, narrow grants silently accepted, or `show config` outputs broken for scripts |
| How is it reverted? | Single spec revert; syntax is already landed by Child 1 |
| Who else touches this path? | Child 1 (`spec-explicit-plugin-and-attach-config.md`) and Child 3 (`spec-bus-3-external-command-gating.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show config running \| implied` and commit of narrow redistribution flow | -> | `ImpliedPlan` resolver and generic narrow-grant check | `test/parse/bus-implied-and-narrow-grant.ci` (planned) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Redistribution rule importing from BGP with no explicit Loc-RIB flow | Config loads at validate, commit, and start; routes redistribute; `\| implied` shows the implied Loc-RIB endpoint and flow naming the rule | A config with no such rule implies no redistribution flow |
| AC-2 | Same redistribution rule with an explicit `bus { flow }` granting a narrower event set than `bgp-rib` declared | Validate, commit, and start refuse with one message shape and typed `FixCommand` naming the missing event types; completing the flow makes it load and redistribute | The operator's narrow flow is never silently widened |
| AC-3 | Redistribution rule exporting to BGP: unwritten flow in one config, narrow explicit command set in another | Unwritten loads, redistributes, and appears in `\| implied`; narrow explicit flow is refused with the completing line | Unrelated endpoints imply nothing |
| AC-4 | FlowSpec family or explicitly configured `aigp` on a peer: unwritten Loc-RIB flow vs narrow explicit flow | Unwritten loads and appears in `\| implied`; narrow explicit flow is refused in the same message shape as AC-2 | Existing narrow-grant refusal semantics preserved |
| AC-5 | Internal BGP peers with `aigp` left at its RFC 7311 default and no explicit `bgp-rib` flow | Config loads at validate, commit, and start without implying `bgp-rib` | Only explicit `aigp` configuration triggers the Loc-RIB requirement |
| AC-6 | Feature blocks (`ntp { }`, shared roots, families, custom event/command tokens, plugin dependencies, data-plane writer) with no explicit `endpoint` | All implied plugins run and `\| implied` on CLI (`show config running\|source\|dump`), editor (`show \| implied`), Web Graphical Inspection, Web Workbench (read-only badges), REST, gRPC, and MCP matches exactly what runs; plain human display shows the R-6 notice while machine outputs remain pure | Operator-written endpoints appear once as written; gNMI `Get` remains explicit-only |
| AC-7 | Running daemon with intrinsic plugin-declared flows (Loc-RIB -> `sysrib`, `sysrib` -> FIB, `WantsValidateOpen` removal cleanup) | `show event delivery` lists written (`declared`), config-implied (`implied`), and intrinsic (`intrinsic`) flows across namespaces | None: every active edge carries an explicit origin |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits a config with `ntp { }` and BGP redistribution, runs `show config running \| implied`, and inspects Web Workbench | Commit -> `ImpliedPlan` -> runtime auto-load + `\| implied` text/JSON + Web Workbench read-only badges | `test/parse/bus-implied-and-narrow-grant.ci` (planned) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [owed at design] | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric inputs introduced | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-implied-and-narrow-grant` (planned) | `test/parse/` | AC-1 to AC-6 | |

## Files to Modify
- `internal/component/config/` - pure `ImpliedPlan` resolver and subsystem registration hooks
- `internal/component/plugin/server/startup_autoload.go`, `delivery_graph.go` - consume `ImpliedPlan`; multi-namespace `DeliveryGraph` with origin
- `internal/component/bgp/config/redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go`, `loader_create.go` - generic plugin-declared flow requirements, explicit AIGP trigger, dynamic-group family/event/send collection
- `internal/component/command/pipe_catalog.go`, `internal/component/config/cli/running_rpc.go`, `cmd_show.go`, `cmd_dump.go`, `internal/component/cli/model_load.go`, `model_commands_show.go` - `| implied` pipe, `show config running` hierarchical `TextRenderer`, R-6 notice
- `internal/component/web/graphical_inspection.go`, `handler_workbench.go` and `.templ` views - Web Graphical Inspection `implied` and Workbench read-only implied badges
- `internal/component/cmd/show/show.go` - `show event delivery` D-6 output

## Files to Create
- `test/parse/bus-implied-and-narrow-grant.ci` - functional test for AC-1 to AC-6

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing functional test `test/parse/bus-implied-and-narrow-grant.ci`
2. [owed at design]

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bare `\| implied` pipe (Q4, 2026-11-23) | `\| display implied`; `\| implied [only]` | `display` already selects row fields; bare `\| implied` shows written + implied together and composes with `\| match` |
| Single `# implied by:` comment marker + structured JSON/YAML envelope (Q4, 2026-11-23) | `##` marker; merging implied keys into raw JSON config | `##` at column 0 breaks `DetectFormat`; structured envelope keeps machine config pure |
| Inline read-only implied rows/badges in Web Workbench; gNMI `Get` explicit-only (Q5, 2026-11-23) | Inspection-only in Web; injecting implied into gNMI | Gives Workbench operators full visibility without breaking gNMI `Get` -> `Set` round-trips |

## Known Limitations
- gNMI `Get` returns explicit written configuration only to preserve round-trip `Set` semantics.
- External command permission gating (D-8) is owned by `plan/release/fixes/spec-bus-3-external-command-gating.md`.
