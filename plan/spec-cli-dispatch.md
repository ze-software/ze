# Spec: cli-dispatch

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/8 |
| Updated | 2026-03-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/cli-patterns.md` - current CLI dispatch patterns
4. `.claude/rules/plugin-design.md` - plugin registration patterns
5. `cmd/ze/main.go` - top-level dispatch and static usage()
6. `cmd/ze/cli/main.go` - BuildCommandTree and YANG-to-path mapping
7. `internal/component/plugin/server/handler.go` - RPCRegistration struct
8. `internal/component/plugin/server/rpc_register.go` - RegisterRPCs

## Task

Replace static CLI dispatch and hardcoded help strings with a unified, YANG-driven command registration system. Every command -- whether it queries daemon state, modifies config, decodes wire bytes, or prints the version -- registers through the same mechanism. The `ze` command line and `ze cli` interactive editor share the same command grammar: one thing to learn, two entry points.

### Problem

Today, 14 static `usage()` functions hardcode command lists across `cmd/ze/*/main.go`. The top-level `ze` dispatch is a 20-case switch statement. Adding a command requires: writing the handler, adding a case to the switch, updating the static help string, and remembering to add it to `ze help --ai`. These are independent, unsynchronized maintenance points.

Meanwhile, daemon commands (`ze show`/`ze run`) already have dynamic dispatch via `RegisterRPCs()` + YANG + `BuildCommandTree()`. This spec extends that pattern to all commands.

### Goal

- All commands registered via YANG + `init()` handler registration
- Help generated from YANG `description` fields at every level
- `ze <verb> <noun-path>` dispatches through the same command tree as `ze cli`
- `ze run` and `ze show` as special subcommands disappear -- they become verbs in the unified tree
- Components own their CLI surface (registration, help text, dispatch)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: BGP Subsystem + Plugin Infrastructure separation
- [ ] `docs/architecture/api/commands.md` - current command dispatch design
  -> Constraint: dispatcher routes by longest-match command prefix
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config tree
  -> Decision: config tree already handles set/del via schema navigation
- [ ] `.claude/rules/cli-patterns.md` - current CLI patterns
  -> Constraint: flag.NewFlagSet per subcommand, exit codes, errors to stderr
- [ ] `.claude/rules/plugin-design.md` - plugin registration
  -> Constraint: init() + blank import pattern, YANG required for all RPCs

### RFC Summaries (MUST for protocol work)
- N/A -- no protocol changes

**Key insights:**
- `RegisterRPCs()` + YANG + `BuildCommandTree()` already works for daemon commands -- extend to all commands
- Config tree (`config true` YANG nodes) already handles `set`/`del` generically -- no per-command handler needed
- `ze:command` YANG extension already maps WireMethod to CLI path
- Plugin registry uses `init()` + blank import -- same pattern for CLI commands
- 71 RPCs already registered dynamically; 14 static `usage()` functions need replacing

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - top-level dispatch: 20-case switch (line 240-297), static `usage()` (line 736-791) listing 18 commands
- [ ] `cmd/ze/help_ai.go` - AI help: static `cliSubcommands()` (line 158-178) + dynamic `printAPICommands()` from YANG
- [ ] `cmd/ze/bgp/main.go` - static switch (decode/encode/plugin), static `usage()` listing 3 commands
- [ ] `cmd/ze/iface/main.go` - static switch (show/create/delete/unit/addr/migrate), static `usage()` listing 6 commands
- [ ] `cmd/ze/config/main.go` - map-based dispatch (14 subcommands), static `usage()` listing 14 commands
- [ ] `cmd/ze/show/main.go` - dynamic: `BuildCommandTree(true)` + `cmdutil.PrintCommandList(tree)`
- [ ] `cmd/ze/run/main.go` - dynamic: `BuildCommandTree(false)` + `cmdutil.PrintCommandList(tree)`
- [ ] `cmd/ze/plugin/main.go` - dynamic: `registry.Lookup()` + `registry.WriteUsage()`
- [ ] `cmd/ze/schema/main.go` - static switch, but content is dynamic (YANG queries)
- [ ] `cmd/ze/cli/main.go` - `BuildCommandTree()`, `cliWireToPath`, `AllCLIRPCs()`, `buildRuntimeTree()`
- [ ] `internal/component/plugin/server/handler.go` - `RPCRegistration` struct (WireMethod, Handler, Help, ReadOnly, RequiresSelector, PluginCommand)
- [ ] `internal/component/plugin/server/rpc_register.go` - `RegisterRPCs()`, `registeredRPCs` global
- [ ] `internal/component/plugin/server/command.go` - `AllBuiltinRPCs()`, `LoadBuiltins()`, Dispatcher
- [ ] `internal/component/config/yang/command.go` - `WireMethodToPath()`, `BuildCommandTree()` from YANG
- [ ] `internal/component/command/node.go` - Node struct, `BuildTree()` from RPCInfo entries

**Behavior to preserve:**
- `ze cli` interactive editor behavior (completion, pipe operators, peer selectors)
- Plugin 5-stage protocol and registration
- Config tree navigation for set/del in CLI editor
- YANG `ze:command` extension semantics
- SSH command dispatch
- Exit codes: 0 = success, 1 = error, 2 = file not found
- Errors to stderr
- `--json` output format where supported

**Behavior to change:**
- Static `usage()` functions replaced by dynamic help from YANG descriptions
- Static dispatch switches replaced by unified command tree lookup
- `ze run` and `ze show` as subcommand packages removed -- become verbs in unified tree
- `RPCRegistration.Help` removed -- YANG `description` is authoritative
- `RPCRegistration.ReadOnly` removed -- verb position in tree encodes this
- Top-level `ze` dispatch becomes dynamic tree walk
- `ze` command line accepts same grammar as `ze cli` editor

## Data Flow (MANDATORY)

### Entry Point
- User types `ze show peer list` at shell, or `show peer list` in `ze cli` editor
- Both enter the same command tree

### Transformation Path
1. Parse first word as verb (`show`, `set`, `del`, `update`, `validate`)
2. Walk remaining words through YANG-derived command tree
3. Find registered handler for the resolved WireMethod
4. For `set`/`del`: config tree handles generically via YANG schema navigation
5. For `show`/`update`/`validate`: dispatch to registered handler function
6. Handler returns response, formatted per output flags

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI entry -> command tree | Verb + noun path lookup in YANG-derived tree | [ ] |
| Command tree -> handler | WireMethod maps to registered Handler | [ ] |
| `ze` shell -> daemon | SSH connection for daemon commands | [ ] |
| `ze` shell -> local | Direct handler call for offline commands | [ ] |
| Config tree -> YANG schema | `set`/`del` walk config YANG nodes | [ ] |

### Integration Points
- `internal/component/config/yang/command.go` -- existing `BuildCommandTree()` from YANG, extend to cover all verbs
- `internal/component/plugin/server/rpc_register.go` -- existing `RegisterRPCs()`, becomes the unified handler registry
- `cmd/ze/cli/main.go` -- existing `BuildCommandTree()`, `cliWireToPath` -- shared with command-line entry
- `internal/component/command/node.go` -- existing Node/BuildTree, used by both entry points
- `internal/component/plugin/server/command.go` -- Dispatcher, handles runtime command routing

### Architectural Verification
- [ ] No bypassed layers -- all commands go through YANG tree + handler registry
- [ ] No unintended coupling -- components register independently via init()
- [ ] No duplicated functionality -- extends existing RegisterRPCs + YANG tree, does not recreate
- [ ] Zero-copy preserved -- command dispatch only, no wire encoding changes

## Design

### Verb Classification

Two mechanisms from YANG, not one:

| YANG Construct | Verbs | Registration | Handler |
|----------------|-------|-------------|---------|
| Config tree (`config true` nodes) | `set`, `del` | None needed -- YANG config schema defines the tree | Config system walks tree, applies/removes values |
| Commands (`config false` + `ze:command`) | `show`, `update`, `validate`, `monitor` | WireMethod + Handler via `RegisterRPCs()` | Explicitly registered per command |

`set`/`del` operate on the config tree generically. The config system already knows how to walk YANG nodes and apply values. No per-command handler registration needed.

`show`/`update`/`validate`/`monitor` are commands with individual handlers. Each is an explicit YANG command node (`ze:command` extension) with a registered handler. `monitor` commands are streaming -- they produce continuous output until cancelled.

### Verb Structure in Core YANG

The core YANG module defines top-level verb containers. Components `augment` these containers to add their noun paths.

| Verb | Semantics | YANG container |
|------|-----------|---------------|
| `show` | Read-only, display result | `config false` container |
| `set` | Config mutation | Config tree navigation |
| `del` | Config removal | Config tree navigation |
| `update` | Refresh from external source | `config false` container |
| `validate` | Check without changing | `config false` container |
| `monitor` | Streaming, continuous observation | `config false` container |

Verbs are defined in core YANG. Components add commands under them via YANG `augment`.

### Command Examples Under New Grammar

| Current | New |
|---------|-----|
| `ze show peer list` | `ze show peer list` (same) |
| `ze run del peer 10.0.0.1` | `ze del peer 10.0.0.1` |
| `ze bgp decode <hex>` | `ze show bgp decode <hex>` |
| `ze bgp encode <route>` | `ze show bgp encode <route>` |
| `ze interface show` | `ze show interface` |
| `ze interface create dummy lo1` | `ze set interface create dummy lo1` |
| `ze interface delete lo1` | `ze del interface lo1` |
| `ze config validate <file>` | `ze validate config <file>` |
| `ze config dump <file>` | `ze show config dump <file>` |
| `ze config edit` | `ze set config edit` |
| `ze version` | `ze show version` |
| `ze completion bash` | `ze show completion bash` |

### Handler Registration

Minimal struct. Everything else from YANG.

| Field | Source | Purpose |
|-------|--------|---------|
| WireMethod | YANG `ze:command` value | Dispatch key |
| Handler | Go function registered in `init()` | Function to call |

Fields removed from current `RPCRegistration`:
- `Help` -- YANG `description` is authoritative
- `ReadOnly` -- verb position in tree encodes this (commands under `show`/`validate` are read-only)
- `RequiresSelector` -- expressed in YANG via extension (e.g., `ze:requires-selector`)

Fields kept:
- `RequiresSelector` -- if not expressible in YANG, keep on registration
- `PluginCommand` -- proxy routing to runtime plugin commands

### Component Registration Pattern

Each component provides:
1. A YANG module that `augment`s the core verb containers with command nodes
2. A `register.go` with `init()` calling `RegisterRPCs()` for its handlers
3. A `schema/register.go` with `init()` calling `yang.RegisterModule()` for its YANG

This is the existing pattern used by `internal/component/cmd/*`. The change is extending it to cover all commands, including those currently hardcoded in `cmd/ze/*/main.go`.

### Dynamic Help Generation

Help at every level is generated from the YANG command tree:
1. `ze help` -- lists top-level verbs and their descriptions from YANG
2. `ze show help` -- lists all commands under `show` from YANG tree
3. `ze show bgp help` -- lists commands under `show bgp` from YANG tree

The `cmdutil.PrintCommandList(tree)` pattern already works for `ze show` and `ze run`. Extend to all levels.

### Top-Level Dispatch

The current 20-case switch in `main.go:240-297` becomes a tree walk:
1. Parse first argument
2. Look up in command tree (verbs + registered commands)
3. If found: dispatch to handler
4. If not found: check if it's a config file path (existing behavior for `ze config.conf`)
5. Otherwise: error with suggestion

### Packages To Remove

| Package | Replaced By |
|---------|-------------|
| `cmd/ze/show/` | `show` verb in unified tree |
| `cmd/ze/run/` | Direct dispatch through unified tree |

### Packages To Refactor

| Package | Change |
|---------|--------|
| `cmd/ze/main.go` | Static switch -> tree walk, static `usage()` -> dynamic |
| `cmd/ze/bgp/main.go` | Commands move to component registration under `show bgp ...` |
| `cmd/ze/iface/main.go` | Commands move to component registration under `show interface ...` / `set interface ...` / `del interface ...` |
| `cmd/ze/config/main.go` | Commands move to component registration under `show config ...` / `validate config ...` / `set config ...` |
| `cmd/ze/cli/main.go` | Shares command tree with top-level dispatch |
| `cmd/ze/signal/` | Moves to component registration |
| `cmd/ze/init/` | Moves to component registration |
| `cmd/ze/completion/` | Moves to component registration under `show completion ...` |
| `cmd/ze/schema/` | Moves to component registration under `show schema ...` |
| `cmd/ze/yang/` | Moves to component registration under `show yang ...` |
| `cmd/ze/data/` | Moves to component registration |
| `cmd/ze/environ/` | Moves to component registration under `show env ...` |
| `cmd/ze/exabgp/` | Moves to component registration |
| `cmd/ze/plugin/` | Already dynamic, integrate into unified tree |
| `cmd/ze/help_ai.go` | `cliSubcommands()` becomes dynamic from tree |

### What Stays in cmd/ze/

After migration, `cmd/ze/` packages become thin wrappers or disappear entirely. The handler logic moves to `internal/component/` packages that register via `init()`. `cmd/ze/main.go` becomes the entry point that:
1. Parses global flags (`--debug`, `--plugin`, `--pprof`, etc.)
2. Imports all component packages (triggering `init()` registrations)
3. Walks the unified command tree for dispatch
4. Falls back to config-file-start for unrecognized first args

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show version` | -> | Version handler registered via YANG + init() | `test/parse/cli-dispatch-show-version.ci` |
| `ze show peer list` (with daemon) | -> | Peer list handler through unified tree | `test/plugin/cli-dispatch-show-peer.ci` |
| `ze show bgp decode <hex>` | -> | BGP decode handler through unified tree | `test/decode/cli-dispatch-decode.ci` |
| `ze help` | -> | Dynamic help from YANG tree | `test/parse/cli-dispatch-help.ci` |
| `ze show help` | -> | Dynamic verb-level help | `test/parse/cli-dispatch-show-help.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze show version` | Prints version (same output as current `ze version`) |
| AC-2 | `ze show bgp decode <hex>` | Decodes BGP message (same output as current `ze bgp decode <hex>`) |
| AC-3 | `ze show interface` | Lists interfaces (same output as current `ze interface show`) |
| AC-4 | `ze help` | Lists available verbs and top-level commands from YANG tree, not static string |
| AC-5 | `ze show help` | Lists all show commands from YANG tree |
| AC-6 | `ze show bgp help` | Lists show bgp subcommands from YANG tree |
| AC-7 | `ze validate config <file>` | Validates config (same output as current `ze config validate <file>`) |
| AC-8 | `ze set interface create dummy lo1` | Creates interface (same behavior as current `ze interface create dummy lo1`) |
| AC-9 | `ze del interface lo1` | Deletes interface (same behavior as current `ze interface delete lo1`) |
| AC-10 | Adding a new component with YANG | New component's commands appear in help automatically without modifying `cmd/ze/main.go` |
| AC-11 | `ze run <anything>` | Error: "unknown command 'run'" with suggestion to use verb directly |
| AC-12 | Unknown command | Error message with "did you mean?" suggestion from tree |
| AC-13 | `show peer list` in `ze cli` | Same result as `ze show peer list` -- same grammar |
| AC-14 | `ze update peeringdb` | PeeringDB refresh handler dispatched through unified tree |
| AC-15 | YANG `description` changed | Help output reflects the new description without Go code changes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnifiedTreeVerbs` | `internal/component/command/node_test.go` | Core verbs present in tree built from YANG | |
| `TestUnifiedTreeHelp` | `internal/component/command/help_test.go` | Help generation from YANG descriptions | |
| `TestVerbClassification` | `internal/component/command/node_test.go` | Commands under show are read-only, under set/del/update are mutating | |
| `TestTreeLookup` | `internal/component/command/node_test.go` | Command lookup by path works for all verbs | |
| `TestHelpAtEveryLevel` | `internal/component/command/help_test.go` | `ze help`, `ze show help`, `ze show bgp help` all produce output | |
| `TestUnknownCommandSuggestion` | `internal/component/command/node_test.go` | Unknown command returns "did you mean?" from tree | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-dispatch-show-version` | `test/parse/cli-dispatch-show-version.ci` | `ze show version` prints version | |
| `cli-dispatch-help` | `test/parse/cli-dispatch-help.ci` | `ze help` lists verbs dynamically | |
| `cli-dispatch-show-help` | `test/parse/cli-dispatch-show-help.ci` | `ze show help` lists show commands | |
| `cli-dispatch-decode` | `test/decode/cli-dispatch-decode.ci` | `ze show bgp decode <hex>` decodes message | |
| `cli-dispatch-unknown` | `test/parse/cli-dispatch-unknown.ci` | Unknown command gives suggestion | |

