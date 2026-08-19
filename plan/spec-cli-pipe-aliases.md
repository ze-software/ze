# Spec: cli-pipe-aliases

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | spec-cli-column-order, spec-cli-order-pipe |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/cli-pipe-aliases.md` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`| display` and `| fill` let an operator name the fields they want. Naming five
fields every time is what an alias removes: `| summary` and `| peers` should be
sayable names for expressions the operator would otherwise retype.

Two things stand in the way, and the first is a payload defect.

**`handleBgpSummary` is the only handler in the tree that wraps its answer in an
envelope named after one of the views it contains.** It returns
`{"summary": {aggregates..., "peers": [...]}}`. Every other multi-view handler
puts the aggregates and the row array as SIBLINGS at the top level:
`handleBgpHealth` (`internal/component/bgp/plugins/cmd/peer/health.go`) answers
`{"peers": rows, "count": N, "not-established": M}`, and the component report
(`internal/component/cmd/show/show.go`) answers
`{"status", "components", "count", "checked-at"}`. Roughly 38 handlers use the
flat shape; one uses the envelope.

That matters beyond tidiness. An alias over the flat shape is a sibling-key
selection, which `| display` already expresses. An alias over the envelope needs
an operator that DESCENDS into a subtree, which would be a fifth independent
implementation of single-key unwrapping — `renderValue`, `selectMap`,
`countItems` and `unwrapSingleKeyArray` are the four that already exist and
would all have to agree with it.

So this spec flattens `handleBgpSummary` first, then adds aliases as named pipe
expressions. No descend operator, no path syntax.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - pipe operators and command-owned pipe filters
  → Constraint: an op kind classified nowhere in `foldFilters` is dropped with no error, because that switch has no `default:` arm
  → Decision: a command's own registered filter resolves BEFORE anything generic, so a filter can shadow an alias of the same name

### Rules
- [ ] `ai/rules/cli.md` - JSON format contract, pipe completeness
  → Constraint: JSON keys are lowercase kebab-case matching the YANG leaf name
  → Constraint: the response payload is structured data; `| json`, `| yaml`, `| table` are renderings of one payload
- [ ] `ai/rules/no-layering.md` - when replacing X with Y, delete X first
  → Constraint: the `summary` envelope is REMOVED, not kept alongside the flat form

**Key insights:**
- The flat shape needs no new selection machinery: after flattening, `summary` is `display` over the aggregate keys and `peers` is `display peers`, whose value is an array the table renderer already renders as rows.
- A global alias CANNOT be an empty-path registration. `commandRegistry.register` skips an empty command outright, and `commandMatchesPrefix(cmd, "")` returns false for every non-empty command, so such a registration would match nothing, silently.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` builds the aggregate record (`router-id`, `local-as`, `uptime`, `peers-configured`, `peers-established`, and `family`/`peers-in-family` when filtered) plus a `peers` array, then wraps the whole thing in `plugin.Map{"summary": summary}`. `mergeRibRouteCounts` adds `routes-received`, `routes-accepted`, `routes-sent` per peer row
- [ ] `internal/component/bgp/plugins/cmd/peer/health.go` - `handleBgpHealth` shows the house shape: `plugin.Map{"peers": rows, "count": ..., "not-established": ...}`, siblings at top level
- [ ] `internal/component/cmd/show/show.go` - the component report, same flat shape with four sibling keys
- [ ] `internal/component/lg/handler_api.go` - `summaryPeers` reads `ze["summary"]["peers"]` and ALREADY falls back to `ze["peers"]`, so the looking-glass API survives flattening unchanged; `parseJSON` promotes a bare array to `{"peers": ...}`
- [ ] `internal/component/lg/handler_ui.go` - navigates `ze["summary"]` with no fallback
- [ ] `internal/component/cli/model_dashboard.go` - `parseDashboardSnapshot` binds `json:"summary"`
- [ ] `internal/component/web/page_bgp_summary.go` - binds `json:"summary"`
- [ ] `internal/component/config/yang/cli/format.go` - binds `json:"summary"`
- [ ] `internal/component/command/column_order.go` - `commandRegistry[T]` resolves by longest matching command prefix; `register` skips an empty command path; `commandMatchesPrefix` returns false for an empty prefix against any non-empty command
- [ ] `internal/component/command/pipe.go` - `foldFilters` is a SINGLE pass with no re-entry and no `default:` arm; `pipeUnknown` resolves against the command's own filter set first; nothing re-parses the rewritten command string for pipes

**Behavior to preserve:**
- Every field `show bgp summary` answers with today still appears. Flattening moves keys; it removes none.
- `summaryPeers` keeps working through its existing flat-form fallback.
- `| display`, `| fill` and every other operator keep working unchanged.
- A command's own registered `PipeFilter` keeps resolving before generic handling.

**Behavior to change:**
- `handleBgpSummary` answers the flat shape. The `summary` envelope is deleted, not kept beside it.
- `handler_ui.go`, `model_dashboard.go`, `page_bgp_summary.go` and `format.go` read the flat shape.
- `| summary` and `| peers` exist as aliases.
- An alias name that a command's own pipe filter shadows is reported, not silently ignored.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp summary | peers` into the SSH CLI, the web CLI, or `ze cli -c`. The text reaches `ParsePipe` (`internal/component/command/pipe.go`) as one string.

