# Spec: cli-show-bgp-is-the-command

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/cli-show-bgp-is-the-command.md` |
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
| CLI ↔ looking glass, web, MCP, REST | Each issues the command string; every one moves from `show bgp summary` to `show bgp` | Yes. `make ze-functional-ui-test` 182/182 and `make ze-functional-plugin-test` 625/625 at the Review Gate, with `grep -rn 'show bgp summary'` leaving no live caller |
| Registry ↔ child commands | Empty registrations block the orders and aliases leaking down the subtree | Yes, after the Review Gate corrected the population from 15 leaves to 10 branch roots. `TestChildCommandsDoNotInheritTheSummaryOrder` and `test/ui/show-bgp-children-do-not-inherit.ci`, both proven to go red when a branch is dropped |

### Integration Points
- `internal/component/bgp/plugins/cmd/peer/peer.go` - the registrations move to `show bgp` and gain the blocking empties
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the `ze-bgp:summary` registration goes; `handleBgpSummary` stays as the implementation behind `handleBgpOverview`
- `internal/component/lg/handler_api.go` - the query string moves

### Architectural Verification
<!-- Filled at the Review Gate, 2026-08-21, by the independent reviewer. -->

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every consumer still issues a command string over the same dispatch channel; `handleBgpOverview` -> `handleBgpSummary` is the one path to the payload, and `handleBgpSummary` is registered against no wire method of its own |
| No unintended coupling (components stay isolated) | Partly | `cmdBgpChildren` (`internal/component/bgp/plugins/cmd/peer/peer.go`) spells four other plugins' command roots and two CLI-local ones. The file already spelled `show bgp rib`, `show bgp irr` and `show bgp health` before this spec. The alternative is each plugin registering its own emptiness, which distributes the knowledge correctly and costs four more packages; not taken here, recorded as Review Gate NOTE-5 |
| No duplicated functionality (extends existing, does not recreate) | Yes | The empty registration is the mechanism `registerPipeFilters` (`internal/component/bgp/plugins/cmd/rib/rib.go`) already uses. No registry gained an exact-path mode |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire or buffer code changes. The payload builder is untouched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `internal/component/command` gained nothing. Every change is a call to the existing `RegisterColumns` / `RegisterAliases` from the owning plugin's `init` |

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

**The empties are not one uniform loop.** `show bgp peer list` already carries its own `ColumnOrder` (`internal/component/bgp/plugins/cmd/peer/peer.go`, `registerColumns`), so an empty COLUMN registration there would destroy a declared order. It carries no alias registration, so it does need an empty ALIAS registration. Column empties and alias empties are two different lists.

**Both notes above are SUPERSEDED. Corrected at the Review Gate, 2026-08-21: the population is 10 BRANCHES, not 15 leaves, and it is not derivable from `make ze-command-list`.**
Registering per leaf was wrong twice over, and both holes shipped in phase 1
(Review Gate BLOCKER-1 and BLOCKER-2):

- `make ze-command-list` is not the population. `collect` (`scripts/inventory/commands.go`) walks `AllBuiltinRPCs` and the streaming prefixes only, so it reports neither the ten plugin commands the rpki, rs, adj-rib-in and healthcheck plugins register as plugin NAMES, nor `show bgp decode` and `show bgp encode`, which `registry.MustRegisterLocal` owns.
- A LEAF path does not prefix its own typed spelling when a selector sits in the middle of it. `commandRegistry.lookup` resolves the string the operator typed, and `matchCommandTokens` (`internal/component/plugin/server/command.go`) folds `192.0.2.1` out of `show bgp peer 192.0.2.1 detail` only afterwards. `commandMatchesPrefix("show bgp peer 192.0.2.1 detail", "show bgp peer detail")` is false, so every selector spelling under `show bgp peer` kept resolving `show bgp`.

Registering at the SHALLOWEST path of each branch answers both: `show bgp peer`
prefixes `show bgp peer 192.0.2.1 detail` and `show bgp peer list` alike, and
`show bgp rpki` covers five plugin commands and any sixth added later. The
population is `show bgp adj-rib-in`, `decode`, `encode`, `health`,
`healthcheck`, `irr`, `peer`, `rib`, `rpki`, `rs`. `show bgp peer list` keeps
its own `ColumnOrder`, because it declares it on a path LONGER than the branch
root and the longest registered prefix resolves.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator runs `show bgp \| summary` | → | alias resolved against `show bgp`, then `handleBgpOverview` | `test/ui/alias-summary.ci` and `test/ui/alias-peers.ci` |
| Operator runs `show bgp summary` | → | no dispatcher key exists; the unknown-command path answers | `test/plugin/show-bgp-summary-is-gone.ci` |
| Operator runs `show bgp peer \<sel\> detail \| peers` | → | the branch's empty registration blocks the inherited alias | `test/ui/show-bgp-children-do-not-inherit.ci` |

**Corrected at the Review Gate, 2026-08-21.** The first row named
`test/ui/show-bgp-alias-summary.ci`, which was never written: `alias-summary.ci`
and `alias-peers.ci` already drove both aliases end to end and phase 2 moved
them onto `show bgp`, so a third fixture would have duplicated them. The third
row named `show bgp rib`, and the path that was actually broken is the SELECTOR
spelling under `show bgp peer` (Review Gate BLOCKER-2), so the fixture drives
that one.

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
| AC-9 | `grep -rn 'show bgp summary'` over the tree | Returns nothing outside the exception list below |

**AC-9's exception list, written at the Review Gate, 2026-08-21.** The criterion
said "nothing outside git history and this spec", which no tree can satisfy: five
kinds of survivor are CORRECT, and phrasing the criterion as an absence left the
implementer judging each hit with no written rule. Four of the five were declared
after the fact and all four hold. The list is what the criterion should have said
from the start:

| Kind | Sites | Correct because |
|------|-------|-----------------|
| An absence assertion | `test/plugin/show-bgp-summary-is-gone.ci`, `internal/component/plugin/server/command_test.go:2065,2067,2075` | A test that proves a name is gone must spell the name |
| A YANG revision history entry | `ze-peer-cmd.yang` revision 2026-06-04 | A revision log records what a past revision did; rewriting it falsifies the changelog |
| A release note | `docs/guide/command-reference.md` | The operator who types the retired spelling is the reader this note exists for |
| Another daemon's command | `docs/comparison.md`, `docs/guide/command-catalogue.md` (VyOS, Junos, Nokia, Arista, FRR columns), `docs/architecture/cli/command-namespacing.md` (JunOS), `website/data/command-equivalents.json` (Junos MX, Cisco IOS XR), `scripts/dev/docker_exec_checked_test.py` (FRR vtysh) | It is FRR's or Junos's command and it still exists there. Changing it makes the document wrong |
| A record of this change | `plan/` (this spec, the deferral shard, `plan/verification-debt/`, `plan/journal/`), `rfc/audit/rfc7606.json` (quotes commit 17f50bd81's subject), and two comments in `internal/component/web/port_check_test.go` that say why four golden fixtures moved | A record describes what happened; it is not a caller |

One kind is a survivor and is NOT correct: six OPEN specs under `plan/` name
`show bgp summary` as a LIVE command in their own acceptance criteria and data
flows (`spec-bgp-per-peer-received-counter`, `spec-bgp-filtered-route-storage`,
`spec-lg-deferred-birdwatcher-route-counts`,
`spec-fixit-redistribute-establishment-stall`,
`plan/future/spec-cli-operator-defined-aliases`,
`plan/future/spec-fixit-migrate-sleeps-infra`). Whoever implements one will write
the retired command into a fixture. Review Gate ISSUE-3 recorded it, and it IS
fixed here: all 15 occurrences across those six specs now name `show bgp`, and
the argument form now reads `show bgp <afi/safi>`. One session owns this
checkout, so "another owner's spec" names nobody who would have fixed it.
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
| `TestChildCommandsDoNotInheritTheSummaryOrder` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-7, A-1, driven per branch, per command beneath it, and per SELECTOR spelling | pass |
| `TestShowBgpSummaryIsNotRegistered` | `internal/component/plugin/server/command_test.go` | AC-4 and AC-10, no dispatcher key and no wire method | pass |
| `TestBgpOverviewAnswersTheSummary/the retired summary subcommand` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-4, the message is the unknown-command error and not the family rejection | pass |
| `TestBgpOverviewAnswersTheSummary/family argument` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-5. This is the row phase 3 wrote instead of a new `TestShowBgpFamilyArgumentStillFilters`: the existing subtest already drove the overview's family path, and it was strengthened to assert the SCOPE (`family`, `peers-in-family`) rather than only the status. A second test asserting the same thing would be a duplicate | pass |
| `TestShowBgpCarriesTheSummaryOrder` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-2 and AC-3, the alias half. Written instead of `TestAliasesResolveAgainstShowBgp` in `alias_test.go`: the registration lives in the peer package, so the assertion belongs beside it, and `internal/component/command` has no BGP registration to resolve. Corrected at the Review Gate, 2026-08-21 | pass |
| `TestShowBgpDoesNotSwallowPluginSubcommands/show bgp rpki summary` | `internal/component/plugin/server/command_test.go` | AC-6 for the one plugin path whose leaf token is the retired word. Added at the Review Gate, 2026-08-21 | pass |

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
   - The blocking empties cover the SHALLOWEST path of every branch under `show bgp`, which is the 10 in the Review Gate correction under Blast Radius, not the four R-2 first named and not the 15 leaves phase 1 shipped. `show bgp peer list` keeps its own `ColumnOrder` because it declares it on a longer path than its branch root
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
| Authorization | ANSWERED, 2026-08-21. Both keyings agree. `IsReadOnlyPath` (`internal/component/plugin/server/command.go:123-137`) cuts at the first space and switches on the verb alone, so both paths give `show` and the class cannot change. `Entry.matches` (`internal/component/authz/authz.go:78-100`) is prefix-with-word-boundary, so a rule keyed on `show` or `show bgp` matches the new path exactly as it matched the old one. One asymmetry survives and is a RELEASE-NOTE item, not a code change: an operator-written DENY rule keyed on the literal `show bgp summary` stops matching, because `show bgp` is shorter than the match string. **CORRECTED at the Review Gate, 2026-08-21: it does NOT widen that operator's read surface, and the claim that it does is false.** `handleBgpOverview` has answered `show bgp` with the identical payload since `647f33121` (2026-08-19, already on `origin/main`), so a DENY on the literal was already failing to hide the summary before this spec removed anything. The widening, if an operator's config predates that commit, belongs to `647f33121`. This spec removes one of two spellings of an already-reachable payload and narrows nothing and widens nothing. The rule that stops matching is still worth a release note, because a rule that silently matches nothing is the operator's problem whichever direction it fails in. No shipped default profile keys on the literal, and phase 2 moved the four `internal/component/authz/authz_test.go` sites, which used the string only as an arbitrary read command |
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
- The demonstration must be re-rendered after this lands, and that is a separate manual step. **It has NO automatic backstop**, corrected at the Review Gate, 2026-08-21: `ze-site-generate` (`Makefile`) runs `render.py --all --stamp-definition-hashes || true` immediately before `render.py --all --check-definition`, and `stamp_definition_hashes` (`demos/terminal/render.py`) writes the very field `verify_assets(definition_only=True)` compares. The check therefore passes unconditionally and the next site build re-records nothing. `render.py --check-definition --demo zefs-config` reports the definition changed today. Journal row: `plan/journal/stale-artifact-reused.md`, 2026-08-21.
- `handleBgpOverview` reads `args[0]` and drops every token after it, so `show bgp ipv4 rubbish` answers the ipv4-scoped summary in silence. It predates this spec (`show bgp summary ipv4 rubbish` behaved the same) and the Boundary Tests row bounds only the case where the FIRST token names no family. Journal row: `plan/journal/silent-fall-through.md`, 2026-08-21.

## Implementation Summary

### What Was Implemented

| Commit | What it did |
|--------|-------------|
| `eb84d7f41` | Phase 1. The column orders and the pipe aliases moved from `show bgp summary` to `show bgp`, with empty registrations blocking the children. `show bgp summary` still answered, so nothing could regress |
| `17f50bd81` | Phase 2. Every in-tree consumer moved to `show bgp` while the old command still answered: the looking glass, the web pages, the CLI dashboard, MCP, REST, the hub session factory, and 40 `.ci` fixtures |
| `6123a6fc7` | Phase 3. The removal. `container summary` left `ze-peer-cmd.yang`, the `ze-bgp:summary` RPC registration and its wire-method snapshot entry went, and `rpc summary` in `ze-bgp-api.yang` became `rpc overview` |
| `83c1cb9a2` | Phase 4. Documents, the command catalogue, the comparison pages and the terminal demonstration |
| `d92d41425` | An RFC 7606 audit re-seal that phase 2 caused: a command name inside one comment of `test/plugin/rfc7606-relay-one-field.ci` staled two file-scoped verdicts, which were re-read rather than re-stamped |
| `94ba0e65a` | The Review Gate's two BLOCKER fixes: the blocking population moved from 15 leaves to 10 branch roots, `test/ui/show-bgp-children-do-not-inherit.ci` was written, and `handleBgpPeerDetail` was unexported |
| `905916157` | Review Gate ISSUE-3: the six open specs that named the retired command as a live command now name `show bgp` |

Phase 5 was verification only. `test/plugin/show-bgp-child-not-swallowed.ci`
passes unedited across all five phases, which is what AC-6 and A-3 ask for.

### Bugs Found/Fixed

- The blocking list missed ten live command paths, because it was built from an inventory that reports one of the three registries the dispatcher resolves from. Covered by `TestChildCommandsDoNotInheritTheSummaryOrder` and `test/ui/show-bgp-children-do-not-inherit.ci`.
- A leaf registration never applied to the spelling an operator types when a selector sits in the middle of the path. `test/ui/show-column-order-absent-unchanged.ci` was red on HEAD for four phases and is green now.
- `rpc summary` in `ze-bgp-api.yang` would have kept the payload schema keyed to a wire method that no longer exists. Covered by `TestDocCommandWithOutputParams`.
- `handleBgpPeerDetail` was exported with no caller outside its package. `make ze-repository-check` is scoped to changed files and had never read it.

### Documentation Updates

- `docs/guide/command-reference.md`: the removal, the release note for an authorization rule keyed on the retired literal, and the branch-root paragraph that replaced a false claim about child aliases.
- `docs/guide/command-catalogue.md`, `docs/comparison.md`, `docs/features/formatting.md`, `docs/architecture/api/commands.md`, `docs/architecture/web-interface.md` and the rest of the 39 markdown files phase 2 and phase 4 moved.
- `website/data/command-equivalents.json` keeps the Junos and Cisco spellings, which are those products' commands and are correct as they stand.
- `make ze-doc-verify` at closure: pass.

### Deviations from Plan

| Deviation | Why |
|-----------|-----|
| A-2 was broken. 151 files and 382 occurrences, not the 131 the spec assumed | The spec's grep covered `internal/ cmd/ test/ docs/ demos/ ai/` and missed `website/`, `scripts/` and `plan/` |
| The blocking population is 10 branch roots, not the 4 R-2 named and not the 15 leaves phase 1 shipped | Review Gate BLOCKER-1 and BLOCKER-2. Both notes under Blast Radius are superseded in place |
| `test/ui/show-bgp-alias-summary.ci` was never created | `test/ui/alias-summary.ci` and `test/ui/alias-peers.ci` already drove both aliases end to end, and phase 2 moved them onto `show bgp`. A third fixture would have duplicated them |
| `test/plugin/show-bgp-summary-family-arg.ci` was renamed `show-bgp-family-arg.ci` | It drove both spellings and only one survives |
| `rpc summary` in `internal/component/bgp/yang/ze-bgp-api.yang` became `rpc overview` | Not in Files to Modify. The parameter schema is keyed by wire method in two readers, and both miss silently |
| `handleBgpPeerDetail` was unexported | Review Gate ISSUE-6. Out of the spec's subject, but the gate red is charged to the tree this closure leaves |
| AC-9 gained an exception list | Review Gate ISSUE-3. An absence criterion is not satisfiable, and the list is what the criterion should have carried from the start |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed 131 files carried the string and that the spec's directory list found them all | 151 files and 382 occurrences, with `website/`, `scripts/` and `plan/` outside the grep | Re-grepped at implementation time rather than trusting the spec | The counts are recorded under Assumptions and the sweep covered all of them |
| approach | Registering the emptiness at each LEAF under `show bgp`, from `make ze-command-list` | The inventory reports one of the three registries, and a leaf path is not a prefix of its own selector spelling | Review Gate BLOCKER-1 measured `AliasesForCommand` and `ColumnsForCommand` per path; BLOCKER-2 ran `make ze-functional-ui-test` | Registration moved to the shallowest path of each branch, which answers both holes with one entry per branch |
| escalation | A verification tool was used to enumerate the population it cannot see | `collect` (`scripts/inventory/commands.go`) walks `AllBuiltinRPCs` and the streaming prefixes only | The same class file already held 54 rows | Routed to `plan/learned/RECURRING-PATTERNS.md`, "An inventory command is not the population it reports on" |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `show bgp summary` stops existing | Done | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`, `container summary` deleted; `summary.go`, the `ze-bgp:summary` registration deleted | `make ze-command-list` names it nowhere |
| `show bgp` is the command, with the declared column order | Done | `peer.go`, `registerColumns` on `cmdBgp` | `TestShowBgpCarriesTheSummaryOrder` |
| The two views are reached by pipe operator | Done | `peer.go`, `registerAliases` on `cmdBgp` | `test/ui/alias-summary.ci`, `test/ui/alias-peers.ci` |
| The ambiguity is removed at its root rather than by exact-path matching | Done | `internal/component/command` gained nothing | Key Design Decisions, row 1 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestShowBgpCarriesTheSummaryOrder`, `test/plugin/show-bgp-bare-runs-summary.ci` | |
| AC-2 | Done | `test/ui/alias-summary.ci` | |
| AC-3 | Done | `test/ui/alias-peers.ci` | |
| AC-4 | Done | `TestShowBgpSummaryIsNotRegistered`, `test/plugin/show-bgp-summary-is-gone.ci` | The unknown-command branch, not the family rejection |
| AC-5 | Done | `TestBgpOverviewAnswersTheSummary/family argument`, `test/plugin/show-bgp-family-arg.ci` | |
| AC-6 | Done | `test/plugin/show-bgp-child-not-swallowed.ci`, unedited across all five phases | `TestShowBgpDoesNotSwallowPluginSubcommands` gained `show bgp rpki summary` |
| AC-7 | Done | `TestChildCommandsDoNotInheritTheSummaryOrder`, `test/ui/show-bgp-children-do-not-inherit.ci`, `test/ui/show-column-order-absent-unchanged.ci` | False as committed in phase 1; true after `94ba0e65a` |
| AC-8 | Done | `make ze-functional-ui-test` 183/183 and `make ze-functional-plugin-test` 625/625 at the Review Gate | |
| AC-9 | Changed | the exception list under Acceptance Criteria | The criterion was unsatisfiable as written and now names the five kinds that stay |
| AC-10 | Done | `make ze-command-list` | 16 lines under `show bgp`, none of them `show bgp summary` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestShowBgpCarriesTheSummaryOrder` | Done | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | Carries the alias half too, instead of a separate `TestAliasesResolveAgainstShowBgp` |
| `TestChildCommandsDoNotInheritTheSummaryOrder` | Done | same file | Drives branches, the commands beneath them, and selector spellings |
| `TestShowBgpSummaryIsNotRegistered` | Done | `internal/component/plugin/server/command_test.go` | |
| `TestBgpOverviewAnswersTheSummary` | Done | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | Both the retired-subcommand and family-argument subtests |
| `TestShowBgpDoesNotSwallowPluginSubcommands` | Done | `internal/component/plugin/server/command_test.go` | Gained `show bgp rpki summary` at the Review Gate |
| `show-bgp-alias-summary` | Changed | not created | `test/ui/alias-summary.ci` and `test/ui/alias-peers.ci` moved onto `show bgp` and already drive it |
| `show-bgp-summary-is-gone` | Done | `test/plugin/show-bgp-summary-is-gone.ci` | |
| `show-bgp-bare-runs-summary` | Done | `test/plugin/show-bgp-bare-runs-summary.ci` | |
| `show-bgp-family-arg` | Done | `test/plugin/show-bgp-family-arg.ci` | Renamed from `show-bgp-summary-family-arg.ci` |
| `show-bgp-child-not-swallowed` | Done | `test/plugin/show-bgp-child-not-swallowed.ci` | Unedited, which is the assertion |
| `show-bgp-children-do-not-inherit` | Done | `test/ui/show-bgp-children-do-not-inherit.ci` | Written at the Review Gate, not by a phase |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The four `container bgp` YANG descriptions | Done | Moved together, byte-identical, as the container-merge gate requires |
| `internal/component/bgp/plugins/cmd/peer/summary.go` | Done | The `ze-bgp:summary` registration is gone and `handleBgpSummary` stays behind `handleBgpOverview` |
| `internal/component/bgp/plugins/cmd/peer/peer.go` | Changed | The registrations moved, and the blocking list is 10 branch roots rather than the 15 leaves the spec named |
| `internal/component/bgp/yang/ze-bgp-api.yang` | Changed | Not in the plan. `rpc summary` became `rpc overview` |
| The looking glass, web, CLI, MCP, REST and hub callers | Done | 24 production `.go` files |
| `test/plugin/`, `test/ui/`, `test/parse/` fixtures | Done | 40 files |
| `docs/`, `website/`, `ai/`, `plan/` prose | Done | 39 markdown files |
| `demos/terminal/zefs-config/` | Partial | The three files moved; the re-render is outstanding and is a Known Limitation with no automatic backstop |

