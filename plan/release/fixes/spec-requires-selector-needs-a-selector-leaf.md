# Spec: requires-selector-needs-a-selector-leaf

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped CLI correctness: the daemon accepts a command registration that refuses every operator line, and the CLI rule tells authors to write that registration. It has shipped twice (`announce` 2026-08-31, `request plugin restart` 2026-11-12), and the second one broke a fix line `ze doctor` prints. Each was fixed for its own command only, so the producer of the defect is still in the tree. That is incorrect shipped behaviour (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`RequiresSelector` on an `RPCRegistration` is the dispatcher's fail-closed
guard for peer commands. `Dispatch` (`internal/component/plugin/server/command.go`)
treats it as satisfied only by a value bound to the leaf named `selector`
(the constant `selectorLeaf`), or by a peer the caller supplied out of band
(`ctx.Peer`). A command whose argument leaf has another name and that sets the
flag refuses every operator line with "requires a selector"
(`missingSelectorError`, `internal/component/plugin/server/incomplete.go`)
before its handler runs. Unit tests that call the handler directly never see it.

It has happened twice:

| Date | Command | How it was found | Fixed by |
|------|---------|------------------|----------|
| 2026-08-31 | `announce unicast`, `announce blackhole`, `announce flowspec` (selector leaf lost in a reshape) | A live daemon through a plugin `dispatch-command` | That command only (`plan/journal/guard-demands-what-the-model-cannot-supply.md`, 2026-08-31 row) |
| 2026-11-12 | `request plugin restart <name>` (leaf `name`, flag set from aa6ebb8d15) | `test/plugin/plugin-restart-external-running.ci` through `ze cli` | ce86ac4a64 removed the flag from that command only |

Two causes make it recur, and the work is one fix for each:

1. **Nothing at registration refuses the flag on a command that has no
   `selector` leaf.** `LoadBuiltins` and `loadBuiltinsWithAliases` copy
   `reg.RequiresSelector` and `pathToArgDefs[name]` into `RegisterOptions`, and
   `RegisterWithOptions` stores both on the `Command`. Its only refusal is a
   name another registration already holds (`heldCommandError`). The work: the
   daemon refuses such a registration at startup, the way it already refuses a
   held name or a doubled wire method, with a message naming the command and
   the fix.
2. **The rule tells authors to set the flag.** The point
   `ai/rules/points/cli/cli-grammar-keywords-before-values/acting-on-everything-needs-an-explicit-all.md`
   (rendered into `ai/rules/cli.md`) lists `restart` among the set actions and
   says "The selector is mandatory in both declarations ... the YANG leaf and
   `RequiresSelector`". For a command that addresses a non-peer member by a
   typed keyword, following that sentence produces exactly this defect.
   `docs/architecture/cli/command-verbs.md`, "Action verbs and their selector",
   now says "`RequiresSelector` is the guard of the leaf named `selector` and
   of no other" and "Set on such a command, `RequiresSelector` refuses every
   line". The work: the point says when the flag applies (a command whose
   model carries the `selector` leaf) and what guards a typed-keyword command
   instead (its mandatory leaf, judged by `command.ValidateArgs`). The same
   page's table above that paragraph ("Two declarations make the selector
   mandatory, and both are owed") reads as an obligation on every command and
   is corrected with it.

