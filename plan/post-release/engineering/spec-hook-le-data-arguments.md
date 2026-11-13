# Spec: hook-le-data-arguments

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Agent tooling: a false refusal costs every session a retry and pushes agents toward wrappers, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - (spec-agent-economy-6-shell-parser, closed: the parse the guards will read, `internal/le/hookruntime/shellparse.go`) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (2026-11-11).** `./le commit create subject ... body "... passes
except below; GOOS=darwin go vet ./internal/test/runner/ clean." file ...` was
refused by `bashRawHeavy` as `go vet` run raw. Its advice began a command at
`GOOS=darwin go vet ./internal/test/runner/ 'clean. \'`, so the line was cut
at the `;` inside the double-quoted body. The same text through `body-file`
passed.

**Cause, read at the producer.** `commandSegments`
(`internal/le/hookruntime/bash.go`) splits a line at every separator, inside
quotes included, "which is the text judgment a guard keeps for any line that
is not a reader line". `bashRawHeavy` then runs `rawHeavy`
(`internal/le/hookruntime/rawheavy.go`) over each piece, so quoted prose after
a `;` becomes a command word.

**Why the shell-parser spec does not fix it.**
`spec-agent-economy-6-shell-parser` lifts the text verdict only on a line made
wholly of readers, and keeps the quote-blind text judgment on every other line
by decision ("a quoted string may be run by the command that receives it").
A `./le commit create` line is not a reader line, so the refusal survives that
spec.

**Goal.** The bash guards know which arguments of a `./le` command are data
that no shell runs (a commit subject or body, a review or approval reason, a
`no-news` reason), declared by the command that owns them rather than listed
in the hook, and judge those arguments as data. A guard still refuses a heavy
job typed anywhere it can run.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: a hook checks what a command is, never how a message is worded (`ai/META-DECISIONS.md`); a refusal is never passed by a wrapper
- [ ] spec-agent-economy-6-shell-parser (closed; read it from git history, the commit before its removal) - Design Insights and Key Design Decisions on the text verdict, and `.claude/hooks/README.md`, which carries the reader line and the text verdict now
  → Constraint: on a non-reader line the text verdict holds "HEAD or stricter"; this spec must lift it only for arguments a declaration proves are data

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/bash.go` - `commandSegments` (quote-blind split), `bashRawHeavy` (text verdict on every non-reader line)
- [ ] `internal/le/hookruntime/rawheavy.go` - `rawHeavy` (judges one segment's first word past assignments and launchers)

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- A quoted data argument of a `./le` command is never read as a command

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [to research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hook ↔ le command registry | [to research] | No |

### Integration Points
- [to research]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A Bash call `./le commit create ... body "<text with ; go vet>"` | → | `bashRawHeavy` | [hook golden row, to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/hookruntime/` | A quoted `;` followed by `go vet` in a commit body passes; the same words as a real command are refused | |

## Files to Modify
- `internal/le/hookruntime/bash.go` - [to design]

## Implementation Steps

1. **Phase: Wiring and red golden row** -- [to design]
2. **Phase: Data-argument declaration** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le ai hooks unit`, `./le test unit pkg path ./internal/le/hookruntime/`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [to research] | `internal/le/hookruntime/bash.go` `ctxReaderLine`; the le command registry | [to research] | [to research] |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `hook-existing-patterns-false-positive` | 2026-11-11 | `bashRawHeavy`, `rawHeavy` on a quoted `;`: prose after a `;` inside a `./le commit create` body read as a raw `go vet` |
| `gate-verdict-depends-on-the-machine` | 2026-11-13 | `fromSubagent`: the hook reads a fork from an environment variable the harness uses as a setting |
| `hook-existing-patterns-false-positive` | 2026-11-12 | `bashTestDeletion` line-wide words: a reader `for` loop's test paths read as targets of a later top-level `rm -f` |
| `hook-existing-patterns-false-positive` | 2026-11-11 | `bashTestDeletion` line-wide words: test paths given to `git ls-tree` piped into `git update-index --index-info` read as targets of a scratch `rm -f` |
| `hook-existing-patterns-false-positive` | 2026-11-13 | `goBuildText` and `textLossyPipe`: `go build` and `test qemu` inside a quoted grep pattern of an `ssh` argument read as commands |
| `hook-existing-patterns-false-positive` | 2026-11-12 | `bashTestDeletion` line-wide words: the word `rm` in a `./le commit create` here-document message read as deleting the `_test.go` files it names |
| `hook-existing-patterns-false-positive` | 2026-11-12 | `bashTestDeletion` line-wide words: the `test/plugin/` destination of an `mv` from `test/draft/plugin/` read as the target of a scratch `rm -f` |