### Audit Summary
- **Total items:** 33 (4 requirements, 10 acceptance criteria, 11 tests, 8 file groups)
- **Done:** 28
- **Partial:** 1 (the demonstration re-render, recorded as a Known Limitation and reported to the owner)
- **Skipped:** 0
- **Changed:** 4 (AC-9's exception list, the branch-root population, the alias fixture that was not needed, the `rpc overview` rename), each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One spelling answers the BGP overview, and it is `show bgp` | functional | `test/plugin/show-bgp-bare-runs-summary.ci` (plugin 605) answers the payload; `test/plugin/show-bgp-summary-is-gone.ci` (plugin 608) proves the other spelling is gone; `make ze-command-list` lists `show bgp` against `ze-bgp:overview` and no `show bgp summary` |
| The operator chooses the view with a pipe rather than a second command | functional | `test/ui/alias-summary.ci` and `test/ui/alias-peers.ci` each run the alias against `show bgp` and compare it to the unpiped answer |
| The aliases and column orders do not leak down the subtree | functional and unit | `test/ui/show-bgp-children-do-not-inherit.ci` drives `show bgp peer list` and three selector spellings with both alias names, with the parent as the control, and fails with "the inherited alias was accepted" when a branch is dropped; `TestChildCommandsDoNotInheritTheSummaryOrder` drives every branch and every command beneath it |
| No subcommand of `show bgp` regressed | functional | `test/plugin/show-bgp-child-not-swallowed.ci` (plugin 606) passes UNEDITED across all five phases; `git log` names only `647f33121` for that file |
| Every outward surface still answers | functional | `make ze-functional-ui-test` 183/183 and `make ze-functional-plugin-test` 625/625 at the Review Gate, over the looking glass, web, dashboard, MCP and REST paths |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| A hint for a retired command, so an operator who types a command that used to exist is told what replaced it | deferred | `plan/future/spec-cli-retired-command-hint.md`, written at this closure. The row named no spec before, which a closing spec cannot leave behind: nothing on disk was going to become the destination |

The shard holds one live row, so it is NOT removed at commit B and it keeps its
source-keyed name. `scripts/dev/deferral_orphans.py` reports it under
"orphaned, live-bearing", which is the correct end state for a shard whose
source spec closed while its row is still outstanding. No foreign shard was
emptied by this closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). Filled by an INDEPENDENT reviewer that did
     not write this code, running every lens itself, over the five committed
     phases and the working tree they left. -->

