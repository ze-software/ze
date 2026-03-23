# Spec: update-verb

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- current handler
3. `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- current YANG

## Task

Move `ze bgp peer * prefix update` to `ze update bgp peer * prefix`.
Establish `update` as a top-level verb for all "refresh stale data" actions.

**Design principle:** `update` is a verb, not buried inside a noun tree.
Like `ze set bgp ...`, `ze update bgp ...` makes the action prominent.
All data-refresh commands live under `ze update`, making them discoverable
(`ze update --help`) and consistent.

## Design

### Command Syntax

| Current | New |
|---------|-----|
| `ze bgp peer * prefix update` | `ze update bgp peer * prefix` |
| `peer * prefix update` (RPC) | `update bgp peer * prefix` (RPC) |

### YANG Structure

New module `ze-update-cmd.yang`:

    module ze-update-cmd {
        container update {
            description "Refresh stale data from external sources";
            container bgp {
                container peer {
                    description "Update BGP peer data";
                    container prefix {
                        ze:command "ze-update:bgp-peer-prefix";
                        description "Update prefix maximums from PeeringDB";
                    }
                }
            }
        }
    }

The `update` container is the top-level verb. Subcontainers organize
by domain (bgp) and target (peer, prefix). Future update commands
(RPKI, IRR) add sibling containers under `update`.

### Wire Method

| Current | New |
|---------|-----|
| `ze-bgp:peer-prefix-update` | `ze-update:bgp-peer-prefix` |

The `ze-update:` prefix groups all update RPCs. The handler code
does not change -- only the wire method name and YANG location.

### Package Location

New package: `internal/component/cmd/update/`

| File | Purpose |
|------|---------|
| `update.go` | RPC registration (init), imports handler |
| `schema/ze-update-cmd.yang` | YANG command tree |
| `schema/embed.go` | Embed YANG |
| `schema/register.go` | Register YANG module |

The handler stays in `plugins/cmd/peer/prefix_update.go` (proximity:
the logic is BGP-peer specific). Only the RPC registration moves.

### What Gets Removed

- `ze-peer-cmd.yang`: remove `prefix { update { } }` block
- `prefix_update.go`: remove `init()` with old RPC registration
- `peer_test.go`: RPC count 14 -> 13

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- handler + init() registration
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- prefix update YANG
- [ ] `internal/component/bgp/plugins/cmd/peer/peer_test.go` -- RPC count assertion
- [ ] `test/plugin/api-peer-prefix-update.ci` -- dispatch command string

**Behavior to preserve:**
- Handler logic (PeeringDB query, margin, config update)
- Peer selector matching (* / IP / AS / name)
- Rate limiting, error handling, suspicious detection

**Behavior to change:**
- CLI command path: `peer * prefix update` -> `update bgp peer * prefix`
- Wire method: `ze-bgp:peer-prefix-update` -> `ze-update:bgp-peer-prefix`
- YANG location: peer-cmd.yang -> update-cmd.yang
- Reserved peer name: remove "prefix" (no longer a peer subcommand)

## Data Flow (MANDATORY)

### Entry Point
1. Operator types `ze update bgp peer * prefix`
2. CLI dispatches via YANG tree: update -> bgp -> peer -> [selector] -> prefix
3. Wire method `ze-update:bgp-peer-prefix` resolved
4. Handler `handleBgpPeerPrefixUpdate` executes (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> YANG dispatch | update container in ze-update-cmd.yang | [ ] |
| YANG -> RPC handler | ze:command annotation maps to wire method | [ ] |
| RPC handler -> PeeringDB | Unchanged from spec-prefix-data | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze update bgp peer * prefix` via dispatch | -> | PeeringDB query + config update | test/plugin/api-peer-prefix-update.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `update bgp peer * prefix` dispatched | Handler executes, PeeringDB queried, config updated |
| AC-2 | `peer * prefix update` (old path) dispatched | Error: unknown command (old path removed) |
| AC-3 | `update --help` or `update` with no args | Shows available update subcommands (bgp peer prefix) |
| AC-4 | `update bgp peer AS65001 prefix` | Selector filters to matching peers |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestUpdateRPCRegistered | update_test.go | Wire method registered | [ ] |
| TestOldPathRemoved | peer_test.go | RPC count decremented | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| api-peer-prefix-update | test/plugin/api-peer-prefix-update.ci | Dispatch via new path | [ ] |

