# Spec: cli-dispatch-child-guard

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`show bgp` runs nothing. The YANG container carries no `ze:command`, so an
operator who types the object without a verb gets an unknown-command error while
`show ospf`, `show ntp`, `show vrrp` and `show l2tp` all answer.

Giving it one is a single YANG line, and it would break four subtrees.

The daemon dispatcher matches the LONGEST registered key and hands the leftover
tokens to that command as arguments. `updateSortedKeys` sorts the keys by length
descending and `matchBuiltinTokens` returns the first prefix match
(`internal/component/plugin/server/command.go`). `Dispatch` reaches the plugin
registry only when that returns nothing. Its own comment says "No builtin match:
the command is a forked subsystem's, a plugin's, or nobody's."

Four `show bgp` subtrees are plugin-registered names rather than builtin keys,
so they live behind that fallback:

| Path | Registered in |
|------|---------------|
| `show bgp rpki status/cache/roa/summary/aspa` | `internal/component/bgp/plugins/rpki/rpki.go` |
| `show bgp rs status`, `show bgp rs peers` | `internal/component/bgp/plugins/rs/server.go` |
| `show bgp adj-rib-in`, `show bgp adj-rib-in status` | `internal/component/bgp/plugins/adj_rib_in/rib.go` |
| `show bgp healthcheck` | `internal/component/bgp/plugins/healthcheck/healthcheck.go` |

Register `show bgp` as a builtin and it becomes the longest match for every one
of them. `show bgp rpki status` would reach `handleBgpSummary` with the
arguments `["rpki", "status"]` and die in `validateFamilyArg`.

**The client side already solved this.** `LookupLocal`
(`internal/component/command/registry/registry.go`) takes its longest-prefix
match and then refuses it when any LONGER prefix is a declared command. Its
comment cites `ai/rules/evidence.md` on failing closed. The daemon dispatcher
has no equivalent. That asymmetry is the defect, and it is what this spec fixes.
`show bgp` is the first caller to need it. Every future parent container needs
it too.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command dispatch and registration
  → Constraint: dispatcher keys are CLI paths derived from YANG, not wire methods
  → Decision: a plugin-registered command is reachable only through the no-builtin-match fallback

### Rules
- [ ] `ai/rules/evidence.md` - a guard must fail closed
  → Constraint: a guard with no data MUST refuse rather than serve the match it cannot judge, which is what `LookupLocal` does with a nil `declared`
- [ ] `ai/rules/cli.md` - command grammar
  → Constraint: keyword before value; a subcommand is grammar, not a flag

**Key insights:**
- `show ospf` already carries a `ze:command` AND child containers, and `show ospf instance` still resolves correctly. It is safe only because every `show ospf *` subcommand is also a builtin key, so no fallback is involved. That is the difference from `show bgp`.
- The fix is a guard on the daemon match, not a change to any registration. Nothing about how plugins register needs to move.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `updateSortedKeys` sorts dispatcher keys by length descending; `matchBuiltinTokens` walks that order and returns the FIRST key whose tokens prefix-match, with `matchCommandTokens` returning the unconsumed tokens as args; `Dispatch` consults subsystems and the plugin registry only after `matchBuiltinTokens` fails
- [ ] `internal/component/command/registry/registry.go` - `LookupLocal` performs the same longest-prefix match and then REFUSES it when any longer prefix satisfies its `declared` callback; a nil callback makes every match unprovable and none is served
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - registers `show bgp rpki *` through `sdk.Registration{Commands: []sdk.CommandDecl{...}}`, so these are plugin names and never dispatcher keys
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - the same pattern for `show bgp adj-rib-in`
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container bgp` carries `config false` and a description, and no `ze:command`. Its child `summary` carries `ze:command "ze-bgp:summary"`, and `peer list` carries `ze-bgp:peer-list`
- [ ] `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - `container ospf` carries `ze:command "ze-show:ospf"` AND child containers, the shape this spec makes safe for `show bgp`
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` validates its first argument as an address family through `validateFamilyArg`, so a swallowed subcommand arrives as a rejected family rather than as a clear error

**Behavior to preserve:**
- Every `show bgp <subcommand>` keeps resolving to the handler it resolves to today, builtin or plugin.
- `show ospf`, `show ntp`, `show vrrp`, `show l2tp` and every other parent container that already carries a command keep working.
- Argument passing is unchanged for a command whose leftover tokens are genuinely arguments, such as `show bgp summary ipv4`.
- Plugin registration is untouched. No plugin moves to a builtin registration.

**Behavior to change:**
- `matchBuiltinTokens` refuses a match when a LONGER command path exists and would consume more of the input. That longer path may be a builtin key, a plugin-registered name, or a subsystem handler.
- `container bgp` gains `ze:command "ze-bgp:summary"`, so bare `show bgp` answers the summary.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp rpki status` into the SSH CLI, the web CLI, or `ze cli -c`. The string reaches `Dispatch` (`internal/component/plugin/server/command.go`) after `tokenize`.

