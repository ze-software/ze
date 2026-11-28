# Spec: Daemon runtime one-off defects (5) - MCP tools/list byte stability, readiness, and plugin/LG functional tests

| Field | Value |
|-------|-------|
| Status | design |
| Scope | api |
| Priority | P1 |
| Release reason | Fixes four open external-API-surface and functional-test defects: non-deterministic bytes in MCP `tools/list`, readiness race in `mcp-header-mismatch-rejected.ci` (`wait-tool ze_execute`), and seven failing `test/plugin` LG/BMP/RIB functional tests |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the four open MCP and `test/plugin` (LG/BMP/RIB) journal rows routed from the umbrella.

## Task

Fix four external-API-surface defects verified open at HEAD on 2026-11-26:

1. **MCP `tools/list` byte-for-byte determinism (`output-not-byte-stable`, 2026-08-10).** `test/plugin/mcp-tools-list-deterministic-order.ci` failed with `tools-order stable=false calls=8 diverged-on-call=7 names=identical byte-offset=22848`. Audit every source that feeds `Streamable.allTools` (`internal/component/mcp/streamable_tools.go`, `internal/component/mcp/tools.go`, `cmd/ze/hub/command_meta.go`): ensure `buildParamMeta` (`wireToPaths` -> `pathToWire`, `defsByPath`, `inputs`), `groupCommands`, `addYANGParams` (`requiredParams`), `buildToolDef`, and any dynamic command registration ordering produce byte-identical JSON across consecutive `tools/list` calls once startup is ready, and prove both with an isolated unit test repeating `tools/list` serialization and with `test/plugin/mcp-tools-list-deterministic-order.ci` passing repeatedly.
2. **MCP `tools/list` readiness race (`unwired-feature`, 2026-08-15 x2).** `test/plugin/mcp-header-mismatch-rejected.ci` failed on its first line (`wait-tool ze_execute`) under both working-tree and clean `git archive HEAD` runs because `wait-tool` polled `tools/list` while the MCP server or command metadata readiness raced daemon startup. Trace what delays or rejects `tools/list` for `ze_execute` during startup in `internal/component/mcp/streamable_tools.go` and `cmd/ze/hub/service_mcp.go`, ensure `ze_execute` is served as soon as the MCP listener accepts requests (or emit an explicit daemon readiness line and wait on it in `mcp-header-mismatch-rejected.ci` and `internal/test/cli/cmd_mcp.go`), and prove `mcp-header-mismatch-rejected.ci` passes cleanly across repeated runs.
3. **Seven `test/plugin` LG/BMP/RIB functional tests (`test-against-broken-path`, 2026-08-23).** Reproduce and fix the root cause (producing code or stale fixture expectation with `ze-test-judge` approval if a fixture expectation itself is wrong) for each of the seven `.ci` tests named in the journal row: `test/plugin/lg-graph-lab.ci`, `test/plugin/task-no-extension.ci`, `test/plugin/task-rib-routes.ci`, `test/plugin/show-rib-under-load.ci`, `test/plugin/bmp-lg-disconnect.ci`, `test/plugin/bmp-lg-ingest.ci`, and `test/plugin/bmp-lg-bestpath-isolation.ci`. Note: all looking-glass `.ci` tests stay in `test/plugin/*.ci`, never `test/web/` (`ai/doors/api-surfaces.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, one command registry behind every interface
- [ ] `ai/doors/api-surfaces.md` - MCP 2026-07-28 profile, LG birdwatcher contract, LG `.ci` suite placement in `test/plugin/`
- [ ] `docs/architecture/mcp/overview.md` - Streamable HTTP transport, tool derivation, caching
- [ ] `docs/guide/mcp/overview.md` - MCP tool auto-generation from command registry
- [ ] `docs/architecture/api/commands.md` - command registry, pipe list schema, `## Design decisions`
- [ ] `docs/architecture/api/birdwatcher-compat.md` - LG JSON contract and BMP-LG route keys

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `internal/component/mcp/tools.go` & `streamable_tools.go` - `allTools` calls `s.cfg.Commands()`, `groupCommands(infos)`, `generateTools(groups, handcraftedNames())`; `groupCommands` iterates `byOne` and `byTwo` Go maps and `addYANGParams` appends `requiredParams` in parameter-slice order without sorting `required`.
- [ ] `cmd/ze/hub/command_meta.go` - `buildParamMeta` (L313-370) ranges over `wireToPaths` and `pathToWire` maps; `buildCommandMeta` sorts `infos` by `a.Name` (`695447fec7`), while `test/plugin/mcp-tools-list-deterministic-order.ci` failed after `695447fec7`.
- [ ] `cmd/ze/hub/service_mcp.go` & `test/plugin/mcp-header-mismatch-rejected.ci` - `mcp-header-mismatch-rejected.ci` runs `wait-tool ze_execute` against `ze --mcp $PORT -` without waiting on a daemon-emitted MCP readiness signal.
- [ ] `test/plugin/{lg-graph-lab,task-no-extension,task-rib-routes,show-rib-under-load,bmp-lg-disconnect,bmp-lg-ingest,bmp-lg-bestpath-isolation}.ci` - all seven `.ci` files exist under `test/plugin/` (last touched in `b1eba99cf9`) and must be verified green at HEAD with root causes documented and repaired if still red.

**Behavior to preserve:**
- MCP 2026-07-28 Streamable HTTP header validation (`Mcp-Method`, `Mcp-Name`, `MCP-Protocol-Version`) continues to reject every mismatch with HTTP 400 and JSON-RPC `-32020`.
- MCP `allTools` continues to return an error (never a truncated list) if command metadata or schema generation fails.
- All LG `.ci` tests remain under `test/plugin/*.ci`, never moved to `test/web/`.
- No test assertions are weakened without an independent `ze-test-judge` approval.

**Behavior to change:**
- Every slice and map-derived collection inside `buildParamMeta`, `groupCommands`, `addYANGParams`, and `buildToolDef` is canonically sorted so `tools/list` output is byte-identical across consecutive calls.
- MCP startup readiness is deterministic so `wait-tool ze_execute` / `mcp-header-mismatch-rejected.ci` never loses a startup race.
- Any still-failing test among the seven `test/plugin/` tests has its underlying producer defect fixed (or verified green and closed with evidence).

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| MCP tool grouping and schema generation | `internal/component/mcp/tools.go` `groupCommands`, `generateTools`, `buildToolDef`, `addYANGParams`; `streamable_tools.go` `allTools`; `cmd/ze/hub/command_meta.go` `buildCommandMeta`, `buildParamMeta` | Derives MCP tool JSON schemas from live command metadata; `buildCommandMeta` sorts `infos` by name, `groupCommands` sorts groups and actions, but map-backed parameter/required/subgroup paths can still diverge across calls or mid-startup | Extends: canonically sorts every slice derived from map iteration (`required`, `byOne`/`byTwo` traversal, parameter lists) and removes startup tool-list instability |
| MCP Streamable HTTP server startup | `cmd/ze/hub/service_mcp.go` `buildMCPService`, `startMCPServer`; `test/plugin/mcp-header-mismatch-rejected.ci` | Starts MCP listener and logs listening line while `mcp-header-mismatch-rejected.ci` polls `wait-tool ze_execute` | Extends: binds MCP readiness so `tools/list` serves `ze_execute` deterministically on first accept and `mcp-header-mismatch-rejected.ci` awaits daemon readiness cleanly |
| LG and BMP functional test suite | `test/plugin/{lg-graph-lab,task-no-extension,task-rib-routes,show-rib-under-load,bmp-lg-disconnect,bmp-lg-ingest,bmp-lg-bestpath-isolation}.ci`; `internal/component/lg/` | Exercises real `bgp-rib`, BMP, MCP tasks, and looking glass over HTTP in `test/plugin/` | Extends: diagnoses and fixes any remaining red producer defect among the seven tests while keeping LG `.ci` files in `test/plugin/` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "MCP derives its tools from it at call time, so a new command appears with no edit to the interface packages" | agrees | Fixes byte-order stability and readiness inside the shared command-metadata and MCP tool derivation pipeline without hardcoding tools. |
| `ai/doors/api-surfaces.md` | Where it sits | the door covering `internal/component/mcp/` and `internal/component/lg/` | "They are thin adapters." | agrees | Keeps MCP and LG as thin adapters over the shared command dispatcher and keeps all LG `.ci` tests under `test/plugin/`. |
| `ai/doors/hub-engine.md` | Where it sits | the door covering `cmd/ze/hub/command_meta.go` and `service_mcp.go` | "`cmd/ze` is one program with several personalities." | agrees | Repairs deterministic parameter sorting in `command_meta.go` and MCP startup readiness in `service_mcp.go` without adding gated imports to always-on hub code. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/mcp-header-mismatch-rejected.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | Updates `mcp-header-mismatch-rejected.ci` to wait on daemon readiness and verifies the seven `test/plugin/*.ci` tests in place. |
| `ai/rules/plugins.md` | Directives | `// Design:` page of `cmd/ze/hub/command_meta.go` and `service_mcp.go` | "The mechanism behind every directive here is documented, and the page MUST be read before the plugin work it" | agrees | Keeps command metadata derivation registration-driven across all builtin and plugin commands. |
| `docs/guide/mcp/overview.md` | Starting the MCP Server | `// Design:` page of `internal/component/mcp/tools.go` | "Starting the MCP Server" — silent (guide covers CLI flags and startup options, not internal map-sort order of `tools/list` JSON) | silent | No MCP startup CLI flags or user-facing options change. |
| `docs/architecture/mcp/overview.md` | Protocol Profile | `// Design:` page of `internal/component/mcp/streamable_tools.go`; anchored to `tools.go` | "`ProtocolVersion` is `2026-07-28` and `supportedProtocolVersions` holds exactly" | agrees | Enforces the MCP `2026-07-28` `tools/list` deterministic-order SHOULD requirement and preserves `-32020` header validation. |
| `docs/architecture/api/commands.md` | Overview | `// Design:` page of `internal/component/mcp/tools.go`; anchored to `streamable_tools.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Sorts command metadata and derived tool schemas canonically without altering any command path or answer format. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Preserves the pipe list parameter on every derived MCP tool while making its surrounding schema serialization byte-stable. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | No pipe override or data format changes; MCP tool derivation continues reading the shared pipe schema unchanged. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Pipe execution semantics are untouched; only `tools/list` byte ordering, MCP readiness, and the seven `test/plugin` tests are repaired. |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `internal/component/mcp/tools.go` | silent (page describes the overall API transport architecture and bus rules, not MCP JSON schema slice sorting) | silent | Preserves all API transport contracts and bus participant isolation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not touch `\| implied` formatting. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant registrations are unchanged; MCP reads the registered command catalog as before. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus send-destination rules are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus receive-source rules are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-list enforcement is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | No bus arbitration is introduced. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus display commands are untouched. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `cmd/ze/hub/command_meta.go` | "**YANG defines format. Extensions declare behavior. Implementation executes behavior.**" | agrees | Preserves YANG as the single source of MCP command parameter metadata while sorting extracted parameter slices canonically. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- MCP `POST` JSON-RPC `tools/list` and `tools/call` against `Streamable`.
- `ze --mcp $PORT -` startup in `test/plugin/mcp-header-mismatch-rejected.ci` and `mcp-tools-list-deterministic-order.ci`.
- HTTP requests against the looking glass (`internal/component/lg`) and MCP task endpoints in the seven `test/plugin/*.ci` files.

### Transformation Path
1. `commandMetaSource` -> `buildParamMeta` (canonically sorted) -> `buildCommandMeta` (deduped, sorted by name) -> `groupCommands` (deterministic traversal) -> `buildToolDef` (sorted `required` and parameter lists) -> `json.Marshal` -> byte-identical `tools/list` response across repeated calls.
2. Daemon binds MCP listener only after `Streamable` and command lister are wired and emits readiness signal -> `mcp-header-mismatch-rejected.ci` waits on readiness -> `wait-tool ze_execute` succeeds immediately on first call.
3. Each of the seven `test/plugin/*.ci` tests drives the real daemon and verifies expected LG/BMP/MCP-task output.

### Boundaries Crossed
- External MCP 2026-07-28 HTTP JSON-RPC contract (`tools/list` determinism SHOULD requirement and `-32020` header validation).
- Looking-glass birdwatcher HTTP JSON contract (`docs/architecture/api/birdwatcher-compat.md`).

### Integration Points
- `internal/component/mcp` (`tools.go`, `streamable_tools.go`, `tools_determinism_test.go`).
- `cmd/ze/hub/command_meta.go`, `cmd/ze/hub/service_mcp.go`.
- `internal/component/lg/` (if BMP-LG key format defect remains open on reproduction).
- `test/plugin/*.ci`.

### Changed-Wire Census

**Search evidence:** Searched `internal/component/mcp/`, `cmd/ze/hub/`, `internal/test/cli/`, and `test/plugin/mcp-*.ci` (excluding `*_test.go`) with `git grep -n -E` for `methodToolsList|allTools|buildParamMeta|groupCommands|addYANGParams|wait-tool|tools/list`. Positive controls confirmed: `internal/component/mcp/streamable_tools.go:39,171,233` (`methodToolsList`, `allTools`), `internal/component/mcp/tools.go:171,402,532` (`groupCommands`, `addYANGParams`), `cmd/ze/hub/command_meta.go:102,307` (`buildParamMeta`), `internal/test/cli/cmd_mcp.go:252` and `internal/test/cli/cmd_mcp_calls.go:95,139,178` (`wait-tool`, `toolsList`, `toolsOrderStable`), and `test/plugin/mcp-header-mismatch-rejected.ci:28` / `test/plugin/mcp-tools-list-deterministic-order.ci`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `mcp:tools/list` | writer | `internal/component/mcp/streamable_tools.go` `allTools`; `internal/component/mcp/tools.go` `groupCommands`, `buildToolDef`, `addYANGParams`; `cmd/ze/hub/command_meta.go` `buildParamMeta` | `streamable_tools.go:171,233` dispatches `methodToolsList` via `allTools` -> `groupCommands` (`tools.go:171`) -> `addYANGParams` (`tools.go:402,532`) fed by `buildParamMeta` (`command_meta.go:102,307`) | Change: canonically sort every map-derived slice (`required`, `byOne`/`byTwo` traversal, parameter lists) and eliminate startup readiness races so `tools/list` emits byte-identical JSON across consecutive calls |
| `mcp:tools/list` | reader | `internal/test/cli/cmd_mcp.go` + `internal/test/cli/cmd_mcp_calls.go` (`wait-tool`, `toolsList`, `toolsOrderStable`); `test/plugin/mcp-tools-list-deterministic-order.ci`; `test/plugin/mcp-header-mismatch-rejected.ci` | `cmd_mcp.go:252` and `cmd_mcp_calls.go:95,139,178` send `methodToolsList` and assert raw byte stability and tool presence for the `.ci` scripts | Change / preserve: update `mcp-header-mismatch-rejected.ci` to wait on the daemon readiness signal while preserving raw byte-stability and `-32020` mismatch assertions unchanged |
| `mcp:tools/list` | reader | External MCP `2026-07-28` Streamable HTTP clients (outside repository) | `ai/doors/api-surfaces.md` and `docs/architecture/mcp/overview.md` define the external `2026-07-28` Streamable HTTP client contract | Preserve: wire schema and JSON-RPC envelope stay unchanged; response bytes become deterministically ordered as MCP `2026-07-28` recommends |

### Architectural Verification
- [ ] `tools/list` produces byte-identical JSON across 100 consecutive calls under randomized Go map iteration
- [ ] `mcp-header-mismatch-rejected.ci` awaits a daemon-emitted readiness signal and passes 3/3 consecutive runs
- [ ] All seven named `test/plugin/*.ci` tests pass serially and in the plugin suite, with all LG `.ci` tests kept under `test/plugin/`

## Risks & Assumptions

### Assumptions
- `test/plugin/mcp-tools-list-deterministic-order.ci` byte divergence at offset 22848 may stem from either an unsorted slice in `tools.go`/`command_meta.go` or a late-arriving plugin command mutating `Registry().All()` between calls 1 and 7; both causes must be checked by reproducing the test first.
- Some of the seven `test/plugin/*.ci` tests may already pass after `b1eba99cf9` and `b243c9221b`; each must be run to confirm pass vs fail before modifying code.

### Risks
- Never weaken assertions in any of the nine `.ci` files to turn a red green; any test expectation change requires proving the old expectation wrong against the spec/producer and obtaining `ze-test-judge` approval.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Repeated `POST` `tools/list` over MCP Streamable HTTP | → | `Streamable.allTools` -> `groupCommands` / `buildToolDef` | `TestToolsListByteIdenticalAcrossMapOrders` (`internal/component/mcp/tools_determinism_test.go`) + `test/plugin/mcp-tools-list-deterministic-order.ci` |
| Startup + `wait-tool ze_execute` | → | `service_mcp.go` + `Streamable.allTools` | `test/plugin/mcp-header-mismatch-rejected.ci` |
| LG / BMP / RIB / MCP-task functional paths | → | `internal/component/lg/`, `internal/component/mcp/`, `bgp-rib` | Seven named `test/plugin/*.ci` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Repeated MCP `POST tools/list` calls against an unchanged command set under randomized Go map iteration (`output-not-byte-stable`, 2026-08-10) | `Streamable.allTools` emits byte-identical JSON across consecutive calls with canonically sorted tool groups, actions, parameter definitions, and `required` arrays, passing `TestToolsListByteIdenticalAcrossMapOrders` and `test/plugin/mcp-tools-list-deterministic-order.ci` | Legitimate runtime changes to registered plugins or commands may update subsequent `tools/list` results once registration changes settle |
| AC-2 | Starting `ze --mcp $PORT -` and running `test/plugin/mcp-header-mismatch-rejected.ci` across 3 consecutive runs (`unwired-feature`, 2026-08-15 x2) | The daemon emits its readiness signal after wiring `Streamable` and the command lister, `mcp-header-mismatch-rejected.ci` awaits that signal, `wait-tool ze_execute` succeeds immediately on first poll, and mismatched headers continue returning HTTP 400 with JSON-RPC `-32020` across 3/3 runs | None; startup readiness must be deterministic with zero polling-timeout reliance |
| AC-3 | Running the seven named `test/plugin/*.ci` tests (`lg-graph-lab`, `task-no-extension`, `task-rib-routes`, `show-rib-under-load`, `bmp-lg-disconnect`, `bmp-lg-ingest`, `bmp-lg-bestpath-isolation`) under `./le test functional plugin` (`test-against-broken-path`, 2026-08-23) | All seven `.ci` tests pass cleanly in `test/plugin/` after fixing each reproduced producer defect (and recording `ze-test-judge` approval if any stale fixture expectation is proven wrong against its contract) | None; all seven named tests stay under `test/plugin/` and must pass with full assertions intact |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestToolsListByteIdenticalAcrossMapOrders` | `internal/component/mcp/tools_determinism_test.go` | AC-1 |
| `TestStreamableToolsListServesExecuteOnFirstCall` | `internal/component/mcp/streamable_test.go` | AC-2 |

### Functional Tests

| Test | AC |
|------|----|
| `test/plugin/mcp-tools-list-deterministic-order.ci` | AC-1 |
| `test/plugin/mcp-header-mismatch-rejected.ci` | AC-2 |
| `test/plugin/lg-graph-lab.ci`, `task-no-extension.ci`, `task-rib-routes.ci`, `show-rib-under-load.ci`, `bmp-lg-disconnect.ci`, `bmp-lg-ingest.ci`, `bmp-lg-bestpath-isolation.ci` | AC-3 |

## Files to Modify
- `internal/component/mcp/tools.go`
- `internal/component/mcp/tools_determinism_test.go`
- `internal/component/mcp/streamable_tools.go`
- `internal/component/mcp/streamable_test.go`
- `cmd/ze/hub/command_meta.go`
- `cmd/ze/hub/service_mcp.go`
- `test/plugin/mcp-header-mismatch-rejected.ci`
- Any producing file under `internal/component/lg/` or `internal/component/mcp/` implicated when reproducing the seven `test/plugin/*.ci` tests

## Files to Create
- None.

### Integration Checklist
- [ ] `mcp-tools-list-deterministic-order.ci` green 3/3
- [ ] `mcp-header-mismatch-rejected.ci` green 3/3 with readiness signal
- [ ] All seven `test/plugin/*.ci` tests green under `./le test functional plugin`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/mcp/overview.md` - confirm deterministic `tools/list` ordering and readiness behavior are accurately described

## Design presentation

### What changes
- Canonically sorts all map-derived slices in MCP tool generation (`internal/component/mcp/tools.go`, `cmd/ze/hub/command_meta.go`) and eliminates startup race windows between MCP listener bind and `tools/list` readiness (`cmd/ze/hub/service_mcp.go`, `test/plugin/mcp-header-mismatch-rejected.ci`).
- Diagnoses and repairs any remaining failures across the seven named `test/plugin/*.ci` tests (keeping all LG `.ci` tests under `test/plugin/`).

### Before and after
- **Before:** `mcp-tools-list-deterministic-order.ci` diverged on repeated `tools/list` bytes; `mcp-header-mismatch-rejected.ci` raced `wait-tool ze_execute` during daemon startup; seven `test/plugin` LG/BMP/RIB tests were recorded red without confirmed fixes.
- **After:** `tools/list` is byte-stable across repeated calls; MCP startup readiness is explicit and race-free; all nine functional `.ci` tests pass cleanly.

### Alternatives set aside
- **Extending the `wait-tool` timeout in `mcp-header-mismatch-rejected.ci` without fixing the readiness race:** Rejected under `ai/rules/testing.md` (assert on state/readiness rather than arbitrary time).
- **Moving LG `.ci` tests to `test/web/`:** Rejected per `ai/doors/api-surfaces.md` (`test/plugin/*.ci` owns LG HTTP tests driving real `bgp-rib`).

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Reproduce `mcp-tools-list-deterministic-order.ci`, `mcp-header-mismatch-rejected.ci`, and the seven named `test/plugin/*.ci` tests at HEAD (recording exact red outputs or confirming which already pass).
2. Write unit tests `TestToolsListByteIdenticalAcrossMapOrders` and `TestStreamableToolsListServesExecuteOnFirstCall`, then fix canonical sorting in `tools.go`/`command_meta.go` and startup readiness in `service_mcp.go` + `mcp-header-mismatch-rejected.ci`.
3. Fix each reproduced producer defect behind any still-red test among the seven `test/plugin/*.ci` tests.
4. Verify all unit tests and all nine `.ci` tests pass cleanly and mark the four owned journal rows `fixed:` in `plan/journal/`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `output-not-byte-stable` | 2026-08-10 | mcp tools list |
| `unwired-feature` | 2026-08-15 | mcp `tools/list` first line fails |
| `unwired-feature` | 2026-08-15 | mcp `tools/list` readiness, rerun from `git archive HEAD` |
| `test-against-broken-path` | 2026-08-23 | seven `test/plugin` tests red |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 1 through 4 and 6 through 7.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] All four owned journal rows marked `fixed:` in `plan/journal/`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label api-surfaces-fixes command go test -race ./internal/component/mcp/... ./internal/component/lg/... ./cmd/ze/hub/...`, `./le test functional plugin`
