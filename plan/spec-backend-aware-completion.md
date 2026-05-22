# Spec: Backend-Aware CLI Completion

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-backend-command-dispatch.md (complementary, not blocking) |
| Phase | 1/9 |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - YANG config design
4. `internal/component/cli/completer.go` - config editor completion
5. `internal/component/command/completer.go` - operational command completion
6. `internal/component/config/backend_gate.go` - existing backend gate logic
7. `internal/component/config/yang/command.go` - command tree building from YANG

## Task

Filter CLI auto-completion based on active backend so users never see options their backend does not support. Both config editor mode (set/delete/edit/show) and operational command mode (show/clear/monitor) must hide nodes annotated with `ze:backend` when the active backend is not in the annotation's list.

The `ze:backend` YANG extension already exists and drives commit-time validation via `backend_gate.go`. This spec extends that to completion time: same annotations, earlier feedback.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG extensions, config editor design
  -> Constraint: YANG defines format, extensions declare behavior, implementation executes. ze:backend is a custom extension alongside ze:validate, ze:command, ze:sensitive.
  -> Decision: Config schemas (-conf.yang) drive CLI autocomplete; API schemas (-cmd.yang) drive command tree. Both are separate loading paths.
- [ ] `docs/architecture/api/commands.md` - command tree structure
  -> Constraint: Commands use verb-first syntax (show/clear/monitor). Command tree built from -cmd.yang modules via BuildCommandTree.

### Source Files
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - ze:backend extension definition
  -> Constraint: ze:backend argument is space-separated list of backend names. Absent = unrestricted (every backend accepted). Consumers match on Keyword == "ze:backend" OR strings.HasSuffix ":backend".
- [ ] `internal/component/config/yang_schema.go` - getBackendExtension (unexported)
  -> Constraint: getBackendExtension is unexported, reads entry.Exts, space-splits argument, deduplicates. Returns nil = unrestricted. Must be promoted to yang package as GetBackendExtension.
- [ ] `internal/component/config/backend_gate.go` - existing backend gate walker
  -> Decision: Commit-time validation, not schema-build-time pruning. Unlike ze:os (which prunes at build time because GOOS is immutable), ze:backend is runtime-dependent.
  -> Constraint: Narrowest-annotation-wins: child annotation overrides parent. The completer filtering should follow the same principle.
- [ ] `internal/component/config/schema.go` - Backend field on schema nodes
  -> Constraint: Backend []string present on LeafNode, ContainerNode, ListNode, InlineListNode, FlexNode. Nil = unrestricted.
- [ ] `internal/component/cli/completer.go` - config editor completion
  -> Constraint: Completer holds *yang.Loader, *config.Tree, *ValidatorRegistry. matchChildren() and matchEditTargets() are the two methods that enumerate YANG children for suggestions. Both iterate entry.Dir and build Completion slices.
  -> Decision: getEntry() returns raw goyang *gyang.Entry, which has .Exts containing all YANG extensions. Backend info accessible there.
  -> Constraint: SetTree() called from model_commands.go after tree changes (commit, save, discard, rollback). Backend name map must be re-derived at SetTree time.
- [ ] `internal/component/cli/completer_command.go` - command mode completion adapter
  -> Constraint: Thin adapter: wraps command.TreeCompleter, converts command.Suggestion to Completion. Backend filtering belongs in TreeCompleter, not the adapter.
- [ ] `internal/component/command/completer.go` - TreeCompleter
  -> Constraint: matchChildren() iterates node.Children map + DynamicChildren + ValueHints. Filtering point for backend-aware command completion.
- [ ] `internal/component/command/node.go` - command.Node type
  -> Constraint: Node has Name, Description, WireMethod, TaskSupport, Children, DynamicChildren, ValueHints. No Backend field. Adding Backend []string follows the TaskSupport pattern.
- [ ] `internal/component/config/yang/command.go` - BuildCommandTree, mergeYANGEntry
  -> Constraint: mergeYANGEntry reads ze:command and ze:task-support from entry.Exts. Adding ze:backend read is ~3 lines, same pattern.
- [ ] `internal/component/config/yang/validator_registry.go` - GetValidateExtension pattern
  -> Decision: GetValidateExtension is the exported pattern: reads entry.Exts, matches keyword suffix. GetBackendExtension should follow the same shape.