| Field | Value |
|-------|-------|
| Reviewer | independent context, Opus 5; did not author any of the five phases |
| Diff under review | `eb84d7f41`, `17f50bd81`, `6123a6fc7`, `83c1cb9a2`, `d92d41425`, plus the working tree |
| Reviewer lenses used | wiring + AC-to-test mapping; removed-behavior + logic + registry semantics; security + authorization; docs/record accuracy; simplicity/altitude |
| Rounds | 3 |
| `review_gate.py` artifact | `tmp/review/cli-show-bgp-is-the-command-7e4e9f00-6a89-4b80-b4c1-573d6037cfc6.md`, recorded by the closing session over the 118 code and test files of the seven commits, verdict clean |
| `review_gate.py check` | OK, 118 code files, clean, hashes match |

### Round scopes, written before each round ran

| Round | Scope |
|-------|-------|
| 1 | The whole diff of the five commits, every AC-1..AC-10 verified independently against source and against a run, plus the five items the commissioning thread named (AC-9 survivors, the child blocking, the dispatcher guard, the `rpc overview` rename, `handleBgpSummary`, the authorization asymmetry, the demo re-render) |
| 2 | Only round 1's fixes and what they touch: `cmdBgpChildren` and both registration functions in `internal/component/bgp/plugins/cmd/peer/peer.go`, `TestChildCommandsDoNotInheritTheSummaryOrder`, `TestShowBgpDoesNotSwallowPluginSubcommands`, the new `test/ui/show-bgp-children-do-not-inherit.ci`, the `command-reference.md` paragraph the fix makes true, and the sibling call sites of `RegisterColumns` / `RegisterAliases` (`internal/component/bgp/plugins/cmd/rib/rib.go`) |
| 3 | Only round 2's one fix and what it touches: the `handleBgpPeerDetail` rename, its `RegisterRPCs` entry, its three `peer_ops_test.go` call sites, and the `docs/architecture/behavior/fsm.md` source anchor |

