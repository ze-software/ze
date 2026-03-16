# Spec: YANG Analysis Tool

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - YANG schema system
4. `docs/architecture/api/commands.md` - command tree structure
5. `internal/component/command/node.go` - command tree types
6. `internal/component/cli/completer.go` - config completion
7. `internal/component/config/yang/rpc.go` - RPC extraction
8. `cmd/ze/schema/main.go` - existing schema CLI (pattern to follow)

## Task

Build a `ze yang` CLI tool that automates two recurring analysis tasks:

1. **Prefix collision detection** -- find sibling commands/config leaves that share a prefix, requiring more than one keypress to disambiguate during Tab completion. This must be re-run after every YANG schema change.

2. **Command/config documentation** -- generate structured documentation of the full YANG tree (config nodes, operational commands, types, constraints, descriptions). This enables a future `ze yang doc` reference and helps maintain naming consistency.

The tool builds a **unified analysis tree** in Go that merges config entries (from YANG `-conf` modules) and command entries (from `RPCRegistration.CLICommand` strings) into a single walkable structure. goyang already puts config containers and RPCs in the same `Entry.Dir` map -- the runtime splits them into two completers, but the analysis tool recombines them to catch cross-domain collisions and provide a single tree view.

The tool parses the same YANG schemas the runtime uses (via `yang.Loader`) and the same RPC registrations that build the command tree. No manual inspection.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - how YANG drives config and CLI
  -> Decision: YANG modules are loaded via `yang.Loader` (embedded + registered). Config modules are `ze-bgp-conf`, `ze-hub-conf`, `ze-plugin-conf`.
  -> Constraint: CLI completions are prefix-match based (`strings.HasPrefix`). Siblings with shared prefixes cannot be disambiguated in one keypress.
- [ ] `docs/architecture/api/commands.md` - command tree and dispatch
  -> Decision: Operational commands come from `RPCRegistration.CLICommand` strings. `BuildTree()` splits on spaces. BGP commands strip `"bgp "` prefix.
  -> Constraint: Command tree is flat (split on space), not hierarchical like config. Collisions happen at each tree level independently.

### RFC Summaries (MUST for protocol work)
Not applicable -- this is a CLI analysis tool, not protocol work.