- [ ] `internal/component/iface/backend.go` - backend registration, DefaultBackendName
  -> Constraint: DefaultBackendName() exposed via default_linux.go/default_other.go. No exported ActiveBackendName() -- only GetBackend() returning the Backend interface.
- [ ] `internal/component/iface/config.go` - backend config parsing
  -> Constraint: ifaceConfig.Backend string, defaults to defaultBackendName. Config path: interface > backend.
- [ ] `internal/component/firewall/backend.go` - firewall backend registration
  -> Decision: firewall has ActiveBackendName() (atomic.Value string). Config path: firewall > backend. Default: "nft".
- [ ] `internal/component/traffic/backend.go` - traffic backend registration
  -> Constraint: DefaultBackendName() exposed. No ActiveBackendName(). Config path: traffic-control > backend. Default: "tc".
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - ze:backend annotations
  -> Constraint: 9 nodes annotated with ze:backend "netlink" (dhcp, tunnel, mirror, vlan, bridge, dummy, veth, macvlan, link-monitoring).
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` - ze:backend annotations
  -> Constraint: 7 nodes annotated with ze:backend "nft" (connection-state, flow-offload, flowtable, counter, quota, limit, log-prefix).

### Learned Summaries
- [ ] `plan/learned/410-validate-completion.md` - ze:validate CompleteFn wiring (same pattern)
  -> Decision: ValidatorRegistry created in NewCompleter(). validateCompletions() reads ze:validate from entry.Exts using GetValidateExtension. Same entry.Exts access pattern for ze:backend.
  -> Constraint: List keys use listKeyCompletions(), a different code path from valueCompletions(). Backend filtering applies to container/list visibility, not leaf values, so this distinction is irrelevant.
- [ ] `plan/learned/383-command-package-extraction.md` - command package structure
  -> Decision: command.Node tree and TreeCompleter live in internal/component/command/. Editor delegates via thin adapter. Backend filtering belongs in TreeCompleter.matchChildren().

**Key insights:**
- ze:backend already exists on ~16 YANG config nodes and drives commit-time validation
- Completer uses raw goyang entries (entry.Exts available) for config mode, command.Node for op mode
- getBackendExtension is unexported in yang_schema.go; promote to yang package
- Three components have backends: iface (netlink/vpp), firewall (nft/vpp), traffic (tc/vpp)
- Backend name comes from config tree leaves (interface/backend, firewall/backend, traffic-control/backend)
- Filtering points: matchChildren() and matchEditTargets() for config, TreeCompleter.matchChildren() for commands
- SetTree() is called on every tree change; re-derive backend map there
- narrowest-annotation-wins must apply in completion too (child with ze:backend overrides filtered parent)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/completer.go` - matchChildren (L641), matchEditTargets (L677)
  -> Constraint: Both iterate entry.Dir children and build Completion slices. No backend filtering. All YANG children shown regardless of active backend.
- [ ] `internal/component/command/completer.go` - matchChildren (L209)
  -> Constraint: Iterates node.Children map. No backend filtering. All command tree children shown.
- [ ] `internal/component/config/yang/command.go` - mergeYANGEntry (L159)
  -> Constraint: Reads ze:command and ze:task-support from entry.Exts. Does not read ze:backend.
- [ ] `internal/component/cli/completer_test.go` - existing test patterns
  -> Constraint: Tests use NewCompleter() with loaded YANG. SetTree with config.Tree for data-aware tests. Must not break existing tests.
- [ ] `internal/component/command/completer_test.go` - testCommandTree()
  -> Constraint: Tests build manual command.Node trees. Adding Backend field does not break existing tests (nil = unrestricted).

**Behavior to preserve:**
- All existing completions when no ze:backend annotation is present (nil = unrestricted)
- Config editor completion flow: tokenize, dispatch by command, walk YANG schema
- Command mode completion flow: walk command.Node tree, pipe operator completion
- Ghost text behavior (single-match inline completion)
- List key completions (separate code path, not affected by backend filtering)
- Value completions (enum, boolean, ze:validate) unchanged
- Pipe operator completions unchanged
- Commit-time backend_gate.go validation remains the authoritative enforcement

**Behavior to change:**
- Config editor: matchChildren() and matchEditTargets() must skip YANG children whose ze:backend annotation excludes the active backend
- Command mode: TreeCompleter.matchChildren() must skip command.Node children whose Backend field excludes the active backend
- Command tree building: mergeYANGEntry() must read ze:backend from YANG entries and store on command.Node