### Future (if deferring any tests)
- `ze set interface create` and `ze del interface` functional tests -- deferred to interface component migration phase
- `ze update peeringdb` functional test -- deferred to PeeringDB component creation
- `ze validate config` functional test -- deferred to config component migration phase

## Files to Modify

- `cmd/ze/main.go` -- replace static switch with unified tree walk, replace static `usage()` with dynamic help
- `cmd/ze/help_ai.go` -- replace static `cliSubcommands()` with dynamic tree query
- `cmd/ze/cli/main.go` -- share `BuildCommandTree()` with command-line entry point
- `internal/component/command/node.go` -- extend Node/BuildTree to support verb classification
- `internal/component/command/completer.go` -- extend completion to work with verb-prefixed commands
- `internal/component/config/yang/command.go` -- extend `BuildCommandTree()` to include verb containers
- `internal/component/plugin/server/handler.go` -- remove `Help` and `ReadOnly` from `RPCRegistration`
- `internal/component/plugin/server/rpc_register.go` -- adapt `RegisterRPCs()` for unified registration
- `internal/component/plugin/server/command.go` -- adapt Dispatcher for verb-based routing
- `cmd/ze/internal/cmdutil/cmdutil.go` -- adapt help formatting for all verbs

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | Core verb YANG module + component augments |
| CLI commands/flags | Yes | All `cmd/ze/*/main.go` migrate to registration |
| Editor autocomplete | Yes | YANG-driven (automatic from tree) |
| Functional test for new RPC/API | Yes | `test/parse/cli-dispatch-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- unified CLI dispatch |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- all commands change to verb-first |
| 4 | API/RPC added/changed? | No | N/A (dispatch changes, RPCs stay same) |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md` -- complete rewrite |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` -- unified dispatch |

## Files to Create

- Core verb YANG module (e.g., `internal/core/ipc/schema/ze-cli-verbs.yang` or in config YANG) defining `show`, `update`, `validate` verb containers
- Component YANG modules augmenting verb containers (for commands currently hardcoded)
- Component `register.go` files for handlers currently in `cmd/ze/*/main.go`
- `internal/component/command/help.go` -- dynamic help generation from YANG tree
- `test/parse/cli-dispatch-show-version.ci`
- `test/parse/cli-dispatch-help.ci`
- `test/parse/cli-dispatch-show-help.ci`
- `test/decode/cli-dispatch-decode.ci`
- `test/parse/cli-dispatch-unknown.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Core YANG verb structure** -- Define core YANG module with verb containers (`show`, `update`, `validate`). Extend `BuildCommandTree()` to produce a verb-rooted tree. Add help generation from YANG descriptions.
   - Tests: `TestUnifiedTreeVerbs`, `TestUnifiedTreeHelp`, `TestVerbClassification`
   - Files: Core verb YANG module, `internal/component/command/node.go`, `internal/component/command/help.go`, `internal/component/config/yang/command.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Unified dispatch in main.go** -- Replace static switch with tree walk. Replace static `usage()` with dynamic help. Keep existing subcommand packages as fallback during migration.
   - Tests: `TestTreeLookup`, `TestUnknownCommandSuggestion`, `TestHelpAtEveryLevel`
   - Files: `cmd/ze/main.go`, `cmd/ze/help_ai.go`, `cmd/ze/internal/cmdutil/cmdutil.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Migrate first component (version/completion)** -- Move `ze version` and `ze completion` to registered commands under `show`. Proves the pattern works end-to-end.
   - Tests: `cli-dispatch-show-version.ci`, functional test for `ze show completion bash`
   - Files: New component registration for version/completion, remove from static switch
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Migrate BGP offline tools** -- Move `ze bgp decode` and `ze bgp encode` to `show bgp decode` / `show bgp encode` via component registration.
   - Tests: `cli-dispatch-decode.ci`
   - Files: BGP decode/encode component YANG + registration, remove from `cmd/ze/bgp/main.go` static switch
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Migrate remaining components** -- Move interface, config, schema, yang, data, environ, exabgp, signal, init, plugin to component registration. Each gets YANG augments and handler registration.
   - Tests: Component-specific functional tests
   - Files: Per-component YANG + registration, remove static switches
   - Verify: per-component, then `make ze-verify`

6. **Phase: Remove old infrastructure** -- Delete `cmd/ze/show/`, `cmd/ze/run/`. Remove `RPCRegistration.Help` and `RPCRegistration.ReadOnly`. Clean up `cmd/ze/main.go` fallback paths.
   - Tests: All existing tests still pass
   - Files: Delete `cmd/ze/show/`, `cmd/ze/run/`, clean `RPCRegistration`
   - Verify: `make ze-verify`

7. **Phase: Dynamic help at all levels** -- `ze help`, `ze show help`, `ze show bgp help` all generate from YANG tree. Update `ze help --ai` to use dynamic tree.
   - Tests: `cli-dispatch-help.ci`, `cli-dispatch-show-help.ci`
   - Files: `internal/component/command/help.go`, `cmd/ze/help_ai.go`
   - Verify: tests fail -> implement -> tests pass

8. **Functional tests + docs + learned summary**
   - Create remaining functional tests
   - Write documentation updates
   - Full verification: `make ze-verify`
   - Write learned summary to `plan/learned/`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Same output for migrated commands (decode, version, interface, etc.) |
| Naming | YANG modules follow `ze-*` naming, verbs are lowercase |
| Data flow | All commands go through unified tree, no static bypass |
| Rule: no-layering | Old static dispatch fully deleted after migration |
| Rule: cli-patterns | Exit codes, stderr errors preserved |
| Rule: plugin-design | YANG required for all commands, init() registration |
| Backward compat | `ze bgp decode` gives error with hint to use `ze show bgp decode` |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Core verb YANG module exists | `ls internal/core/ipc/schema/ze-cli-verbs.yang` or equivalent |
| `ze help` is dynamic | Run `ze help`, verify output matches YANG tree |
| `ze show version` works | `test/parse/cli-dispatch-show-version.ci` passes |
| `ze show bgp decode` works | `test/decode/cli-dispatch-decode.ci` passes |
| No static `usage()` remains in migrated packages | `grep -r "func usage()" cmd/ze/` shows only unmigrated |
| `RPCRegistration.Help` removed | `grep "Help " internal/component/plugin/server/handler.go` finds no field |
| `RPCRegistration.ReadOnly` removed | `grep "ReadOnly " internal/component/plugin/server/handler.go` finds no field |
| `cmd/ze/show/` deleted | `ls cmd/ze/show/` returns error |
| `cmd/ze/run/` deleted | `ls cmd/ze/run/` returns error |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | First argument validated against tree before dispatch |
| Command injection | No shell evaluation of user-provided command words |
| Authorization | Verb classification (show=read, set/del/update=write) used for access control |
| Suggestion leakage | "Did you mean?" does not reveal commands the user lacks permission for |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Migrated command produces different output | Compare with old handler, fix new registration |
| YANG augment fails to resolve | Check imports and module dependencies |
| Help output missing commands | Verify YANG module registered and loaded |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Config tree (`set`/`del`) and commands (`show`/`update`/`validate`/`monitor`) are two mechanisms from YANG, not one
- The verb IS the read/write classification -- no need for a `ReadOnly` flag
- `ze run` and `ze show` were workarounds for static dispatch, not fundamental design
- One grammar, two entry points: `ze <verb> <noun>` and `ze cli` then `<verb> <noun>`
- Components register their CLI surface; help reflects registration automatically

## RFC Documentation

N/A -- no protocol changes.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
