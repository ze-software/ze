# Spec: cli-column-order

| Field | Value |
|-------|-------|
| Status | done |
| Scope | cli |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every table Ze renders puts its columns in alphabetical order, because the
renderers sort the JSON keys and no command can say anything about them.

`show bgp summary` shows the cost. Its per-peer row carries 19 keys
(`handleBgpSummary` and `mergeRibRouteCounts`,
`internal/component/bgp/plugins/cmd/peer/summary.go`), so the operator reads:

```
address  connections-dropped  description  eor-received  eor-sent
keepalives-received  keepalives-sent  last-error  name  peer-type
remote-as  routes-accepted  routes-received  routes-sent  state
state-changed  updates-received  updates-sent  uptime
```

`connections-dropped` is the second column and `state` is the fifteenth. Every
other network OS leads with neighbor, AS, state, uptime, because that is the
order an operator reads them in. Ze cannot express that today at all.

The goal is a built-in column order per command that makes operator sense, used
by the two renderings a person reads. Alphabetical order stays the fallback for
every key the command did not name, and for every command that declares
nothing.

**Owner directive, 2026-08-19: `| table` and `| text` get the order; `| yaml`
and `| json` do not.** Both are consumed programmatically, so column order
carries no meaning for their reader, and `encoding/json` sorts map keys with no
override in any case.

This is ORDERING only, and the order is declared in code. Two adjacent features
are deliberately out of scope and get their own specs: an ad-hoc `| sort <field>`
that orders ROWS, and a saved per-command operator preference that overrides the
built-in order. See Known Limitations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - CLI pipe operators and the rib pipeline terminals
  → Decision: pipes are client-side filters over the dispatcher's JSON; command-owned filters fold into server-side arguments instead
  → Constraint: a command's answer is structured data, and the pipe layer renders it

### Rules
- [ ] `ai/rules/cli.md` - pipe completeness and the JSON format contract
  → Constraint: a command's response payload MUST be structured data, never text a renderer already formatted
  → Constraint: JSON keys are lowercase kebab-case and match the YANG leaf name, so a declared column name is the JSON key verbatim
  → Decision: every command that produces output supports all pipe operators, so the change must leave every operator working