### Round 1 findings

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| BLOCKER-1 | BLOCKER | AC-7 was FALSE for ten live command paths. The blocking list was built from `make ze-command-list`, which walks `AllBuiltinRPCs` and the streaming prefixes only (`collect`, `scripts/inventory/commands.go`), so it reports neither the rpki, rs, adj-rib-in and healthcheck commands (registered as PLUGIN names) nor `show bgp decode` / `show bgp encode` (`registry.MustRegisterLocal`, `internal/component/bgp/cli/register.go`). Measured, not argued: `AliasesForCommand` returned `[peers summary]` and `ColumnsForCommand` returned both summary orders for `show bgp rpki status`, `rpki summary`, `rpki cache`, `rpki roa`, `rpki aspa`, `rs status`, `rs peers`, `adj-rib-in`, `adj-rib-in status`, `healthcheck`. `TreeCompleter.Complete` (`internal/component/command/completer.go`) offers those alias names after `\|` for any command string, so an operator was offered `\| peers` on route and ROA output. Introduced by `eb84d7f41`: the old registration on `show bgp summary` is not a prefix of any of them | `internal/component/bgp/plugins/cmd/peer/peer.go`, `cmdBgpChildren` / `registerColumns` / `registerAliases` | The list now registers the SHALLOWEST path of each branch. `TestChildCommandsDoNotInheritTheSummaryOrder` drives every branch and every command beneath it; removing `show bgp rpki` from the list turns 7 subtests red |
| BLOCKER-2 | BLOCKER | `test/ui/show-column-order-absent-unchanged.ci` was RED on HEAD, and it is the file whose stated purpose is "the prefix lookup is proven not to leak an order down to a command that did not ask for one". `show bgp peer 192.0.2.1 detail` rendered in the summary's peer-row order. Cause: the registry resolves the string the operator TYPED, and `matchCommandTokens` (`internal/component/plugin/server/command.go`) folds the selector out of the middle only afterwards, so `commandMatchesPrefix("show bgp peer 192.0.2.1 detail", "show bgp peer detail")` is false and a per-leaf registration never applies to the typed spelling. Phase 5 was verification only and no phase ran `make ze-functional-ui-test` | `internal/component/bgp/plugins/cmd/peer/peer.go`, `cmdBgpChildren` | Same fix: `show bgp peer` prefixes both spellings. `show-column-order-absent-unchanged` is green; removing `show bgp peer` from the list turns it red again |
| ISSUE-1 | ISSUE | AC-7 had no functional test, and the Wiring Test table named `test/ui/show-bgp-children-do-not-inherit.ci`, which was never written. The unit table drove the registry directly and could not see BLOCKER-2, because it spelled the paths the way the REGISTRATION spells them rather than the way an operator types them | `test/ui/show-bgp-children-do-not-inherit.ci` | Written. It drives `show bgp peer list` and three selector spellings with both alias names, with the parent as the control. Removing `show bgp peer` from the list makes it fail with "the inherited alias was accepted" |
| ISSUE-2 | ISSUE | `docs/guide/command-reference.md` asserted a product property that was false: "Every child path under `show bgp` declares no alias, so none of them offers `\| summary` or `\| peers`" (`ai/rules/evidence.md`, a false safety claim) | `docs/guide/command-reference.md` | The paragraph now states the branch-root mechanism and is true of the code |
| ISSUE-3 | ISSUE | AC-9 is written as an absence ("returns nothing"), which no tree can satisfy, and it left the implementer judging each hit with no written rule. Six OPEN specs under `plan/` still name `show bgp summary` as a LIVE command in their own ACs and data flows, so whoever implements one writes the retired command into a fixture | `plan/spec-bgp-per-peer-received-counter.md`, `plan/spec-bgp-filtered-route-storage.md`, `plan/spec-lg-deferred-birdwatcher-route-counts.md`, `plan/spec-fixit-redistribute-establishment-stall.md`, `plan/future/spec-cli-operator-defined-aliases.md`, `plan/future/spec-fixit-migrate-sleeps-infra.md` | FIXED. AC-9 now carries the exception list it should have had. The reviewer left the six specs alone, which was the right hand for a reviewer; the closing session then corrected all 15 occurrences, because one session owns this checkout and the deferral named nobody |
| ISSUE-4 | ISSUE | The Security Review's authorization row asserted a false security property: that removing the command "widens that operator's read surface by the summary payload". `handleBgpOverview` has answered `show bgp` with the identical payload since `647f33121` (2026-08-19, on `origin/main`), so a DENY keyed on the literal was already failing to hide it. The removal widens nothing | `plan/spec-cli-show-bgp-is-the-command.md`, Security Review Checklist | Corrected in place. The release note in `docs/guide/command-reference.md` is factually right as written ("matches nothing after the removal; key it on `show bgp`") and stays |
| ISSUE-5 | ISSUE | The Known Limitation said the demo re-render was a manual step, and the phase-4 commit said `ze-site-generate` re-records when the check fails. It does not: the recipe runs `render.py --all --stamp-definition-hashes \|\| true` immediately before `--check-definition`, and the stamp rewrites the field the check compares, so the check cannot fail | `Makefile`, `ze-site-generate`; `demos/terminal/render.py`, `stamp_definition_hashes` | The Known Limitation now states the truth. The Makefile ordering is NOT fixed here: `--stamp-definition-hashes` exists to backfill artifacts rendered before the field existed, and removing it re-records every such demo, which is the website owner's cost to accept. Journal row in `plan/journal/stale-artifact-reused.md` |
| NOTE-1 | NOTE | `handleBgpSummary` keeps the `handle` prefix that every OTHER function in `summary.go` carries because it is a registered wire handler. It is now registered against nothing. The doc comment says so in its first paragraph, so a reader is not misled | `internal/component/bgp/plugins/cmd/peer/summary.go` | Not changed. Renaming it would churn a 150-line function's call sites for a naming nuance the comment already answers |
| NOTE-2 | NOTE | `ze-bgp-api.yang` renamed `rpc summary` to `rpc overview` and added no `revision` entry, while the sibling `ze-peer-cmd.yang` recorded one for the same change. `ai/rules/config.md` requires at least one revision and this module has one, so no gate fires | `internal/component/bgp/yang/ze-bgp-api.yang` | Not changed |
| NOTE-3 | NOTE | `python3 scripts/dev/audit-test-relaxation.py origin/main` reports one `[WEAKENED]` finding, an RFC-tagged change in `internal/component/bgp/reactor/peer_initial_sync_test.go`. It belongs to `478dd21a5`, an unrelated commit in the same unpushed range, and no phase of this spec touched that file | `internal/component/bgp/reactor/peer_initial_sync_test.go` | Not this spec's. Reported to the owner: it needs a row in `test/rfc-changed.md`, which only he approves |
| NOTE-4 | NOTE | `handleBgpOverview` drops every token after `args[0]`, so `show bgp ipv4 rubbish` answers the ipv4 summary in silence. Pre-existing: `show bgp summary ipv4 rubbish` behaved the same | `internal/component/bgp/plugins/cmd/peer/summary.go`, `handleBgpOverview` | Not changed. Journal row in `plan/journal/silent-fall-through.md`; recorded as a Known Limitation |
| NOTE-5 | NOTE | The blocking list is still hand-maintained. A new BRANCH under `show bgp` inherits silently until somebody adds it. The branch shape makes this much rarer than the leaf shape it replaces (a new leaf is now covered for free), and no derivation is available at `init` time, when neither the YANG tree nor the plugin registry is populated | `internal/component/bgp/plugins/cmd/peer/peer.go`, `cmdBgpChildren` | Not changed. Journal row in `plan/journal/gate-excludes-part-of-its-population.md` |