## Data Flow (MANDATORY)

### Entry Point
- Config editor: user types in CLI input field, Model dispatches to Completer.Complete(input, contextPath)
- Command mode: user types in command input field, CommandCompleter.Complete(input) delegates to TreeCompleter.Complete(input)

### Transformation Path

#### Config Completion (edit mode)
1. User keystroke -> Model.Update() -> completer.Complete(input, contextPath)
2. Completer tokenizes input, dispatches by command (set/delete/edit/show)
3. completeSetPath / matchChildren / matchEditTargets walk YANG entry.Dir
4. **NEW:** For each child entry, call GetBackendExtension(child). If non-nil and active backend not in list, skip.
5. Return filtered Completion slice to editor

#### Command Completion (operational mode)
1. User keystroke -> Model.Update() -> commandCompleter.Complete(input)
2. CommandCompleter delegates to TreeCompleter.Complete(input)
3. TreeCompleter navigates command.Node tree by completed words
4. matchChildren iterates node.Children map
5. **NEW:** For each child node, check node.Backend. If non-nil and active backend not in list, skip.
6. Return filtered Suggestion slice

#### Backend Name Resolution
1. Completer.SetTree(tree) called (on editor init, commit, save, discard, rollback)
2. **NEW:** Walk tree to read backend leaf values: tree.GetContainer("interface").Get("backend"), same for firewall and traffic-control
3. Store in backends map[string]string keyed by component root name
4. matchChildren uses the map to determine which backend is active for the current path

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI editor -> Completer | Complete(input, contextPath) call | [ ] |
| Completer -> goyang entry | getEntry() returns *gyang.Entry with .Exts | [ ] |
| Completer -> config.Tree | navigateTreeToPath() + Get("backend") for active name | [ ] |
| CLI editor -> CommandCompleter | Complete(input) call | [ ] |
| CommandCompleter -> TreeCompleter | Delegation, Suggestion -> Completion conversion | [ ] |
| YANG module -> command.Node | mergeYANGEntry reads ze:backend from entry.Exts | [ ] |

### Integration Points
- `internal/component/config/yang/` - new GetBackendExtension (exported helper)
- `internal/component/cli/completer.go` - backend map field, filtering in matchChildren/matchEditTargets
- `internal/component/command/node.go` - Backend field
- `internal/component/command/completer.go` - backend filtering in matchChildren
- `internal/component/config/yang/command.go` - read ze:backend in mergeYANGEntry