**Key insights:**
- Runtime has two completion domains: config (YANG entry tree) and command (RPC-derived tree). The analysis tool merges them into one unified tree.
- goyang already puts config containers and RPCs in the same `Entry.Dir` map (see `TestYANGModuleWithRPCAndConfig`). The two-completer split is a Ze design choice, not a YANG limitation.
- Config tree: `yang.Loader.GetEntry()` returns `*gyang.Entry` with `.Dir` children map. Entries have `.RPC` field (non-nil for RPCs) and `.Kind` to distinguish node types.
- Command tree: `command.BuildTree()` returns `*command.Node` with `.Children` map, built from `CLICommand` strings split on spaces.
- The unified analysis tree merges both into a common node type with a `Source` field ("config", "command", or "both") so collisions across domains are caught.
- Existing `cmd/ze/schema/` provides the pattern for YANG-loading CLI subcommands
- `cli/completer.go` merges 3 config modules into a virtual root via `mergedRoot()` -- analysis must do the same for config, then merge command nodes on top

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/node.go` - `BuildTree()` creates command tree from `[]RPCInfo`. `Node` has `Name`, `Description`, `Children map[string]*Node`.
- [ ] `internal/component/command/completer.go` - `TreeCompleter.Complete()` navigates tree, `matchChildren()` filters by `strings.HasPrefix`. `GhostText()` uses `commonPrefix()` for partial completion.
- [ ] `internal/component/cli/completer.go` - `Completer` walks YANG entry tree for config completions. `matchChildren()` filters by prefix. `getEntry()` navigates path with list key skipping. `confModules` = `["ze-bgp-conf", "ze-hub-conf", "ze-plugin-conf"]`. `mergedRoot()` combines all module roots.
- [ ] `internal/component/config/yang/loader.go` - `Loader` wraps goyang. `LoadEmbedded()` + `LoadRegistered()` + `Resolve()` pipeline. `GetEntry()` returns resolved entry tree.
- [ ] `internal/component/config/yang/rpc.go` - `ExtractRPCs()` extracts `RPCMeta` from YANG API modules. `RPCMeta` has `Module`, `Name`, `Description`, `Input`, `Output` with `LeafMeta`.
- [ ] `cmd/ze/schema/main.go` - existing `ze schema` subcommand with `list`, `show`, `handlers`, `methods`, `events`, `protocol`. Loads YANG via `buildSchemaRegistry()`. Pattern to follow for `ze yang`.

**Behavior to preserve:**
- Existing `ze schema` subcommand unchanged
- YANG module registration via `init()` + `RegisterModule()` unchanged
- Command tree building via `BuildTree()` unchanged
- All existing completion behavior unchanged

**Behavior to change:**
- None -- this adds new analysis commands, does not change existing behavior

## Data Flow (MANDATORY)

### Entry Point
- CLI: `ze yang <subcommand>` invoked by user or CI
- Data source: embedded YANG modules (same as runtime) + RPC registrations (same as runtime)

### Transformation Path
1. **YANG loading**: `yang.NewLoader()` -> `LoadEmbedded()` -> `LoadRegistered()` -> `Resolve()` -> resolved entry trees
2. **Config tree extraction**: `loader.GetEntry(moduleName)` for each conf module -> merge roots (same as `mergedRoot()` in completer) -> recursive walk of `*gyang.Entry.Dir` children -> convert to unified analysis nodes
3. **Command tree extraction**: collect all `RPCRegistration.CLICommand` strings -> `command.BuildTree()` -> recursive walk of `*command.Node.Children` -> convert to unified analysis nodes
4. **Merge**: insert command nodes into the unified tree alongside config nodes. Each node carries a `Source` tag. Nodes present in both domains get `Source: "both"`.
5. **Prefix analysis**: at each tree level, group siblings by first character -> report groups with >1 member. Source tags let the report show which domain each sibling comes from.
6. **Documentation generation**: at each tree level, collect name + type + description + constraints -> format as text or JSON. Unified tree means one walk produces the full picture.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG loader -> analysis | `GetEntry()` returns `*gyang.Entry` tree | [ ] |
| RPC registration -> command tree | `BuildTree()` from `[]RPCInfo` | [ ] |
| Analysis -> stdout | `fmt.Printf` / `json.Encoder` | [ ] |

### Integration Points
- `internal/component/config/yang.Loader` -- reuse for YANG loading
- `internal/component/config/yang.ExtractRPCs()` -- reuse for RPC metadata
- `internal/component/command.BuildTree()` -- reuse for command tree
- `internal/component/plugin/server.RPCRegistration` -- source of CLI command strings
- `cmd/ze/schema/main.go` -- pattern for YANG-loading CLI, `buildSchemaRegistry()` and `loadAPIRPCs()` for discovering all registrations

### Architectural Verification
- [ ] No bypassed layers -- uses same YANG loading as runtime
- [ ] No unintended coupling -- read-only analysis, no mutations
- [ ] No duplicated functionality -- reuses existing loader/builder infrastructure
- [ ] Zero-copy preserved -- not applicable (analysis tool, not wire path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze yang completion` CLI | -> | prefix collision analyzer | `test/ui/cli-yang-completion-analysis.ci` |
| `ze yang tree` CLI | -> | config/command tree printer | `test/ui/cli-yang-tree.ci` |
| `ze yang doc` CLI | -> | command documentation generator | `test/ui/cli-yang-doc.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze yang completion` with current YANG schemas | Outputs all prefix collision groups across config and command trees. Each group shows: path, colliding siblings, minimum disambiguation prefix length. Exit 0. |
| AC-2 | `ze yang completion --json` | Same data as AC-1 but in JSON format suitable for CI parsing |
| AC-3 | `ze yang completion --min-prefix N` | Only reports collisions where N or more characters are needed to disambiguate (default: 1) |
| AC-4 | `ze yang tree` | Prints unified hierarchical view of all nodes (config + command) with source tag, name, type, description, constraints. Exit 0. |
| AC-5 | `ze yang tree --commands` | Filters to command nodes only. `--config` filters to config only. |
| AC-6 | `ze yang tree --json` | JSON format of unified tree |
| AC-7 | `ze yang doc <command>` | Prints documentation for a specific operational command: syntax, parameters (name, type, mandatory), output fields, description |
| AC-8 | `ze yang doc --list` | Lists all documented commands with one-line descriptions |
| AC-9 | `ze yang help` | Shows usage text for all subcommands |
| AC-10 | Prefix analysis correctly handles list nodes | List nodes (e.g., `peer`, `group`) are analyzed for sibling collisions. List key values (dynamic) are skipped. |
| AC-11 | Unified tree merges config and command domains | One tree walk finds collisions everywhere, including cross-domain (e.g., a config keyword and a command keyword sharing a prefix at the same level). Each node tagged with source domain. |
| AC-12 | Cross-domain collision at top level | If a config container name (e.g., `bgp`) collides with a command name at the same level, report it. Currently command mode merges both at top level (`model.go:889-901`), so this is a real user-facing collision. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixCollisions` | `cmd/ze/yang/prefix_test.go` | Groups siblings by shared prefix, returns collision info | |
| `TestPrefixCollisionsNone` | `cmd/ze/yang/prefix_test.go` | Returns empty when all siblings have unique first chars | |
| `TestPrefixCollisionsMinPrefix` | `cmd/ze/yang/prefix_test.go` | --min-prefix filtering works correctly | |
| `TestPrefixCollisionDepth` | `cmd/ze/yang/prefix_test.go` | Reports minimum chars needed to disambiguate each group | |
| `TestUnifiedTreeBuild` | `cmd/ze/yang/tree_test.go` | Builds unified tree from real YANG + RPCs, finds both config and command nodes | |
| `TestUnifiedTreeConfigNodes` | `cmd/ze/yang/tree_test.go` | Config nodes present with correct types and descriptions (e.g., bgp > peer > hold-time) | |
| `TestUnifiedTreeCommandNodes` | `cmd/ze/yang/tree_test.go` | Command nodes present at correct paths (e.g., peer > list, peer > add) | |
| `TestUnifiedTreeCrossDomain` | `cmd/ze/yang/tree_test.go` | Nodes present in both domains tagged as `Source: "both"` | |
| `TestUnifiedTreeCollisions` | `cmd/ze/yang/tree_test.go` | Detects known collisions: bgp peer-fields (local-as/local-address/link-local), peer commands (raw/refresh/remove/resume) | |
| `TestTreeFormatText` | `cmd/ze/yang/format_test.go` | Text tree output has correct indentation and fields | |
| `TestTreeFormatJSON` | `cmd/ze/yang/format_test.go` | JSON tree output has correct structure | |
| `TestDocCommand` | `cmd/ze/yang/doc_test.go` | Generates correct doc for a known command (e.g., peer-list) | |
| `TestDocList` | `cmd/ze/yang/doc_test.go` | Lists all commands with descriptions | |
| `TestRunCompletion` | `cmd/ze/yang/main_test.go` | CLI dispatch to completion subcommand | |
| `TestRunTree` | `cmd/ze/yang/main_test.go` | CLI dispatch to tree subcommand | |
| `TestRunDoc` | `cmd/ze/yang/main_test.go` | CLI dispatch to doc subcommand | |
| `TestRunHelp` | `cmd/ze/yang/main_test.go` | CLI dispatch to help | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--min-prefix` | 1-10 | 10 | 0 | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-yang-completion-analysis` | `test/ui/cli-yang-completion-analysis.ci` | User runs `ze yang completion`, gets collision report | |
| `cli-yang-tree` | `test/ui/cli-yang-tree.ci` | User runs `ze yang tree`, gets config tree | |
| `cli-yang-doc` | `test/ui/cli-yang-doc.ci` | User runs `ze yang doc --list`, gets command list | |

### Future (if deferring any tests)
- Interactive `ze yang rename` subcommand (proposes YANG node renames to fix collisions) -- requires schema modification, separate spec
- CI integration with threshold-based exit codes -- can be added after tool proves useful

## Files to Modify
- `cmd/ze/main.go` - add `"yang"` dispatch case to main CLI router

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- this is a CLI tool, not a runtime RPC |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | Yes | `cmd/ze/main.go` -- add `yang` dispatch |
| CLI usage/help text | Yes | `cmd/ze/yang/main.go` -- usage function |
| API commands doc | No | N/A -- offline tool |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A -- offline tool |
| Functional test for new RPC/API | N/A | Functional tests in `test/ui/` |

## Files to Create
- `cmd/ze/yang/main.go` - CLI dispatch for `ze yang` subcommands
- `cmd/ze/yang/prefix.go` - generic prefix collision analyzer
- `cmd/ze/yang/prefix_test.go` - unit tests for prefix analyzer
- `cmd/ze/yang/tree.go` - unified analysis tree: builds from YANG config entries + RPC command nodes, tags each node with source domain
- `cmd/ze/yang/tree_test.go` - unit tests for unified tree builder
- `cmd/ze/yang/format.go` - text and JSON output formatting
- `cmd/ze/yang/format_test.go` - unit tests for formatters
- `cmd/ze/yang/doc.go` - per-command documentation generator
- `cmd/ze/yang/doc_test.go` - unit tests for doc generator
- `cmd/ze/yang/main_test.go` - CLI dispatch tests
- `test/ui/cli-yang-completion-analysis.ci` - functional test
- `test/ui/cli-yang-tree.ci` - functional test
- `test/ui/cli-yang-doc.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
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