### Transformation Path
1. `Dispatch` calls `matchBuiltinTokens` with the tokens.
2. `matchBuiltinTokens` walks `sortedKeys`, longest first, and finds `show bgp` matches.
3. **New:** before returning, it asks whether any longer registered path also matches. `show bgp rpki status` does, in the plugin registry, so the builtin match is refused.
4. `Dispatch` falls through to the subsystem and plugin resolution it already has, and the rpki plugin answers.
5. For `show bgp` with no further tokens, no longer path matches, so the builtin answers and the operator gets the summary.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher ↔ plugin registry | The guard must consult the plugin names, not only the builtin keys, or it cannot see the four subtrees it exists to protect | Yes. `CommandRegistry.hasCommandPath` and `SubsystemManager.hasCommandPath`, proven by `TestShowBgpDoesNotSwallowPluginSubcommands` and `test/plugin/show-bgp-child-not-swallowed.ci` |
| YANG ↔ dispatcher | `container bgp` gains a `ze:command`, which `WireMethodToPaths` turns into a new dispatcher key | Yes. `make ze-command-list` prints `\| show \| show bgp \| ze-bgp:overview \| builtin \|` |

### Integration Points
- `internal/component/plugin/server/command.go` - `matchBuiltinTokens` gains the guard
- `internal/component/command/registry/registry.go` - `LookupLocal` is the reference implementation and the naming to follow
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - the one-line YANG change that motivates the guard

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `Dispatch` keeps its order. The guard only makes `matchBuiltinTokens` decline, and the plugin and subsystem fallbacks below it are unchanged |
| No unintended coupling (components stay isolated) | Yes | `isCommandPath` asks each registry the dispatcher already owns. No plugin is named anywhere in the dispatcher |
| No duplicated functionality (extends existing, does not recreate) | Yes | One guard in `matchBuiltinTokens`. `LookupLocal` stays the client's own, because the two read different registries |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The path is built once in a `textbuf.Buffer` and the builtin lookup indexes the map with `string(b)`, which allocates nothing. `declaresCommand` reads the subsystem slice under its lock rather than copying it through `Commands()` |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `show bgp` arrives through YANG and `RegisterRPCs`, the route every other command takes. No registration moved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The guard can see plugin-registered names at match time | `Dispatch` already resolves them through `matchPluginCommand` and the subsystem registry after the builtin miss | The guard protects builtin children only, and the four subtrees still break | `TestShowBgpDoesNotSwallowPluginSubcommands` | confirmed. `isCommandPath` reads all three registries, and the test fails with its plugin and subsystem branches removed |
| A-2 | No command relies on a parent swallowing tokens a longer path would claim | Argument-taking commands take VALUES, not registered command paths | A command that took an argument colliding with a registered path stops working | Enumerate every dispatcher key that takes args and check none is a prefix of another registered path | broken as stated, and the harm it feared does not follow. NINE argument-taking keys ARE strict prefixes of a longer path, `show route` before `show route lookup` among them. `TestNoArgTakingKeyIsAPrefixOfAnotherPath` asserts the property that matters instead: for each collision the guarded match and the unguarded match agree once the leftover token names no command |
| A-3 | `show bgp` with a `ze:command` and children resolves like `show ospf` does | `mergeYANGEntry` sets `WireMethod` and recurses into children unconditionally | `show bgp summary` stops resolving to its own handler | `TestShowBgpSummaryStillResolvesToItsOwnHandler` | confirmed, over the real YANG tree |
| A-4 | The guard costs nothing measurable on the dispatch path | The check runs once per command invocation, over a key set of a few hundred | Interactive latency regresses | A benchmark over `matchBuiltinTokens` before and after | confirmed. `BenchmarkMatchBuiltinTokens/guard/*` gives 3.2 ns and 0 allocs when the match consumed every token, and 333 ns and 3 allocs with one token left over, against 87 to 117 microseconds for the walk itself |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The guard is too eager and refuses a legitimate parent match whose leftover tokens are arguments | `show bgp summary ipv4` stops working because `ipv4` looks like a longer path | The guard tests for a registered PATH, never for the presence of leftover tokens. AC-4 pins the argument case |
| R-2 | The guard sees builtin keys only, and the four plugin subtrees still break | `show bgp rpki status` returns a family-validation error | A-1's test drives a real plugin subcommand, not a synthetic builtin |
| R-3 | Another parent container elsewhere in the tree already relies on swallowing | An unrelated command regresses | A-2's enumeration runs across every registered key, not only `show bgp` |
| R-4 | `show bgp` answering the summary surprises somebody who expected the config subtree view | The config editor's `show bgp` renders differently from the operational one | They are different surfaces already: the editor's `show bgp` is `(*Model).cmdShow`. This spec changes only the operational dispatcher |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Command routing for every command in the daemon. This is the hottest shared path in the CLI, so the guard must be conservative: when in doubt it must serve the match it serves today |
| How is it reverted? | Single commit revert. The guard and the YANG line land together |
| Who else touches this path? | Every plugin that registers a command; `spec-cli-pipe-aliases` resolves aliases before dispatch and is unaffected |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator runs `show bgp rpki status` after `show bgp` becomes a command | → | the guard in `matchBuiltinTokens` refuses the parent match | `test/plugin/show-bgp-child-not-swallowed.ci` |
| Operator runs bare `show bgp` | → | `container bgp`'s `ze:command` resolves to `handleBgpSummary` | `test/plugin/show-bgp-bare-runs-summary.ci` |
| Operator runs `show bgp summary ipv4` | → | leftover tokens still reach the handler as arguments | `test/plugin/show-bgp-summary-family-arg.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp` with no further tokens | The BGP summary, the same answer `show bgp summary` gives |
| AC-2 | `show bgp rpki status`, `show bgp rs peers`, `show bgp adj-rib-in`, `show bgp healthcheck` | Each reaches its own plugin handler, unchanged. None reaches `handleBgpSummary` |
| AC-3 | `show bgp summary` | Resolves to the summary container's own handler, not to `show bgp` with `summary` as an argument |
| AC-4 | `show bgp summary ipv4` | `ipv4` still arrives as an argument. The guard does not refuse a match merely because tokens are left over |
| AC-5 | `show ospf instance`, `show ntp status` and the other parents that already carry a command | Unchanged |
| AC-6 | A command path that exists nowhere, such as `show bgp nonsense` | A clear unknown-command error, not a family-validation error from a swallowed match |
| AC-7 | Every registered dispatcher key enumerated | No key that takes arguments is a strict prefix of another registered path, or the collision is recorded and handled |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types `show bgp` expecting an overview, as they would `show ospf` | tokenize → `matchBuiltinTokens` → `handleBgpSummary` | `test/plugin/show-bgp-bare-runs-summary.ci` |
| 2 | Types `show bgp rpki status` and still gets RPKI state | tokenize → guard refuses the parent → plugin fallback → rpki plugin | `test/plugin/show-bgp-child-not-swallowed.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMatchBuiltinRefusesWhenLongerPathMatches` | `internal/component/plugin/server/command_test.go` | The guard itself, in isolation | |
| `TestMatchBuiltinServesWhenNoLongerPathMatches` | `internal/component/plugin/server/command_test.go` | The guard is not over-eager (R-1) | |
| `TestMatchBuiltinKeepsArgumentsForLeftoverValues` | `internal/component/plugin/server/command_test.go` | AC-4 | |
| `TestShowBgpDoesNotSwallowPluginSubcommands` | `internal/component/plugin/server/command_test.go` | AC-2, A-1: the guard sees PLUGIN names, not only builtin keys | |
| `TestShowBgpSummaryStillResolvesToItsOwnHandler` | `internal/component/plugin/server/command_test.go` | AC-3, A-3 | |
| `TestNoArgTakingKeyIsAPrefixOfAnotherPath` | `internal/component/plugin/server/command_test.go` | AC-7, A-2, across every registered key | |
| `BenchmarkMatchBuiltinTokens` | `internal/component/plugin/server/command_test.go` | A-4, the guard's cost on the dispatch path | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tokens consumed by the matched key | 1 to the token count | equal to the token count, an exact match | 0, which is no match at all | more than the input, impossible by construction |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-bare-runs-summary` | `test/plugin/show-bgp-bare-runs-summary.ci` | An operator types `show bgp` and gets the summary | |
| `show-bgp-child-not-swallowed` | `test/plugin/show-bgp-child-not-swallowed.ci` | Every one of the four plugin subtrees still answers | |
| `show-bgp-summary-family-arg` | `test/plugin/show-bgp-summary-family-arg.ci` | `show bgp summary ipv4` still filters by family | |