### Round 2 findings

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| ISSUE-6 | ISSUE | `make ze-repository-check` went RED once round 1's fix touched `peer.go`. The check is scoped to CHANGED files (`check_cross_package_wiring`, `scripts/dev/validate.py`), so it had never examined this file: `HandleBgpPeerDetail` is exported and has no caller outside its own package. It is a handler value passed to `RegisterRPCs` beside eight lowercase siblings. The gate red is charged to the tree this review leaves, so it is fixed here rather than journalled | `internal/component/bgp/plugins/cmd/peer/peer.go`, `handleBgpPeerDetail` | Unexported, with its three `peer_ops_test.go` call sites and the `docs/architecture/behavior/fsm.md` source anchor. `make ze-repository-check` passes |

The rest of round 2's scope holds. No other BLOCKER and no other ISSUE inside
it, and no always-in-scope finding anywhere.

- `RegisterAliases` with an empty alias slice never reaches `checkedAlias`, so no branch registration can widen the `filterShadowing` panic surface. Read at `internal/component/command/alias.go`, `RegisterAliases`.
- `registerPipeFilters` (`internal/component/bgp/plugins/cmd/rib/rib.go`) is the sibling call site of the same mechanism. It registers no filter named `summary` or `peers`, so `aliasShadowing` finds no collision, and its own empty registrations sit on paths that do not carry a mid-path selector.
- `show bgp peer list` keeps its declared `ColumnOrder` under a `show bgp peer` branch registration, because the longest registered prefix resolves. Asserted in `TestChildCommandsDoNotInheritTheSummaryOrder`.
- Every fix is on a cold path: `init`-time registrations, one `.ci`, one doc paragraph.