### Transformation Path
1. `ParsePipe` splits the command from the operator chain.
2. **New:** alias expansion replaces an alias op with the operator chain it names, resolved against the command by longest prefix, then against the global table. It runs ONCE per chain with a depth bound.
3. `foldFilters` classifies the resulting ops. An alias that expanded to nothing recognised is an error, never a silent drop.
4. The dispatcher runs the command; `ResponseJSON` marshals the flat payload.
5. `ApplyPipes` applies the expanded chain exactly as if the operator had typed it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ CLI | The payload shape changes; the alias table lives entirely on the CLI side and never enters the payload | No |
| Alias table ↔ pipe parser | Expansion happens before classification, so an alias can only produce ops the parser already knows | No |

### Integration Points
- `internal/component/command/column_order.go` - `commandRegistry[T]` is the third user of the same longest-prefix lookup
- `internal/component/command/pipe.go` - expansion sits between `ParsePipe` and `foldFilters`
- `internal/component/lg/handler_api.go` - `summaryPeers`' existing fallback is what makes the flatten cheap

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `parsePipeChain` (`internal/component/command/pipe.go`) is the only producer of an expanded chain, and all five `ProcessPipes*` entry points call it. `ParsePipe` keeps two callers, `model_ping.go` and `model_traceroute.go`, and both discard the ops and keep only the command, so neither needs expansion |
| No unintended coupling (components stay isolated) | Yes | `internal/component/command` imports nothing new. The BGP peer plugin calls `command.RegisterAliases` the way it already calls `command.RegisterColumns` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `aliasRegistry` is `newCommandRegistry[aliasSet]()`, the third user of the lookup `ColumnsForCommand` and `lookupPipeFilters` share. `RegisterAliases` parses an expansion with `parsePipeOps`, extracted from `ParsePipe`, so one reading of the operator names exists |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding. `expandAliases` splices `entry.ops`, a slice fixed at registration, rather than re-parsing per command |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The two aliases are declared in `registerAliases` (`internal/component/bgp/plugins/cmd/peer/peer.go`), and `internal/component/command` names neither. `foldFilters` LOST a twelve-kind case list and gained a `default:` arm, so the change moves in the direction this check asks for |

## The flat payload

Before:

```json
{"summary": {"router-id": "…", "local-as": 64500, "uptime": "4h", "peers-configured": 2, "peers-established": 1, "peers": [ … ]}}
```

After:

```json
{"router-id": "…", "local-as": 64500, "uptime": "4h", "peers-configured": 2, "peers-established": 1, "peers": [ … ]}
```

`family` and `peers-in-family` keep their current behaviour: present only when
the command was given a family filter.

The two aliases then need no machinery beyond `| display`:

| Alias | Expands to |
|-------|-----------|
| `summary` | `display router-id local-as uptime peers-configured peers-established` |
| `peers` | `display peers` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `summaryPeers` needs no BEHAVIORAL change | It already falls back to `ze["peers"]` when the envelope is absent | The public looking glass loses its peer table | `TestSummaryPeersReadsFlatPayload` plus the lg golden tests | confirmed — the fallback carried it, so nothing broke. The file still changed: the dead envelope branch is deleted under `ai/rules/no-layering.md`, and the doc comment stated the envelope as fact |
| A-2 | Four consumers bind the envelope and no more | grep for `"summary"` across `internal/` named exactly `handler_ui.go`, `model_dashboard.go`, `page_bgp_summary.go`, `format.go` | A surface renders empty after the flatten | Re-grep at implementation time; run the web, lg and cli suites | broken — `config/yang/cli/format.go` is NOT a consumer: its `json:"summary"` belongs to `formatCollisionsJSON`, the completion-collision report. `handler_ui.go` holds TWO sites. Beyond production code the envelope was pinned by 12 Go test fixtures and 9 `.ci` tests, all of which had to be corrected |
| A-3 | `display peers` renders the rows array as a table | `renderValue` unwraps a single-key map whose value is an array, and `renderList` renders an array of objects | `\| peers` renders a cell rather than a table | `TestPeersAliasRendersRows` | confirmed — `display peers` answers `{"peers": [...]}`, which `renderValue` unwraps into rows. The test asserts the header names the peer columns |
| A-4 | One expansion pass is enough | No re-entrant rewrite exists in the pipe layer today; `foldFilters` is a single pass | An alias naming an alias silently half-expands | `TestAliasExpandsOnce` and `TestAliasRecursionIsRefused` | confirmed — an alias naming another is refused at registration, and a second pass over an expanded chain returns it unchanged |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The flatten breaks a consumer nobody grepped | A dashboard or web page renders empty rather than erroring | A-2's re-grep plus the functional suites; an empty render is the signal to look for, since a missing key unmarshals to a zero value rather than failing |
| R-2 | A command's own `PipeFilter` silently shadows an alias of the same name | `show bgp rib \| histogram` resolves the filter while a global `histogram` alias exists and is never reached | Refuse the REGISTRATION when an alias name collides with a filter on the same command path, so the conflict surfaces at startup rather than at use |
| R-3 | An alias expands to an op kind `foldFilters` does not classify, and vanishes | The alias appears to do nothing on a command with registered filters | Expansion produces only parser-known ops, and AC-8 exercises an alias on a filtered command |
| R-4 | Alias expansion becomes a general macro facility nobody can reason about | Aliases naming aliases, aliases carrying arguments | Bounded here: an alias expands to a fixed operator chain, takes no arguments, and may not name another alias |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The `show bgp summary` payload, which the CLI dashboard, the web summary page and the public looking glass all read. No protocol behavior and no wire format |
| How is it reverted? | Single commit revert. The payload change and the alias table land together |
| Who else touches this path? | `spec-cli-column-order` declares column orders for `show bgp summary`, which the flatten reshapes; `spec-cli-order-pipe` owns `display` and `fill` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp summary` over the CLI | → | `handleBgpSummary` returns sibling keys, no envelope | `test/plugin/bgp-summary-flat-payload.ci` |
| `show bgp summary \| peers` | → | alias expansion into `display peers` | `test/ui/alias-peers.ci` |
| `show bgp summary \| summary` | → | alias expansion into the aggregate `display` | `test/ui/alias-summary.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp summary \| json` | Top-level keys are `router-id`, `local-as`, `uptime`, `peers-configured`, `peers-established`, `peers`. No `summary` key exists |
| AC-2 | `show bgp summary ipv4` (family filtered) | `family` and `peers-in-family` appear as top-level siblings, as they do today under the envelope |
| AC-3 | The public looking glass peer page | Renders the peer table unchanged, through `summaryPeers`' existing flat-form fallback |
| AC-4 | The CLI dashboard, the web summary page, and the config CLI formatter | All render unchanged against the flat payload |
| AC-5 | `show bgp summary \| peers` | Only the peer rows render, as a table |
| AC-6 | `show bgp summary \| summary` | Only the aggregate fields render; the `peers` array is absent |
| AC-7 | An alias is registered globally and a command registers no alias of that name | The global alias resolves for that command |
| AC-8 | An alias used on a command WITH registered pipe filters (`show bgp rib`) | It expands and applies; it is not silently dropped by `foldFilters` |
| AC-9 | An alias name collides with a `PipeFilter` registered on the same command path | The REGISTRATION is refused at startup with a message naming both, rather than the filter silently winning at use time |
| AC-10 | An alias whose expansion names another alias | Refused at registration. Expansion is one pass and cannot recurse |
| AC-11 | A command-specific alias and a global alias share a name | The command-specific one wins, by the same longest-prefix rule the column registry uses |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types `show bgp summary \| peers` instead of naming the rows field | `ParsePipe` → alias expansion → `foldFilters` → `ApplyPipes` → `applyDisplaySelect` | `test/ui/alias-peers.ci` |
| 2 | Reads the public looking glass after the payload change | engine JSON → `parseJSON` → `summaryPeers` flat fallback → peer table | `test/ui/lg-peer-table-flat-payload.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBgpSummaryPayloadIsFlat` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-1, no `summary` key | |
| `TestBgpSummaryFamilyKeysAreSiblings` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-2 | |
| `TestSummaryPeersReadsFlatPayload` | `internal/component/lg/handler_api_test.go` | AC-3, A-1 | |
| `TestDashboardParsesFlatPayload` | `internal/component/cli/model_dashboard_test.go` | AC-4 | |
| `TestAliasResolvesGlobal` | `internal/component/command/alias_test.go` | AC-7 | PASS |
| `TestAliasCommandSpecificBeatsGlobal` | `internal/component/command/alias_test.go` | AC-11 | PASS |
| `TestAliasRegistrationRefusesFilterCollision` | `internal/component/command/alias_test.go` | AC-9, R-2 | PASS |
| `TestAliasRecursionIsRefused` | `internal/component/command/alias_test.go` | AC-10, A-4 | PASS |
| `TestAliasExpandsOnce` | `internal/component/command/alias_test.go` | A-4 | PASS |
| `TestAliasTakesNoArgument` | `internal/component/command/alias_test.go` | an alias carries no argument, and a word after it is refused | PASS |
| `TestAliasRegistrationRefusesOperatorName` | `internal/component/command/alias_test.go` | a name `knownPipeOps` already carries is refused | PASS |
| `TestCommandModePipeCompletion_WithAliases` | `internal/component/command/completer_test.go` | alias names complete in the pipe position | PASS |
| `TestAliasSurvivesFoldFiltersOnFilteredCommand` | `internal/component/command/pipe_test.go` | AC-8, R-3 | PASS |
| `TestFoldFiltersKeepsAnInvalidOpOnAFilteredCommand` | `internal/component/command/pipe_test.go` | AC-8: the `default:` arm carries a refusal a filtered command would have dropped | PASS |
| `TestPeersAliasRendersRows` | `internal/component/command/pipe_table_test.go` | AC-5, A-3 | PASS |
| `TestSummaryAliasDropsRows` | `internal/component/command/pipe_table_test.go` | AC-6 | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Alias expansion depth | exactly 1 pass | 1 | 0, which would mean the alias never expands | 2, refused at registration by AC-10 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-summary-flat-payload` | `test/plugin/bgp-summary-flat-payload.ci` | An agent parses `show bgp summary \| json` and finds sibling keys | |
| `alias-peers` | `test/ui/alias-peers.ci` | An operator types `\| peers` and sees the peer table | written, NOT RUN — the tree's functional harness is down by owner instruction |
| `alias-summary` | `test/ui/alias-summary.ci` | An operator types `\| summary` and sees the aggregates only | written, NOT RUN — same |
| `lg-peer-table-flat-payload` | `test/ui/lg-peer-table-flat-payload.ci` | The looking glass peer table still renders | written, NOT RUN — same |