### Interop Tests (Scope: protocol)
Not applicable. Scope is `cli`; no wire-visible behavior changes.

## Files to Modify
- `internal/component/plugin/server/command.go` - `matchBuiltinTokens` gains the longer-path guard, named and documented after `LookupLocal`, with `longerCommandPath` and `isCommandPath` beside it
- `internal/component/plugin/server/command_registry.go` - `hasCommandPath`, the plugin half of the guard's lookup, which logs no deprecation warning
- `internal/component/plugin/server/subsystem.go` - `hasCommandPath` and `declaresCommand`, the subsystem half, comparing exactly where `FindHandler` compares a prefix
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container bgp` gains `ze:command "ze-bgp:overview"`
- `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpOverview` and `isFamilyArg`, registered on `ze-bgp:overview`
- `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang`, `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang`, `internal/component/bgp/cli/yang/ze-bgp-tools-cmd.yang` - the same `show/bgp` container description, because the merge warns when two modules describe one node differently
- `docs/architecture/api/commands.md` - records that the daemon match is guarded, and why the client and daemon rules now agree
- `docs/guide/command-reference.md` - `show bgp` is a command

## Files to Create
- `test/plugin/show-bgp-bare-runs-summary.ci`
- `test/plugin/show-bgp-child-not-swallowed.ci`
- `test/plugin/show-bgp-summary-family-arg.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` gains one `ze:command` on an existing container |
| YANG validation constraints | N-A | No leaf is added |
| YANG custom validators | N-A | No leaf is added |
| CLI commands/flags | Yes | `show bgp` becomes a command |
| CLI grammar (keyword before value) | Yes | Unchanged. The guard protects the grammar rather than altering it |
| Editor autocomplete | Yes | `show bgp` becomes completable as a terminal command as well as a prefix |
| Functional test for new RPC/API | Yes | The three `.ci` files above |
| Pipe completeness | Yes | `show bgp` answers structured data through the same handler, so every operator already works on it |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new dependency |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/command-reference.md` |
| 2 | Config syntax changed? | No | No config surface changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | `ze-bgp:summary` is unchanged; only a second path reaches it |
| 5 | Plugin added/changed? | No | No plugin registration moves |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md` |
| 7 | Wire format changed? | N-A | Scope is cli |
| 8 | Plugin SDK/protocol changed? | No | Registration is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is cli |
| 10 | Test infrastructure changed? | No | New tests use the existing runner |
| 11 | Affects daemon comparison? | No | No feature-parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` records the guard |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `make ze-command-list` gains `show bgp`; check the inventory docs |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `command.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any doc saying `show bgp` is not a command must change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the guard exists and refuses the swallow, BEFORE the YANG changes
   - Tests: `TestMatchBuiltinRefusesWhenLongerPathMatches`, `TestMatchBuiltinServesWhenNoLongerPathMatches`, `TestMatchBuiltinKeepsArgumentsForLeftoverValues`
   - Files: `internal/component/plugin/server/command.go`
   - Verify: the guard is in place while `show bgp` is still NOT a command, so nothing can regress yet. This ordering is deliberate: the guard must be proven before the change that needs it
2. **Phase: The guard sees plugin names** -- not only builtin keys
   - Tests: `TestShowBgpDoesNotSwallowPluginSubcommands`
   - Files: `internal/component/plugin/server/command.go`
   - Verify: A-1. A guard that consults only `sortedKeys` passes phase 1 and fails here, which is the point of splitting them
3. **Phase: `show bgp` becomes a command**
   - Tests: `TestShowBgpSummaryStillResolvesToItsOwnHandler`, all three `.ci`
   - Files: `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`, then `make generate`
   - Verify: AC-1 through AC-6
4. **Phase: The collision sweep**
   - Tests: `TestNoArgTakingKeyIsAPrefixOfAnotherPath`, `BenchmarkMatchBuiltinTokens`
   - Files: `internal/component/plugin/server/command_test.go`
   - Verify: AC-7 and A-2 across every registered key, and A-4's cost
5. **Phase: Docs**
   - Files: every doc row answered Yes above
   - Verify: no doc still says `show bgp` is not a command

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at file:symbol |
| Feature completeness | All four plugin subtrees exercised by a real functional test, not a synthetic key |
| Correctness | The guard tests for a registered PATH, never for leftover tokens, and it fails closed on missing data as `LookupLocal` does |
| Naming | The guard is named after what it prevents, and its comment points at `LookupLocal` as the sibling rule |
| Data flow | No plugin registration moves. The guard is the only behavioural change besides the YANG line |
| Rule: `ai/rules/evidence.md` | A guard with no data refuses rather than serving a match it cannot judge |
| Rule: `ai/rules/simplicity.md` | One guard in one function. No registration rework, no second dispatch path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The guard exists | `grep -n 'func (d \*Dispatcher) matchBuiltinTokens' -A 20 internal/component/plugin/server/command.go` shows the longer-path check |
| `show bgp` is a command | `make ze-command-list` names it |
| The four subtrees still answer | `test/plugin/show-bgp-child-not-swallowed.ci` passes |
| No other parent regressed | `TestNoArgTakingKeyIsAPrefixOfAnotherPath` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Command text is operator input, already tokenized. The guard only compares it against registered paths and never widens what a caller may reach |
| Authorization | `Dispatch` authorizes AFTER resolution. Confirm the guard cannot route a command to a handler with a different authorization class than it reaches today |
| Resource exhaustion | The guard runs per dispatch over the key set. Confirm it is not quadratic in the number of registered commands |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A plugin subcommand returns a family-validation error | The guard does not see plugin names. Fix the guard, never the handler |
| An unrelated command regresses | The guard is too eager. Narrow it to a registered-path test; do not special-case the command |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The client and the daemon implement the same longest-prefix lookup and only one of them guards it. Writing the guard twice was never the risk. The risk was that nobody noticed only one existed.
- Phase ordering carries the proof. The guard lands while `show bgp` is still not a command, so phase 1 cannot regress anything, and phase 2 fails for a guard that reads only the builtin keys.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Guard the daemon match | Register the four plugin subtrees as builtins, the OSPF shape | The narrow fix leaves the trap armed for the next parent container anyone adds. The guard closes it once |
| The guard tests for a registered PATH | Refuse any match that leaves tokens over | Leftover tokens are how every argument-taking command works. That rule would break `show bgp summary ipv4` |
| The guard consults plugin names too | Guard over `sortedKeys` alone | The four subtrees this exists to protect are plugin-registered. A builtin-only guard passes its unit test and fails in production |
| `show bgp` answers the summary | Leave it an error, or list the subcommands | Every sibling object command answers with an overview. An operator typing the object expects what `show ospf` gives them |
| `container bgp` carries `ze:command "ze-bgp:overview"`, a second wire method, rather than a second path onto `ze-bgp:summary` | The one YANG line this spec's task names, binding the container to `ze-bgp:summary` | AC-6 requires `show bgp nonsense` to come back as an unknown command. One handler serving both paths cannot tell which path it was reached by, so under the single-method design `nonsense` would be reported as an invalid address family, which AC-6 names as the wrong answer. `handleBgpOverview` is 12 lines, delegates to `handleBgpSummary`, and leaves `show bgp summary` byte-identical. An AC outranks the spelling in Files to Modify |

## Known Limitations
- Only `show bgp` gains a command here. Other parent containers that answer nothing keep answering nothing. The guard makes giving them one safe, but each is a separate judgement about what the overview should be.
- The guard makes the daemon agree with `LookupLocal`. It does not merge the two implementations, which stay separate because they read different registries.
- `show bgp` in the config editor is a different surface (`(*Model).cmdShow`) and is untouched.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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