### Round 3 findings

Scope as written above. No BLOCKER, no ISSUE and no NOTE inside it, and no
always-in-scope finding anywhere. The rename changes no behavior: the same
function value reaches the same `RegisterRPCs` entry. `make ze-repository-check`
passes, `make ze-lint-changed` reports 0 issues on both passes,
`audit-test-relaxation.py` over the working tree is clean across 3 changed test
files, and the peer, `plugin/server` and `command` packages plus the six `test/ui`
and four `test/plugin` fixtures are green against a rebuilt daemon.

### AC verification, each against source AND a run

| AC | Implementation, file:symbol | Test | Verdict |
|----|-----------------------------|------|---------|
| AC-1 | `internal/component/bgp/plugins/cmd/peer/peer.go`, `registerColumns` on `cmdBgp`; `summary.go`, `handleBgpOverview` -> `handleBgpSummary` | `TestShowBgpCarriesTheSummaryOrder`; `test/plugin/show-bgp-bare-runs-summary.ci` (605, pass); `test/ui/show-bgp-summary-column-order.ci` (pass) | met |
| AC-2 | `peer.go`, `registerAliases`, alias `summary` | `TestShowBgpCarriesTheSummaryOrder`; `test/ui/alias-summary.ci` (pass) | met |
| AC-3 | `peer.go`, `registerAliases`, alias `peers` | `TestShowBgpCarriesTheSummaryOrder`; `test/ui/alias-peers.ci` (pass) | met |
| AC-4 | `summary.go`, `handleBgpOverview` unknown-command branch; `ze-peer-cmd.yang` no longer declares `container summary` | `TestShowBgpSummaryIsNotRegistered`; `TestBgpOverviewAnswersTheSummary/the retired summary subcommand`; `test/plugin/show-bgp-summary-is-gone.ci` (608, pass) | met |
| AC-5 | `summary.go`, `isFamilyArg` then `handleBgpSummary` | `TestBgpOverviewAnswersTheSummary/family argument`; `test/plugin/show-bgp-family-arg.ci` (607, pass) | met |
| AC-6 | `internal/component/plugin/server/command.go`, `matchBuiltinTokens` -> `longerCommandPath` -> `isCommandPath` | `test/plugin/show-bgp-child-not-swallowed.ci` (606, pass, UNEDITED across all five phases: `git log` names only `647f33121`); `TestShowBgpDoesNotSwallowPluginSubcommands`, now including `show bgp rpki summary`; `TestBuiltinPathsResolveToTheirOwnHandler` | met |
| AC-7 | `peer.go`, `cmdBgpChildren` with `registerColumns` and `registerAliases` | `TestChildCommandsDoNotInheritTheSummaryOrder` (branches, commands beneath them, and selector spellings); `test/ui/show-bgp-children-do-not-inherit.ci`; `test/ui/show-column-order-absent-unchanged.ci` | met AFTER BLOCKER-1 and BLOCKER-2; false as committed |
| AC-8 | `internal/component/lg/handler_api.go` and `handler_ui.go` (5 query sites), `internal/component/web/page_bgp_summary.go`, `internal/component/cli/client/main.go`, `internal/component/cli/model_dashboard.go`, `internal/component/mcp/tools.go`, `internal/component/api/rest/server.go` `ConvenienceCommands`, `cmd/ze/hub/session_factory.go` | `make ze-functional-ui-test` 182/182; `make ze-functional-plugin-test` 625/625; `test/ui/lg-peer-table-flat-payload.ci` | met |
| AC-9 | the sweep of 151 files | `grep -rn 'show bgp summary'` over the tree; every survivor classified in the exception list under Acceptance Criteria | met. Five kinds of survivor are correct and listed in the exception list; the six open specs that named it as live were corrected, 15 occurrences |
| AC-10 | `ze-peer-cmd.yang`, `container summary` deleted; `wire-methods.snapshot` entry removed | `make ze-command-list` names `show bgp` (`ze-bgp:overview`) and 15 children, and no `show bgp summary`; `TestShowBgpSummaryIsNotRegistered`; `TestRegisteredWireMethods` | met |