1. **Phase: Prefix analyzer** -- pure algorithm, no YANG dependency
   - Tests: `TestPrefixCollisions`, `TestPrefixCollisionsNone`, `TestPrefixCollisionsMinPrefix`, `TestPrefixCollisionDepth`
   - Files: `cmd/ze/yang/prefix.go`, `cmd/ze/yang/prefix_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Unified analysis tree** -- load YANG config entries + RPC command nodes, merge into one tree
   - Tests: `TestUnifiedTreeBuild`, `TestUnifiedTreeConfigNodes`, `TestUnifiedTreeCommandNodes`, `TestUnifiedTreeCrossDomain`, `TestUnifiedTreeCollisions`
   - Files: `cmd/ze/yang/tree.go`, `cmd/ze/yang/tree_test.go`
   - Steps: (a) define unified node type with Name, Source, Type, Description, Children; (b) walk YANG conf module entries into it; (c) walk command tree into it, merging at shared paths; (d) run prefix analyzer at each level
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Formatters** -- text and JSON output for unified tree and collision reports
   - Tests: `TestTreeFormatText`, `TestTreeFormatJSON`
   - Files: `cmd/ze/yang/format.go`, `cmd/ze/yang/format_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Doc generator** -- per-command documentation from RPC metadata
   - Tests: `TestDocCommand`, `TestDocList`
   - Files: `cmd/ze/yang/doc.go`, `cmd/ze/yang/doc_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: CLI wiring** -- main dispatch, help, integration with `cmd/ze/main.go`
   - Tests: `TestRunCompletion`, `TestRunTree`, `TestRunDoc`, `TestRunHelp`
   - Files: `cmd/ze/yang/main.go`, `cmd/ze/yang/main_test.go`, `cmd/ze/main.go`
   - Verify: tests fail -> implement -> tests pass

7. **Functional tests** -- create `.ci` tests after feature works
   - Files: `test/ui/cli-yang-completion-analysis.ci`, `test/ui/cli-yang-tree.ci`, `test/ui/cli-yang-doc.ci`

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Prefix collision algorithm correctly computes minimum disambiguation length |
| Correctness | Unified tree correctly merges config entries (list keys skipped, containers/leaves included) and command entries (BGP prefix stripped same as `BuildTree()`) |
| Correctness | Cross-domain nodes tagged correctly -- nodes present in both domains get `Source: "both"` |
| Naming | CLI flags use `--kebab-case`, JSON keys use `kebab-case` |
| Data flow | YANG loading uses same `LoadEmbedded + LoadRegistered + Resolve` as runtime |
| Consistency | Output format matches existing `ze schema` style (table headers, alignment) |
| Rule: cli-patterns | Each subcommand has `flag.NewFlagSet`, `fs.Usage`, proper exit codes |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/yang/main.go` exists | `ls cmd/ze/yang/main.go` |
| `cmd/ze/yang/prefix.go` exists | `ls cmd/ze/yang/prefix.go` |
| `cmd/ze/yang/tree.go` exists | `ls cmd/ze/yang/tree.go` |
| `cmd/ze/yang/format.go` exists | `ls cmd/ze/yang/format.go` |
| `cmd/ze/yang/doc.go` exists | `ls cmd/ze/yang/doc.go` |
| `ze yang completion` runs and produces output | `go run ./cmd/ze yang completion` |
| `ze yang tree` runs and produces output | `go run ./cmd/ze yang tree` |
| `ze yang doc --list` runs and produces output | `go run ./cmd/ze yang doc --list` |
| `ze yang help` shows usage | `go run ./cmd/ze yang help` |
| `.ci` test files exist | `ls test/ui/cli-yang-*.ci` |
| All unit tests pass | `go test ./cmd/ze/yang/...` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `--min-prefix` flag bounds checked (1-10) |
| No file writes | Tool is read-only analysis -- verify no file system writes |
| No network access | Tool is offline -- verify no network calls |
| No exec | Tool does not exec external programs (unlike `ze schema` which queries plugins) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### D-1: Separate `ze yang` vs extending `ze schema`

