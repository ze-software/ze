# Spec: fmt-2-json-append

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
3. `.claude/rules/buffer-first.md` -- Append vs WriteTo discipline
4. `plan/learned/614-fmt-0-append.md` -- pattern established
5. `internal/component/bgp/format/format_buffer.go` -- functions to migrate
6. `internal/component/bgp/format/format_buffer_test.go` -- existing tests

## Task

Migrate the JSON attribute writers in
`internal/component/bgp/format/format_buffer.go` from the current
`io.Writer` + `fmt.Fprintf` shape onto the `AppendXxx(buf []byte, ...) []byte`
shape established by `spec-fmt-0-append`.

In-scope functions (currently writing into `io.Writer`):
- `FormatASPathJSON(data []byte, asn4 bool, w io.Writer) error`
- `FormatCommunitiesJSON(data []byte, w io.Writer) error`
- `FormatOriginJSON(value byte, w io.Writer)`
- `FormatMEDJSON(data []byte, w io.Writer)`
- `FormatLocalPrefJSON(data []byte, w io.Writer)`

Out of scope: text-side UPDATE-path formatting (`spec-fmt-1-text-update`),
plugin IPC raw-bytes transport (`spec-plugin-ipc-raw-bytes`).

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/buffer-first.md` -- Append shape
  -> Constraint: No `make` in helpers; caller owns buffer.
- [ ] `plan/learned/614-fmt-0-append.md` -- consistent shape
  -> Decision: Signature is `AppendXxx(buf []byte, args...) []byte`.
- [ ] `docs/architecture/api/json-format.md` -- JSON array / number format
  -> Constraint: JSON output must stay `[a,b,c]`, integer JSON numbers,
     unquoted strings for `"no-export"` etc.

### RFC Summaries
None.

**Key insights:**
- The JSON-writer functions are consumed by the UPDATE JSON formatters
  (`text_json.go` etc.); migration ordering depends on whether `spec-fmt-1`
  is merged first. Either order works; the migration is mechanical.
- `fmt.Fprintf(w, "%d", ...)` -> `strconv.AppendUint(buf, ..., 10)`.
- Well-known community names are already in `wellKnownCommunityName`;
  no duplication needed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/format/format_buffer.go` (~230L) --
  `FormatASPathJSON`, `FormatCommunitiesJSON`, `FormatOriginJSON`,
  `FormatMEDJSON`, `FormatLocalPrefJSON`. Uses `fmt.Fprintf(w, ...)`.
- [ ] `internal/component/bgp/format/format_buffer_test.go` -- existing
  unit tests for each function.

**Behavior to preserve:**
- Output bytes identical per function.
- AS_SET braces `{asn1,asn2}` inside bracketed JSON array.
- `wellKnownCommunityName` resolution.

**Behavior to change:**
- None. Pure allocation refactor.

## Data Flow (MANDATORY)

### Entry Point
- `format/text_json.go:formatFilterResultJSON` calls
  `FormatASPathJSON(data, asn4, &sb)` / `FormatCommunitiesJSON(data, &sb)`
  into a `strings.Builder`. That Builder flushes to a final `string`.
- Same for MED / LocalPref / Origin inside the per-attribute JSON loop.

### Transformation Path
1. Raw wire bytes + asn4 flag -> AppendASPathJSON.
2. Per-segment iteration: AS_SEQUENCE inline, AS_SET braces.
3. Result: `[65001,65002,{65003,65004}]`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| wire bytes -> JSON writer | `append(buf, ...)` | [ ] |
| JSON writer -> caller | `[]byte` return | [ ] |

### Integration Points
- `format/text_json.go` callers of the 5 JSON writers -- all internal
  to the format package. No public API change outside `format/`.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Subscriber UPDATE JSON | -> | UPDATE JSON writers | existing `.ci` JSON subscriber tests |
| RIB `lookup` JSON | -> | `FormatASPathJSON` etc. | existing `.ci` RIB tests |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestAppendASPathJSON_Parity` | `format/format_buffer_append_test.go` | Byte-identical to `FormatASPathJSON` |
| `TestAppendCommunitiesJSON_Parity` | `format/format_buffer_append_test.go` | Byte-identical |
| `TestAppendOriginJSON_Parity` | `format/format_buffer_append_test.go` | All 4 enum values |
| `TestAppendMEDJSON_Parity` | `format/format_buffer_append_test.go` | Boundary + short-data |
| `TestAppendLocalPrefJSON_Parity` | `format/format_buffer_append_test.go` | Boundary + short-data |

## Files to Modify

- `internal/component/bgp/format/format_buffer.go` -- rewrite 5 functions
- `internal/component/bgp/format/text_json.go` -- migrate 5 call sites
- `internal/component/bgp/format/format_buffer_test.go` -- retarget

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | n/a |
| CLI commands | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No | existing `.ci` coverage |

## Implementation Steps

1. Add `AppendASPathJSON`, `AppendCommunitiesJSON`, `AppendOriginJSON`,
   `AppendMEDJSON`, `AppendLocalPrefJSON` side-by-side with legacy versions.
2. Parity tests pass.
3. Migrate callers in `text_json.go`.
4. Delete legacy `Format*JSON` functions (no-layering).
5. Verify: `make ze-verify-fast`.

## Checklist

### Goal Gates
- [ ] 5 new `AppendXxx` functions byte-identical to legacy `Format*JSON`
- [ ] All callers in `text_json.go` migrated
- [ ] Legacy `Format*JSON(io.Writer)` deleted
- [ ] `make ze-verify-fast` passes

### Quality Gates
- [ ] Benchmarks show zero-alloc on warm scratch

### Completion
- [ ] Learned summary written to `plan/learned/NNN-fmt-2-json-append.md`
- [ ] Deferral entry in `plan/deferrals.md` closed
