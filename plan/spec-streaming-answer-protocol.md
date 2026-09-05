# Spec: streaming-answer-protocol

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The work.** Record-level streaming for the REST, gRPC, web, MCP and
looking-glass surfaces. The record stream exists and is proven on the operator
and plugin wire; these five surfaces still buffer the whole answer into one
string before any of it leaves.

**What landed, and where the seam is.** `CommandDispatcher.Answer`
(`internal/component/plugin/dispatch.go`) is the streaming route: a payload that
is a row generator is NOT flattened, `Output` stays empty, and `Response`
carries the generator for the surface to walk. Its own comment names the one
surface that takes it today, the SSH exec channel. `CommandDispatcher.JSON` is
its sibling and does the opposite: it calls `ResponseJSON` and returns a
`RenderedResponse` whose `Output` is the entire answer as one string. Both
report a failed generator the same way, through `responseFailure`, so the two
routes already agree about errors.

**Who is on the buffering side.** The 2026-08-19 design counted 24 consumers
calling `CommandDispatcher.JSON` and reading `RenderedResponse.Output` as one
string. Measured again on 2026-09-05, the call sites live in
`internal/component/web/` (ten files), `internal/component/mcp/tools.go`,
`internal/component/lg/server.go`, `cmd/ze/hub/main.go` and
`cmd/ze/hub/main_servers.go`. Each can take the buffering path unchanged, which
is why none of them blocked the operator and plugin wire this spec's predecessor
delivered.

**Why it is worth doing.** A large answer is held whole in memory once per
reader on every one of these surfaces, and the first byte does not leave until
the last row is built. Streaming them is separable work with its own consumers
and its own failure modes: an HTTP response whose status is written before a row
fails, a gRPC stream whose framing is per record, an SSE feed the web already
owns, and an MCP tool result whose shape the protocol fixes.

**A permanent limit, decided and not to be revisited.** `table` and `text`
rendering buffer whatever the wire does, because column widths need every row
before the first line can be printed. Declared widths were considered and
rejected as an option nobody asked for.

**The question to put to the owner first.** The row's own destination note says
this spec is "to be raised with the owner when this one closes". Five surfaces
is five designs; the first decision is which of them ship streaming and in what
order, and that decision is his.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the answer contract every surface renders
  → Decision: [fill during research]
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- `Answer` and `JSON` are the two routes; the streaming half exists and only the SSH exec channel takes it.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/plugin/dispatch.go` - `CommandDispatcher.JSON` flattens every payload through `ResponseJSON` into `RenderedResponse.Output`; `CommandDispatcher.Answer` hands a row generator back unflattened for a record surface to walk; `RecordRows` is the one test for whether a payload is a generator, because `Records` is walked once.
- [ ] `internal/component/mcp/tools.go` - calls the flattening route and reads `Output` as one string.
- [ ] `internal/component/lg/server.go` - the same.
- [ ] `cmd/ze/hub/main_servers.go` - the same, for the REST and gRPC servers.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every surface that stays on the buffering path answers byte for byte what it answers today.
- A failed generator reports its failure identically on both routes.
- `table` and `text` keep buffering.

**Behavior to change:** (only what the user asked for)
- The named surfaces write a record stream where the payload is a generator.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An HTTP request to the REST server or the looking glass, a gRPC call, a web page request, or an MCP tool call.
- The request carries a command string; the answer leaves as a rendered payload.

### Transformation Path
1. The surface dispatches the command through `CommandDispatcher`.
2. `JSON` flattens the answer, or `Answer` hands back a row generator.
3. The surface renders or writes what it received.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface ↔ dispatcher | `RenderedResponse`, with `Output` or a generator | No |
| Surface ↔ client | HTTP chunked body, gRPC stream, SSE, or an MCP tool result | No |

### Integration Points
- `RecordRows` and `responseFailure` (`internal/component/plugin/dispatch.go`) - the two helpers a streaming surface must use rather than reimplement.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each surface's transport can frame per record | HTTP chunking, gRPC streams and SSE all can | that surface stays on the buffering path, by decision rather than by omission | reading each transport during research | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A row fails after the HTTP status is written, so the client sees a truncated success | a partial answer with a 200 status | decide the failure protocol per surface before any code, and test it |
| R-2 | A generator is walked twice, so a surface answers no rows | an empty answer for a command that has rows | route every surface through `RecordRows`, which exists to answer this |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a REST, gRPC, web, MCP or looking-glass answer is truncated or empty |
| How is it reverted? | per-surface revert; each surface can stay on the buffering path |
| Who else touches this path? | every spec touching the command answer contract |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a REST request for a command whose payload is a generator | → | the streaming route through `Answer` | `TestRESTWritesRecordsAsTheyAreProduced` |
| a row fails mid-stream | → | `responseFailure` on the surface | `TestStreamedAnswerReportsAMidStreamFailure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a streaming-capable surface asks for a generator-backed answer | records leave before the last row is built |
| AC-2 | the payload is not a generator | the surface answers exactly what it answers today |
| AC-3 | a row fails after the first record left | the client is told, and does not read the answer as complete |
| AC-4 | the requested rendering is `table` or `text` | the answer buffers, as decided |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRESTWritesRecordsAsTheyAreProduced` | `cmd/ze/hub/main_servers_test.go` | AC-1 | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->
| `TestStreamedAnswerReportsAMidStreamFailure` | `internal/component/plugin/dispatch_test.go` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `streaming-rest-records` | `test/plugin/streaming-rest-records.ci` | a REST client reads rows before the answer is complete | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

## Files to Modify
- `internal/component/mcp/tools.go` - the MCP tool result path
- `internal/component/lg/server.go` - the looking-glass answer path
- `cmd/ze/hub/main_servers.go` - the REST and gRPC servers
- `internal/component/web/cli_terminal.go` - the web terminal's answer path

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Functional test for new RPC/API | | `test/plugin/streaming-rest-records.ci` |
| Pipe completeness | | [answered at design] |
| Prometheus counters/metrics | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- put the surface order to the owner, then wire the first surface's streaming route and write its failing test
   - Tests: `TestRESTWritesRecordsAsTheyAreProduced`
   - Files: `cmd/ze/hub/main_servers.go`
   - Verify: the test fails because the surface flattens the answer
2. **Phase: [one phase for each surface the owner names]**

## Known Limitations
- `table` and `text` rendering buffer whatever the wire does, permanently, because column widths need every row.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