### Gates run at this Review Gate

| Gate | Result |
|------|--------|
| `make ze-repository-check` | pass |
| `python3 scripts/dev/audit-test-relaxation.py` (working tree) | clean, 2 changed test files examined |
| `python3 scripts/dev/audit-test-relaxation.py origin/main` | 1 finding, NOTE-3, owned by `478dd21a5` |
| `make ze-lint-changed` | 0 issues, both host and `GOOS=linux` passes |
| `make ze-rfc-check` | pass, 2966 gated MUST-level requirements. The first run went red on a corrupted go build cache ("package internal/strconv is not in std"), which this machine's near-full disk explains; the re-run is green and nothing in the tree changed between them |
| `make ze-unit-pkg-test` over peer, command, plugin/server, plugin/all, config/yang, config/yang/cli, hub, authz | pass |
| `make ze-functional-ui-test` | 183/183 pass, 10 skipped, the new fixture included |
| `make ze-functional-plugin-test` | 625/625 pass, 56 skipped |
| `make ze-repository-check`, `ze-lint-changed`, both suites, re-run after the round-2 fix | all green over the final tree |
| `python3 demos/terminal/render.py --check-definition --demo zefs-config` | RED, deliberately: the re-render is outstanding (see Known Limitations and ISSUE-5) |

### Final status

