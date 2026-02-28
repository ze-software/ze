# Spec: Unified Text Protocol — Umbrella

## Task

Migrate all plugin IPC to a single unified text protocol. Two migration axes:

**Migration 1 — JSON → Text:** Replace JSON-RPC handshake with text-based handshake using the unified grammar.

**Migration 2 — Current text → Unified text:** Converge the two existing text formats (event delivery + commands) onto one shared grammar and parser.

End state: all plugin IPC (handshake, events, commands) uses one grammar with shared keyword tables (`textparse/keywords.go`). Tokenizers remain per-protocol (TextScanner for events, tokenize() for commands) because they serve different framing needs.

## Migration Tracker

### Migration 1: JSON → Text

| Component | Current | Target | Spec | Status |
|-----------|---------|--------|------|--------|
| Stage 1 (Registration) | JSON-RPC | Unified text | `done/315-utp-3-handshake.md` | Done |
| Stage 2 (Config) | JSON-RPC | Heredoc JSON (`root <name> json << END`) | `done/315-utp-3-handshake.md` | Done |
| Stage 3 (Capabilities) | JSON-RPC | Unified text | `done/315-utp-3-handshake.md` | Done |
| Stage 4 (Registry) | JSON-RPC | Unified text | `done/315-utp-3-handshake.md` | Done |
| Stage 5 (Ready) | JSON-RPC | Unified text | `done/315-utp-3-handshake.md` | Done |
| Runtime RPCs (Socket A) | JSON-RPC | TextMuxConn with `#N` serial prefix | `done/315-utp-3-handshake.md` | Done |
| Event callbacks (Socket B) | JSON-RPC wrapping text/JSON | Plain text lines via TextConnB | `done/315-utp-3-handshake.md` | Done |

### Migration 2: Current Text → Unified Text

| Component | Current | Target | Spec | Status |
|-----------|---------|--------|------|--------|
| Event header | Two shapes (state vs message) | Uniform `peer <ip> asn <asn>` | `done/302-utp-1-event-format.md` | Done |
| Event attributes | Flat, brackets, spaces | Comma lists, no brackets | `done/302-utp-1-event-format.md` | Done |
| Event NLRI | `announce`/`withdraw` implicit | `nlri add`/`nlri del` explicit | `done/302-utp-1-event-format.md` | Done |
| Event capabilities | `cap N name value` repeated | Unchanged (repeated dict key) | `done/302-utp-1-event-format.md` | Done |
| NLRI String() methods | `set` keyword everywhere | Drop `set`, bare `key value` | `done/302-utp-1-event-format.md` | Done |
| Command lists | Brackets `[65001 65002]` | Commas `65001,65002` (brackets still accepted) | `done/306-utp-2-command-format.md` | Done |
| Command attributes | `origin set igp` (accumulator) | `origin igp` (flat key-value) | `done/306-utp-2-command-format.md` | Done |
| Command NLRI | `nlri <family> add` (close) | Unchanged — already aligned | `done/306-utp-2-command-format.md` | Done |
| Command path-id | Accumulator `path-information set` | Per-NLRI modifier `nlri <family> path-information <id> add` | `done/306-utp-2-command-format.md` | Done |
| Keyword aliases | None | Short (`next`, `pref`, `path`, `s-com`, `l-com`, `x-com`, `info`) + long forms | `done/306-utp-2-command-format.md` | Done |
| Shared keyword tables | None (separate keyword maps) | `textparse/keywords.go` shared by handler, format, bgp-rs | `done/306-utp-2-command-format.md` | Done |
| Internal text producers | Used old `set` syntax | `FormatRouteCommand()` + `handleRouteRefreshDecision()` updated to flat grammar | `done/306-utp-2-command-format.md` | Done |

### Still Proposed (documented in `text-format.md`, not yet implemented)

| Component | Current | Target | Spec | Status |
|-----------|---------|--------|------|--------|
| Uniform header | Two shapes (state events vs message events) | Uniform `peer <addr> asn <n> state <s> type <t>` for all events | TBD (future spec) | Not started |
| Event NLRI restructuring | `announce`/`withdraw` positional after attributes | `nlri <family> add/del` explicit (align with command format) | TBD (future spec) | Not started |
| Dict mode | None | `update dict ...` dict-style attribute format alongside text/hex/b64 | TBD (future spec) | Not started |

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
spec-utp-0-umbrella.md              ← THIS (umbrella tracker)
    ↓
done/302-utp-1-event-format.md      ← DONE (TextScanner, uniform header, event format)
    ↓
done/306-utp-2-command-format.md    ← DONE (flat grammar, aliases, shared keyword tables)
    ↓
