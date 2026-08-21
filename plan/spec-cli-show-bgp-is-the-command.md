# Spec: cli-show-bgp-is-the-command

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 3/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`show bgp` and `show bgp summary` answer the same payload from the same code,
and the second one answers it better. `handleBgpOverview` delegates to
`handleBgpSummary`, so the data is identical, but the column orders and the pipe
aliases register against the literal string `"show bgp summary"`. The registry
matches by longest prefix, and `commandMatchesPrefix("show bgp", "show bgp
summary")` is false because the command is shorter than the prefix. So:

| Command | Columns | `\| peers` |
|---------|---------|-----------|
| `show bgp summary` | the declared operator-first order | works |
| `show bgp` | alphabetical | "unknown pipe operator" |

Two spellings of one answer, and the newer one is degraded.

Owner decision, 2026-08-19: **remove `show bgp summary`.** `show bgp` is the
command, and `| display`, `| fill` and the aliases are how an operator chooses
what to see. `show bgp | summary` reproduces the old view.

That removes the ambiguity at its root rather than teaching the registry to
match two paths for one answer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - dispatch, registration, the pipe surface
  → Constraint: dispatcher keys are CLI paths derived from YANG, and the path is the dispatch key
  → Decision: `matchBuiltinTokens` refuses its match when a longer prefix of the input is itself a registered command, so `show bgp rpki status` still reaches its plugin

### Rules
- [ ] `ai/rules/cli.md` - command grammar and the agent-facing contract
  → Constraint: a removed command is an agent-facing break, so every in-tree caller moves in the same change
- [ ] `ai/rules/no-layering.md` - replacement, not accumulation
  → Constraint: `show bgp summary` is DELETED, not aliased to `show bgp` and left in the tree

**Key insights:**
- The leak this avoids is real and already solved elsewhere. Registering the orders and aliases against `show bgp` makes them reachable from `show bgp rib`, `rpki`, `rs` and `healthcheck` by longest prefix. `registerPipeFilters` (`internal/component/bgp/plugins/cmd/rib/rib.go`) already answers that with an EMPTY registration on the child paths, and its comment says so. This spec uses the same mechanism rather than adding exact-path matching to the registry.
- `handleBgpOverview` already distinguishes a family argument from an unregistered subcommand, so `show bgp nonsense` answers "unknown command" rather than "invalid address family". That behaviour is kept.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `init` registers four RPCs including `ze-bgp:summary` (`handleBgpSummary`) and `ze-bgp:overview` (`handleBgpOverview`); `handleBgpOverview` rejects a first argument that names no address family and otherwise delegates to `handleBgpSummary`
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container bgp` carries `ze:command "ze-bgp:overview"`, and its child `container summary` carries `ze:command "ze-bgp:summary"`. The parent's description still refers the reader to `show bgp summary`
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `registerColumns` and `registerAliases` both register against `cmdBgpSummary`, the literal `"show bgp summary"`
- [ ] `internal/component/command/column_order.go` - `commandRegistry.lookup` picks the longest matching prefix; `commandMatchesPrefix` returns false when the command is shorter than the prefix; `register` skips an empty command path
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` registers an EMPTY filter set on the scalar rib commands precisely to stop them inheriting a parent's registration, and its comment explains why
- [ ] `internal/component/plugin/server/command.go` - `matchBuiltinTokens` refuses a builtin match when a longer registered path also matches, so removing the `summary` key cannot expose the plugin subtrees
- [ ] `internal/component/lg/handler_api.go` - `handleAPIProtocols` queries the literal string `show bgp summary`; `summaryPeers` reads the rows

**Behavior to preserve:**
- The PAYLOAD is unchanged. `show bgp` answers exactly what `show bgp summary` answered: the flat record with `router-id`, `local-as`, `uptime`, `peers-configured`, `peers-established` and `peers`.
- `show bgp ipv4` and the other family filters keep working, as arguments to `show bgp`.
- `show bgp rpki status`, `show bgp rs peers`, `show bgp adj-rib-in`, `show bgp healthcheck`, `show bgp rib`, `show bgp peer list` and every other subcommand resolve exactly as they do today.
- `show bgp nonsense` answers "unknown command", not an address-family error.
- The public looking glass, the web UI, the CLI dashboard, MCP and REST all keep answering; they move to the new command in this change.