### Interop Tests (Scope: protocol)
Not applicable. Scope is `cli`; no wire-visible behavior changes.

## Files to Modify
- `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` returns the flat map; the `summary` envelope is deleted
- `internal/component/bgp/plugins/cmd/peer/peer.go` - the registered column orders follow the reshaped payload, and the two aliases are registered here
- `internal/component/lg/handler_ui.go` - reads the flat shape
- `internal/component/cli/model_dashboard.go` - `parseDashboardSnapshot` binds the flat shape
- `internal/component/web/page_bgp_summary.go` - binds the flat shape
- `internal/component/command/pipe.go` - alias expansion between `ParsePipe` and `foldFilters`
- `docs/architecture/api/commands.md` - the payload shape and the alias mechanism
- `docs/features/formatting.md` - aliases beside the operators
- `docs/guide/command-reference.md` - `show bgp summary` output shape

## Files to Create
- `internal/component/command/alias.go` - `RegisterAliases`, `AliasesForCommand`, the global table, and the collision and recursion refusals
- `internal/component/command/alias_test.go`
- `test/plugin/bgp-summary-flat-payload.ci`
- `test/ui/alias-peers.ci`
- `test/ui/alias-summary.ci`
- `test/ui/lg-peer-table-flat-payload.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Aliases register in Go at init, like `RegisterPipeFilters`. An operator-defined alias in config is out of scope; see Known Limitations |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | No | No command or flag; a pipe alias resolves to existing operators |
| CLI grammar (keyword before value) | Yes | An alias occupies the operator slot and takes no arguments |
| Editor autocomplete | Yes | Alias names complete in the pipe position beside the operators |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-summary-flat-payload.ci` covers the changed payload |
| Pipe completeness | Yes | AC-8 pins that an alias survives `foldFilters` on a filtered command |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/formatting.md` |
| 2 | Config syntax changed? | No | No config surface changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — the `show bgp summary` payload shape |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` — the payload is an agent-facing contract |
| 5 | Plugin added/changed? | No | No plugin boundary changes |
| 6 | Has a user guide page? | Yes | `docs/features/formatting.md` |
| 7 | Wire format changed? | N-A | Scope is cli |
| 8 | Plugin SDK/protocol changed? | No | The alias table never enters the payload |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is cli |
| 10 | Test infrastructure changed? | No | New tests use the existing runners |
| 11 | Affects daemon comparison? | No | No feature-parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing enters an inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `summary.go`, `handler_api.go`, `pipe.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any doc showing `show bgp summary` JSON shows the envelope and must be corrected |
| 18 | `docs/architecture/web-interface.md`, the design doc `internal/component/lg/handler_ui.go` declares? | No | Unaffected. `grep -n summary docs/architecture/web-interface.md` returns nothing: the page's design doc describes the looking-glass surface and states no payload shape. The flatten changed how `extractPeers` and `findPeer` navigate, not what the page is |
| 19 | `docs/architecture/web-workbench-pages.md`, the design doc `internal/component/web/page_bgp_summary.go` declares? | No | Unaffected. Its one hit is the source anchor `<!-- source: internal/component/web/page_bgp_summary.go -- summary table -->`, which names the table's existence. The prose beneath it is about `collectPeers` and the row actions, and states no JSON shape |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the flat payload exists and every consumer reads it
   - Tests: `TestBgpSummaryPayloadIsFlat`, `TestBgpSummaryFamilyKeysAreSiblings`, `TestSummaryPeersReadsFlatPayload`, `TestDashboardParsesFlatPayload`, `test/plugin/bgp-summary-flat-payload.ci`
   - Files: `internal/component/bgp/plugins/cmd/peer/summary.go`, `internal/component/lg/handler_ui.go`, `internal/component/cli/model_dashboard.go`, `internal/component/web/page_bgp_summary.go`, `internal/component/config/yang/cli/format.go`
   - Verify: re-grep `internal/` for the `summary` envelope before declaring the consumer list complete (A-2). A consumer that renders EMPTY rather than failing is the signal, since a missing key unmarshals to a zero value
2. **Phase: The alias registry** -- registration, resolution, and both refusals
   - Tests: `TestAliasResolvesGlobal`, `TestAliasCommandSpecificBeatsGlobal`, `TestAliasRegistrationRefusesFilterCollision`, `TestAliasRecursionIsRefused`, `TestAliasExpandsOnce`
   - Files: `internal/component/command/alias.go`
   - Verify: a global alias must NOT be an empty-path registration in `commandRegistry` — that registers nothing and matches nothing. It needs its own table consulted when the per-command lookup misses
3. **Phase: Expansion in the chain**
   - Tests: `TestAliasSurvivesFoldFiltersOnFilteredCommand`, `TestPeersAliasRendersRows`, `TestSummaryAliasDropsRows`
   - Files: `internal/component/command/pipe.go`
   - Verify: expansion happens before `foldFilters`, produces only parser-known ops, and runs exactly one pass
4. **Phase: The two aliases and completion**
   - Tests: `test/ui/alias-peers.ci`, `test/ui/alias-summary.ci`, `test/ui/lg-peer-table-flat-payload.ci`
   - Files: `internal/component/bgp/plugins/cmd/peer/peer.go`, `internal/component/command/completer.go`
   - Verify: alias names complete in the pipe position; AC-5 and AC-6 render as specified
5. **Phase: Docs**
   - Files: every doc row answered Yes above
   - Verify: no doc still shows the `summary` envelope

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has an implementation at file:symbol |
| Feature completeness | Every consumer of the old envelope reads the flat shape; none renders empty |
| Correctness | The global alias table is separate from the prefix registry; expansion is one pass; both refusals fire at registration, not at use |
| Naming | Alias names are lowercase and do not collide with an operator in `knownPipeOps` |
| Data flow | The alias table never enters the payload; expansion produces only ops the parser already knows |
| Rule: `ai/rules/no-layering.md` | The `summary` envelope is DELETED, not accepted alongside the flat form |
| Rule: `ai/rules/evidence.md` | The consumer list came from a re-grep at implementation time, not from this spec's list alone |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The envelope is gone | `grep -rn '"summary"' internal/component/bgp/plugins/cmd/peer/summary.go` returns nothing |
| No consumer still binds it | `grep -rn 'json:"summary"' internal/` returns nothing |
| Aliases resolve per command and globally | `TestAliasCommandSpecificBeatsGlobal` and `TestAliasResolvesGlobal` pass |
| A shadowing alias is refused loudly | `TestAliasRegistrationRefusesFilterCollision` passes |
| Functional coverage exists | the four `.ci` files above run |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An alias name comes from a registration, not from operator input. Confirm an operator cannot define one at runtime in this spec's scope |
| Resource exhaustion | Expansion is one bounded pass and cannot recurse. Confirm no path allows an alias chain to grow the op list without bound |
| Information leakage | The flatten moves keys and removes none. Confirm no field that used to be nested becomes unreachable to a consumer that has not been updated |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A surface renders EMPTY after the flatten | A consumer was missed. Re-grep; do not patch the payload back |
| An alias silently does nothing | `foldFilters` classification, or a filter shadowing it. Fix the cause, never the test |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The flatten is not tidiness: it is what removes the need for a descend operator, and with it a fifth independent implementation of single-key unwrapping that would have to agree with `renderValue`, `selectMap`, `countItems` and `unwrapSingleKeyArray`.
- A global alias looks like an empty-path registration and cannot be one. The guard in `commandRegistry.register` drops it, and `commandMatchesPrefix` would refuse it anyway. That is a silent double failure, which is why it gets its own test.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Flatten the payload | Add a descend operator to reach into the envelope | One handler in the tree uses the envelope and ~38 use the flat shape. Building an operator for the outlier costs a fifth JSON walker; changing the outlier costs four consumer updates |
| An alias expands to a fixed operator chain and takes no arguments | A macro facility with parameters | An alias with arguments is a small language. Every reader would then need to know both the alias and its expansion to predict the output |
| An alias may not name another alias | Bounded recursion with a depth limit | There is no re-entrant rewrite anywhere in the pipe layer today. One pass is enough for the stated need and cannot loop |
| A name collision between an alias and a command's pipe filter is refused at REGISTRATION | Let the filter win, as it does today | The filter winning is invisible at use time. A startup refusal names both sides while somebody can still act on it |
| The global alias table is separate from `commandRegistry` | Register globals against the empty command path | Verified: `register` skips an empty path and `commandMatchesPrefix` returns false for it, so that registration would match nothing, silently |

## Known Limitations
- Operator-defined aliases in config are out of scope. `container cli` (`internal/component/hub/yang/ze-hub-conf.yang`) could express a keyed list, but no list-valued config reaches `internal/component/command` today — the one existing `environment cli` leaf arrives as a scalar env string. That plumbing is its own spec.
- Aliases take no arguments.
- Only `show bgp summary` is flattened here. Other handlers already use the flat shape, so no sweep is needed, but any future handler that adopts an envelope would reintroduce the problem.
- Alias completion offers names but not their expansions.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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

- `handleBgpSummary` (`internal/component/bgp/plugins/cmd/peer/summary.go`)
  answers a flat `plugin.Map`. The `summary` envelope is deleted, not kept
  beside the flat form.
- Four consumers read the flat shape: `summaryPeers` and its dead envelope
  branch removed (`internal/component/lg/handler_api.go`), `extractPeers` and
  `findPeer` (`internal/component/lg/handler_ui.go`),
  `parseDashboardSnapshot` (`internal/component/cli/model_dashboard.go`), and
  `fetchBGPSummaryPeers` (`internal/component/web/page_bgp_summary.go`).
- `internal/component/command/alias.go` is new: `Alias`, `RegisterAliases`,
  `lookupAlias`, `AliasesForCommand`, `aliasSuggestions`, the `globalAliases`
  table, and the two collision refusals `filterShadowing` and
  `aliasShadowing`.
- `parsePipeChain` and `expandAliases` (`internal/component/command/pipe.go`)
  expand a chain once, between `ParsePipe` and `foldFilters`. All five
  `ProcessPipes*` entry points call `parsePipeChain`.
- `registerAliases` (`internal/component/bgp/plugins/cmd/peer/peer.go`)
  declares `summary` and `peers` on `show bgp summary`.
- `pipeExtras` (`internal/component/command/completer.go`) offers alias names
  in the pipe position beside the operators.

### Bugs Found/Fixed

- `foldFilters` (`internal/component/command/pipe.go`) had no `default:` arm,
  so any op kind absent from its twelve-case client-side list was dropped with
  no error on every command that registers filters. `pipeInvalid` was one of
  them, so a filtered command discarded its own validation errors. The case
  list is gone and a `default:` arm does the same job for every kind.
  `TestFoldFiltersKeepsAnInvalidOpOnAFilteredCommand` covers it.
- Three `.ci` assertions were wrong about their subject and the functional
  suites found all three, fixed in `88de3212f`: `alias-summary` used a
  router-id containing a peer address as a substring, `lg-peer-table-flat-payload`
  waited on an endpoint the flatten never touched, and
  `cli-verb-daemon-dispatch` used `show bgp` as its example of a container that
  is not a command, which it had stopped being.

### Documentation Updates

- `docs/architecture/api/commands.md` -- the flat payload and the alias
  mechanism, anchored `<!-- source: internal/component/command/alias.go --
  RegisterAliases, lookupAlias, AliasesForCommand -->` and
  `<!-- source: internal/component/command/pipe.go -- parsePipeChain,
  expandAliases, parsePipeOps -->`.
- `docs/features/formatting.md` -- aliases beside the operators, anchored on
  `alias.go`.
- `docs/guide/command-reference.md` -- the `show bgp summary` output shape.
- `docs/architecture/api/birdwatcher-compat.md` -- Section 9 said the engine
  answers `{"summary":{"peers":[...]}}`; it now records that as the history it
  is and states the flat shape.
- `make ze-doc-verify` is RED on one anchor and it is not this spec's:
  `docs/architecture/api/process-protocol.md` names `runHub` in
  `cmd/ze/hub/main.go`. `git grep 'func runHub' HEAD` is empty and so is the
  worktree, and that doc's 27 added lines are another session's uncommitted
  work. No file this commit carries is in that stage.

### Deviations from Plan

- `internal/component/config/yang/cli/format.go` was listed under Files to
  Modify and was NOT modified. Its `json:"summary"` belongs to
  `formatCollisionsJSON`, the completion-collision report, and has nothing to
  do with the BGP summary. It is removed from the list; A-2 records the
  correction.
- `internal/component/lg/handler_api.go` was expected to need no change (A-1)
  and changed anyway: `ai/rules/no-layering.md` requires the dead envelope
  branch to go, and the doc comment stated the envelope as current fact.
- `spec-cli-column-order` had declared the summary's column orders against the
  pre-flatten key layout, so `registerColumns` moved with the payload in the
  same commit.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 said four consumers bind the envelope and no more | `format.go` was never a consumer, `handler_ui.go` holds TWO sites, and beyond production code 12 Go fixtures and 9 `.ci` tests pinned the shape | the re-grep the spec's own Implementation Step 1 required | consumer list re-derived at implementation time and again at closure; Files to Modify corrected |
| approach | A global alias was expected to be an empty-path registration in `commandRegistry` | `register` skips an empty command and `commandMatchesPrefix` returns false for an empty prefix, so it would match nothing and report nothing | read during research, before any code | `globalAliases` is its own mutex-guarded table, and the reason is a comment above it |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `handleBgpSummary` stops wrapping its answer in an envelope named after one of its views | Done | `internal/component/bgp/plugins/cmd/peer/summary.go`, `handleBgpSummary` | Returns `plugin.Map` directly |
| `summary` and `peers` are sayable names for expressions an operator would retype | Done | `internal/component/bgp/plugins/cmd/peer/peer.go`, `registerAliases` | Both registered on `show bgp summary` |
| No descend operator and no path syntax | Done | `internal/component/command/alias.go` | An alias expands to a fixed operator chain; no fifth JSON walker was added |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBgpSummaryPayloadIsFlat`, `test/plugin/bgp-summary-flat-payload.ci` | The test asserts the `summary` key is ABSENT, which nothing did before |
| AC-2 | Done | `TestBgpSummaryFamilyKeysAreSiblings` | |
| AC-3 | Done | `TestSummaryPeersReadsFlatPayload`, `test/ui/lg-peer-table-flat-payload.ci` | |
| AC-4 | Done | `TestDashboardParsesFlatPayload`, the `web` package suite | `format.go` is not a consumer, per A-2 |
| AC-5 | Done | `TestPeersAliasRendersRows`, `test/ui/alias-peers.ci` | The `.ci` asserts the table header names the peer columns |
| AC-6 | Done | `TestSummaryAliasDropsRows`, `test/ui/alias-summary.ci` | |
| AC-7 | Done | `TestAliasResolvesGlobal` | |
| AC-8 | Done | `TestAliasSurvivesFoldFiltersOnFilteredCommand`, `TestFoldFiltersKeepsAnInvalidOpOnAFilteredCommand` | The second pins the `default:` arm that a filtered command used to drop |
| AC-9 | Done | `TestAliasRegistrationRefusesFilterCollision` | Refused from both sides, since init order picks which registration runs second |
| AC-10 | Done | `TestAliasRecursionIsRefused`, `TestAliasExpandsOnce` | |
| AC-11 | Done | `TestAliasCommandSpecificBeatsGlobal` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 6 payload and consumer unit tests | Done | `summary_test.go`, `handler_api_test.go`, `model_dashboard_test.go` | |
| The 11 alias unit tests | Done | `alias_test.go`, `pipe_test.go`, `pipe_table_test.go`, `completer_test.go` | |
| `bgp-summary-flat-payload` | Done | `test/plugin/bgp-summary-flat-payload.ci` | Ran: plugin suite 671/672 |
| `alias-peers`, `alias-summary`, `lg-peer-table-flat-payload` | Done | `test/ui/` | Ran: ui suite 191/191, skip 1. The spec's own table still said "written, NOT RUN"; the harness came back and all three ran |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file under Files to Create | Done | `ls` output in Pre-Commit Verification |
| Every file under Files to Modify | Changed | `internal/component/config/yang/cli/format.go` removed from the list, see Deviations |