`ze schema` is discovery-oriented (list modules, show YANG text, map handlers).
`ze yang` is analysis-oriented (find naming problems, generate documentation).
Different concerns, different audiences (developer vs operator).
Keeps both subcommands focused and avoids bloating `ze schema`.

### D-2: Unified analysis tree built in Go

goyang already puts config containers, RPCs, and notifications in the same `Entry.Dir` map. The runtime splits them into two completers (`cli/completer.go` for config, `command/completer.go` for commands). The analysis tool recombines them: it builds one tree where each node carries a `Source` tag ("config", "command", or "both"). This catches cross-domain collisions that the split completers miss -- notably at the top level where command mode merges both (`model.go:889-901`).

This is approach "A" (merge in Go). Approach "B" -- restructuring YANG so commands are defined hierarchically -- is covered by `spec-yang-command-tree.md`. After that spec, the analysis tool walks one native YANG tree instead of merging two sources.

### D-3: Reuse `yang.Loader` not raw file parsing

The tool must analyze the same tree the runtime sees. Using `yang.Loader` with `LoadEmbedded() + LoadRegistered() + Resolve()` guarantees this.
If a module fails to load in the tool, it fails in the runtime too -- so the tool never produces stale results.

### D-4: Generic prefix analyzer separated from tree builder