**Behavior to change:**
- `show bgp summary` stops existing. The YANG container, the `ze-bgp:summary` wire method and the `handleBgpSummary` RPC registration all go.
- The column orders and the aliases register against `show bgp`, with empty registrations on the child paths that must not inherit them.
- `show bgp | summary` and `show bgp | peers` are how the two views are reached.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp | summary` into the SSH CLI, the web CLI, or `ze cli -c`. An in-tree consumer such as the looking glass issues `show bgp` over the exec channel.

### Transformation Path
1. `ParsePipe` splits `show bgp` from the operator chain.
2. Alias expansion turns `summary` into the `display` over the aggregate fields, resolved against `show bgp` rather than `show bgp summary`.
3. `matchBuiltinTokens` matches the `show bgp` key. No longer registered path matches, so the guard serves it.
4. `handleBgpOverview` accepts a family argument or refuses an unknown token, then delegates to `handleBgpSummary`.
5. `ApplyPipes` renders with the column order registered against `show bgp`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ looking glass, web, MCP, REST | Each issues the command string; every one moves from `show bgp summary` to `show bgp` | No |
| Registry ↔ child commands | Empty registrations block the orders and aliases leaking down the subtree | No |

### Integration Points
- `internal/component/bgp/plugins/cmd/peer/peer.go` - the registrations move to `show bgp` and gain the blocking empties
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the `ze-bgp:summary` registration goes; `handleBgpSummary` stays as the implementation behind `handleBgpOverview`
- `internal/component/lg/handler_api.go` - the query string moves

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
| A-1 | Empty registrations on the child paths stop the orders and aliases leaking | `registerPipeFilters` uses exactly this for the scalar rib commands, with a comment stating the mechanism | `show bgp rib` renders with the summary's column order and offers `\| peers` | `TestChildCommandsDoNotInheritTheSummaryOrder` | confirmed |
| A-2 | 131 files reference the string, and every one is in this tree | `grep -rl 'show bgp summary'` over `internal/ cmd/ test/ docs/ demos/ ai/` | An out-of-tree consumer breaks with no warning | Re-grep at implementation time, including `.templ`, `.json` and `.py` | broken |
| A-3 | Removing the `show bgp summary` dispatcher key cannot expose the plugin subtrees | `matchBuiltinTokens` refuses a match when a longer registered path also matches, and the four subtrees are plugin names reached by fallback | `show bgp rpki status` regresses | `test/plugin/show-bgp-child-not-swallowed.ci`, unchanged | confirmed |
| A-4 | No RFC or external contract names `show bgp summary` | It is a CLI path, not a protocol element | An interop or birdwatcher-compatibility surface breaks | Grep `rfc/`, and check `internal/component/lg/handler_api.go` for birdwatcher route names | confirmed |

**Audit findings, 2026-08-21** (each read against the producing symbol, then re-verified by the main thread):

- A-1 `confirmed`. The mechanism is stated twice in the tree: `RegisterColumns` (`internal/component/command/column_order.go:41-61`) says "Passing no order registers the command as declaring none, which stops a shorter registered command path from ordering it", and `registerPipeFilters` (`internal/component/bgp/plugins/cmd/rib/rib.go:52-88`) uses it with a comment naming the longest-prefix inheritance it blocks. `commandRegistry.register:93-104` skips an empty PATH, never an empty VALUE, so the empty registration is stored and `lookup:106-132` returns it.
- **A-2 `broken`. The real count is 151 files, 382 occurrences**, excluding `.git/` and another agent's checkout under `.claude/worktrees/`. The spec's grep covered `internal/ cmd/ test/ docs/ demos/ ai/` and missed `website/` (4), `scripts/` (6) and `plan/` (10). By directory: `internal/` 56, `test/` 42, `docs/` 23, `plan/` 10, `scripts/` 6, `website/` 4, `demos/` 4, `cmd/` 3, `ai/` 3. By extension: `.go` 53 (24 production, 29 test), `.ci` 40, `.md` 39, `.py` 6, `.yang` 4, `.html` 4, `.sh` 2, `.txt` 1, `.tape` 1, `.json` 1. **`.templ` is 0**: 69 `.templ` files exist and none names the string, because the web surface goes through `page_bgp_summary.go` and `page_bgp_peers.go`. `ze-bgp:summary` is a separate sweep of 13 files, one of which is the generated snapshot `internal/component/plugin/all/testdata/wire-methods.snapshot`.
- A-3 `confirmed`, and it delivers AC-4 as a bonus. `matchBuiltinTokens` (`internal/component/plugin/server/command.go:574-604`) calls `longerCommandPath` after every key match; `isCommandPath:633-651` reads `d.commands`, `d.registry.hasCommandPath` and `d.subsystems.hasCommandPath`, which is where the four plugin subtrees live. Removing the `summary` key deletes one entry from `d.commands` and touches neither the guard nor those registries. With the key gone, `show bgp summary` matches `show bgp`, `longerCommandPath` finds nothing longer, and `handleBgpOverview:174-190` answers the unknown-command error AC-4 asks for.
- A-4 `confirmed`. `rfc/` has zero hits. The birdwatcher contract in `internal/component/lg/handler_api.go` is keyed on route names such as `/api/looking-glass/protocols/bgp`; the ze command string is an argument to `s.query`, never a wire field.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A consumer is missed and answers "unknown command" | A page, a dashboard or an agent surface errors rather than rendering | This failure is LOUD, unlike the payload flatten whose misses rendered empty. Re-grep rather than trust the list, and run the web, lg and plugin suites |
| R-2 | The orders and aliases leak onto every child path, not the four this row first named | `show bgp rib` renders with peer columns, or accepts `\| peers` | A-1's test, driven per child path rather than for one example. **The child population is 16 builtin paths, not 4** (see the audit note below) |
| R-5 | An alias name later collides with a pipe filter on an overlapping path, and the daemon panics at init | `panic("BUG: pipe alias ...")` from `aliasShadowing` on startup | `filterShadowing` (`internal/component/command/alias.go:151-171`) and `aliasShadowing:177-212` panic when an alias and a filter share a name on overlapping paths, and `pathsOverlap:216-233` makes every `show bgp *` path overlap `show bgp`. No filter is named `summary` or `peers` today, so the move is safe now. Registering the aliases one level up widens what a future filter registration can collide with, so the registration site carries a comment saying so |
| R-3 | An operator's muscle memory breaks with no hint | `show bgp summary` answers "unknown command" | Keeping the command is what this spec exists to undo, so the mitigation is the message rather than the command. It must be the clear unknown-command error and never a family-validation error. `handleBgpOverview` already answers that way for `show bgp nonsense` |
| R-4 | The demo and the quickstart still show the old command | The website serves a recording of a command that no longer exists | The demo's three files and the guides move in this change; a re-render is owed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every surface that asks for a BGP summary: the public looking glass, the web UI, the CLI dashboard, MCP, REST, and 40 functional tests. No protocol behaviour and no wire format |
| How is it reverted? | Single commit revert. The payload is unchanged, so a revert restores the command without touching data |
| Who else touches this path? | `main-c2` owns `internal/component/command/` for the streaming work; this spec does not need those files. `spec-cli-dispatch-child-guard` **has CLOSED** (`b62c52fef close: remove spec-cli-dispatch-child-guard`, after `647f33121` landed the guard), so this spec has no open dependency. `longerCommandPath` and `isCommandPath` are in the tree |

**Audit note, 2026-08-21: the child population is 15 to block, out of 16 under `show bgp`.**
Corrected during phase 1: the note below said 16 and then listed 15. The
sixteenth is `show bgp summary` ITSELF, and it must NOT be blocked. It has to
keep inheriting the orders and the aliases until phase 3 removes it, which is
what makes phase 1 safe to land on its own. The 15 named below are the ones that
take an empty registration. `make ze-command-list` reports these builtin paths under `show bgp`, every one of which would inherit the `summary` and `peers` aliases from a `show bgp` registration: `health`, `irr`, `irr check`, `irr prefix`, `peer capabilities`, `peer detail`, `peer history`, `peer list`, `peer rib`, `peer statistics`, `rib`, `rib best`, `rib best status`, `rib rpf`, `rib status`. The four plugin subtrees (`rpki`, `rs`, `adj-rib-in`, `healthcheck`) are reached by fallback and are not builtins. AC-7 already says "every other child path"; the Implementation Steps and R-2 drove only four, and now drive all of them.

**The empties are not one uniform loop.** `show bgp peer list` already carries its own `ColumnOrder` (`internal/component/bgp/plugins/cmd/peer/peer.go:registerColumns:99-102`), so an empty COLUMN registration there would destroy a declared order. It carries no alias registration, so it does need an empty ALIAS registration. Column empties and alias empties are two different lists.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator runs `show bgp \| summary` | → | alias resolved against `show bgp`, then `handleBgpOverview` | `test/ui/show-bgp-alias-summary.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| Operator runs `show bgp summary` | → | no dispatcher key exists; the unknown-command path answers | `test/plugin/show-bgp-summary-is-gone.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| Operator runs `show bgp rib \| peers` | → | the empty registration blocks the inherited alias | `test/ui/show-bgp-children-do-not-inherit.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp` | The summary payload, rendered in the declared operator-first column order |
| AC-2 | `show bgp \| summary` | The aggregate fields alone, as `show bgp summary \| summary` gave before |
| AC-3 | `show bgp \| peers` | The peer rows alone |
| AC-4 | `show bgp summary` | A clear unknown-command error. Not a family-validation error, and not a working command |
| AC-5 | `show bgp ipv4` | The family-filtered summary, unchanged |
| AC-6 | `show bgp rpki status`, `show bgp rs peers`, `show bgp adj-rib-in`, `show bgp healthcheck`, `show bgp rib`, `show bgp peer list` | Each resolves to its own handler, unchanged |
| AC-7 | `show bgp rib`, and every other child path | Renders alphabetically and rejects `\| peers`. The summary's orders and aliases do not leak down the subtree |
| AC-8 | The looking glass peer page, the web summary page, the CLI dashboard, MCP and REST | All answer, each having moved to `show bgp` |
| AC-9 | `grep -rn 'show bgp summary'` over the tree | Returns nothing outside git history and this spec |
| AC-10 | `make ze-command-list` | Names `show bgp` and does not name `show bgp summary` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types `show bgp` for an overview and `show bgp \| peers` for the rows | CLI → alias expansion → `matchBuiltinTokens` → `handleBgpOverview` | `test/ui/show-bgp-alias-summary.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| 2 | Loads the public looking glass peer page | LG issues `show bgp` → `summaryPeers` → birdwatcher protocols | `test/ui/lg-peer-table-flat-payload.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowBgpCarriesTheSummaryOrder` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-1, the orders resolve against `show bgp` | pass |
| `TestChildCommandsDoNotInheritTheSummaryOrder` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-7, A-1, driven per child path | pass |
| `TestShowBgpSummaryIsNotRegistered` | `internal/component/plugin/server/command_test.go` | AC-4 and AC-10, no dispatcher key and no wire method | pass |
| `TestBgpOverviewAnswersTheSummary/the retired summary subcommand` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-4, the message is the unknown-command error and not the family rejection | pass |
| `TestBgpOverviewAnswersTheSummary/family argument` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-5. This is the row phase 3 wrote instead of a new `TestShowBgpFamilyArgumentStillFilters`: the existing subtest already drove the overview's family path, and it was strengthened to assert the SCOPE (`family`, `peers-in-family`) rather than only the status. A second test asserting the same thing would be a duplicate | pass |
| `TestAliasesResolveAgainstShowBgp` | `internal/component/command/alias_test.go` | AC-2 and AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tokens after `show bgp` | 0 or 1 | 1, a family name | 0, the bare overview, which is valid | 2 or more, which names no command and no family |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-alias-summary` | `test/ui/show-bgp-alias-summary.ci` | An operator gets both views from `show bgp` and a pipe | | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| `show-bgp-summary-is-gone` | `test/plugin/show-bgp-summary-is-gone.ci` | The old command answers unknown-command | pass (608) |
| `show-bgp-bare-runs-summary` | `test/plugin/show-bgp-bare-runs-summary.ci` | AC-1 end to end: `show bgp` answers the summary payload, and it is now the only spelling | pass (605) |
| `show-bgp-family-arg` | `test/plugin/show-bgp-family-arg.ci` | AC-5 end to end. Renamed from `show-bgp-summary-family-arg.ci` in phase 3: it drove both spellings, and only one survives | pass (607) |
| `show-bgp-child-not-swallowed` | `test/plugin/show-bgp-child-not-swallowed.ci` | AC-6 and A-3, UNCHANGED by this spec | pass (606) |
| `show-bgp-children-do-not-inherit` | `test/ui/show-bgp-children-do-not-inherit.ci` | AC-7 across the child paths | | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->

### Interop Tests (Scope: protocol)
Not applicable. Scope is `cli`; no wire-visible protocol behaviour changes.

## Files to Modify
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container summary` is deleted; the parent description stops naming `show bgp summary`
- `internal/component/bgp/cli/yang/ze-bgp-tools-cmd.yang`, `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang`, `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` - each carries a BYTE-IDENTICAL `container bgp` description naming `show bgp summary`. They are container-merge duplicates and a gate requires them to agree, so all four move together or none does
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the `ze-bgp:summary` RPC registration goes; `handleBgpSummary` stays as the implementation behind `handleBgpOverview`
- `internal/component/bgp/plugins/cmd/peer/peer.go` - `registerColumns` and `registerAliases` move to `show bgp`; empty registrations block the child paths
- `internal/component/lg/handler_api.go`, `internal/component/lg/handler_ui.go` - the query strings move
- `internal/component/web/`, `internal/component/cli/`, `internal/component/mcp/`, `internal/component/api/rest/`, `cmd/ze/hub/` - each caller moves
- `test/plugin/*.ci`, `test/ui/*.ci`, `test/parse/*.ci` - 40 files carrying the command string (34 under `test/plugin/`, 5 under `test/ui/`, 1 under `test/parse/`)
- `docs/guide/`, `docs/features/`, `docs/architecture/api/commands.md` - 23 files under `docs/`, plus 4 under `website/`, 3 under `ai/` and 10 under `plan/`: 39 `.md` in total
- `scripts/dev/` (4 `.py`), `test/scripts/ze_api*.py` (2), `website/data/command-equivalents.json`, and 4 golden files `internal/component/web/testdata/golden/component/tool_overlay--*.html`
- `docs/architecture/web-interface.md` - the design doc the changed `internal/component/web/` files name in their `// Design:` headers; the summary page moves to the new command
- `demos/terminal/zefs-config/{demo.tape,transcript.txt,validate.sh}` - the demonstration and its narration

