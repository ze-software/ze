# Spec: fmt-1-text-update

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fmt-0-append (completed, see plan/learned/614-fmt-0-append.md) |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/buffer-first.md` -- existing Append / WriteTo patterns
4. `plan/learned/614-fmt-0-append.md` -- non-UPDATE migration already landed
5. `internal/component/bgp/format/text_update.go` -- UPDATE-path code to migrate
6. `internal/component/bgp/format/text_human.go`, `text_json.go` -- UPDATE-path helpers

## Task

Complete the companion migration to `spec-fmt-0-append`: move the UPDATE-path
formatters in `internal/component/bgp/format/text_update.go` (and the helpers
in `text_human.go` / `text_json.go` that they delegate to) off
`fmt.Sprintf` / `fmt.Fprintf` / `strings.Builder` / `strings.Replace` onto
`AppendXxx(buf []byte, ...) []byte` helpers that mirror the spec-fmt-0
non-UPDATE shape.

In-scope functions (all currently returning `string`):
- `FormatMessage` -- outer dispatcher
- `formatEmptyUpdate`
- `formatNonUpdate`
- `formatFromFilterResult`, `formatRawFromResult`, `formatParsedFromResult`, `formatFullFromResult`
- `formatSummary` (in `summary.go`)
- `FormatSentMessage` -- sent-UPDATE override
- `FormatNegotiated` -- capability negotiation event
- `formatFilterResultText`, `formatFilterResultJSON` (in `text_human.go` / `text_json.go`)
- `formatStateChangeText`, `formatStateChangeJSON` (already in scope from fmt-0's AC-5 callers)

Out of scope: `format/format_buffer.go` (tracked separately as `spec-fmt-2-json-append`),
plugin IPC raw-bytes transport (tracked as `spec-plugin-ipc-raw-bytes`).

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/buffer-first.md` -- WriteTo vs AppendText discipline
  -> Constraint: No `make([]byte, N)` in helpers; caller owns the buffer.
- [ ] `plan/learned/614-fmt-0-append.md` -- pattern established by fmt-0
  -> Decision: 7 non-UPDATE formatters already migrated; reuse the same
     stack-scratch + string(scratch) boundary pattern at the caller.
  -> Constraint: Preserve byte-identical output with the existing string
     form (filter plugins + CLI monitors consume the text).

### RFC Summaries
None -- output format is already RFC-compliant; this spec is a pure
allocation refactor.

**Key insights:**
- fmt-0-append already added `appendJSONString`, `appendReplacingByte`,
  `appendPeerJSON` helpers and `hex.AppendEncode` usage. This spec reuses
  them.
- The UPDATE path has more nesting: filter-result traversal emits
  per-attribute per-family per-prefix tokens; the scratch buffer must
  cover realistic UPDATE sizes (typical 2-8KB, pathological 32KB+).
  Either size the scratch larger, or let the heap spill handle the tail.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/format/text_update.go` (~300L) -- 9 functions
  still using `fmt.Sprintf`, `fmt.Fprintf` (via `strings.Builder`), and
  `strings.Replace` for JSON suffix surgery.
- [ ] `internal/component/bgp/format/text_human.go` (~220L) --
  `formatFilterResultText` + `formatStateChangeText`; uses `fmt.Fprintf`.
- [ ] `internal/component/bgp/format/text_json.go` (~370L) --
  `formatFilterResultJSON` + `formatStateChangeJSON`; uses `fmt.Fprintf`,
  `strings.Builder`, and `writeJSONEscapedString`.
- [ ] `internal/component/bgp/format/summary.go` -- `formatSummary` path.

**Behavior to preserve:**
- Byte-identical output for every existing `format.FormatMessage` /
  `format.FormatSentMessage` / `format.FormatNegotiated` caller.
- All `.ci` tests in `test/plugin/` that consume UPDATE text or JSON
  subscriber output must remain byte-identical.
- The `[appendJSONString, appendReplacingByte, appendPeerJSON]` helpers
  must continue to be the single source of truth for JSON escaping.

**Behavior to change:**
- None. Pure allocation refactor.

## Data Flow (MANDATORY)

### Entry Point
- Subscriber receives UPDATE: `server/events.go:formatMessageForSubscription`
  -> `format.FormatMessage` -> `formatFromFilterResult` -> text/JSON.
- Subscriber receives sent UPDATE: `format.FormatSentMessage` ->
  `FormatMessage` with override direction.
- Subscriber receives Negotiated: `format.FormatNegotiated` -> `JSONEncoder`.

### Transformation Path
1. Raw UPDATE bytes + ContentConfig -> `FormatMessage`.
2. Filter result built (attrs + NLRI per family).
3. `formatFilterResultText` / `formatFilterResultJSON` render into string.
4. Caller assigns to `fmtCache.set(..., string)` or `output = string`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| format pkg -> server pkg | `string` return | [ ] |
| format pkg -> plugin IPC | `string` in `FilterUpdateInput.Update` | [ ] |
| format pkg -> CLI monitors | `string` via `monitorDeliver` | [ ] |

### Integration Points
- `server/events.go` UPDATE callers (lines 153, 264, 316, 674, 705) --
  all currently assigning `format.FormatMessage` / `FormatSentMessage`
  result into `fmtCache` / `jsonOutput`. These call sites must migrate
  to stack-scratch + `string(scratch)` pattern.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Subscriber UPDATE text | -> | `format.FormatMessage` | existing `.ci` suite in `test/plugin/` |
| Subscriber sent UPDATE | -> | `format.FormatSentMessage` | existing `.ci` |
| Subscriber Negotiated | -> | `format.FormatNegotiated` | existing `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestAppendMessage_UpdateParity` | `format/text_update_append_test.go` | Byte-identical output vs legacy |
| `TestAppendSentMessage_Parity` | `format/text_update_append_test.go` | Byte-identical sent-UPDATE |
| `TestAppendFullResult_Parity` | `format/text_update_append_test.go` | Full (parsed + raw) encoding |

## Files to Modify

- `internal/component/bgp/format/text_update.go` -- rewrite 9 functions to Append shape
- `internal/component/bgp/format/text_human.go` -- rewrite to Append shape
- `internal/component/bgp/format/text_json.go` -- rewrite to Append shape
- `internal/component/bgp/format/summary.go` -- rewrite `formatSummary`
- `internal/component/bgp/server/events.go` -- migrate 5 UPDATE call sites

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | n/a |
| CLI commands | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No | existing `.ci` coverage |

## Implementation Steps

1. Design: map each current function to an `AppendXxx(buf []byte, ...) []byte` signature.
2. Implement leaf helpers first (`appendFilterResultText/JSON`), then orchestrators.
3. Preserve byte-identical output via parity tests before deleting legacy.
4. Migrate 5 server/events.go UPDATE call sites to stack-scratch + Append.
5. Verify: `make ze-verify-fast`, `make ze-race-reactor`, representative `.ci` runs.

## Checklist

### Goal Gates
- [ ] Byte-identical UPDATE text / JSON for every existing fixture
- [ ] Zero `fmt.Sprintf` / `strings.Builder` / `strings.Replace` in `text_update.go` + `text_human.go` + `text_json.go` + `summary.go`
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes

### Quality Gates
- [ ] Benchmarks show zero-alloc `Append*` path on warm scratch
- [ ] Legacy `FormatMessage`/`FormatSentMessage`/`FormatNegotiated` renamed or deleted per no-layering

### Completion
- [ ] Learned summary written to `plan/learned/NNN-fmt-1-text-update.md`
- [ ] Deferral entry in `plan/deferrals.md` closed (status -> done)
