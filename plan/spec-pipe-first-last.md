# Spec: `| first N`, `| last N`, and pipe metadata

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/8 |
| Updated | 2026-05-30 |

> Origin: discovered missing during review of `spec-rib-show-bounded-dump.md`. Ze's pipe
> framework has no way to bound output to the first/last N items. `first`/`last` should be
> generic pipe operators (like `count`) with an optimization path: commands that register
> them as pipe filters get server-side early termination via `FoldFilters`, so the generator
> stops producing items instead of building the full result and truncating client-side.
>
> Additionally, JSON output should carry a `"pipe"` metadata dict recording which
> data-shaping modifiers were active, so consumers know the output was filtered, capped,
> or aggregated.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/pipe-completeness.md` - every output command supports all pipe operators
4. `internal/component/command/pipe.go` - generic pipe parsing, `FoldFilters`, `ApplyPipes`
5. `internal/component/command/pipe_filter.go` - command-specific filter registration
6. `internal/component/command/completer.go` - `PipeOperators` slice (hardcoded, independent of `knownPipeOps`)
7. `internal/component/bgp/plugins/cmd/rib/rib.go` - RIB pipe filter registration
8. `internal/component/bgp/plugins/rib/rib_pipeline.go` - RIB show pipeline (first consumer)

## Task

Ze's pipe framework supports `| count`, `| match`, `| json`, `| table`, etc. but has no way
to take only the first or last N items from a command's output. This means any bounded view
(e.g. "show me the first 100 routes") requires the command to produce the full result, the
full JSON to be marshalled, and only then the client discards everything past N.

Goal: add `| first N` and `| last N` as generic pipe operators that work on any command's
JSON output. Additionally, commands that register `first`/`last` as command-specific pipe
filters (via `RegisterPipeFilters`) get the value folded into server-side args, enabling
the command handler to stop generation early (for `first`) or to buffer only the trailing
window (for `last`).

The pattern already exists: `count` is both a generic pipe operator (`pipeCount` with
`applyCount` client-side) and a command-specific filter that the RIB registers. `FoldFilters`
promotes it to a server-side arg when the command supports it. `first`/`last` follow the
same dual path.

Second goal: add a `"pipe"` metadata dict to JSON output recording which data-shaping
modifiers were applied. This lets consumers distinguish a full result from a filtered,
capped, or aggregated one. The dict uses the pipe operator name as key, with typed values:
`true` for no-arg operators, the integer for numeric args (`first`, `last`), and the
string for string args (`family`, `prefix`, `path`, `community`, `match`, `peer`).
Display-only pipes (`json`, `table`, `text`, `yaml`, `ndjson`, `resolve`, `origin`,
`log`, `no-more`) do not appear in the metadata since they don't change what data is
present.

Example: `show bgp rib | received | family ipv4-unicast | first 100 | count` produces:

```json
{
  "count": 100,
  "pipe": {
    "received": true,
    "family": "ipv4-unicast",
    "first": 100,
    "count": true
  }
}
```

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/pipe-completeness.md` - every output command supports all pipe operators
  -> Constraint: `first`/`last` must work on any command that routes through `ApplyPipes`.
- [ ] `ai/rules/cli-grammar.md` - action before identifier
  -> Constraint: pipe syntax is `| first 10`, `| last 5` (operator then numeric arg).
- [ ] `ai/rules/json-format.md` - kebab-case JSON keys
  -> Constraint: the `"pipe"` metadata key and all its sub-keys must be kebab-case.

