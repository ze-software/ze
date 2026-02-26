# Spec: Unified Text Protocol — Umbrella

## Task

Migrate all plugin IPC to a single unified text protocol. Two migration axes:

**Migration 1 — JSON → Text:** Replace JSON-RPC handshake with text-based handshake using the unified grammar.

**Migration 2 — Current text → Unified text:** Converge the two existing text formats (event delivery + commands) onto one shared grammar and parser.

End state: all plugin IPC (handshake, events, commands) uses one grammar, one tokenizer, one parser.

## Migration Tracker

### Migration 1: JSON → Text

| Component | Current | Target | Spec | Status |
|-----------|---------|--------|------|--------|
| Stage 1 (Registration) | JSON-RPC | Unified text | `spec-utp-3-handshake.md` | Not started |
| Stage 2 (Config) | JSON-RPC | Text (or hybrid — design TBD) | `spec-utp-3-handshake.md` | Not started |
| Stage 3 (Capabilities) | JSON-RPC | Unified text | `spec-utp-3-handshake.md` | Not started |
| Stage 4 (Registry) | JSON-RPC | Unified text | `spec-utp-3-handshake.md` | Not started |
| Stage 5 (Ready) | JSON-RPC | Unified text | `spec-utp-3-handshake.md` | Not started |
| Runtime RPCs (Socket A) | JSON-RPC | Unified text | `spec-utp-3-handshake.md` | Not started |
| Event callbacks (Socket B) | JSON-RPC wrapping text/JSON | Unified text directly | `spec-utp-3-handshake.md` | Not started |

### Migration 2: Current Text → Unified Text

| Component | Current | Target | Spec | Status |
|-----------|---------|--------|------|--------|
| Event header | Two shapes (state vs message) | Uniform `peer <ip> asn <asn>` | `spec-utp-1-event-format.md` | Not started |
| Event attributes | Flat, brackets, spaces | Comma lists, no brackets | `spec-utp-1-event-format.md` | Not started |
| Event NLRI | `announce`/`withdraw` implicit | `nlri add`/`nlri del` explicit | `spec-utp-1-event-format.md` | Not started |
| Event capabilities | `cap N name value` repeated | Unchanged (repeated dict key) | `spec-utp-1-event-format.md` | Not started |
| NLRI String() methods | `set` keyword everywhere | Drop `set`, bare `key value` | `spec-utp-1-event-format.md` | Not started |
| Command lists | Brackets `[65001 65002]` | Commas `65001,65002` | `spec-utp-2-command-format.md` | Not started |
| Command NLRI | `nlri <family> add` (close) | Align fully with event format | `spec-utp-2-command-format.md` | Not started |
| Command path-id | Accumulator `path-information set` | Modifier `nlri path-id <id> add` | `spec-utp-2-command-format.md` | Not started |
| Shared tokenizer | None (separate parsers) | One tokenizer in shared package | `spec-utp-2-command-format.md` | Not started |

### Documentation

| Doc | Current | Target | Spec | Status |
|-----|---------|--------|------|--------|
| Delete fabricated docs | 5 AI-generated docs | Deleted | `done/300-text-format-docs.md` | Done |
| Current format reference | None | `text-format.md` (current section) | `done/300-text-format-docs.md` | Done |
| Proposed format reference | None | `text-format.md` (proposed section) | `done/300-text-format-docs.md` | Done |
| Parser architecture | None | `text-parser.md` | `done/300-text-format-docs.md` | Done |
| Coverage table | None | `text-coverage.md` | `done/300-text-format-docs.md` | Done |

### Execution Order

```
spec-utp-0-umbrella.md          ← THIS (umbrella tracker)
    ↓
spec-utp-1-event-format.md     ← NEXT (implement proposed event format)
    ↓
spec-utp-2-command-format.md   ← (refactor commands to unified grammar, extract shared tokenizer)
    ↓
spec-utp-3-handshake.md        ← (text alternative for JSON-RPC handshake)
```

