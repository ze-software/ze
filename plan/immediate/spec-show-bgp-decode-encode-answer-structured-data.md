# Spec: show bgp decode and show bgp encode answer text, so no operator chain reaches them

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | `plan/immediate/spec-cli-show-bgp-answer-shapes.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`show bgp decode` and `show bgp encode` print finished text and answer nothing
else. Both are registered with `registry.MustRegisterLocal`
(`internal/component/bgp/cli/register.go`), whose handler type is
`LocalHandler`, declared in `internal/component/command/registry/registry.go` as
`func(args []string) int`. A handler of that type returns an exit code and
writes its own output, so it produces no `ResponseData`, reaches no
`ApplyPipes`, and declares no fields.

The consequence for the operator: no pipe and no operator chain reaches either
command. `| json`, `| yaml`, `| table`, `display`, `where` and every other
operator act on structured data, and these two commands hand the pipe layer
nothing to act on. `ai/rules/cli.md` requires a structured payload of every
command, so this is a gap in the CLI contract rather than a preference.

The registry already holds the answer shape. `MustRegisterLocalData` takes a
`LocalDataHandler`, `func(args []string) (any, int)`, described in the same file
as answering "a command with structured DATA instead of printing text, so the
answer can go through the pipe layer like any other". `show env list`,
`show config dump` and `show config diff` are already registered that way. The
work is to convert these two handlers to that contract and decide what a decoded
or encoded BGP message looks like as data: the field names, the nesting, and
what the text renderer prints from it so the human-readable output survives.

The row was raised on 2026-08-24 by `spec-cli-show-bgp-answer-shapes`, which
declared what each command answers and could not take these two, because the
change is to two handlers' output contract rather than to what they declare.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli.md` - every command answers structured data and supports every pipe operator
  → Constraint: <to be filled>
- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Decision: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/cli/register.go` - registers `show bgp decode` and `show bgp encode` with `registry.MustRegisterLocal`, each closure calling `Run` with a leading subcommand token and returning its exit code
- [ ] `internal/component/command/registry/registry.go` - `LocalHandler` is `func(args []string) int`; `LocalDataHandler` is `func(args []string) (any, int)` and is the contract that reaches the pipe layer
- [ ] `internal/component/config/cli/register.go` - `show config dump` and `show config diff` are already registered through `MustRegisterLocalData`, so the conversion has a worked example in the tree

**Behavior to preserve:** (unless the user explicitly said to change it)
- the human-readable decode and encode output an operator reads today
- <to be filled>

**Behavior to change:** (only what the user asked for)
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show bgp decode <hex>` and `show bgp encode <...>`, typed by the operator or reached from the web tool page, which builds the same command string
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI registry ↔ pipe layer | `LocalDataHandler` answer value | No |

### Integration Points
- `internal/component/web/page_tools.go` builds a `show bgp decode` command string for the web tool page and reads its output

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the web tool page parses the current text output and breaks on a new renderer | `handler_tool_pages_test.go` goes red | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the decode output an operator and the web tool page both read |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-bgp-decode-render`, `spec-bgp-pcap-decode` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp decode <hex> \| json` | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp decode <hex> \| json` | the decoded message is answered as JSON |
| AC-2 | `show bgp encode <...> \| json` | the encoded message is answered as JSON |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | pipes a decoded UPDATE into `json` | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | <to be filled> | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

## Files to Modify
- `internal/component/bgp/cli/register.go` - <to be filled>
- `internal/component/bgp/cli/` - the decode and encode handlers behind `Run`

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI grammar (keyword before value) | | <to be filled> |
| Pipe completeness | | <to be filled> |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Naming | field names agree with the ones `show bgp rib` already uses for the same facts |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| <to be filled> | <to be filled> |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
