# Spec: Text Command Format Unification

## Task

Refactor the plugin→engine text command format (`update text`) to use the unified text grammar from `spec-utp-0-umbrella.md`. The command parser must share the same tokenizer and key-dispatch logic as the event parser.

Parent spec: `spec-utp-0-umbrella.md`.
Depends on: `spec-utp-1-event-format.md` (unified grammar established first).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/text-format.md` — unified grammar reference
  → Decision:
- [ ] `docs/architecture/api/text-parser.md` — shared parser design
  → Decision:
- [ ] `docs/architecture/api/process-protocol.md` — command dispatch architecture
  → Constraint:
- [ ] `docs/architecture/api/commands.md` — command vocabulary
  → Constraint:

### Source Files
- [ ] `internal/plugins/bgp/handler/update_text.go` — current command parser (source of truth)
- [ ] `internal/plugins/bgp/handler/update_text_nlri.go` — NLRI section parsing
- [ ] `internal/plugins/bgp/handler/update_text_evpn.go` — EVPN command parsing
- [ ] `internal/plugins/bgp/handler/update_text_flowspec.go` — FlowSpec command parsing
- [ ] `internal/plugins/bgp/handler/update_text_vpls.go` — VPLS command parsing
- [ ] `internal/plugin/command.go` — tokenizer + dispatcher
- [ ] `internal/plugins/bgp/handler/update_wire.go` — hex/b64 mode (not changed, but boundary)

### RFC Summaries (not protocol work — N/A)

## Current Behavior (MANDATORY)

### Command Format Today

The `update text` command uses an accumulator-based parser:
- Attributes: `origin set igp`, `med set 100`, `as-path set [65001 65002]`
- Actions: `set` (replace), `add` (prepend), `del` (remove/clear)
- NLRI: `nlri <family> add <prefix>...` / `nlri <family> del <prefix>...`
- Accumulators: `nhop set <ip>`, `path-information set <id>`, `rd set <value>`, `label set <value>`
- Lists use brackets: `[65001 65002]` or `[65001,65002]` or `65001,65002`
- Extra: `watchdog set <name>`, `eor`

### Key Differences From Event Format

| Aspect | Events (current) | Commands (current) | Unified Target |
|--------|-----------------|-------------------|----------------|
| Attribute actions | None (flat reporting) | `set`/`add`/`del` | Commands keep actions, events stay flat |
| List format | Brackets + spaces | Brackets + spaces/commas | Commas, no brackets |
| Next-hop | Per-family `next-hop <ip>` | Top-level `nhop set <ip>` | (to be decided) |
| Path-ID | In NLRI string | Accumulator `path-information set` | `nlri path-id <id> add` |
| VPN RD | In NLRI string | Accumulator `rd set` | (to be decided) |
| Family | Positional | Explicit `nlri <family>` | Explicit `family` / `nlri` |
| Tokenizer | `strings.Fields()` | Custom tokenizer (quoted strings) | Shared tokenizer |

## Data Flow (MANDATORY)

### Entry Points
- CLI: `ze cli --run "bgp peer * update text ..."` → JSON-RPC → `handleUpdate()`
- Plugin: `ze-plugin-engine:update-route` RPC → `handleUpdateRouteRPC()` → `Dispatch()`

### Transformation Path
1. User/plugin sends text command string
2. `command.go:tokenize()` splits into tokens (handles quotes)
3. `handleUpdate()` dispatches by encoding (`text`/`hex`/`b64`)
4. `ParseUpdateText()` parses tokens into `UpdateTextResult`
5. `DispatchNLRIGroups()` sends to reactor
6. Reactor builds wire UPDATE, sends to peers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → JSON-RPC | `ze-bgp:peer-update` method | [ ] |
| Plugin → JSON-RPC | `ze-plugin-engine:update-route` method | [ ] |
| Command parser → reactor | `AnnounceNLRIBatch()` / `WithdrawNLRIBatch()` | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (to be filled) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Command tokenizer | Shared with event parser (same function/package) |
| AC-2 | List format | Comma-separated, no brackets: `as-path set 65001,65002` |
| AC-3 | NLRI operations | Same `nlri <family> add/del` syntax as event format |
| AC-4 | Complex NLRI | Same dict mode as events: `nlri ipv4/vpn add rd 65000:100 prefix 10.0.0.0/24` |
| AC-5 | Path-ID | `nlri path-id 42 add` modifier (same as events) |
| AC-6 | Accumulator semantics | `set`/`add`/`del` preserved (command-specific extension) |
| AC-7 | Backward compat | Old bracket syntax `[65001 65002]` still accepted during transition |
| AC-8 | Existing tests | All command-related tests still pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (to be filled — update ParseUpdateText tests) | | | |

### Functional Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (to be filled — CLI update commands, plugin update-route RPC) | | | |

## Files to Modify
- `internal/plugins/bgp/handler/update_text.go` — main command parser
- `internal/plugins/bgp/handler/update_text_nlri.go` — NLRI section parsing
- `internal/plugins/bgp/handler/update_text_evpn.go` — EVPN
- `internal/plugins/bgp/handler/update_text_flowspec.go` — FlowSpec
- `internal/plugins/bgp/handler/update_text_vpls.go` — VPLS
- `internal/plugin/command.go` — tokenizer (extract to shared package)
- Tests for all above

## Files to Create
- Shared tokenizer package (location TBD — possibly `internal/plugin/bgp/shared/textparse.go`)

## Implementation Steps

1. Extract shared tokenizer from `command.go` to shared package
2. TDD: Update command parser tests for new list format
3. Refactor list parsing: brackets → commas (accept both during transition)
4. Refactor NLRI section: align with event format `nlri <family> add/del`
5. Refactor path-id: accumulator → `nlri path-id <id> add` modifier
6. Wire shared tokenizer into both event parser and command parser
7. Run full test suite

### Failure Routing
| Failure | Route To |
|---------|----------|
| CLI commands break | Bracket tolerance layer missing |
| Plugin update-route fails | Trace tokenizer change |
| FlowSpec/EVPN parsing breaks | Family-specific parsers need update |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Documentation Updates
- (to be filled after implementation)

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

## Checklist

### Goal Gates
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] Architecture docs updated

### Quality Gates
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit

### Verification
- [ ] `make ze-lint` passes
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-utp-2-command-format.md`
- [ ] Spec included in commit