### Architectural Verification
- [ ] No bypassed layers (completion uses same ze:backend data as commit-time gate)
- [ ] No unintended coupling (completer reads goyang entry.Exts directly; no import of iface/firewall/traffic packages)
- [ ] No duplicated functionality (extends existing completion, does not recreate backend gate)
- [ ] Zero-copy preserved where applicable (string slices, no allocations on hot path beyond what completer already does)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Completer.Complete("set ", ["interface"]) with backend="vpp" | -> | matchChildren filters by ze:backend | TestCompleterBackendFiltersChildren |
| Completer.Complete("edit ", ["interface"]) with backend="vpp" | -> | matchEditTargets filters by ze:backend | TestCompleterBackendFiltersEditTargets |
| TreeCompleter.Complete("show interface ") with Backend on node | -> | matchChildren filters by Backend | TestCommandCompleterBackendFilter |
| BuildCommandTree from YANG with ze:backend | -> | mergeYANGEntry stores Backend | TestBuildCommandTreeBackend |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config completer with backend "vpp", YANG child has ze:backend "netlink" | Child excluded from completions |
| AC-2 | Config completer with backend "netlink", YANG child has ze:backend "netlink" | Child included in completions |
| AC-3 | Config completer, YANG child has no ze:backend annotation | Child included regardless of active backend |
| AC-4 | Config completer with backend "vpp", YANG child has ze:backend "netlink vpp" | Child included (multi-backend annotation) |
| AC-5 | Config tree has no backend leaf set (empty/new config) | Use platform default backend name; completions filtered accordingly |
| AC-6 | Command tree node has Backend ["netlink"], active backend is "vpp" | Node excluded from command completions |
| AC-7 | Command tree node has nil Backend (no annotation) | Node included regardless of active backend |
| AC-8 | GetBackendExtension called on entry without ze:backend | Returns nil (unrestricted) |
| AC-9 | GetBackendExtension called on entry with ze:backend "netlink" | Returns ["netlink"] |
| AC-10 | GetBackendExtension called on entry with ze:backend "netlink vpp" | Returns ["netlink","vpp"], deduplicated |
| AC-11 | All existing completer tests run | No regression in current completion behavior |
| AC-12 | `show vpp` subtree in ze-cli-show-cmd.yang | Annotated with ze:backend "vpp"; hidden when iface backend is netlink |
| AC-13 | `show ip route` and `show kernel-routes` YANG descriptions | Fix stale "rejects on VPP" text. VPP backend already implements ListKernelRoutes via ip_route_v2_dump (fib.go). No ze:backend annotation needed; both backends work. |
| AC-14 | `show firewall ruleset` and `show firewall group` in ze-cli-show-cmd.yang | Annotated with ze:backend "nft"; hidden when firewall backend is not nft |
| AC-15 | Interface create-dummy/create-veth/create-bridge in ze-iface-cmd.yang | Annotated with ze:backend "netlink"; hidden when iface backend is vpp |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestGetBackendExtension_Absent | `internal/component/config/yang/command_test.go` | AC-8: nil for no annotation | |
| TestGetBackendExtension_Single | `internal/component/config/yang/command_test.go` | AC-9: ["netlink"] for single | |
| TestGetBackendExtension_Multi | `internal/component/config/yang/command_test.go` | AC-10: multi-backend, dedup | |
| TestCompleterBackendFiltersChildren | `internal/component/cli/completer_test.go` | AC-1: excluded on mismatch | |
| TestCompleterBackendIncludesMatching | `internal/component/cli/completer_test.go` | AC-2: included on match | |
| TestCompleterBackendUnrestricted | `internal/component/cli/completer_test.go` | AC-3: included when nil | |
| TestCompleterBackendMulti | `internal/component/cli/completer_test.go` | AC-4: included on multi match | |
| TestCompleterBackendDefaultFromTree | `internal/component/cli/completer_test.go` | AC-5: reads default when absent | |
| TestCompleterBackendFiltersEditTargets | `internal/component/cli/completer_test.go` | AC-1 via edit targets path | |
| TestCommandCompleterBackendFilter | `internal/component/command/completer_test.go` | AC-6: excluded on mismatch | |
| TestCommandCompleterBackendUnrestricted | `internal/component/command/completer_test.go` | AC-7: included when nil | |
| TestBuildCommandTreeBackend | `internal/component/config/yang/command_test.go` | mergeYANGEntry stores Backend | |

### Boundary Tests
No numeric inputs; not applicable.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-backend-completion-filter | `test/editor/*.ci` | Config editor with VPP backend does not show netlink-only options in tab completion | |

### Interop Tests
N/A - this is a CLI/UX feature, not a protocol feature.

### Future
- Consider dimmed/grayed completions instead of hiding (lower priority, requires Completion type change).
- Annotate additional -cmd.yang nodes as backend-specific features are added.

## Files to Modify