This is a skeleton: idea capture. The design (where the check sits, what it
reads, its message) is not written here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/command-verbs.md` - "Action verbs and their selector": the two declarations, and the paragraph on what `RequiresSelector` guards
  → Constraint: `RequiresSelector` is the guard of the leaf named `selector` and of no other; a typed-keyword command is guarded by its mandatory leaf
- [ ] `docs/architecture/api/commands.md` - where a builtin command is served and how its YANG path becomes its dispatch key
- [ ] `docs/contributing/rule-authoring.md` - a point is edited, then `./le ai rules update render` renders `ai/rules/cli.md`, and `./le ai rules update condensed` regenerates `TRIGGERS.md` and `CORE.md`
- [ ] `ai/rules/cli.md` - "CLI Grammar: Keywords Before Values"
  → Constraint: a command addressing one member of a set types the selector with `name`, `id`, `index`, ...; `selector` is never an operator keyword; peer commands are the one exception

**Key insights:**
- `plan/learned/DESIGN-HISTORY.md`, Plugin system: "Commands that mutate a peer declare `RequiresSelector`, enforced in the dispatcher before the handler". The design record scopes the flag to peer commands; the rule point widened it to every set action.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `LoadBuiltins`, `loadBuiltinsWithAliases`, `RegisterWithOptions` (no pairing check between the flag and the ArgDefs); `Dispatch` (adopts a lone positional as the selector only when the flag is set, then refuses through `missingSelectorError` unless `selectors[selectorLeaf]` or `ctx.Peer` is set); `adoptablePositional` (binds only `validated.Positional(selectorLeaf)`); `const selectorLeaf = "selector"`
- [ ] `internal/component/plugin/server/incomplete.go` - `missingSelectorError`: the "requires a selector" refusal, which names the selector line from the `selector` ArgDef
- [ ] `internal/component/plugin/server/arg_faults.go` - also branches on `matchedCmd.RequiresSelector` when restating argument faults
- [ ] `internal/component/plugin/server/register_plugin_restart.go` - the `init` comment that records why the flag is absent there since ce86ac4a64
- [ ] `internal/component/command/argbind.go` - `ValidateArgs`, `MissingArgumentError` ("required argument missing: <leaf>"): the mandatory-leaf guard of a typed-keyword command
- [ ] `internal/component/config/yang/command.go` - `PathToArgDefs` calls `BuildCommandTree`, which merges every `-cmd` module (`mergeYANGEntry`) and then runs `inheritArgDefs` once over the root, so a command inherits the `selector` leaf a container in another module declares (`send bgp` from `ze-raw-cmd.yang`, `request peer` from `ze-peer-cmd.yang`)
- [ ] `internal/component/plugin/process/stopped.go` - `RestartCommand`, the doctor fix line `ze request plugin restart <name>`, the command the 2026-11-12 occurrence made unreachable

**Every registration that sets the flag today (28), and where its `selector` leaf comes from:**

| Wire methods (`ze-bgp:`) | Registered in | Selector leaf |
|--------------------------|---------------|---------------|
| `peer-create`, `peer-delete`, `rib-clear-advertised` | `internal/component/bgp/plugins/cmd/peer/peer.go` | On the command node itself, `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` |
| `rib-clear-in`, `rib-clear-out` | `internal/component/bgp/plugins/cmd/rib/rib.go` | On the command node itself, `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` |
| `peer-history`, `peer-rib` | `internal/component/bgp/plugins/cmd/peer/peer.go`, `internal/component/bgp/plugins/cmd/rib/rib.go` | Inherited from `show bgp peer`, `ze-peer-cmd.yang` |
| `update-peer-prefix` | `internal/component/bgp/plugins/cmd/peer/peer.go` | Inherited from `update bgp peer`, `ze-peer-cmd.yang` |
| `peer-teardown`, `peer-notify`, `peer-disable`, `peer-enable`, `peer-pause`, `peer-resume`, `peer-flush` | `internal/component/bgp/plugins/cmd/peer/peer.go` | Inherited from `request peer`, `ze-peer-cmd.yang` |
| `peer-refresh`, `peer-borr`, `peer-eorr`, `peer-clear-soft` | `internal/component/bgp/plugins/route_refresh/handler/refresh.go`, `internal/component/bgp/plugins/route_refresh/handler/clear_soft.go` | Inherited from `request peer` (`ze-peer-cmd.yang`, whose description says so), merged with `internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang` |
| `peer-raw`, `peer-update`, `announce-unicast`, `announce-blackhole`, `announce-flowspec`, `withdraw-tag`, `withdraw-id`, `withdraw-all`, `cache-forward` | `internal/component/bgp/plugins/cmd/raw/raw.go`, `internal/component/bgp/plugins/cmd/update/update_text.go`, `internal/component/bgp/plugins/cmd/announce/announce.go`, `internal/component/bgp/plugins/cmd/cache/cache.go` | Inherited from `send bgp`, declared in `internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang`; the update, announce and cache modules state in a comment that they inherit it |

All 28 are peer commands with a `selector` leaf, so a refusal keyed on that
leaf refuses none of them. No `pkg/` SDK type carries the flag, so only
builtin `RPCRegistration`s can set it. No command under any of these
containers sets `ze:inherit "none"` except `show bgp peer list`, which does
not set the flag.

**Behavior to preserve:**
- Every one of the 28 registrations above still loads, and each keeps its "requires a selector" refusal and its adoption of a lone positional as the selector
- The `ErrCommandHeld` and `ErrWireMethodHeld` startup refusals, and "The server MUST NOT serve after a refusal"
- `request plugin restart <name>` and `system command help <name>` guarded by their mandatory `name` leaf through `command.ValidateArgs`

**Behavior to change:**
- A registration that sets the flag on a command whose model carries no `selector` leaf is accepted today and refuses every line; it is refused at startup instead
- The rule point stops telling an author to set the flag on a command that is not a peer command

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The dispatcher selector guard | `internal/component/plugin/server/command.go` `Dispatch`, `adoptablePositional`, `selectorLeaf`; `internal/component/plugin/server/incomplete.go` `missingSelectorError` | Refuses a flagged command unless the `selector` leaf is bound or `ctx.Peer` is set | Preserved unchanged. The new refusal moves the impossible case from every operator line to startup |
| The mandatory-leaf guard | `internal/component/command/argbind.go` `ValidateArgs`, `MissingArgumentError`; `command-verbs.md`, "Action verbs and their selector" | Binds the lone value or the keyword pair to a typed leaf, and refuses a line with neither before the handler | Does not change. It is what the corrected rule names as the guard of a typed-keyword command |
| The builtin registration path | `internal/component/plugin/server/command.go` `LoadBuiltins`, `loadBuiltinsWithAliases`, `RegisterWithOptions`, `heldCommandError`, `ErrWireMethodHeld` | Pairs each `RPCRegistration` with its YANG path and ArgDefs, and refuses a held name or a doubled wire method at startup | Extends: the same path, which already holds both the flag and the ArgDefs, gains the third refusal. Which of the three functions carries it is a design decision |
| ArgDef inheritance | `internal/component/config/yang/command.go` `PathToArgDefs`, `BuildCommandTree`, `inheritArgDefs`, `appendAnchored`, `mergeYANGEntry` | Merges every module, then copies a container's leaves onto every command below it | Does not change. The check must read the ArgDefs after inheritance, or it refuses the 23 commands whose leaf is inherited |
| The rule point and its renderers | `ai/rules/points/cli/cli-grammar-keywords-before-values/acting-on-everything-needs-an-explicit-all.md`; `./le ai rules update render` (`ai/rules/cli.md`), `./le ai rules update condensed` (`TRIGGERS.md`, `CORE.md`) | States the flag is owed on every set action | Extends: the point is corrected and re-rendered, never `cli.md` edited by hand |
| The design record | `plan/learned/DESIGN-HISTORY.md`, Plugin system, "Commands that mutate a peer declare `RequiresSelector`" | Scopes the flag to peer commands | Does not change. The corrected rule agrees with it |
| The registration table test | `plan/learned/DESIGN-HISTORY.md`, "Every YANG RPC must have a matching registered handler" (`TestRPCRegistrationTable`) | Pairs YANG RPCs and handlers in a unit test | A candidate home for the proof test, judged at design |
| Functional coverage of registered verbs | `plan/post-release/engineering/spec-ci-coverage-gate-sees-every-registered-verb.md` | Skeleton: the `.ci` coverage gate cannot see a verb a BGP plugin registers | Neighbour, does not apply here. It would have found the occurrence by demanding a `.ci`; this spec refuses the shape itself. Neither replaces the other |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Daemon startup: builtin `RPCRegistration`s from each package's `init()`, paired with the YANG command tree]

### Transformation Path
1. [Written at design]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [Written at design] | | No |

### Changed-Wire Census

**Search evidence:** [Written at design. Expected: no wire changes, a startup refusal and a rule text only]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- [Written at design]

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
| A-1 | The `pathToArgDefs` map the loaders receive carries the ArgDefs after `inheritArgDefs` | `PathToArgDefs` builds from `BuildCommandTree`, which runs `inheritArgDefs` (`internal/component/config/yang/command.go`); the caller that hands the map to `LoadBuiltins` and `loadBuiltinsWithAliases` is not yet read | The check refuses the 23 commands whose leaf is inherited, and the daemon does not start | Read the caller, and a test that loads all 28 | unvalidated |
| A-2 | `ctx.Peer` is never the only way a flagged command gets its peer, so no legitimate flagged command lacks a `selector` leaf | All 28 have the leaf today | A plugin-only command with no leaf would be refused wrongly | Grep every `ctx.Peer` producer at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A startup refusal in a build flavour where the module that declares an inherited selector is compiled out but a command inheriting it is not | All modules involved sit under `ze_bgp` today (`feature-gates.txt`); a new gate would split them | Design states the gate dependency |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The daemon refuses to start in every flavour that links a refused registration |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every builtin command owner; `plugin/server` startup |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Daemon startup with a flagged registration whose model has no `selector` leaf | → | [the refusal, placed at design] | [named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A builtin `RPCRegistration` sets `RequiresSelector` and its command's ArgDefs, after inheritance, carry no leaf named `selector` | The daemon refuses the registration at startup, as it refuses a held command name, with a message that names the command path and its wire method and says to remove the flag or declare the `selector` leaf | None: such a command refuses every operator line, so no registration of this shape can work |
| AC-2 | Startup with every registration in the tree, in every shipped flavour | All 28 flagged registrations listed under Current Behavior load, and each still answers a selector-less line with "requires a selector" naming the selector line | Every flagged command whose leaf is direct or inherited, because the guard is satisfiable for each |
| AC-3 | `request plugin restart` with no name, and with `name <value>` or a lone value | Refused as "required argument missing: name", and run with the value bound, through `ze cli` | The mandatory-leaf guard is the guard of a typed-keyword command |
| AC-4 | The rule point and `ai/rules/cli.md` after `./le ai rules update render` | The text says `RequiresSelector` is owed by a command whose model carries the `selector` leaf (a peer command), and that a command addressing a member by a typed keyword is guarded by its mandatory leaf; `command-verbs.md`'s two-declaration table says the same | The explicit-`all` obligation of every set action stays whole: only the sentence naming the flag changes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/component/plugin/server/` | AC-1, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A: no numeric input | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-restart-external-running` (exists) | `test/plugin/plugin-restart-external-running.ci` | AC-3 through `ze cli` | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/plugin/server/command.go` - the startup refusal (exact function decided at design)
- `ai/rules/points/cli/cli-grammar-keywords-before-values/acting-on-everything-needs-an-explicit-all.md` - say when the flag applies
- `ai/rules/cli.md` - re-rendered, never hand-edited
- `docs/architecture/cli/command-verbs.md` - the two-declaration table

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Doctor code per problem class | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | `docs/architecture/cli/command-verbs.md` |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [written at design]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The check reads ArgDefs after inheritance |
| Operator errors and doctor codes | The startup refusal names the command, why, and the fix on its last line (`ai/rules/user-facing-errors.md`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [written at design] | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | The refusal must not weaken the dispatcher guard for the 28 peer commands |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

The 2026-08-31 and 2026-09-15 rows of the same class are already `fixed:` for their own commands; this spec owns the producer they share.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-demands-what-the-model-cannot-supply.md` | 2026-11-12 | `request plugin restart <name>` refused every line under `RequiresSelector` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [Written at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Targeted tests named for this spec pass: `./le test unit pkg path ./internal/component/plugin/server/` (AC-1, AC-2) and `./le test functional plugin` on `plugin-restart-external-running` (AC-3)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-requires-selector-needs-a-selector-leaf.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
