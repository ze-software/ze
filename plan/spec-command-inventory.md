# Spec: command-inventory

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `scripts/inventory.go` - existing inventory tool pattern
4. `internal/component/plugin/server/command.go` - RPC registration
5. `internal/component/plugin/server/handler.go` - streaming handler registry

## Task

Create `make ze-command-list` (and `make ze-command-list-json`) that generates a complete inventory of all registered commands. This serves two purposes:

1. **Documentation:** Generate the command tree for `docs/architecture/api/commands.md` so it stays accurate automatically instead of being hand-maintained.
2. **User discovery:** Users can run `ze command list` to find available commands, their verbs, help text, and read-only status.

The tool follows the same pattern as `scripts/inventory.go`: a build-time Go script that imports real registrations (no regex) and outputs markdown or JSON.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command verb taxonomy and dispatch
  -> Constraint: verb-first convention `<action> <module>` (show, set, del, update, monitor)
  -> Decision: commands categorized by verb, module implements action
- [ ] `scripts/inventory.go` - existing inventory pattern (imports, output, Makefile target)
  -> Constraint: same structure -- `go run scripts/command_inventory.go [--json]`

### Learned Summaries
- [ ] `plan/learned/431-update-verb.md` - verb-first CLI taxonomy
  -> Decision: `update` promoted to first-class verb alongside `show`, `set`, `del`
  -> Constraint: all user-facing commands follow verb-first pattern
- [ ] `plan/learned/395-yang-command-tree.md` - YANG command tree mapping
  -> Decision: `WireMethodToPath` maps wire methods to CLI paths
  -> Constraint: `-cmd.yang` modules define command containers

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/inventory.go` - collects plugin, YANG, RPC, test, and package data. Does NOT collect command-level data (only RPC counts per YANG module)
- [ ] `internal/component/plugin/server/command.go` - `AllBuiltinRPCs()` returns all registered RPCs with WireMethod, Help, ReadOnly, RequiresSelector
- [ ] `internal/component/plugin/server/handler.go` - streaming handler map is private, needs a `StreamingPrefixes()` accessor
- [ ] `internal/component/config/yang/command.go` - `WireMethodToPath()` maps wire methods to CLI paths
- [ ] `internal/component/cmd/show/`, `set/`, `del/`, `update/` - verb packages with YANG schemas

**Behavior to preserve:**
- `make ze-inventory` unchanged
- `AllBuiltinRPCs()` API unchanged
- Streaming handler registration unchanged

**Behavior to change:**
- Add `StreamingPrefixes()` to handler.go (expose streaming command prefixes)
- Add `scripts/command_inventory.go` for `make ze-command-list`
- Add `make ze-command-list` and `make ze-command-list-json` targets

## Data Flow (MANDATORY)

### Entry Point
- `make ze-command-list` runs `go run scripts/command_inventory.go`
- The script imports `plugin/all` (triggers init registrations) and queries registries

### Transformation Path
1. Import `plugin/all` to trigger all init() registrations
2. Load YANG modules via `yang.DefaultLoader()`
3. Query `pluginserver.AllBuiltinRPCs()` for all registered RPC handlers
4. Query `yang.WireMethodToPath(loader)` to map wire methods to CLI paths
5. Query `pluginserver.StreamingPrefixes()` for streaming commands (new accessor)
6. Classify each command by verb (show/set/del/update/monitor/other)
7. Output sorted table: verb, CLI path, wire method, help, read-only, source

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script to plugin server | `AllBuiltinRPCs()` import | [ ] |
| Script to YANG loader | `WireMethodToPath()` import | [ ] |
| Script to streaming registry | `StreamingPrefixes()` import | [ ] |

### Architectural Verification
- [ ] No bypassed layers (uses existing registries)
- [ ] No unintended coupling (build-time script, not imported by engine)
- [ ] No duplicated functionality (extends inventory pattern, doesn't replace it)

## Wiring Test (MANDATORY)

| Entry Point | via | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-command-list` | `go run scripts/command_inventory.go` | command_inventory.go | `test/plugin/command-inventory.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-command-list` | Outputs markdown table of all registered commands |
| AC-2 | `make ze-command-list-json` | Outputs JSON array with command metadata |
| AC-3 | Command table | Each row has: verb, CLI path, wire method, help, read-only flag |
| AC-4 | Verb classification | Commands classified by first keyword (show, set, del, update, monitor, other) |
| AC-5 | Streaming commands | Streaming handler prefixes appear in the output with source "streaming" |
| AC-6 | Sort order | Commands sorted by verb, then CLI path |
| AC-7 | New RPC added | Adding a new RPC via `RegisterRPCs()` automatically appears in output (no manual update) |
| AC-8 | Wire-to-path mapping | Commands with YANG `-cmd` modules show CLI path, not wire method |
| AC-9 | Commands without YANG path | Commands that have no YANG mapping show wire method as path |
| AC-10 | JSON output | JSON includes all fields: verb, path, wire-method, help, read-only, source |