**Key insights:**
- The command string is already in hand at every point where a formatter is built (`ProcessPipesChecked`, `ProcessPipesDetectLog`, `ProcessPipesDefaultFormatChecked`, `ProcessPipesDefaultFunc`, all in `internal/component/command/pipe.go`), so the declared order reaches the renderer by closure capture, not by new plumbing through callers.
- `pipe_filter.go` already solves "look up a per-command declaration by longest matching command prefix", including the trap that a parent's declaration is inherited by children unless the child registers an empty one.
- The order is a rendering property and never enters the payload, so no plugin, RPC, or JSON consumer changes.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe_table.go` - renders JSON as a box-drawing table (`ApplyTable`) or space-aligned columns (`applyText`); both go through `applyTableStyled`. Column order is produced in three places: `renderList` sorts the union of row keys with `sort.Strings`; `renderRecord` orders through `tableSortedKeys`, which also sorts; `renderMapOfMaps` takes its child-key order from `homogeneousMapOfMapsKeys`. No caller can influence any of them.
- [ ] `internal/component/command/format.go` - `RenderYAML` walks the same payload; `writeMap` and `writeMapItem` each sort keys with `sort.Strings`. Read to establish that YAML is alphabetical for the same reason, and deliberately left that way by the owner directive above.
- [ ] `internal/component/command/pipe.go` - `ApplyPipes` dispatches each operator and calls `ApplyTable` / `applyText` / `applyYAML`; the four `ProcessPipes*` wrappers each hold the command string and close over the pipe chain to build the format function
- [ ] `internal/component/command/pipe_filter.go` - `RegisterPipeFilters` stores a per-command declaration; `lookupPipeFilters` resolves it by longest matching command prefix; registering an empty set blocks a parent's declaration from being inherited
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` builds the peer row and the outer `summary` record; `mergeRibRouteCounts` adds `routes-received`, `routes-accepted` and `routes-sent` when the rib plugin answers
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` is the existing example of a command declaring something about itself, including the deliberate empty registrations on the scalar rib commands

**Behavior to preserve:**
- `| json`, `| ndjson` and `| yaml` output is unchanged, byte for byte. All three are read by programs.
- A command that declares no column order renders exactly as it does today, alphabetically.
- A key the command did not name is never dropped and never hidden. It renders after the declared keys.
- `ApplyTable` stays exported and keeps its current single-argument form for its outside caller (`internal/component/iface/cli/scan.go`).
- The `pipe` metadata key stays excluded from column output, as `tableSortedKeys` and `renderList` already exclude it.

**Behavior to change:**
- `renderList`, `renderRecord` and `renderMapOfMaps` order declared keys first, in declared order, before the alphabetical remainder.
- `show bgp summary` and `show bgp peer list` declare an operator-first column order.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types a command with an optional pipe chain into the SSH CLI, the web CLI, or `ze` on the command line. The text reaches `ParsePipe` (`internal/component/command/pipe.go`) as one string.

### Transformation Path
1. `ParsePipe` splits the input into the command string and the operator chain.
2. `foldFilters` rewrites command-owned pipe filters into server-side arguments and returns the remaining client-side operators plus the pipe metadata.
3. **New:** the same wrapper looks the command up in the column registry and captures the declared order alongside the metadata.
4. The dispatcher runs the command and marshals its `plugin.ResponseData` into a JSON string (`ResponseJSON`, `internal/component/plugin/dispatch.go`).
5. `ApplyPipes` runs the operator chain over that string. Only the `| table` and `| text` operators reach the ordering path; `| json`, `| ndjson` and `| yaml` are untouched.
6. **New:** the table renderer orders keys with the declared order first and the alphabetical remainder after.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ CLI | The plugin answers structured data; the declared order lives on the CLI side and never travels in the payload | Yes -- `handleBgpSummary` is unchanged, and `TestApplyJSONIgnoresColumnOrder` pins the payload |
| Command registration ↔ renderer | A registry keyed by command path, read when the formatter is built | Yes -- `ColumnsForCommand` is called in the four `ProcessPipes*` wrappers and captured by the returned closure |

### Integration Points
- `internal/component/command/pipe_filter.go` - the longest-prefix command lookup this registry needs already exists here for pipe filters, and is factored out rather than copied
- `internal/component/command/pipe_table.go` - `tableStyle` is the receiver every table renderer already carries, so the declared order rides on it without new parameters
- `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` is the placement precedent: a command declares its own surface in its own package

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The order is read once, in the four `ProcessPipes*` wrappers, from the command string they already hold, and travels to the renderer on `tableStyle`. No caller outside `internal/component/command` changed |
| No unintended coupling (components stay isolated) | Yes | The plugin answers structured data and the declaration lives on the CLI side. `internal/component/bgp/plugins/cmd/peer` gains one import of `internal/component/command`, the same edge `internal/component/bgp/plugins/cmd/rib` already has for `RegisterPipeFilters` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `commandRegistry[T]` in `column_order.go` is the one longest-prefix lookup, and `pipe_filter.go` now calls it. `grep -c 'func commandMatchesPrefix' internal/component/command/*.go` reports a single definition |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `orderKeys` builds one rank map per rendered record and partitions the key slice. No payload is copied and no encoding path is touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | A command declares its order from its own package through `RegisterColumns`. `internal/component/command` names no command and no key |

## The built-in orders

Both declarations cover the FULL key set of their payload, so no half-ordered
table results (see R-3).

`show bgp summary`, per-peer row. Identity first, then session state, then the
counters an operator reaches for only when something is wrong:

| # | Key | Why here |
|---|-----|----------|
| 1-5 | `address`, `name`, `description`, `remote-as`, `peer-type` | Which peer this row is about |
| 6-9 | `state`, `uptime`, `state-changed`, `last-error` | Is it up, for how long, and why did it last go down |
| 10-12 | `routes-received`, `routes-accepted`, `routes-sent` | What the session is carrying |
| 13-18 | `updates-received`, `updates-sent`, `keepalives-received`, `keepalives-sent`, `eor-received`, `eor-sent` | Protocol counters |
| 19 | `connections-dropped` | Historical fault counter |

`show bgp summary`, outer record: `router-id`, `local-as`, `uptime`,
`peers-configured`, `peers-established`, `family`, `peers-in-family`, `peers`.

`show bgp peer list` declares the identity-then-state prefix of the same order,
matching the fields that command returns.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The declared order can reach the renderer by closure capture in the four `ProcessPipes*` wrappers, with no signature change reaching callers outside `internal/component/command` | Every wrapper in `pipe.go` already receives the command string and returns a closure | Threading reaches every CLI call site and the change stops being contained | Compile the tree after the change; grep for callers of the four wrappers | confirmed -- `ApplyPipes` gained a fourth parameter and has no caller outside `internal/component/command`, so only in-package tests were adapted. Each wrapper calls `ColumnsForCommand` after `ParsePipe` and closes over the result |
| A-2 | A declared column name is the JSON key verbatim | `ai/rules/cli.md` fixes JSON keys as kebab-case matching the YANG leaf | A declaration silently matches nothing and the order is alphabetical anyway | A unit test asserting an unknown declared name is inert, and `TestBgpSummaryColumnsMatchPayload` | confirmed -- `TestDeclaredColumnUnknownKeyIsInert` and `TestBgpSummaryColumnsMatchPayload` pass, and the second fails when one declared name is misspelled |
| A-3 | Nothing outside `internal/component/command` depends on the alphabetical order of table columns | `ApplyTable` has one outside caller, `internal/component/iface/cli/scan.go` | Golden files and `.ci` expectations break in places this spec did not name | Run the functional suites; the golden tests in `internal/component/lg` cover the rendered path | confirmed -- the ui (185), plugin, editor and web functional suites are green, as are the unit tests of every package that consumes the pipe pipeline. `ApplyTable` keeps its single-argument form, so its outside caller renders alphabetically as before |
| A-4 | The full key set of `show bgp summary` is the 19 keys listed above | `handleBgpSummary` builds 16 and `mergeRibRouteCounts` adds 3 | The declaration is partial and the table is half-ordered | `TestBgpSummaryColumnsMatchPayload` compares the declaration against the real row | confirmed -- the test compares both ways: no payload key is undeclared, and no declared name is absent from the payload |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The registry inherits the longest-prefix trap: a declaration on `show bgp` silently orders `show bgp peer statistics` too | A command with unrelated keys renders with a partial declared order | Same answer as `registerPipeFilters` uses: register an empty declaration on a child to block inheritance, and cover it with a unit test |
| R-2 | Declared order and payload drift apart when a handler renames a key | A column silently returns to the alphabetical tail | A unit test per declaring command asserting every declared name exists in that command's real payload |
| R-3 | Ordering only the keys a command names leaves a half-ordered table that reads worse than pure alphabet | A declaration covering 4 of 19 keys leaves 15 alphabetical after them | Both declarations in this spec cover their full key set; a partial declaration is the exception, not the norm |
| R-4 | `mergeRibRouteCounts` only adds its three keys when the rib plugin answers, so the same command has two key sets | A declaration naming `routes-sent` looks partial when the plugin is absent | AC-6 makes an unmatched declared name inert, which is exactly this case; the test covers the plugin-absent shape |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Rendered CLI output only. No protocol behavior, no config, no wire format, and no programmatic output since JSON and YAML are excluded. The worst case is a table whose columns are in a confusing order, or a `.ci` expectation matching the old alphabetical header |
| How is it reverted? | Single commit revert. The declared orders are data; removing them restores alphabetical rendering |
| Who else touches this path? | `plan/spec-fixit-cli-format-default-everywhere.md` works on the CLI default format and reads the same renderers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator runs `show bgp summary \| text` in the CLI | → | `renderList` orders keys from the registry before the alphabetical remainder | `test/ui/show-bgp-summary-column-order.ci` |
| A command registers a column order at startup | → | `RegisterColumns` stores it, `ColumnsForCommand` resolves it by longest command prefix | `TestColumnsForCommandLongestPrefix` |
| Operator runs `show bgp summary \| yaml` | → | `applyYAML` never consults the registry | `TestRenderYAMLIgnoresColumnOrder` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A command declares the order `address, remote-as, state, uptime` and its payload rows carry exactly those keys | `\| table` and `\| text` render the columns in that order, not alphabetically |
| AC-2 | A command declares `state, address` and its rows also carry `description` and `uptime` | `state` and `address` render first in that order, then `description` and `uptime` alphabetically. No key is dropped |
| AC-3 | A command declares nothing | Output is byte-identical to today's alphabetical rendering |
| AC-4 | The same payload is rendered with `\| yaml` | Output is byte-identical to today. YAML is read by programs and takes no column order |
| AC-5 | The same payload is rendered with `\| json` or `\| ndjson` | Output is byte-identical to today. The declaration does not reach JSON |
| AC-6 | A declaration names a key the payload does not carry | The name is inert. No empty column, no placeholder, no error |
| AC-7 | A parent command declares an order and a child command registers an empty declaration | The child renders alphabetically, not with the parent's order |
| AC-8 | An operator runs `show bgp summary` | The peer columns follow the built-in order above: `address` first, `state` sixth, `connections-dropped` last |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show bgp summary` over SSH and reads peer state | CLI input → `ProcessPipesDefaultFormatChecked` → dispatcher → `ApplyPipes` → `applyText` → `renderList` with declared order | `test/ui/show-bgp-summary-column-order.ci` |
| 2 | Runs `show bgp peer list \| table` and reads the same fields in the same positions | same path, `ApplyTable` | `test/ui/show-bgp-peer-list-column-order.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterColumnsRoundTrip` | `internal/component/command/column_order_test.go` | A registered order is returned for the exact command path | pass |
| `TestColumnsForCommandLongestPrefix` | `internal/component/command/column_order_test.go` | The longest matching command prefix wins (AC-7 inheritance) | pass |
| `TestColumnsForCommandEmptyBlocksInheritance` | `internal/component/command/column_order_test.go` | An empty declaration on a child blocks the parent's (AC-7) | pass |
| `TestRenderListDeclaredOrder` | `internal/component/command/pipe_table_test.go` | Declared keys first in declared order (AC-1) | pass |
| `TestRenderListUndeclaredKeysFollowAlphabetically` | `internal/component/command/pipe_table_test.go` | Remainder keeps alphabetical order, nothing dropped (AC-2) | pass |
| `TestRenderListNoDeclarationUnchanged` | `internal/component/command/pipe_table_test.go` | Byte-identical to the pre-change rendering (AC-3) | pass |
| `TestRenderRecordDeclaredOrder` | `internal/component/command/pipe_table_test.go` | The key-value record form honors the order too | pass |
| `TestRenderYAMLIgnoresColumnOrder` | `internal/component/command/format_test.go` | `\| yaml` is byte-identical to today (AC-4) | pass |
| `TestApplyJSONIgnoresColumnOrder` | `internal/component/command/pipe_test.go` | `\| json` and `\| ndjson` are unchanged (AC-5) | pass |
| `TestDeclaredColumnUnknownKeyIsInert` | `internal/component/command/pipe_table_test.go` | A name absent from the payload produces no column (AC-6, R-4) | pass |
| `TestBgpSummaryColumnsMatchPayload` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | Every declared name exists in the row `handleBgpSummary` and `mergeRibRouteCounts` build (R-2, A-4) | pass |
| `TestRegisterColumnsNormalizes` | `internal/component/command/column_order_test.go` | Case and whitespace in a declaration are normalized, so a name cannot be inert by typing (A-2) | pass |
| `TestRegisterColumnsMultipleShapes` | `internal/component/command/column_order_test.go` | A command declares one order per record shape | pass |
| `TestRenderMapOfMapsDeclaredOrder` | `internal/component/command/pipe_table_test.go` | The map-of-maps rendering orders its child columns, which is the shape `show bgp peer list` answers | pass |
| `TestRenderPicksOrderMatchingTheRecordShape` | `internal/component/command/pipe_table_test.go` | Each record takes the declaration that names it, which is what resolves the two `uptime` positions of `show bgp summary` | pass |
| `TestBgpPeerListColumnsMatchPayload` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | Every name `show bgp peer list` declares is a key its handler builds (R-2) | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Declared column count | 0 to the payload key count | equal to the key count | N/A (0 means no declaration, a valid state) | more names than keys, which AC-6 covers as inert |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-summary-column-order` | `test/ui/show-bgp-summary-column-order.ci` | An operator runs `show bgp summary \| text` and reads state near the front, not in the middle | pass |
| `show-bgp-peer-list-column-order` | `test/ui/show-bgp-peer-list-column-order.ci` | An operator runs `show bgp peer list \| table` and gets the same field order | pass |
| `show-column-order-absent-unchanged` | `test/ui/show-column-order-absent-unchanged.ci` | A command with no declaration renders alphabetically, proving the change is opt-in | pass |

### Interop Tests (Scope: protocol)
Not applicable. Scope is `cli`; no wire-visible behavior changes.

## Files to Modify
- `internal/component/command/pipe_table.go` - `tableStyle` carries the declared order; `renderList`, `renderRecord` and `renderMapOfMaps` consult it before falling back to `sort.Strings`
- `internal/component/command/pipe.go` - `ApplyPipes` and the four `ProcessPipes*` wrappers carry the declared order from the registry to the table renderer only
- `internal/component/command/pipe_filter.go` - the longest-prefix command lookup is factored out for reuse by the new registry, with no change to `RegisterPipeFilters` behavior
- `internal/component/bgp/plugins/cmd/peer/peer.go` - registers the column order for `show bgp summary` and `show bgp peer list` beside the existing registrations
- `docs/features/formatting.md` - documents the built-in per-command order, that it applies to `table` and `text` only, and that the fallback is alphabetical
- `docs/architecture/api/commands.md` - records the registry beside the pipe-filter registry it mirrors

## Files to Create
- `internal/component/command/column_order.go` - `RegisterColumns`, `ColumnsForCommand`, and the shared longest-prefix lookup
- `internal/component/command/column_order_test.go` - registry unit tests
- `test/ui/show-bgp-summary-column-order.ci` - functional coverage for the motivating command
- `test/ui/show-bgp-peer-list-column-order.ci` - functional coverage for a second declaring command
- `test/ui/show-column-order-absent-unchanged.ci` - functional coverage that the change is opt-in

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The built-in order is a CLI rendering property, not command structure or semantics. `ai/rules/cli.md` keeps presentation out of YANG. The operator-saved override is a separate spec and owns its own config surface |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | No | No command, flag or grammar token is added. Existing commands gain a declaration |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | No | Column order changes rendering, not completion |
| Functional test for new RPC/API | No | No new RPC. Functional coverage is the three `.ci` tests above |
| Pipe completeness | Yes | The change lives inside `ApplyPipes` and its table renderers, so every pipe path keeps working; AC-4 and AC-5 pin `\| yaml`, `\| json` and `\| ndjson` as unchanged |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/formatting.md` |
| 2 | Config syntax changed? | No | No config surface changes in this spec |
| 3 | CLI command added/changed? | No | Output ordering changes; no command is added or renamed |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | No | No plugin boundary changes |
| 6 | Has a user guide page? | Yes | `docs/features/formatting.md` is that page for pipes and rendering |
| 7 | Wire format changed? | N-A | Scope is cli |
| 8 | Plugin SDK/protocol changed? | No | The declaration lives on the CLI side and never enters the payload |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is cli |
| 10 | Test infrastructure changed? | No | New `.ci` tests use the existing runner |
| 11 | Affects daemon comparison? | No | No feature-parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` gains the registry beside the pipe-filter registry |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing enters an inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `pipe_table.go` and `pipe.go` and correct any claim about alphabetical ordering |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/formatting.md` shows rendered tables; re-check its examples against the new order |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the registry exists and the renderer reads it
   - Tests: `TestRegisterColumnsRoundTrip`, `TestColumnsForCommandLongestPrefix`, `TestColumnsForCommandEmptyBlocksInheritance`
   - Files: `internal/component/command/column_order.go`, `internal/component/command/column_order_test.go`, and the lookup factored out of `internal/component/command/pipe_filter.go`
   - Verify: the registry resolves a declaration by command path, and `renderList` can reach one. The wiring test fails because no renderer consults it yet
2. **Phase: Table and text rendering** -- declared order first, alphabetical remainder after
   - Tests: `TestRenderListDeclaredOrder`, `TestRenderListUndeclaredKeysFollowAlphabetically`, `TestRenderListNoDeclarationUnchanged`, `TestRenderRecordDeclaredOrder`, `TestDeclaredColumnUnknownKeyIsInert`
   - Files: `internal/component/command/pipe_table.go`, `internal/component/command/pipe.go`
   - Verify: AC-1, AC-2, AC-3 and AC-6 pass
3. **Phase: Prove the programmatic formats are untouched** -- the exclusion is a property, not an omission
   - Tests: `TestRenderYAMLIgnoresColumnOrder`, `TestApplyJSONIgnoresColumnOrder`
   - Files: no production change expected; a change here means the ordering leaked out of the table path
   - Verify: AC-4 and AC-5 pass
4. **Phase: Declare the orders** -- the motivating commands stop rendering alphabetically
   - Tests: `TestBgpSummaryColumnsMatchPayload`, the three `.ci` tests
   - Files: `internal/component/bgp/plugins/cmd/peer/peer.go`, `test/ui/*.ci`, `docs/features/formatting.md`, `docs/architecture/api/commands.md`
   - Verify: AC-7 and AC-8 pass, and `show bgp summary` reads the way an operator expects

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol, and every declaring command has a payload-agreement test |
| Feature completeness | Both human-facing renderings honor the order; neither programmatic format changed by a byte |
| Correctness | A key absent from the declaration is never dropped; a declared key absent from the payload never creates a column |
| Naming | Declared names are the JSON keys verbatim, kebab-case, matching `ai/rules/cli.md` |
| Data flow | The declaration never enters the payload. Nothing a plugin returns carries rendering information |
| Rule: `ai/rules/simplicity.md` | The longest-prefix lookup is shared with `pipe_filter.go`, not copied. No second lookup implementation lands |
| Rule: `ai/rules/cli.md` | `\| json`, `\| ndjson` and `\| yaml` are byte-identical to today |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Column registry exists and is opt-in | `grep -n 'func RegisterColumns\|func ColumnsForCommand' internal/component/command/column_order.go` |
| One lookup implementation, not two | `grep -c 'func commandMatchesPrefix' internal/component/command/*.go` returns a single definition |
| `show bgp summary` declares the full order | `TestBgpSummaryColumnsMatchPayload` passes |
| Programmatic output unchanged | `TestApplyJSONIgnoresColumnOrder` and `TestRenderYAMLIgnoresColumnOrder` pass |
| Functional coverage exists | `ls test/ui/show-bgp-summary-column-order.ci test/ui/show-bgp-peer-list-column-order.ci test/ui/show-column-order-absent-unchanged.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The declaration is compile-time data from a registration call, never operator input, so no untrusted string reaches the ordering path. Confirm no CLI token can add or reorder a column in this spec's scope |
| Resource exhaustion | Ordering is over the key set of one payload; confirm the implementation does not go quadratic on a wide row (use a map from name to position, not a linear scan per key) |
| Information leakage | Ordering must not gain the ability to hide a key, since a hidden field reads as an absent field. AC-2 pins this |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A golden test breaks on the new order | Expected for declaring commands. Confirm the new order is right, then update the golden. A golden break on a NON-declaring command is a defect in the fallback |
| `\| yaml` or `\| json` output changes | The ordering leaked out of the table path. Fix the leak; do not update the expectation |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The command string is available at every formatter construction point, so the feature needs no new plumbing. That is what makes it small; a design that put the order in the payload would have needed none of this and would have paid for it on every response instead.
- `pipe_filter.go` already answers "per-command declaration, resolved by longest command prefix, empty blocks inheritance". The second user of that answer is what justifies factoring it out.
- Splitting the renderings by their reader rather than by their format is what keeps this small: a person reads `table` and `text`, a program reads `json`, `ndjson` and `yaml`, and only the first pair has a reason to care about order.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The declared order lives in a CLI-side registry | A `meta` block inside each command's JSON response | Rendering instructions in the payload make the renderer an interpreter and charge every consumer for presentation data they did not ask for. Column order is a property of the command, not of one response |
| Declared keys first, alphabetical remainder after | Render only declared keys | Dropping a key is column selection, a different feature with a different failure mode: a hidden field reads as an absent field |
| `\| yaml`, `\| json` and `\| ndjson` keep alphabetical keys | Order YAML too, for consistency with the tables | Owner directive, 2026-08-19: all three are consumed programmatically, so order carries no meaning for their reader. `encoding/json` also sorts map keys with no override, so ordering JSON would mean hand-rolled marshalling for no reader benefit |
| Reuse the longest-prefix lookup from `pipe_filter.go` | A second, independent registry | Two copies of a subtle rule (empty blocks inheritance) drift. `registerPipeFilters` in `rib.go` documents the trap; one implementation keeps one place to document it |
| Each declaration covers its payload's full key set | Declare only the leading few | A partial declaration leaves the tail alphabetical, which reads as an arbitrary break rather than an order |

## Known Limitations
- Column SELECTION is not in scope. Choosing which columns to show belongs to a separate operator (`| columns a,b,c`).
- ROW ordering is not in scope. An ad-hoc `| sort <field>` is a separate spec; Ze has no sort operator today (`knownPipeOps`, `internal/component/command/pipe.go`).
- An operator-saved per-command override of this built-in order is a separate spec. It must resolve daemon-side, because nothing on the `ze cli` client startup path loads config (established in `plan/spec-fixit-cli-format-default-everywhere.md`).
- Only `show bgp summary` and `show bgp peer list` declare an order here. Every other command keeps rendering alphabetically until it declares one; each declaration needs an operator judgment about what leads, so the rollout is incremental by design.
- Column headers keep the JSON key as their label. Renaming a header for display would reintroduce the payload-versus-registry question for labels.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `internal/component/command/column_order.go`: `ColumnOrder`, `RegisterColumns`, `ColumnsForCommand`, `ResetColumnsForTest`, and the generic `commandRegistry[T]` that resolves a command to the declaration on the longest registered command path.
- `internal/component/command/pipe_filter.go`: `pipeFilterRegistry` now IS a `commandRegistry[pipeFilterSet]`. `normalizeCommand` and `commandMatchesPrefix` moved to `column_order.go`, so one lookup implementation serves both registries.
- `internal/component/command/pipe_table.go`: `tableStyle` carries `orders`. `orderKeys` is the one entry point, and `declaredKeys`, `splitByOrder` and `bestColumnOrder` do the work. `renderList`, `renderRecord` and `renderMapOfMaps` each call `orderKeys` in place of a bare `sort.Strings`.
- `internal/component/command/pipe.go`: `ApplyPipes` takes the declared orders as a fourth parameter and hands them to the `pipeTable` and `pipeText` arms alone. The four `ProcessPipes*` wrappers each call `ColumnsForCommand` on the parsed command and close over the result.
- `internal/component/bgp/plugins/cmd/peer/peer.go`: `registerColumns` declares two orders for `show bgp summary` (the peer row and the record that holds it) and one for `show bgp peer list`.
- Docs: `docs/features/formatting.md` and `docs/architecture/api/commands.md`.

### Bugs Found/Fixed
- None. No defect was found in the reviewed code.

### Documentation Updates
- `docs/features/formatting.md`, "Column order": states the built-in order, that `\| table` and `\| text` take it, that `\| json`, `\| ndjson` and `\| yaml` keep alphabetical keys, and that ordering never hides a column. Anchors: `column_order.go -- RegisterColumns, ColumnsForCommand`, `pipe_table.go -- tableStyle.orderKeys, bestColumnOrder`, `peer.go -- registerColumns`.
- `docs/architecture/api/commands.md`, "Per-command declarations: pipe filters and column order": records the registry beside the pipe-filter registry that shares its lookup. Anchor: `column_order.go -- RegisterColumns, ColumnsForCommand, commandRegistry`.
- `make ze-doc-verify` was not run at closure. The owner is mid-rewrite of the test harness and the tree is deliberately red, so the run would report someone else's state. Both anchored claims were verified against source instead, symbol by symbol.

### Deviations from Plan
- The spec's Current Behavior describes `handleBgpSummary` building "the outer summary record". A later commit in the same series flattened that payload, so the aggregates and the peer rows are now siblings at the top level (`TestBgpSummaryPayloadIsFlat`). The declared record order names the flat keys, and `TestBgpSummaryColumnsMatchPayload` compares the declaration against the payload the handler actually builds.
- `commandRegistry[T]` gained an `each` method after this spec's commit, for the alias registry that landed next. The method is used by `alias.go` at three call sites.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A built-in column order per command, used by the two renderings a person reads | Done | `internal/component/command/pipe_table.go` `tableStyle.orderKeys`, reached from the `pipeTable` and `pipeText` arms of `ApplyPipes` | `\| json`, `\| ndjson` and `\| yaml` construct no `tableStyle` |
| Alphabetical order stays the fallback for undeclared keys and undeclaring commands | Done | `declaredKeys` returns `keys` unchanged when `bestColumnOrder` finds no order | Callers hand `orderKeys` an already-sorted key slice |
| `show bgp summary` reads operator-first | Done | `internal/component/bgp/plugins/cmd/peer/peer.go` `registerColumns` | `address` first, `state` sixth, `connections-dropped` last |
| Order declared in code, never in the payload | Done | `RegisterColumns` is called from `init()`; no handler returns an order | `handleBgpSummary` is unchanged |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRenderListDeclaredOrder`; `test/ui/show-bgp-summary-column-order.ci`; `test/ui/show-bgp-peer-list-column-order.ci` | Proven through the daemon over `\| text` and `\| table` |
| AC-2 | Done | `TestRenderListUndeclaredKeysFollowAlphabetically` | `splitByOrder` returns the unnamed keys in arrival order, which every caller has sorted |
| AC-3 | Done | `TestRenderListNoDeclarationUnchanged`; `test/ui/show-column-order-absent-unchanged.ci` | The `.ci` uses `show bgp peer <selector> detail`, a sibling of a declaring command |
| AC-4 | Done | `TestRenderYAMLIgnoresColumnOrder`; the `\| yaml` half of `show-bgp-peer-list-column-order.ci` | The `.ci` requires `group` before `name` in YAML and `name` before `group` in the table, from one run |
| AC-5 | Done | `TestApplyJSONIgnoresColumnOrder`; the `\| json` half of `show-bgp-summary-column-order.ci` | `ApplyJSON` takes no orders; `encoding/json` sorts map keys |
| AC-6 | Done | `TestDeclaredColumnUnknownKeyIsInert` | `splitByOrder` walks the keys, never the declaration, so a name absent from the payload creates nothing |
| AC-7 | Done | `TestColumnsForCommandEmptyBlocksInheritance` | `RegisterColumns(commands)` with no order stores an empty declaration, which `lookup` finds and `bestColumnOrder` reads as none |
| AC-8 | Done | `test/ui/show-bgp-summary-column-order.ci` | Asserts `header[0] == 'address'`, `header[5] == 'state'`, `header[-1] == 'connections-dropped'`, and `header != sorted(header)` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 16 unit tests of the plan | Done | `column_order_test.go`, `pipe_table_test.go`, `format_test.go`, `pipe_test.go`, `summary_test.go` | Every name in the plan resolves to a `func Test...` in the named file |
| `show-bgp-summary-column-order` | Done | `test/ui/show-bgp-summary-column-order.ci` | PASS 170 of 192 in the ui suite |
| `show-bgp-peer-list-column-order` | Done | `test/ui/show-bgp-peer-list-column-order.ci` | PASS 169 of 192 |
| `show-column-order-absent-unchanged` | Done | `test/ui/show-column-order-absent-unchanged.ci` | PASS 171 of 192 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/command/column_order.go` | Done | Created; also holds the shared lookup |
| `internal/component/command/column_order_test.go` | Done | Created |
| `internal/component/command/pipe_table.go` | Done | Modified |
| `internal/component/command/pipe.go` | Done | Modified |
| `internal/component/command/pipe_filter.go` | Done | Modified; its own behavior is unchanged |
| `internal/component/bgp/plugins/cmd/peer/peer.go` | Changed | The spec named this file, and `registerColumns` landed here rather than in `summary.go` |
| `test/ui/*.ci` (three) | Done | Created |
| `docs/features/formatting.md`, `docs/architecture/api/commands.md` | Done | Modified |

### Audit Summary
- **Total items:** 23
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator reading `show bgp summary` sees identity, then state, then counters | functional | `test/ui/show-bgp-summary-column-order.ci`, PASS 170 of 192 in `make ze-functional-ui-test` (192 of 192, 0 FAIL). It requires `address` first, `state` sixth, `connections-dropped` last, and `header != sorted(header)`, so a revert of the feature fails it |
| Alphabetical order stays the fallback, and the change is opt-in | functional | `test/ui/show-column-order-absent-unchanged.ci`, PASS 171 of 192. One run asserts an undeclaring command is alphabetical AND that `show bgp summary` is not, so an inert build fails it |
| The programmatic formats carry no column order | functional | `test/ui/show-bgp-peer-list-column-order.ci`, PASS 169 of 192. One run requires `group` before `name` under `\| yaml` and `name` before `group` under `\| table`. Leaking the order into YAML fails it; an inert order fails the table half |
| Nothing outside the CLI learns about rendering | unit | `TestBgpSummaryColumnsMatchPayload` reads the payload `handleBgpSummary` and `mergeRibRouteCounts` build and finds no order in it. `handleBgpSummary` is unchanged by this spec |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata declares no deferral shard, and `plan/deferrals/` holds no `cli-column-order.md`. `plan/deferrals/cli-order-pipe.md` belongs to the sibling spec and stays |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-column-order-2e38eb27-078b-4f5b-a456-56437e962d09.md`, over the 15 feature files |
| `review_gate.py check` | `review_gate: OK (0 code files, clean, hashes match)` |
| Rounds | 1 |
| Reviewer lenses used | wiring and functional coverage, logic and edge cases, security and allocation, simplicity and style, documentation drift |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE was found | - | - |

Three NOTEs were recorded and none blocks. First, `ColumnsForCommand` documents a
nil return for a command that declares none, and an explicit empty registration
returns an empty non-nil slice. Every consumer reads it with `len`, so the two
answers do not differ in behavior. Second, `ColumnsForCommand` returns the
registry's own slice, so a caller outside the package could edit a declaration.
The four in-package `ProcessPipes*` wrappers are the only callers. Third, AC-5's
JSON evidence is structural: `ApplyJSON` takes no orders and `encoding/json`
sorts map keys, and the `.ci` half asserts only that the answer stayed JSON. The
discriminating format assertion is the YAML one in
`show-bgp-peer-list-column-order.ci`.

Two automated pre-checks were run and both reds belong to the owner's
uncommitted harness rewrite. `make ze-repository-check` reports 22 unwired
exports in `internal/component/config/validators.go`,
`internal/component/plugin/process/manager.go` and
`internal/test/peer/checker.go`, all three modified in the working tree and none
touched by this spec. `scripts/dev/audit-test-relaxation.py 53e84d473~1` reports
11 weakened tests, all in `scripts/dev/changed_pkgs_test.go` and
`scripts/status/verify_run_test.go`, both uncommitted.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/command/column_order.go` | Yes | `ls -l` reports 5101 bytes |
| `internal/component/command/column_order_test.go` | Yes | `ls -l` reports 4612 bytes |
| `test/ui/show-bgp-summary-column-order.ci` | Yes | `ls -l` reports 7082 bytes |
| `test/ui/show-bgp-peer-list-column-order.ci` | Yes | `ls -l` reports 5618 bytes |
| `test/ui/show-column-order-absent-unchanged.ci` | Yes | `ls -l` reports 4948 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-8 | The declared order reaches `\| text` and `\| table` through the daemon | ui suite log: `PASS 170 show-bgp-summary-column-order`, `PASS 169 show-bgp-peer-list-column-order` |
| AC-2, AC-3, AC-6 | Nothing is dropped, an undeclaring command is unchanged, an unmatched name is inert | `make ze-unit-pkg-test PKG=./internal/component/command/...` exits 0 and holds `TestRenderListUndeclaredKeysFollowAlphabetically`, `TestRenderListNoDeclarationUnchanged`, `TestDeclaredColumnUnknownKeyIsInert` |
| AC-4, AC-5 | The programmatic formats are unchanged | `internal/component/command/pipe.go` `ApplyPipes`: only the `pipeTable` and `pipeText` arms build a `tableStyle`; `pipeYAML` calls `applyYAML(result)` and `pipeJSON` calls `ApplyJSON(result, op.arg)` |
| AC-7 | An empty declaration on a child blocks the parent's | `TestColumnsForCommandEmptyBlocksInheritance` in `column_order_test.go`; `RegisterColumns` stores an empty slice, which `commandRegistry.lookup` returns with `ok` true |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Operator runs `show bgp summary \| text` | `test/ui/show-bgp-summary-column-order.ci` | Yes. It runs that command against a started daemon and reads the rendered header |
| A command registers a column order at startup | `TestColumnsForCommandLongestPrefix` | Yes. `registerColumns` is called from `init()` in `peer.go`, and `ColumnsForCommand` is called in all four `ProcessPipes*` wrappers in `pipe.go` |
| Operator runs `show bgp summary \| yaml` | `TestRenderYAMLIgnoresColumnOrder`; `show-bgp-peer-list-column-order.ci` | Yes. The `.ci` runs `show bgp peer list \| yaml` and requires the alphabetical order |
| Operator runs `show bgp summary` with no pipe | `ProcessPipesDefaultFormatChecked` | Yes. It reads `ColumnsForCommand` before `foldFilters`, appends `configuredDefault(...)` when no format op is typed, and `configuredDefault` returns `pipeText` by default, so the bare command takes the order |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ApplyPipes` gained a fourth parameter. Every caller of it is in `pipe.go` |
| A-2 | confirmed | `TestBgpSummaryColumnsMatchPayload` compares both directions: no payload key is undeclared, and no declared name is absent from a payload |
| A-3 | confirmed | `ApplyTable` keeps its single-argument form, so `internal/component/iface/cli/scan.go` renders as before. The ui suite is 192 of 192 |
| A-4 | confirmed | The same test merges the three `mergeRibRouteCounts` keys into the row before comparing, so the comparison covers the full key set the command can produce |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/formatting.md`: the three programmatic formats keep their alphabetical keys | `ApplyPipes` builds a `tableStyle` on the `pipeTable` and `pipeText` arms alone | Yes |
| `docs/features/formatting.md`: "A column is never hidden" | `declaredKeys` returns `append(declared, rest...)`, and `rest` holds every key the order does not name | Yes |
| `docs/features/formatting.md`: "Only `show bgp summary` and `show bgp peer list` declare an order today" | `grep -rn 'RegisterColumns(' internal/` finds one caller, `registerColumns` in `peer.go` | Yes |
| `docs/architecture/api/commands.md`: `RegisterColumns` reaches `tableStyle.orderKeys` through the four `ProcessPipes*` wrappers | `ColumnsForCommand` in each of the four wrappers in `pipe.go`; `orderKeys` in `pipe_table.go` | Yes |
| Category 16, anchors naming the changed files | `grep -n 'source:' docs/features/formatting.md` lists the three anchors added for this feature, each naming a symbol that exists | Yes |

## Core Insight

One command can render more than one record shape, and the shapes can share a
key name. `show bgp summary` carries "uptime" in the peer row and in the record
that holds the rows, and the two belong in different positions. A declaration
keyed by command name alone cannot state both, so the key SET is what identifies
the shape: `bestColumnOrder` picks the declared order that names the most of the
keys in hand. The next command to declare an order will meet the same question
the moment it answers with more than one shape.