- `internal/component/config/yang/command.go` - add GetBackendExtension; read ze:backend in mergeYANGEntry
- `internal/component/config/yang_schema.go` - refactor getBackendExtension to delegate to yang.GetBackendExtension
- `internal/component/command/node.go` - add Backend []string field to Node
- `internal/component/command/completer.go` - add activeBackends map, filter in matchChildren
- `internal/component/cli/completer.go` - add backends map, derive on SetTree, filter in matchChildren and matchEditTargets
- `internal/component/config/yang/command_test.go` - tests for GetBackendExtension
- `internal/component/cli/completer_test.go` - tests for backend filtering
- `internal/component/command/completer_test.go` - tests for command backend filtering
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add ze:backend annotations to backend-specific show commands
- `internal/component/iface/schema/ze-iface-cmd.yang` - add ze:backend annotations to netlink-only interface commands

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | Yes | `internal/component/cli/completer.go` |
| Functional test for new RPC/API | No | |
| Doctor check for runtime dependencies | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - mention backend-aware completion |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` - document ze:backend completion filtering |

## Files to Create
- No new files; all changes are to existing files

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint && make ze-unit-test && make ze-functional-test |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases
Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- export GetBackendExtension, add Backend field, write failing wiring tests
   - Tests: TestGetBackendExtension_Absent, TestGetBackendExtension_Single, TestGetBackendExtension_Multi, TestCompleterBackendFiltersChildren (failing), TestCommandCompleterBackendFilter (failing)
   - Files: `yang/command.go` (GetBackendExtension), `command/node.go` (Backend field), `yang/command_test.go`, `cli/completer_test.go`, `command/completer_test.go`
   - Verify: GetBackendExtension tests pass; completer tests fail (filtering not implemented yet)

2. **Phase: Config completer filtering** -- add backends map to Completer, derive on SetTree, filter in matchChildren and matchEditTargets
   - Tests: TestCompleterBackendFiltersChildren, TestCompleterBackendIncludesMatching, TestCompleterBackendUnrestricted, TestCompleterBackendMulti, TestCompleterBackendDefaultFromTree, TestCompleterBackendFiltersEditTargets
   - Files: `cli/completer.go`
   - Verify: All config completer backend tests pass; existing tests pass unchanged

3. **Phase: Command tree backend** -- read ze:backend in mergeYANGEntry, store on command.Node, filter in TreeCompleter.matchChildren
   - Tests: TestBuildCommandTreeBackend, TestCommandCompleterBackendFilter, TestCommandCompleterBackendUnrestricted
   - Files: `yang/command.go` (mergeYANGEntry), `command/completer.go` (matchChildren filter)
   - Verify: All command backend tests pass; existing command tests pass unchanged

4. **Phase: Refactor getBackendExtension** -- make yang_schema.go delegate to yang.GetBackendExtension
   - Tests: existing yang_schema_test.go backend tests still pass
   - Files: `config/yang_schema.go`
   - Verify: TestGetBackendExtension_* still pass via delegation

5. **Phase: YANG command annotations** -- add ze:backend to backend-specific command nodes
   - Files: `cmd/show/schema/ze-cli-show-cmd.yang`, `iface/schema/ze-iface-cmd.yang`
   - Annotations:
     - `show vpp` subtree: ze:backend "vpp" (AC-12)
     - `show ip route`, `show kernel-routes`: fix stale YANG descriptions that say "rejects on VPP" (AC-13). No ze:backend annotation; both backends work via ListKernelRoutes.
     - `show firewall ruleset`, `show firewall group`: ze:backend "nft" (AC-14)
     - `show system conntrack`: ze:backend "nft"
     - `interface create-dummy`, `create-veth`, `create-bridge`: ze:backend "netlink" (AC-15)
   - Verify: YANG modules still load (make ze-unit-test); BuildCommandTree populates Backend on annotated nodes

6. **Functional tests** -- create editor functional test verifying completion filtering
7. **Documentation** -- update features.md and yang-config-design.md
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | nil Backend means unrestricted (never filtered); non-nil filters correctly |
| Naming | GetBackendExtension follows GetValidateExtension/GetCommandExtension naming |
| Data flow | Backend names derived from config tree, not component imports |
| Regression | All existing completer and command completer tests pass unchanged |
| Rule: no-coupling | Completer does NOT import iface/firewall/traffic packages |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| GetBackendExtension exported in yang package | `grep -n 'func GetBackendExtension' internal/component/config/yang/command.go` |
| Backend field on command.Node | `grep -n 'Backend' internal/component/command/node.go` |
| Config completer filters by backend | `grep -n 'GetBackendExtension\|backends' internal/component/cli/completer.go` |
| Command completer filters by backend | `grep -n 'Backend\|activeBackend' internal/component/command/completer.go` |
| mergeYANGEntry reads ze:backend | `grep -n 'GetBackendExtension\|Backend' internal/component/config/yang/command.go` |
| All tests pass | `make ze-unit-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Backend names from config tree are trusted (config already validated by YANG). No external input. |
| Information leakage | Filtering hides features, does not expose them. No security concern. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, DESIGN; if correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

- `show ip route` and `show kernel-routes` already work on VPP: the VPP backend implements ListKernelRoutes via ip_route_v2_dump in fib.go. The YANG descriptions claiming "rejects on VPP" are stale.
- Cross-plugin dependency problem identified: `cmd/show/vpp_trace.go` imports the `vpp` package directly, creating compile-time coupling. A separate spec (spec-backend-command-dispatch.md) will address this via handler registration from backend packages. The completion spec builds the filtering UX; the dispatch spec builds the runtime mechanism.

## RFC Documentation
N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled during implementation)

### Documentation Updates
- (to be filled during implementation)

### Deviations from Plan
- (to be filled during implementation)

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/752-backend-aware-completion.md`
- [ ] Summary included in commit