## Files to Create
- `test/ui/show-bgp-alias-summary.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
- `test/plugin/show-bgp-summary-is-gone.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
- `test/ui/show-bgp-children-do-not-inherit.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `container summary` is removed from `ze-peer-cmd.yang`, then `make generate` |
| YANG validation constraints | N-A | No leaf is added |
| YANG custom validators | N-A | No leaf is added |
| CLI commands/flags | Yes | A command is REMOVED; the views move to pipe operators |
| CLI grammar (keyword before value) | Yes | `show bgp ipv4` keeps keyword-before-value; the family stays an argument |
| Editor autocomplete | Yes | `show bgp summary` must stop completing; `show bgp` completes and its aliases complete after the pipe |
| Functional test for new RPC/API | Yes | The three `.ci` files above |
| Pipe completeness | Yes | Every operator must work on `show bgp`, which AC-2, AC-3 and AC-7 exercise |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new dependency |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/formatting.md`, and every guide showing the old command |
| 2 | Config syntax changed? | No | No config surface changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, a REMOVAL |
| 4 | API/RPC added/changed? | Yes | `ze-bgp:summary` is retired; `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No plugin registration moves |
| 6 | Has a user guide page? | Yes | 12 files under `docs/guide/` |
| 7 | Wire format changed? | N-A | Scope is cli |
| 8 | Plugin SDK/protocol changed? | Yes | A wire method is retired; check `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is cli |
| 10 | Test infrastructure changed? | No | Existing runners |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it names the command |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `make ze-command-list` and the inventory docs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `summary.go` and `peer.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | 16 doc files carry the command string |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the registrations move and the children are blocked, BEFORE the command is removed
   - Tests: `TestShowBgpCarriesTheSummaryOrder`, `TestChildCommandsDoNotInheritTheSummaryOrder`
   - Files: `internal/component/bgp/plugins/cmd/peer/peer.go`
   - Verify: `show bgp` renders in the declared order and accepts the aliases while `show bgp summary` still exists. Nothing is removed yet, so nothing can regress
   - The blocking empties cover ALL 16 builtin child paths listed in Blast Radius, not the four R-2 first named, and the column list and the alias list differ: `show bgp peer list` takes an empty ALIAS registration only, because it already declares its own `ColumnOrder`
2. **Phase: The consumer sweep** -- every caller moves while the old command still answers
   - Tests: the web, lg, cli and plugin suites
   - Files: **151 files, 382 occurrences** (A-2 is broken; the counts are in the audit note under Assumptions). 24 production `.go`, 29 test `.go`, 40 `.ci`, 39 `.md`, 6 `.py`, 4 `.yang`, 4 `.html` golden files, 2 `.sh`, 1 each of `.txt`, `.tape`, `.json`. `handler_ui.go` carries 3 query sites and `handler_api.go` 2, so the looking glass alone is 5 <!-- doc-links: ignore (a REST route prefix, not a path in the tree) -->
   - Verify: re-grep rather than trust the spec's list. Every consumer works against `show bgp` with the old command still present, so a miss is visible before the removal makes it fatal
3. **Phase: The removal**
   - Tests: `TestShowBgpSummaryIsNotRegistered`, `test/plugin/show-bgp-summary-is-gone.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
   - Files: `ze-peer-cmd.yang`, `summary.go`, then `make generate`
   - Verify: AC-4, AC-6, AC-9, AC-10