### Audit Summary
- **Total items:** 11 AC + 3 task requirements + 17 tests + 2 file groups = 33
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (Files to Modify, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator names a view instead of retyping five fields | functional | `test/ui/alias-summary.ci` pipes `show bgp summary` through the `summary` alias and requires all four aggregates present and both peer addresses absent, then requires the same of the `json` rendering. `test/ui/alias-peers.ci` does the mirror for the rows and asserts the table header leads with `address`. Both compare against the whole answer FIRST, so each assertion measures a CHANGE. UI suite: `pass 191/191 100.0% 33.8s skip 1` |
| The `show bgp summary` payload matches the house shape | functional | `test/plugin/bgp-summary-flat-payload.ci`, run in the plugin suite at 671/672; the one red, `remove-private-as-replace-peer`, passes 3/3 alone and is recorded in `plan/journal/gate-verdict-depends-on-the-machine.md` |
| No consumer of the old envelope renders empty | functional and unit | The consumer list was re-derived at closure, not trusted: a grep for `json:"summary` across `internal/`, `cmd/` and `pkg/` leaves only `formatCollisionsJSON`; a grep for the map index leaves only tests of other subjects; no `.js` or `.templ` reads the key. All four consumer packages are green: `command` 1.093s, `peer` 46.018s, `lg` 1.954s, `web` 465.130s, `cli` dashboard tests 1.149s. `test/ui/lg-peer-table-flat-payload.ci` drives the public peer table over HTTP |
| An alias that silently does nothing is impossible | unit | Four registration refusals, each with a test: `TestAliasRegistrationRefusesFilterCollision`, `TestAliasRegistrationRefusesOperatorName`, `TestAliasRecursionIsRefused`, and the no-name and empty-expansion subtests beside it. `TestFoldFiltersKeepsAnInvalidOpOnAFilteredCommand` proves the drop that used to be possible is not |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Operator-defined aliases in configuration | deferred | `plan/future/spec-cli-operator-defined-aliases.md`, created at this closure. It carries the four design questions the row was waiting on: which registry an operator's alias joins, what replaces the four `panic("BUG:")` refusals in `checkedAlias` when a typo arrives from config, what a reload does to a table written once at init, and whether an operator's alias may shadow a shipped one |

The shard still holds this live row, so it is NOT removed at closure: the row
is homed at the future spec, and the shard is where it is written down
(`ai/rules/planning.md`). No foreign shard was emptied by this resolution.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-pipe-aliases-2e38eb27-078b-4f5b-a456-56437e962d09.md`, 16 files, verdict clean |
| `review_gate.py check` | `review_gate: OK (2 code files, clean, hashes match)`, exit 0 |
| Rounds | 2 |
| Reviewer lenses used | wiring and logic and removed-behavior; security and guard audit; simplicity and the style pass over every changed Go file |

Independence: this closure agent did not author the diff. It read
`f539aebab`, `5d0d73222` and the four later commits from source, and
re-derived the envelope consumer list itself rather than reading the spec's.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The no-argument check in `alias-peers.ci` did not discriminate. It asserted that the word `peers` appears in the output, and that word is the table header the ACCEPTED answer prints, so the assertion passes whether the argument is refused or silently taken (`ai/rules/interop-and-goal-validation.md`, "Prove the test discriminates") | `test/ui/alias-peers.ci` | Asserts the message `does not accept an argument`, that the message names the alias, and that no peer row was answered. Re-run: ui suite `pass 191/191`, `alias-peers` PASS 8.8s |

### Findings recorded, not fixed
| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| 2 | ISSUE, FIXED IN THE TREE, NOT IN THIS COMMIT | The `pipeInvalid` declaration comment still said "produced while folding command filters". `expandAliases` produces it too, for an alias given an argument, so the comment named one of its two producers (`ai/rules/stale-comments.md`). The one-line correction is in the working tree | `internal/component/command/pipe.go` is NOT in commit A. When this closure opened it, the file already carried ANOTHER session's uncommitted work: a package doc comment about the daemon running the chain, and a rename of `clientOps` to `chainOps` through `foldFilters`. Naming the file in this commit would carry that work under this commit's message, which is the cross-commit the explicit `--file` list exists to prevent (`ai/rules/git-safety.md`). The correction is left in the tree for whoever commits that rename. It is a comment, so nothing in the product depends on landing it here |
| 3 | NOTE | `ApplyJSON` (`internal/component/command/pipe.go`) has no cross-package non-test caller, and `ze-repository-check` reports it | Pre-existing, not this spec's. A grep for `command.ApplyJSON` is empty at `53e84d473~1`, at `45ebae909~1` and at HEAD, and none of this spec's commits touches the symbol |
| 4 | NOTE | `AliasesForCommand` is exported and its only non-test caller is `aliasSuggestions`, in the same file. The nearest analogue, `filterSuggestions` (`pipe_filter.go`), is unexported | Not dead: it is reached in production through `aliasSuggestions`, then `pipeExtras`, then `CompletePipe`, so tab completion is the user action. The name follows the exported `ColumnsForCommand`. Two readings, and neither is a defect, so the export stands |
| 5 | NOTE | The `test/weakened.md` row for `TestBgpSummaryNoPeers` says the assertion reads the top-level map directly, where the code reads the same map through a local conversion | Record defect, no product effect. `test/weakened.md` is replaced per commit and the row is already committed; adding a row to a commit that weakens nothing is what its own gate refuses |
| 6 | FIND | `handleBgpOverview` answers `show bgp` with the same payload, and neither the column orders nor the aliases reach that path: both register against `cmdBgpSummary` and `commandMatchesPrefix` refuses a prefix longer than the command | Journal row in `plan/journal/unwired-feature.md`, 2026-08-19. Not fixed here: re-registering on `show bgp` leaks both onto `show bgp rib`, `rpki`, `rs` and `healthcheck`, so the repair needs an exact-path registration no registry has. It does not block this spec's goal, which is the aliases on `show bgp summary` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/command/alias.go` | Yes | `ls -la` 10030 bytes |
| `internal/component/command/alias_test.go` | Yes | `ls -la` 9937 bytes |
| `test/plugin/bgp-summary-flat-payload.ci` | Yes | `ls -la` 5266 bytes |
| `test/ui/alias-peers.ci` | Yes | `ls -la` 6492 bytes |
| `test/ui/alias-summary.ci` | Yes | `ls -la` 5344 bytes |
| `test/ui/lg-peer-table-flat-payload.ci` | Yes | `ls -la` 3099 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No `summary` key exists | A grep for `summary` in `internal/component/bgp/plugins/cmd/peer/summary.go` shows the local variable and the wire method only; the return is `Data: summary`, not `plugin.Map{"summary": summary}` |
| AC-2 | `family` and `peers-in-family` are top-level siblings | Same grep: both are set on the same map the handler returns |
| AC-3, AC-4 | Every consumer reads the flat shape | A grep for `json:"summary` across `internal/`, `cmd/` and `pkg/` leaves `format.go` (`formatCollisionsJSON`), `ospf`, `diagnostic` and the test runner, none a summary consumer. `summaryPeers` reads `ze["peers"]` with no envelope branch |
| AC-5 to AC-11 | Every alias behaviour | `make ze-unit-pkg-test PKG=./internal/component/command` exits 0, `ok github.com/ze-software/ze/internal/component/command 1.093s`, and every named test is present in the package |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show bgp summary` over the CLI | `test/plugin/bgp-summary-flat-payload.ci` | Read: it parses the command's JSON and asserts the sibling keys |
| `show bgp summary` through the `peers` alias | `test/ui/alias-peers.ci` | Read: its `cli()` helper requires exit 0, so the alias not resolving fails the test; the rows, the table header and the JSON form are each asserted, and the aggregates are asserted ABSENT |
| `show bgp summary` through the `summary` alias | `test/ui/alias-summary.ci` | Read: the four aggregates asserted present, both peer addresses asserted absent, and the `peers` key looked for as the first field on a line so a prefix match cannot satisfy it |
| The public looking glass peer page | `test/ui/lg-peer-table-flat-payload.ci` | Read: drives `/protocols/bgp`, the endpoint that reads the summary |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with a change | `summaryPeers` needed no behavioural change; the flat fallback carried it. The dead envelope branch was deleted under `ai/rules/no-layering.md` |
| A-2 | broken | `format.go` is not a consumer, `handler_ui.go` holds two sites, and 12 Go fixtures plus 9 `.ci` tests pinned the shape. Mistake Log and Deviations both carry it |
| A-3 | confirmed | `TestPeersAliasRendersRows` asserts the header names the peer columns; `test/ui/alias-peers.ci` asserts the same through the daemon |
| A-4 | confirmed | `TestAliasExpandsOnce` and `TestAliasRecursionIsRefused`. An alias naming another is refused at registration, so one pass cannot leave an alias behind |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/api/commands.md` alias table | The two rows match `registerAliases` (`peer.go`) field for field | Yes |
| `docs/guide/command-reference.md` payload example | A grep for `peers-configured` across `docs/` shows the flat example and no envelope anywhere | Yes |
| `docs/architecture/api/birdwatcher-compat.md` Section 9 | States the envelope as history and the flat shape as current; matches `summaryPeers` | Yes |
| `docs/architecture/web-interface.md`, `docs/architecture/web-workbench-pages.md` | A grep for `summary` returns nothing in the first and only a source anchor in the second; neither states a payload shape | Yes, unaffected |
| `make ze-doc-verify` | RED on one anchor, in `process-protocol.md`, naming `runHub`, which exists neither at HEAD nor in the worktree; that file's 27 added lines are another session's uncommitted work | Not this commit's |

## Core Insight

The flatten was not tidiness, and the review confirmed the spec's own
reasoning: an alias over an envelope needs an operator that DESCENDS, and that
operator would be a fifth independent implementation of single-key unwrapping
which `renderValue`, `selectMap`, `countItems` and `unwrapSingleKeyArray` would
all have to agree with. Changing the one outlier handler cost four consumer
updates. Building the operator would have cost a walker that four existing ones
must never disagree with.

The second insight is smaller and cost more: `foldFilters` had no `default:`
arm. A switch over a kind enum, with no default, on a shared classification
path, is a silent drop for every kind added after it was written. The alias
work only found it because `pipeInvalid` had to survive that switch.