## Output Format

### Markdown (default)

| Verb | CLI Path | Wire Method | Help | RO | Source |
|------|----------|-------------|------|----|--------|
| monitor | monitor bgp | (TUI) | Live peer dashboard | yes | cli |
| show | bgp peer list | ze-bgp:peer-list | List peer(s) (brief) | yes | builtin |
| show | bgp peer detail | ze-bgp:peer-detail | Peer details (config, state, counters) | yes | builtin |
| show | bgp summary | ze-bgp:summary | Show BGP summary | yes | builtin |
| - | event monitor | (streaming) | Stream live BGP events | yes | streaming |
| update | update bgp peer ... | ze-update:bgp-route | Announce/withdraw routes | no | builtin |

### JSON

Each command as an object with: verb, path, wire-method, help, read-only, source.

## Files to Modify

- `internal/component/plugin/server/handler.go` - ADD: `StreamingPrefixes()` accessor to expose streaming command prefix list
- `Makefile` - ADD: `ze-command-list` and `ze-command-list-json` targets

## Files to Create

- `scripts/command_inventory.go` - command inventory tool (same pattern as `inventory.go`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Makefile targets | [x] | `Makefile` |
| Streaming handler accessor | [x] | `handler.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | |

## Implementation Steps

### Implementation Phases

1. **Phase: StreamingPrefixes accessor** -- add `StreamingPrefixes() []string` to handler.go, returning sorted keys from the streaming handler map
   - Tests: `TestStreamingPrefixesReturnsRegistered`
   - Files: `handler.go`, `handler_test.go`

2. **Phase: Command inventory script** -- `scripts/command_inventory.go` that imports plugin/all, queries AllBuiltinRPCs + WireMethodToPath + StreamingPrefixes, classifies by verb, outputs markdown or JSON
   - Tests: manual `make ze-command-list` verification
   - Files: `scripts/command_inventory.go`

3. **Phase: Makefile targets** -- add `ze-command-list` and `ze-command-list-json`
   - Files: `Makefile`

4. **Phase: Verb classification** -- classify commands by first path component into show/set/del/update/monitor/other. TUI-only commands (monitor bgp) tagged as source "cli"
   - Tests: verify output includes all verb categories

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every registered RPC appears in output |
| Correctness | Wire-to-path mapping matches YANG |
| Naming | Verb classification matches documented taxonomy |
| Accuracy | `make ze-command-list` output matches `AllBuiltinRPCs()` count |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `make ze-command-list` outputs table | Run and check output |
| `make ze-command-list-json` outputs JSON | Run and parse with jq |
| All RPCs present | Compare count with `AllBuiltinRPCs()` |
| Streaming commands present | Check `event monitor` appears |
| Verb classification correct | Spot-check show/set/update categories |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Script takes no user input beyond --json flag |
| Resource exhaustion | Build-time tool, not runtime |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

The verb taxonomy (`<action> <module>`) is central to Ze's CLI identity. The command inventory tool makes this taxonomy machine-queryable, ensuring docs stay accurate and users can discover commands without reading source. The tool should be the authoritative source for the command tree in `docs/architecture/api/commands.md`.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