4. **Phase: Docs and the demonstration**
   - Files: 16 doc files, the demo's three files
   - Verify: a re-render is owed and is run once, after the tree settles
5. **Phase: The guard still holds**
   - Tests: `test/plugin/show-bgp-child-not-swallowed.ci`, unchanged
   - Verify: AC-6 and A-3. Removing the `summary` key must not change how the four plugin subtrees resolve

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has an implementation at file:symbol |
| Feature completeness | Every outward-facing surface moved: looking glass, web, dashboard, MCP, REST |
| Correctness | The child paths are blocked per path, not for one example; `show bgp nonsense` still answers unknown-command |
| Naming | No survivor of the string `show bgp summary` outside git history |
| Data flow | The payload is untouched. This spec moves a command name, never a field |
| Rule: `ai/rules/no-layering.md` | The command is DELETED, not aliased and kept |
| Rule: `ai/rules/cli.md` | A removed command is an agent-facing break, so in-tree callers move in the same change |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The command is gone | `make ze-command-list` does not name it |
| The wire method is retired | `grep -rn 'ze-bgp:summary' internal/` returns nothing |
| Nothing still asks for it | `grep -rn 'show bgp summary' internal/ cmd/ test/ docs/ demos/` returns nothing |
| The children did not inherit | `TestChildCommandsDoNotInheritTheSummaryOrder` passes |
| The guard still holds | `test/plugin/show-bgp-child-not-swallowed.ci` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `handleBgpOverview` bounds the token it echoes for an unknown subcommand. Confirm that bound survives, since the token is operator input reaching the response envelope |
| Authorization | ANSWERED, 2026-08-21. Both keyings agree. `IsReadOnlyPath` (`internal/component/plugin/server/command.go:123-137`) cuts at the first space and switches on the verb alone, so both paths give `show` and the class cannot change. `Entry.matches` (`internal/component/authz/authz.go:78-100`) is prefix-with-word-boundary, so a rule keyed on `show` or `show bgp` matches the new path exactly as it matched the old one. One asymmetry survives and is a RELEASE-NOTE item, not a code change: an operator-written DENY rule keyed on the literal `show bgp summary` stops matching, because `show bgp` is shorter than the match string, which widens that operator's read surface by the summary payload. No shipped default profile keys on the literal; in-tree it appears only in `internal/component/authz/authz_test.go:854,891,920,934` |
| Information leakage | The payload is unchanged, so no field becomes newly visible |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A surface answers "unknown command" | A consumer was missed in phase 2. Re-grep; do not restore the command |
| `show bgp rib` gains peer columns or accepts an alias | The blocking empty registration is missing for that path |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Phase ordering is the safety here, as it was for the dispatcher guard. The registrations move and the consumers move while the old command still answers, so every miss is visible before the removal makes it fatal.
- Removing the command is the cheaper fix than teaching the registry exact-path matching, and it leaves one spelling of one answer rather than two.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Remove `show bgp summary` | Add exact-path registration to `commandRegistry` so `show bgp` gets its own orders | Exact-path matching would leave two commands answering identically, which is the confusion rather than the fix. Removing one leaves a single spelling |
| Block children with EMPTY registrations | Add exact-path matching to the registry | The empty registration is the mechanism `registerPipeFilters` already uses for the scalar rib commands, documented in its own comment. Reusing it adds no machinery |
| `handleBgpSummary` survives as the implementation | Rename it into `handleBgpOverview` | The two do different jobs: the overview validates the argument and refuses an unknown subcommand, and the summary builds the payload. Merging them would put a dispatch concern inside a payload builder |

## Known Limitations
- An operator with muscle memory for `show bgp summary` gets an unknown-command error rather than a hint. Teaching the CLI to suggest `show bgp` for a retired command is a general facility and is not built here.
- Only the BGP tree is treated. Other parent containers that answer nothing keep answering nothing.
- The demonstration must be re-rendered after this lands, and that is a separate manual step.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
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