The prefix collision algorithm (`[]string` -> collision groups) is domain-independent.
The unified tree builder feeds sibling groups into it at each level.
This makes the algorithm unit-testable without YANG loading.

### D-5: `--min-prefix` flag for threshold control

Default `--min-prefix 1` reports all collisions (any siblings sharing first character).
Higher values (e.g., `--min-prefix 3`) find only severe cases where 3+ chars needed.
Useful for CI gating: `ze yang completion --min-prefix 3 --json` to fail on bad names.

### D-6: RPC registrations as command source

The command tree comes from `pluginserver.RPCRegistration` structs, not YANG RPC names directly.
This is because `BuildTree()` uses `CLICommand` strings (e.g., `"bgp peer list"`) which may differ from YANG RPC names (e.g., `"peer-list"`).
The tool must use the same source as the runtime so collisions match what users experience.

### D-7: `ze yang doc` for command documentation

Command documentation comes from combining:
- `CLICommand` string (the user-facing syntax)
- `Help` string (one-line description)
- YANG RPC `Input`/`Output` leaves from `ExtractRPCs()` (parameter details)

This enables `ze yang doc peer list` to show full parameter documentation derived from YANG, not hand-written.

## Output Format Specification

### `ze yang completion` (text, default)

Each collision group shows: path in the unified tree, colliding siblings with their source domain tag, minimum disambiguation prefix. Groups are sorted by path depth then alphabetically.