Each spec builds on the shared infrastructure from the previous one. The event format change is smallest (formatter + parser for one direction). The command refactor extracts the shared tokenizer. The handshake is the biggest change (new protocol mode).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/text-format.md` — current + proposed event format
  → Decision:
- [ ] `docs/architecture/api/text-parser.md` — current + proposed parser design
  → Decision:
- [ ] `docs/architecture/api/text-coverage.md` — coverage gaps
  → Constraint:
- [ ] `docs/architecture/api/architecture.md` — API architecture
  → Constraint:
- [ ] `docs/architecture/api/process-protocol.md` — handshake + command dispatch
  → Constraint:
- [ ] `docs/architecture/api/ipc_protocol.md` — IPC framing
  → Constraint:

### Source Files
- [ ] `internal/plugins/bgp/format/text.go` — event formatter
- [ ] `internal/plugins/bgp-rs/server.go` — event parser
- [ ] `internal/plugins/bgp/handler/update_text.go` — command parser
- [ ] `internal/plugin/command.go` — command dispatcher + tokenizer
- [ ] `pkg/plugin/rpc/types.go` — JSON-RPC type definitions
- [ ] `pkg/plugin/sdk/` — SDK handshake implementation

### RFC Summaries (not protocol work — N/A)

## Current Behavior (MANDATORY)

### Three Separate Protocols Today

**Event delivery** (`text.go` formatter, `server.go` parser):
- Newline-framed text on Socket B
- Two header shapes (state vs message)
- Flat attribute reporting (no actions)
- `announce`/`withdraw` implicit NLRI operations
- `strings.Fields()` parsing, no shared tokenizer

**Text commands** (`update_text.go` parser, `command.go` dispatcher):
- JSON-RPC wrapped (args array inside `ze-bgp:peer-update`)
- Accumulator-based attribute building (set/add/del)
- Explicit `nlri <family> add/del` operations
- Bracket-delimited lists `[65001 65002]`
- Quoted string support in tokenizer
- Extra features: nhop accumulator, rd, label, path-information, watchdog, eor

**Handshake** (`process.go`, `rpc/types.go`, SDK):
- NUL-framed JSON-RPC 2.0 on Socket A and B
- 5 stages with complex nested structures
- Config delivery as JSON blob
- No text alternative exists

### Key Divergences

| Aspect | Events | Commands | Unification Approach |
|--------|--------|----------|---------------------|
| Attribute format | `origin igp` (flat) | `origin set igp` (action) | Events stay flat (reporting), commands keep actions (mutation) |
| List delimiter | brackets + spaces | brackets + spaces/commas | Commas everywhere, no brackets |
| NLRI grouping | positional after attrs | explicit `nlri <family>` | Explicit `family` + `nlri add/del` everywhere |
| Next-hop | per-family inline | top-level accumulator | (to be decided) |
| Path-ID | in NLRI string | separate accumulator | Modifier `nlri path-id <id> add` everywhere |
| Peer selector | single address | wildcards (`*`, `!ip`) | Commands keep wildcards, events use single address |

## Data Flow (MANDATORY)

### Entry Points
- Event delivery: `FormatMessage()` → Socket B → plugin parser
- Text commands: CLI/plugin → JSON-RPC → `Dispatch()` → `ParseUpdateText()`
- Handshake: `Process.runStages()` → Socket A/B → `SDK.Startup()`

### Transformation Path
(to be completed during research)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin (events) | Text lines on Socket B | [ ] |
| Plugin → Engine (commands) | JSON-RPC on Socket A wrapping text args | [ ] |
| Engine ↔ Plugin (handshake) | JSON-RPC on Socket A and B | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (to be filled during implementation) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Single BNF grammar | Covers event delivery, commands, and handshake |
| AC-2 | Shared tokenizer | Used by all three protocol paths |
| AC-3 | Backward direction | Event format parseable by same parser as command format |
| AC-4 | Forward direction | Command format generatable by same formatter |
| AC-5 | Handshake text mode | All 5 stages expressible in unified grammar |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (to be filled) | | | |

## Files to Modify
(to be filled during research)

## Files to Create
(to be filled during research)

## Implementation Steps

This is the **umbrella spec**. It defines the unified grammar and delegates implementation to child specs:

1. `spec-utp-1-event-format.md` — implement proposed event format (code changes to `text.go` + `server.go`)
2. `spec-utp-2-command-format.md` — refactor command parser to use unified grammar
3. `spec-utp-3-handshake.md` — text alternative for 5-stage JSON-RPC

Order: event format first (smallest change, validates grammar), then command refactor, then handshake.

### Failure Routing
| Failure | Route To |
|---------|----------|
| Grammar can't cover all three | Revisit unification — may need per-protocol extensions |
| Handshake too complex for text | Keep JSON-RPC for handshake, unify only events + commands |

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
- [ ] AC-1..AC-5 all demonstrated
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
- [ ] Spec moved to `docs/plan/done/NNN-utp-0-umbrella.md`
- [ ] Spec included in commit