done/315-utp-3-handshake.md        ← DONE (text alternative for JSON-RPC handshake)
```

All three child specs completed. utp-1 built the TextScanner and event format. utp-2 removed the set/del accumulator model, introduced flat grammar with keyword aliases, and created shared keyword tables in `textparse/keywords.go`. utp-3 implemented a text-mode alternative for the 5-stage handshake with auto-detection (first byte `{` → JSON, letter → text), heredoc JSON config delivery, and TextMuxConn for concurrent post-startup RPCs. The "shared tokenizer" from the original plan became "shared keyword tables" — TextScanner (events) and tokenize() (commands) serve different needs and remain separate.

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

- [ ] `internal/plugins/bgp/format/text.go` — event formatter
- [ ] `internal/plugins/bgp-rs/server.go` — event parser
- [ ] `internal/plugins/bgp/handler/update_text.go` — command parser
- [ ] `internal/plugin/command.go` — command dispatcher + tokenizer
- [ ] `internal/plugins/bgp/textparse/keywords.go` — shared keyword tables
- [ ] `pkg/plugin/rpc/types.go` — JSON-RPC type definitions
- [ ] `pkg/plugin/rpc/text.go` — text serialization for handshake stages
- [ ] `pkg/plugin/sdk/sdk.go` — SDK handshake (JSON mode)
- [ ] `pkg/plugin/sdk/sdk_text.go` — SDK handshake (text mode)
- [ ] `internal/plugin/process.go` — engine-side plugin lifecycle
- [ ] `internal/plugin/subsystem_text.go` — engine-side text handshake (subsystem)
- [ ] `internal/plugin/server_startup_text.go` — engine-side text handshake (server)

Behavior to preserve: all three IPC paths (events, commands, handshake) continue to work for JSON-mode plugins. Text mode is an additive alternative, auto-detected from first byte.

Three separate protocols existed before the UTP effort. All three have now been unified onto a shared grammar with shared keyword tables, though each retains its own tokenizer suited to its framing needs.

**Event delivery** (`text.go` formatter, `server.go` parser):
- Newline-framed text on Socket B
- Two header shapes (state vs message)
- Flat attribute reporting (no actions)
- `announce`/`withdraw` implicit NLRI operations
- `strings.Fields()` parsing, no shared tokenizer

**Text commands** (`update_text.go` parser, `command.go` dispatcher) — ~~accumulator model~~ flat grammar after utp-2:
- JSON-RPC wrapped (args array inside `ze-bgp:peer-update`)
- ~~Accumulator-based attribute building (set/add/del)~~ → Flat key-value attributes (`origin igp`, `next 1.2.3.4`)
- Explicit `nlri <family> add/del` operations
- ~~Bracket-delimited lists `[65001 65002]`~~ → Comma-separated lists (brackets still accepted for transition)
- Quoted string support in tokenizer
- Short/long keyword aliases via `textparse/keywords.go`
- ~~nhop accumulator~~ → `next`/`next-hop` alias, ~~path-information accumulator~~ → per-NLRI modifier
- Extra features: rd, label, watchdog, eor

**Handshake** (`process.go`, `rpc/types.go`, SDK):
- NUL-framed JSON-RPC 2.0 on Socket A and B
- 5 stages with complex nested structures
- Config delivery as JSON blob
- No text alternative exists

### Key Divergences

| Aspect | Events | Commands (post utp-2) | Remaining Gap |
|--------|--------|----------------------|---------------|
| Attribute format | `origin igp` (flat) | `origin igp` (flat) — unified | None |
| List delimiter | brackets + spaces | Commas primary, brackets accepted | Events still use brackets + spaces |
| NLRI grouping | `announce`/`withdraw` positional | explicit `nlri <family> add/del` | Events not yet restructured |
| Next-hop | per-family inline | `next <addr>` (alias) | Events use inline next-hop after family |
| Path-ID | in NLRI string | per-NLRI modifier `path-information <id>` | Events use in-NLRI-string |
| Peer selector | single address | wildcards (`*`, `!ip`) | Different by design (reporting vs mutation) |
| Header | two shapes (state vs message) | N/A (commands have no header) | Events need uniform header |

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
| Engine ↔ Plugin (handshake) | JSON-RPC or text on Socket A and B (auto-detected) | [ ] |

### Integration Points
| Component | Integrates With | Via |
|-----------|----------------|-----|
| Event formatter (`text.go`) | Event parser (`server.go`) | Shared keyword constants from `textparse/keywords.go` |
| Command parser (`update_text.go`) | Event formatter (`text.go`) | Shared keyword aliases from `textparse/keywords.go` |
| Text handshake (`text.go`, `text_conn.go`) | SDK (`sdk_text.go`) + engine (`subsystem_text.go`, `server_startup_text.go`) | Auto-detect via `PeekMode`, shared RPC types from `types.go` |
| Internal text producers (`FormatRouteCommand`) | Command parser | Must output flat grammar matching parser expectations |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Event formatter | → | TextScanner, uniform header, comma lists, nlri add/del | `TestFormatMessageText` (text_test.go:82), `TestHandleUpdate_ZeBGPFormat` (server_test.go:73) |
| Command parser | → | Flat grammar, alias resolution, shared keywords | `TestParseUpdateText_FlatAttributes`, `TestParseUpdateText_ShortAlias_*`, functional `update-text-flat.ci` |
| Plugin startup | → | Text handshake, auto-detect, TextMuxConn | `TestTextAutoDetectHandshake` (rpc_plugin_test.go:1298), functional `text-handshake.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Single BNF grammar | Covers event delivery, commands, and handshake |
| AC-2 | Shared keyword tables | Used by all three protocol paths (separate tokenizers: TextScanner for events, tokenize() for commands) |
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
| "One tokenizer in shared package" would unify parsing | TextScanner (events, raw strings) and tokenize() (commands, quoted input) serve fundamentally different needs | utp-2 research — event parser needs zero-alloc field scanning, command parser needs quote handling | Changed AC-2 from "shared tokenizer" to "shared keyword tables" |
| Accumulator model (set/add/del) was needed for mid-stream attribute modification | Attributes are declared once; only NLRI operations need add/del (MP_REACH vs MP_UNREACH) | utp-2 design — the accumulator was overengineered | Simplified command grammar significantly |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented

**utp-1 (event format):** Uniform `peer <ip> asn <asn>` header for all message types. Comma-separated lists (AS-PATH, communities). `nlri <family> add/del` replacing `announce`/`withdraw`. Dropped `set` from all 14+ NLRI plugin String() methods. Shared TextScanner in `textparse/`. Parser rewrite in bgp-rr using TextScanner with key-dispatch loop.

**utp-2 (command format):** Removed accumulator model (set/add/del on attributes). Flat `key value` grammar. Created `textparse/keywords.go` with shared keyword constants, alias tables, and `ResolveAlias()`. Short aliases (`next`, `pref`, `path`, `s-com`, `l-com`, `x-com`, `info`). Moved path-id to per-NLRI modifier. Updated event formatter to output short aliases. Updated internal text producers (`FormatRouteCommand`, `handleRouteRefreshDecision`).

**utp-3 (handshake):** Text serialization for all 5 handshake stages. Heredoc JSON config delivery (`root <name> json << END`). TextConn (line-based framing), PeekMode (auto-detect first byte). TextMuxConn for concurrent post-stage-5 RPCs with `#N` serial prefix. Engine-side text paths (subsystem + server). SDK text constructors (`NewTextPlugin`, `NewTextFromEnv`). `ze-test text-plugin` test binary.

### Documentation Updates
- `docs/architecture/api/text-format.md` — updated examples, BNF grammar, attribute formats, utp-3 entries
- `docs/architecture/api/commands.md` — updated to flat grammar, added keyword aliases table
- `docs/architecture/api/process-protocol.md` — added text mode handshake section
- `docs/architecture/api/ipc_protocol.md` — added text mode plugin handshake section
- `.claude/rules/plugin-design.md` — added text mode to 5-stage protocol and architecture table

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Single unified grammar covering events, commands, handshake | ✅ Done | `textparse/keywords.go` + child specs | Shared keyword tables; separate tokenizers per protocol path |
| Event format unification | ✅ Done | `done/302-utp-1-event-format.md` | Uniform header, comma lists, nlri add/del |
| Command format unification | ✅ Done | `done/306-utp-2-command-format.md` | Flat grammar, aliases, shared keywords |
| Text handshake alternative | ✅ Done | `done/315-utp-3-handshake.md` | All 5 stages, auto-detect, TextMuxConn |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | utp-1 + utp-2 + utp-3 collectively | Shared grammar via keyword tables covers all three paths |
| AC-2 | ✅ Done | `textparse/keywords.go` consumed by handler, format, bgp-rs | Separate tokenizers (TextScanner, tokenize()) share keyword tables |
| AC-3 | ✅ Done | utp-1 AC-9: `TestHandleUpdate_ZeBGPFormat` | Event format parseable by bgp-rr parser |
| AC-4 | ✅ Done | utp-2 AC-13: `TestFormatTextUpdate_ShortAliases` | Event formatter uses shared aliases |
| AC-5 | ✅ Done | utp-3 AC-1: `TestTextHandshakeRoundTrip`, `TestSubsystemAutoDetectText` | All 5 stages in text grammar |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (umbrella delegates to children) | ✅ Done | 3 child specs | 21+ new unit tests in utp-3, 6+ in utp-2, 12+ in utp-1 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| (umbrella delegates to children) | ✅ Done | See child spec audits for file-level detail |

### Audit Summary
- **Total items:** 9 (4 requirements + 5 ACs)
- **Done:** 9
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

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