## Files to Modify

- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- remove init() RPC registration
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- remove prefix update block
- `internal/component/bgp/plugins/cmd/peer/peer_test.go` -- RPC count 14 -> 13
- `internal/component/bgp/config/resolve.go` -- remove "prefix" from reservedPeerNames (no longer peer subcommand)
- `test/plugin/api-peer-prefix-update.ci` -- change dispatch command
- `docs/guide/command-reference.md` -- move to Update Commands section
- `docs/features.md` -- update command syntax

## Files to Create

- `internal/component/cmd/update/update.go` -- RPC registration
- `internal/component/cmd/update/schema/ze-update-cmd.yang` -- YANG
- `internal/component/cmd/update/schema/embed.go` -- embed
- `internal/component/cmd/update/schema/register.go` -- register

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (move, not new) | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- move to Update Commands |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- new wire method |
| 5-12 | Other | No | |

## Implementation Steps

### Implementation Phases

| Phase | What |
|-------|------|
| 1 | Create update package with YANG + embed + register + RPC registration |
| 2 | Remove old registration from prefix_update.go and ze-peer-cmd.yang |
| 3 | Update .ci test dispatch command |
| 4 | Update docs |

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Old path removed | `peer * prefix update` returns error |
| New path works | `update bgp peer * prefix` dispatches correctly |
| Handler unchanged | Same PeeringDB logic, same response format |
| YANG registered | `ze update --help` shows prefix subcommand |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| New command path works | .ci test |
| Old path removed | RPC count test |
| YANG module loaded | make ze-verify |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| No new attack surface | Same handler, different dispatch path |

### Failure Routing

| Failure | Route To |
|---------|----------|
| YANG not loading | Check register.go blank import chain |
| Dispatch not finding handler | Check ze:command annotation matches wire method |

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

### Verb-First CLI Taxonomy (Next Steps)

This spec establishes `update` as the first top-level verb. The dispatcher was
generalized to extract `peer <selector>` at any position in the command path
(not just position 0). This enables migrating all peer commands to verb-first.

**Four verbs:**

| Verb | Semantics | When to use |
|------|-----------|-------------|
| `show` | Read-only display | Inspecting state, no side effects |
| `set` | Change configuration | Adding, removing, saving config state |
| `update` | Refresh from external source | PeeringDB, RPKI, IRR data refresh |
| `run` | Execute an action (side effect) | Teardown, pause, resume, flush |

**Migration table:**

| Current | Verb-first | Verb |
|---------|-----------|------|
| `peer * list` | `show bgp peer * list` | show |
| `peer * detail` | `show bgp peer * detail` | show |
| `peer * capabilities` | `show bgp peer * capabilities` | show |
| `peer * statistics` | `show bgp peer * statistics` | show |
| `bgp summary` | `show bgp summary` | show |
| `peer * add` | `set bgp peer * add` | set |
| `peer * remove` | `set bgp peer * remove` | set |
| `peer * save` | `set bgp peer * save` | set |
| `peer * teardown` | `run bgp peer * teardown` | run |
| `peer * pause` | `run bgp peer * pause` | run |
| `peer * resume` | `run bgp peer * resume` | run |
| `peer * flush` | `run bgp peer * flush` | run |
| `update bgp peer * prefix` | (already done) | update |

**Dispatcher ready:** `extractPeerSelector` in `command.go` now scans all
positions for `peer <selector>`, so all paths above work without further
dispatcher changes. Each verb needs a `ze-cli-<verb>-cmd.yang` module and
a package under `internal/component/cmd/<verb>/`.

## Implementation Summary

### What Was Implemented

### Deviations from Plan

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