Expected output format (illustrative, actual output determined by current YANG schemas):

```
bgp > peer > (3 siblings share prefix "l", need 2-5 chars)
  li  link-local     [config]  ipv6-address   IPv6 link-local address
  lo  local-address  [config]  ip-address     Local address for connection
  lo  local-as       [config]  asn            Local AS for this peer

bgp > peer > (4 siblings share prefix "a", need 2-4 chars)
  ad  add-path       [config]  list           Per-family ADD-PATH
  ad  adj-rib-in     [config]  boolean        Maintain Adj-RIB-In
  ad  adj-rib-out    [config]  boolean        Maintain Adj-RIB-Out
  au  auto-flush     [config]  boolean        Auto-flush routes

peer > (4 siblings share prefix "r", need 2-3 chars)
  ra  raw            [command]                Send raw bytes to peer
  re  refresh        [command]                Send ROUTE-REFRESH to peer
  re  remove         [command]                Remove a peer dynamically
  re  resume         [command]                Resume peer read loop

Summary: 5 collision groups, 15 affected nodes
```

The `[config]`/`[command]` tag shows which domain the node comes from. A `[both]` tag would appear if a name exists in both domains at the same tree level.

### `ze yang completion --json`

JSON output uses a flat list of collision groups (not split by domain, since the tree is unified). Each sibling carries its source tag.

Expected structure:

```
collisions: array of objects, each with:
  path: array of strings (tree path to the parent)
  prefix: string (shared prefix character(s))
  min-chars: integer (minimum chars to disambiguate)
  max-chars: integer (maximum chars needed for worst pair)
  siblings: array of objects, each with:
    name: string
    source: "config" or "command" or "both"
    type: string (YANG type or empty for commands)
    description: string
summary: object with total-groups, total-affected counts
```

### `ze yang tree` (text)

Shows the unified tree. Each node is tagged with its source domain. Config nodes show YANG type info. Command nodes show "(cmd)" and their help text. `--commands` flag filters to command nodes only. `--config` flag filters to config nodes only. Default shows both.

Expected format (illustrative):

```
bgp                           [config]  container    BGP configuration
  router-id                   [config]  ipv4-address BGP Router ID (required) [mandatory]
  local-as                    [config]  asn          Local ASN (required) [mandatory]
  listen                      [config]  string       Listen address and port
  group                       [config]  list[name]   Peer group
    peer                      [config]  list[address] BGP peer in this group
      hold-time               [config]  uint16       Hold time (0|3..65535) [default: 90]
      ...
  peer                        [both]    list/cmd     BGP peer
    list                      [command]              List peer(s) (brief)
    detail                    [command]              Peer details
    add                       [command]              Add a peer dynamically
    hold-time                 [config]  uint16       Hold time (0|3..65535) [default: 90]
    ...
  cache                       [command]              BGP message cache operations
  summary                     [command]              Show BGP summary
```

### `ze yang doc <command>` (text)

```
bgp peer list
  List configured peers (read-only)

  Parameters:
    selector    peer-selector    Peer filter (optional)

  Output:
    peer        list
      address   ip-address       Peer address
      asn       asn              Peer ASN
      state     string           FSM state
```

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

Not applicable -- this is a CLI analysis tool, not protocol work.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze/yang/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