**Key insights:**
- `count` already demonstrates the dual generic+command-specific pattern. `pipeCount` in `knownPipeOps` handles client-side; the RIB registers `count` as a `PipeFilter` so `FoldFilters` promotes it to a server arg. Same pattern for `first`/`last`.
- `FoldFilters` classifies known pipe kinds (pipeCount, pipeMatch, etc.) by their enum, then falls through to `pipeUnknown` for command-specific filters. `first`/`last` get their own `pipeKind` enums so `FoldFilters` can handle promotion.
- Client-side `applyFirst`/`applyLast` operates on parsed JSON: for arrays, take first/last N elements; for single-key wrapper maps (the `unwrapSingleKeyArray` pattern), walk the inner array. For multi-key maps and non-array values, pass through unchanged (server-side promotion avoids this case for RIB commands).
- Server-side: when a command like `show bgp rib` registers `first` as a `PipeFilter`, `FoldFilters` appends `first 10` to the command string. The handler sees the arg and stops the iterator at N. For `last`, the handler keeps a sliding window of N items. Note: `last` cannot short-circuit the source iterator (must drain fully to find the trailing window), so it only saves serialization cost, not iteration cost.
- Pipe completion: `PipeOperators` in `completer.go` is a hardcoded slice independent of `knownPipeOps`. Both must be updated.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/component/command/pipe.go` - `pipeKind` enum (22-34), `knownPipeOps` (49-61), `ParsePipe` (65-101), `FoldFilters` (105-159), `ApplyPipes` (210-256), `applyCount` (319-336). Generic pipe operators are parsed by `ParsePipe`, classified by `pipeKind`, and applied by `ApplyPipes`. `FoldFilters` checks if the command registers a matching filter and promotes the op from client-side to server-side arg. `ParsePipe`'s switch (86-96) extracts args only for `pipeMatch` and `pipeJSON`; `pipeFirst`/`pipeLast` need their own cases to extract the numeric arg.
  -> Constraint: `pipeKind` enum values must be added before `pipeUnknown` so that `FoldFilters` switch handles them.
  -> Constraint: `applyFirst`/`applyLast` must follow the same JSON-or-line-fallback pattern as `applyCount`.
  -> Constraint: `ParsePipe` switch must extract `op.arg = fields[1]` for `pipeFirst`/`pipeLast` (like `pipeMatch` extracts its pattern).
- [ ] `internal/component/command/pipe_filter.go` - `PipeFilter` struct (14-20), `RegisterPipeFilters` (35-55). Commands register filters with `Name`, `Description`, `TakesArg`, `Leading`. `FoldFilters` looks up unknown ops against the command's registered set.
  -> Decision: `first` and `last` are `TakesArg: true` (the N value).
  -> Decision: `first` and `last` are NOT `Leading: true` (they don't affect scope selection; they control output volume).
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` (51-78). The RIB registers `count`, `prefix-summary`, `graph` as pipe filters. Same registration point for `first`/`last`.
  -> Constraint: RIB is the first consumer that will register `first`/`last` for server-side early termination.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `parsePipelineArgs` (1009+), `pipelineStage.apply` (957-981), `showPipeline` (901-948). Pipeline args are parsed into scope + filter/terminal stages. A `first N` arg would become a pipeline stage that wraps the upstream iterator and stops after N items. `last N` would buffer a trailing window.
  -> Decision: `first`/`last` are filter stages (not terminals). User controls ordering: `| family ipv4 | first 5` = first 5 IPv4 routes; `| first 5 | family ipv4` = from first 5 routes, keep IPv4 only. Position matters, as with all other filters. When composed with a terminal: `| first 100 | count` returns 100.

**Behavior to preserve:**
- All existing pipe operators unchanged.
- All existing command-specific filters unchanged.
- `FoldFilters` promotion logic for existing filters (count, match, received, etc.).
- JSON output shapes for all commands.

**Behavior to change:**
- Add `pipeFirst`/`pipeLast` to the `pipeKind` enum and `knownPipeOps`.
- Add `pipeFirst`/`pipeLast` cases in `ParsePipe` switch to extract the numeric arg.
- Add `applyFirst`/`applyLast` in `ApplyPipes`.
- Add `first`/`last` cases in `FoldFilters` for server-side promotion.
- Add `first`/`last` validation in `ValidatePipes` (positive integer required).
- Add pipe completion entries in `PipeOperators` (completer.go).
- `FoldFilters` collects pipe metadata (both folded and remaining) into a `map[string]any`; return type changes to `(string, []pipeOp, map[string]any)`. All 6 callers (`ProcessPipes`, `ProcessPipesChecked`, `ProcessPipesDefaultFormat`, `ProcessPipesDefaultFormatChecked`, `ProcessPipesDetectLog`, `ProcessPipesDefaultFunc`) must accept and forward the metadata map.
- `ApplyPipes` injects `"pipe"` dict into JSON output after all data transforms (parse JSON, add key, re-serialize). Table/text renderers must skip the `"pipe"` key when rendering rows.
- RIB: register `first`/`last` as pipe filters; add pipeline stages. Server-side handlers (`showPipeline`) include folded-filter metadata in their JSON output via a `"pipe"` key.