- [ ] Round 3 found no BLOCKER and no ISSUE inside its scope, and no always-in-scope finding anywhere. The loop ends here
- [ ] 0 BLOCKER outstanding
- [ ] 0 ISSUE outstanding. ISSUE-3 was the last one open, and the closing session fixed it rather than reporting it: the six specs that named the retired command as live now name `show bgp`
- [ ] NOTE-1..NOTE-5 recorded above, none of them reopening a round

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/show-bgp-summary-is-gone.ci` | Yes | `ls -la`, 4.3K, 2026-08-21 17:12 |
| `test/ui/show-bgp-children-do-not-inherit.ci` | Yes | `ls -la`, 6.1K, 2026-08-21 18:42 |
| `test/ui/show-bgp-alias-summary.ci` | No, and correctly so | `ls` reports "No such file or directory". `test/ui/alias-summary.ci` (5.2K) and `test/ui/alias-peers.ci` (6.4K) cover it; see Deviations |
| `test/plugin/show-bgp-bare-runs-summary.ci` | Yes | `ls -la`, 3.9K |
| `test/plugin/show-bgp-family-arg.ci` | Yes | `ls -la`, 4.4K |
| `test/plugin/show-bgp-child-not-swallowed.ci` | Yes | `ls -la`, 3.9K, dated 2026-08-20, which is before phase 1 and is the point |
| `test/ui/show-column-order-absent-unchanged.ci` | Yes | `ls -la`, 4.9K |
| `test/ui/show-bgp-summary-column-order.ci` | Yes | `ls -la`, 6.9K |
| `test/ui/lg-peer-table-flat-payload.ci` | Yes | `ls -la`, 3.1K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3 | The orders and the aliases resolve against `show bgp` | `make ze-unit-pkg-test PKG=./internal/component/bgp/plugins/cmd/peer RUN=...` re-run at closure: ok, 3.689s. `peer.go:125` reads `command.RegisterColumns([]string{cmdBgp}, ...)` and `cmdBgp = "show bgp"` at `:25` |
| AC-4 | The retired spelling answers an unknown-command error | `TestShowBgpSummaryIsNotRegistered` re-run at closure: ok, 2.153s. `summary.go:184` produces `%q names no subcommand and no address family: %w`, and `show-bgp-summary-is-gone.ci:67` drives both `show bgp summary` and `show bgp summary ipv4` and refuses an answer that omits "unknown command" |
| AC-5 | The family argument still filters | `TestBgpOverviewAnswersTheSummary` in the same closure run: ok |
| AC-6 | Every plugin subcommand resolves to its own handler | `TestShowBgpDoesNotSwallowPluginSubcommands` re-run at closure: ok. `git log --oneline -- test/plugin/show-bgp-child-not-swallowed.ci` names `647f33121` alone |
| AC-7 | The children inherit nothing | `TestChildCommandsDoNotInheritTheSummaryOrder` re-run at closure: ok. `peer.go:66-77` lists the ten branch roots, and the comment at `:41-45` states why the shallowest path is load-bearing |
| AC-8 | Every outward surface moved | `grep -rn 'show bgp summary'` over `internal/ cmd/` at closure returns three sites in `command_test.go`, two comments in `port_check_test.go` and one YANG revision entry, and no caller |
| AC-9 | Every survivor of the string is one of five correct kinds | The closure grep over the whole tree, classified: an absence assertion (2 files), a YANG revision entry (1), a release note and another daemon's commands (5 documents plus `website/data/command-equivalents.json` and `scripts/dev/docker_exec_checked_test.py`), and records of the change (`plan/`, `rfc/audit/rfc7606.json`, two `port_check_test.go` comments). No caller survives |
| AC-10 | The inventory names `show bgp` and not `show bgp summary` | `make ze-command-list` at closure: `show bgp` against `ze-bgp:overview`, 16 lines under the prefix, and `grep -c 'show bgp summary'` over its output returns 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show bgp` with the summary alias | `test/ui/alias-summary.ci` | Yes. Read at closure: it runs `cli('show bgp .. text')` and `cli('show bgp .. summary .. text')` and compares the two answers, so it exercises the alias against the new command path rather than asserting a string |
| `show bgp` with the peers alias | `test/ui/alias-peers.ci` | Yes. Same shape, and it also drives `ze cli -c` with `show bgp .. peers established`, which is the operator's own client |
| `show bgp summary` | `test/plugin/show-bgp-summary-is-gone.ci` | Yes. It loops over the retired spelling with and without a family and refuses any answer whose message lacks "unknown command" or the command it typed |
| `show bgp peer <selector> detail` with an inherited alias | `test/ui/show-bgp-children-do-not-inherit.ci` | Yes. Read at closure: the parent is the control and MUST accept both aliases, then four child spellings, three of them carrying a selector, MUST be refused. The failure text is "the inherited alias was accepted" |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `RegisterColumns` (`internal/component/command/column_order.go`) documents the empty registration, `commandRegistry.register` skips an empty PATH and not an empty VALUE, and `TestChildCommandsDoNotInheritTheSummaryOrder` passes at closure |
| A-2 | broken | 151 files and 382 occurrences, not 131. Recorded in the audit note under Assumptions, in the Mistake Log and in Deviations |
| A-3 | confirmed | `matchBuiltinTokens` calls `longerCommandPath`, which reads the registries the plugin subtrees live in. `test/plugin/show-bgp-child-not-swallowed.ci` is unedited and passes |
| A-4 | confirmed | `grep -rn 'show bgp summary' rfc/` returns only `rfc/audit/rfc7606.json`, which quotes a commit subject. The birdwatcher contract is keyed on route names, not on the ze command string |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/command-reference.md` says child paths do not offer the aliases | The paragraph now states the branch-root mechanism, and `peer.go`, `cmdBgpChildren` is the code it describes | Yes, corrected at the Review Gate as ISSUE-2 |
| `docs/guide/command-reference.md` release note for an authorization rule keyed on the retired literal | `Entry.matches` (`internal/component/authz/authz.go`) is prefix-with-word-boundary, so a rule on the literal stops matching | Yes |
| Another daemon's spelling in `docs/comparison.md`, `docs/guide/command-catalogue.md`, `docs/architecture/cli/command-namespacing.md`, `website/data/command-equivalents.json` | Each is FRR's, Junos's, Nokia's, Arista's or Cisco's command and still exists there | Yes, correct unchanged |
| The API and RPC pages | `ze-bgp:summary` is retired and `ze-bgp:overview` answers; `make ze-command-list` agrees | Yes |
| Every other category | `make ze-doc-verify` at closure: pass | Yes |

## Core Insight

A population you cannot enumerate from the runtime is a population you must not
enumerate at all. The blocking list was correct against the tool that produced
it and false against the daemon, because the tool reads one of three registries
and because a registry keyed on the string an operator TYPES cannot be indexed
by the paths a registration DECLARES. Moving the registration to the shallowest
path of each branch removes the enumeration problem instead of solving it: a
branch root prefixes every spelling below it, the ones nobody has written yet
included.

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
- [ ] Learned summary written to `plan/learned/RECURRING-PATTERNS.md` (four entries), plus a journal row in `plan/journal/zero-value-as-valid-answer.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