## Data Flow (MANDATORY)

### Entry Point
- User types: `show bgp rib | received | first 100 | table`
- `ParsePipe` splits into command `show bgp rib` and ops `[{pipeFirst,"100"}, {pipeTable,""}]` (after `received` is classified as `pipeUnknown` since it's command-specific).

### Transformation Path
1. `ParsePipe` -> command string + `[]pipeOp`.
2. `FoldFilters` -> checks command's registered filters. `received` (Leading) -> prepended to command args. `first` -> if command registers it, promoted to server arg (`show bgp rib received first 100`); if not, stays as `pipeFirst` client-side op.
3. Command handler executes with server args. RIB: `parsePipelineArgs` sees `first 100`, creates a `firstFilter{limit: 100}` pipeline stage.
4. Pipeline runs: source -> filters -> firstFilter (stops at 100) -> terminal.
5. Terminal output returned as JSON string.
6. `ApplyPipes` runs remaining client-side ops (`| table` formatting).
7. If `first` was NOT promoted (command doesn't register it), `ApplyPipes` runs `applyFirst` on the JSON string: parse, take first 100 items from arrays (or unwrap single-key wrapper first), re-serialize. Non-array, non-single-key-wrapper values pass through unchanged.
8. `ApplyPipes` injects/merges `"pipe"` metadata dict into the JSON output for all data-shaping ops that were applied (both server-folded and client-side). The metadata map is built by `FoldFilters` (which records both promoted and remaining ops) and passed through to `ApplyPipes`. If the server already added a `"pipe"` key (from folded filters), client-side entries are merged into it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| User input -> pipe parsing | `ParsePipe` splits on `\|` | [ ] |
| Client pipe -> server arg | `FoldFilters` promotes registered filters | [ ] |
| Server arg -> pipeline stage | `parsePipelineArgs` in RIB | [ ] |
| Pipeline -> JSON output | terminal `drain` + `json.Marshal` | [ ] |

### Integration Points
- `pipe.go` - new `pipeKind` values, `knownPipeOps` entries, `ParsePipe` arg extraction, `FoldFilters` cases + return type change (adds `map[string]any` metadata), `ApplyPipes` cases + `"pipe"` dict injection, `ValidatePipes` cases.
- `pipe_filter.go` - no changes (registration mechanism is generic).
- `completer.go` - `first`/`last` entries in `PipeOperators` slice (hardcoded, independent of `knownPipeOps`).
- All `ProcessPipes*` variants in `pipe.go` - accept and forward metadata map from `FoldFilters` to `ApplyPipes`. 6 callers: `ProcessPipes`, `ProcessPipesChecked`, `ProcessPipesDefaultFormat`, `ProcessPipesDefaultFormatChecked`, `ProcessPipesDetectLog`, `ProcessPipesDefaultFunc`.
- `pipe_table.go` / `pipe_text.go` - table and text renderers must skip the `"pipe"` key when iterating map keys for column/row rendering.
- `rib/rib.go` - register `first`/`last` pipe filters.
- `rib/rib_pipeline.go` - `parsePipelineArgs` recognizes `first`/`last`; new `firstFilter`/`lastFilter` pipeline stage types. `showPipeline` injects `"pipe"` metadata from folded server args into JSON output.

### Architectural Verification
- [ ] No bypassed layers (flows through ParsePipe -> FoldFilters -> handler -> ApplyPipes)
- [ ] No unintended coupling (generic pipe code knows `first`/`last` as pipe kinds, not as BGP concepts)
- [ ] No duplicated functionality (reuses the FoldFilters promotion pattern from `count`)
- [ ] Zero-copy preserved where applicable (server-side stops iteration; client-side parses JSON once)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp rib \| received \| first 5` via CLI | -> | `FoldFilters` + `parsePipelineArgs` + `firstFilter` stage | `test-pipe-first-last.ci` -- output capped at 5 routes |
| Any command `\| first 3` without server registration | -> | `applyFirst` client-side in `ApplyPipes` | `TestApplyFirst` unit test |
| Any command `\| last 3` without server registration | -> | `applyLast` client-side in `ApplyPipes` | `TestApplyLast` unit test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp rib \| received \| first 5` with 100 routes | Output contains exactly 5 routes; generator stops at 5 (server-side) |
| AC-2 | `show bgp rib \| received \| last 5` with 100 routes | Output contains the last 5 routes in iteration order |
| AC-3 | `show bgp rib \| received \| first 5 \| count` | Returns `{"count": 5}` (count sees only the first 5) |
| AC-4 | `<any-command> \| first 3` where the command does NOT register first as a pipe filter | Client-side `applyFirst` takes first 3 items from the JSON output |
| AC-5 | `<any-command> \| last 3` where the command does NOT register last as a pipe filter | Client-side `applyLast` takes last 3 items from the JSON output |
| AC-6 | `show bgp rib \| first 5 \| table` | first applied before table formatting; table shows 5 rows |
| AC-7 | `\| first` without a number | Error: "first requires a numeric argument" |
| AC-8 | `\| first -1` or `\| first 0` | Error: "first requires a positive number" |
| AC-9 | `\| first 5` with fewer than 5 items | All items returned; `"pipe": {"first": 5}` still present (records what was requested, not what happened) |
| AC-10 | Pipe completion after `\|` | `first` and `last` appear in suggestions |
| AC-11 | `show bgp rib \| received \| family ipv4-unicast \| first 100` | JSON includes `"pipe": {"received": true, "family": "ipv4-unicast", "first": 100}` |
| AC-12 | `show bgp rib \| received \| count` | JSON includes `"pipe": {"received": true, "count": true}` alongside `"count": N` |
| AC-13 | `show bgp rib` with no pipe modifiers | No `"pipe"` key in JSON output (backward compatible) |
| AC-14 | `show bgp rib \| received \| first 5 \| table` | `"pipe"` metadata present in the JSON before `\| table` renders it; table renderer skips the `"pipe"` key (does not render it as a data row) |
| AC-15 | `show bgp rib \| first 5 \| family ipv4-unicast` | Position matters: from first 5 routes, keep only ipv4-unicast (may yield fewer than 5) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyFirst` | `internal/component/command/pipe_test.go` | AC-4 client-side first on JSON array, map, single-key wrapper | |
| `TestApplyLast` | `internal/component/command/pipe_test.go` | AC-5 client-side last | |
| `TestApplyFirstUnderCount` | `internal/component/command/pipe_test.go` | AC-9 fewer items than N | |
| `TestParsePipeFirst` | `internal/component/command/pipe_test.go` | AC-7 first parsed with arg | |
| `TestParsePipeLast` | `internal/component/command/pipe_test.go` | last parsed with arg | |
| `TestFoldFiltersFirst` | `internal/component/command/pipe_test.go` | first promoted to server arg when registered | |
| `TestFoldFiltersFirstNotRegistered` | `internal/component/command/pipe_test.go` | first stays client-side when not registered | |
| `TestFirstValidation` | `internal/component/command/pipe_test.go` | AC-7/AC-8 missing/invalid arg rejected | |
| `TestShowRibFirstPipeline` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-1 server-side first stops iterator | |
| `TestShowRibLastPipeline` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-2 server-side last buffers trailing window | |
| `TestShowRibFirstThenCount` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-3 first composes with count | |
| `TestApplyFirstNonArray` | `internal/component/command/pipe_test.go` | Non-array, non-single-key-wrapper input passes through unchanged | |
| `TestPipeMetadataInjected` | `internal/component/command/pipe_test.go` | AC-11/AC-12 pipe dict present with correct types | |
| `TestPipeMetadataAbsentWhenNoPipes` | `internal/component/command/pipe_test.go` | AC-13 no pipe key without modifiers | |
| `TestPipeMetadataTableSkipped` | `internal/component/command/pipe_test.go` | AC-14 table renderer does not render "pipe" key as a data row | |
| `TestPipeMetadataServerSide` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-11 server-folded filters appear in pipe dict | |
| `TestApplyFirstPositionMatters` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-15 first then family yields subset | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N (first/last arg) | >= 1 | 1 | 0 (reject) | none (large values allowed) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-pipe-first-last` | `test/plugin/*.ci` | Inject routes, `show bgp rib \| received \| first 5`, confirm exactly 5 routes; `\| last 3`, confirm last 3 | |

### Interop Tests (MANDATORY for protocol features)
<!-- N/A: pipe operators are CLI/observability, not wire-protocol behavior. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A -- CLI pipe, not wire protocol | - | - | - | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/command/pipe.go` - add `pipeFirst`/`pipeLast` to `pipeKind` enum + `knownPipeOps`; add `ParsePipe` switch cases for arg extraction; add `FoldFilters` cases + change return type to `(string, []pipeOp, map[string]any)` to carry pipe metadata; add `applyFirst`/`applyLast` in `ApplyPipes`; `ApplyPipes` injects `"pipe"` dict into JSON output; add validation in `ValidatePipes`. Update all 6 `ProcessPipes*` callers to accept and forward the metadata map.
- `internal/component/command/completer.go` - add `first`/`last` entries to `PipeOperators` slice.
- `internal/component/command/pipe_table.go` - table renderer skips `"pipe"` key when rendering map keys as columns/rows.
- `internal/component/command/pipe_text.go` - text renderer skips `"pipe"` key (same as table).
- `internal/component/bgp/plugins/cmd/rib/rib.go` - register `first`/`last` as `PipeFilter` entries.
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - `parsePipelineArgs` recognizes `first`/`last`; add `firstFilter`/`lastFilter` pipeline stage types. `showPipeline` injects pipe metadata from folded server args into JSON output.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | - |
| CLI commands/flags | [ ] No | pipe operators, not commands |
| CLI grammar (action before identifier) | [ ] N/A | pipe syntax, not command grammar |
| Editor autocomplete | [ ] Yes | `first`/`last` in `PipeOperators` slice (completer.go) |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/test-pipe-first-last.ci` |
| Pipe completeness | [ ] Yes | `first`/`last` ARE pipe operators; they must compose with all other pipes |
| Doctor check for runtime dependencies | [ ] No | - |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` -- pipe operator list |
| 3 | CLI command added/changed? | [ ] Yes | `docs/guide/command-reference.md` -- pipe operator table |
| 4 | API/RPC added/changed? | [ ] No | |
| 12 | Internal architecture changed? | [ ] No | pipe framework extended, not redesigned |
| 16 | Changed source referenced by doc anchors? | [ ] Check | grep `docs/` for `pipe.go` anchors |

## Files to Create
- `test/plugin/test-pipe-first-last.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm pipe enum + FoldFilters + ApplyPipes baseline |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- add `pipeFirst`/`pipeLast` enums and `knownPipeOps` entries; add `ParsePipe` switch cases to extract the numeric arg; `TestParsePipeFirst`/`TestParsePipeLast` pass. `ApplyPipes` has stub cases. `TestApplyFirst` fails (stub returns input unchanged). Add entries to `PipeOperators` in `completer.go`.
   - Tests: `TestParsePipeFirst`, `TestParsePipeLast`
   - Files: `pipe.go`, `completer.go`
   - Verify: parse works; apply stubs fail; completion includes first/last.
2. **Phase: Client-side apply** -- implement `applyFirst`/`applyLast` in `ApplyPipes`. For arrays: take first/last N elements. For single-key wrapper maps: unwrap, apply to inner array, re-wrap. For anything else (multi-key maps, scalars): pass through unchanged.
   - Tests: `TestApplyFirst`, `TestApplyLast`, `TestApplyFirstUnderCount`, `TestApplyFirstNonArray`
   - Verify: client-side truncation works on JSON arrays, wrapper maps, and passes through non-array input.
3. **Phase: Validation** -- `ValidatePipes` rejects missing/invalid args for `first`/`last`.
   - Tests: `TestFirstValidation`
   - Verify: `| first` without arg, `| first 0`, `| first -1` all rejected.
4. **Phase: FoldFilters promotion** -- add `pipeFirst`/`pipeLast` cases in `FoldFilters` (same pattern as `pipeCount`, but passing `op.arg` for the N value).
   - Tests: `TestFoldFiltersFirst`, `TestFoldFiltersFirstNotRegistered`
   - Verify: promoted when registered, stays client-side when not.
5. **Phase: Pipe metadata** -- `FoldFilters` return type changes to `(string, []pipeOp, map[string]any)`, collecting all data-shaping ops (both server-folded and client-remaining) into a metadata map. Update all 6 `ProcessPipes*` callers to accept and forward the map. `ApplyPipes` injects `"pipe"` dict into JSON output (parse, add key, re-serialize). Table/text renderers skip the `"pipe"` key. No `"pipe"` key when no modifiers are active.
   - Tests: `TestPipeMetadataInjected`, `TestPipeMetadataAbsentWhenNoPipes`, `TestPipeMetadataTableSkipped`
   - Verify: metadata dict has correct types; absent when no pipes; table/text don't render it as data.
6. **Phase: RIB server-side** -- register `first`/`last` as pipe filters in `rib.go`; add `firstFilter`/`lastFilter` pipeline stages in `rib_pipeline.go`; `parsePipelineArgs` recognizes them. `first` stops the upstream iterator after N items. `last` drains the upstream, keeping a sliding window of N items. `showPipeline` injects pipe metadata from folded args into JSON output.
   - Tests: `TestShowRibFirstPipeline`, `TestShowRibLastPipeline`, `TestShowRibFirstThenCount`, `TestPipeMetadataServerSide`, `TestApplyFirstPositionMatters`
   - Verify: server-side early termination works; composes with other stages; pipe metadata present; position ordering respected.
7. **Functional** -- `test-pipe-first-last.ci`.
8. **Full verification** -- `make ze-verify`.
9. **Complete spec** -- audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has code + test at file:line |
| Correctness | `first` stops iteration server-side; `last` keeps trailing window; client-side fallback produces correct subset |
| Naming | `pipeFirst`/`pipeLast` enum names; `first`/`last` in `knownPipeOps` |
| Data flow | `ParsePipe` -> `FoldFilters` (promotion or passthrough + metadata map) -> handler or `ApplyPipes` |
| Composition | `first` composes with `count`, `table`, `match`, `json`, etc. |
| Position ordering | `\| first 5 \| family ipv4` differs from `\| family ipv4 \| first 5`; both are valid |
| Non-array fallback | `applyFirst`/`applyLast` pass through non-array, non-single-key-wrapper JSON unchanged |
| Pipe metadata | `"pipe"` dict present with correct typed values; absent when no modifiers; display pipes excluded |
| Pipe metadata renderers | Table/text renderers skip `"pipe"` key (AC-14) |
| Backward compat | Existing JSON output unchanged when no pipes are used (AC-13) |
| Rule: pipe-completeness | `first`/`last` are themselves pipe operators; they must appear in pipe completion (`PipeOperators` in completer.go) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `pipeFirst`/`pipeLast` in pipe.go | `grep 'pipeFirst\|pipeLast' internal/component/command/pipe.go` |
| `first`/`last` in PipeOperators | `grep 'first\|last' internal/component/command/completer.go` |
| Client-side apply | `go test -run TestApplyFirst` |
| FoldFilters promotion | `go test -run TestFoldFiltersFirst` |
| Pipe metadata injected | `go test -run TestPipeMetadataInjected` |
| Pipe metadata absent when clean | `go test -run TestPipeMetadataAbsentWhenNoPipes` |
| Table skips pipe key | `go test -run TestPipeMetadataTableSkipped` |
| RIB pipeline stages | `go test -run TestShowRibFirstPipeline` |
| Functional test | `test-pipe-first-last.ci` passes |

### Security Review Checklist (/implement stage 9)
| Check | What to look for |
|-------|-----------------|
| Input validation | N must be a positive integer; reject 0, negative, non-numeric, overflow |
| Resource exhaustion | `last N` with very large N buffers N items in memory; acceptable since it's user-requested and bounded |

### Failure Routing
| Failure | Route To |
|---------|----------|
| JSON contract drifts | Re-read `applyCount` pattern; match it |
| FoldFilters doesn't promote | Check `pipeKind` enum ordering; `first`/`last` must have cases before the `pipeUnknown` default |
| Pipeline stage ordering wrong | Re-read `parsePipelineArgs`; `first`/`last` are filter-class stages (user controls position relative to other filters); must precede terminals |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `first`/`last` had already been implemented | They were discussed but never coded; `IterateSorted` (bart ordered iteration) was done but `first`/`last` pipe operators were not | grep of entire codebase found no `first`/`last` pipe entries | Spec created to fill the gap |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The `count` dual-path (generic pipe + command-specific filter with FoldFilters promotion) is the reusable pattern for any pipe operator that benefits from server-side optimization. `first`/`last` follow it exactly.
- `first` as a pipeline filter stage (not a terminal) composes naturally: `| first 100 | count` returns 100, `| first 100 | table` shows 100 rows. Position in the pipeline is user-controlled and meaningful.
- `first` and `last` have an asymmetry: `first` can stop the source iterator early (real performance win), while `last` must drain the full iterator to find the trailing window (saves only serialization cost). Both are still valuable as server-side stages.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Generic pipe operators with server-side promotion | RIB-only pipeline stages (no generic fallback) | Every `show` command should support `\| first N` even without explicit registration; client-side fallback ensures universality |
| `first`/`last` are filter stages, not terminals | Terminal-level truncation | Filter stage composes with any terminal (`count`, `table`, `json`, `prefix-summary`); terminal-level truncation would require each terminal to implement limit logic |
| Positive integer only (no 0, no negative) | 0 = unbounded | 0 is confusing ("first 0 items" = none?); omitting `first` entirely means unbounded; no need for a sentinel |
| User controls ordering (position matters) | Enforce first/last after content filters | Simpler, consistent with how all other filters work; `\| first 5 \| family ipv4` is valid (if unusual) |
| Non-array input passes through unchanged | Error on non-array | Server-side promotion avoids this case for RIB; client-side fallback on multi-key maps has no sensible semantics, so pass-through is safest |
| Pipe metadata as a dict with typed values | List of strings (`["received", "first 5"]`) | Dict allows structured access (`pipe.first` is an integer, `pipe.family` is a string); a flat string list requires parsing and loses type info |
| Display-only pipes excluded from metadata | Include all pipes | Display pipes (`table`, `json`, `yaml`, etc.) don't change what data is present, only how it's rendered; including them adds noise without consumer value |
| `"pipe"` key absent when no modifiers active | Always present (empty dict) | Backward compatible; existing consumers that don't know about `"pipe"` see no change |
| Table/text renderers skip `"pipe"` key | Render it as a data row | The `"pipe"` key is infrastructure metadata, not user data; rendering it as a table column confuses the output |
| `FoldFilters` return type change (3-tuple) | Side channel or global state | Explicit return value is cleanest; the 6 callers are all in `pipe.go` so the blast radius is contained to one file |

## Known Limitations
- Client-side `applyFirst`/`applyLast` must parse the full JSON string before truncating. For very large outputs from commands that don't register server-side support, the memory peak is unchanged. This is acceptable: the optimization path (server-side registration) exists for commands that need it.
- Client-side `applyFirst`/`applyLast` only work on JSON arrays (or single-key wrapper maps containing arrays). Multi-key maps and scalar values pass through unchanged. This is by design: server-side promotion handles the complex cases.
- `last` server-side requires buffering N items in memory (sliding window) and draining the full upstream iterator. Unlike `first` (which stops the source early), `last` saves only serialization cost, not iteration cost. For `last 1000000` the buffer is large, but it's explicitly user-requested.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
