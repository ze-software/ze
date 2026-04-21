# Ze - Agent Instructions

Instructions for AI agents working on this codebase. All rules live in `.claude/rules/` as markdown files. Read them — they apply to you too.

For architecture docs and RFC navigation, see `ai/INDEX.md`.

## Before You Do Anything

Read `.claude/rules/session-start.md` first. It contains the TOP 6 blocking rules that prevent the most expensive mistakes. The short version:

1. Read the selected spec (`tmp/session/selected-spec`) before doing anything
2. Read source code before writing code — understand what exists
3. No code without understanding — you must be able to name 3 related files
4. TDD: test must FAIL before implementation
5. Preserve existing behavior unless explicitly told to change it
6. Confirm file paths with search before editing

## Rules Reference

All rules are in `.claude/rules/`. Read the ones relevant to your task.

### Session Discipline

| File | What It Covers |
|------|----------------|
| `session-start.md` | TOP 6 rules, session checklist |
| `post-compaction.md` | Recovery after context loss — re-read everything |
| `before-writing-code.md` | Pre-code checklist: search, read, understand, verify paths |

### Code Quality

| File | What It Covers |
|------|----------------|
| `tdd.md` | TDD cycle, boundary tests, investigation-to-test rule |
| `go-standards.md` | slog logging, error wrapping, fail-early, per-subsystem logging |
| `quality.md` | Never disable linters, critical reviews, paste proof |
| `design-principles.md` | YAGNI, no premature abstraction, single responsibility, 3-fix rule |
| `anti-rationalization.md` | Pre-addressed excuses for skipping discipline |

### BGP Protocol

| File | What It Covers |
|------|----------------|
| `rfc-compliance.md` | RFC citations, constraint comments with quoted requirements |
| `buffer-first.md` | Wire encoding: `WriteTo(buf, off)`, never allocate-and-return |
| `json-format.md` | kebab-case keys, ze-bgp IPC envelope, family format |
| `architecture-summary.md` | System overview: Engine/Plugin boundary, capabilities, WireUpdate vs RIB |

### Planning & Specs

| File | What It Covers |
|------|----------------|
| `planning.md` | Full workflow: research, design, implement, verify. Spec template |
| `spec-no-code.md` | Specs use tables and prose, never code snippets |
| `spec-preservation.md` | Completed specs keep knowledge, strip scaffolding |
| `implementation-audit.md` | Line-by-line verification before marking done |
| `integration-completeness.md` | Features proven end-to-end, not just in isolation |
| `data-flow-tracing.md` | Trace data through layers before modifying architecture |

### Infrastructure

| File | What It Covers |
|------|----------------|
| `plugin-design.md` | Plugin registry, 5-stage protocol, SDK callbacks |
| `cli-patterns.md` | CLI dispatch, flags, exit codes, usage text |
| `config-design.md` | No version numbers, fail on unknown keys |
| `naming.md` | "Ze" = "The" (French accent). Naming conventions for CLI, YANG, Go |

### Process

| File | What It Covers |
|------|----------------|
| `git-safety.md` | Commit only when asked, scope to task, Codeberg CLI (`tea`) |
| `no-layering.md` | Replace, don't layer. Delete old before writing new |
| `compatibility.md` | No backwards compat — Ze has no released versions |
| `no-test-deletion.md` | Ask before deleting tests |
| `testing.md` | Test locations, make targets, linter list |
| `documentation.md` | File naming, doc placement, directory structure |
| `hook-errors.md` | Fix hook validation errors before proceeding |
| `memory.md` | Project knowledge: YANG conventions, config pipeline, patterns |

## Commands

```bash
make ze-unit-test         # Ze unit tests with race detector (excludes chaos)
make ze-functional-test   # Ze functional tests (encode, plugin, decode, parse, reload, editor)
make ze-exabgp-test       # ExaBGP compatibility tests
make ze-fuzz-test         # Fuzz tests (10s per target)
make chaos-unit-test      # Chaos unit tests with race detector
make chaos-functional-test # In-process BGP chaos simulation
make ze-test              # All ze tests (unit + functional + exabgp + fuzz)
make chaos-test           # All chaos tests (unit + functional)
make lint                 # 26 linters via golangci-lint
make verify               # lint + ze-unit-test + ze-functional-test
make ze-test              # All tests: lint + unit + functional + exabgp + chaos + fuzz
```

## Key Architecture

Ze is a BGP daemon with a plugin architecture. Read `docs/architecture/core-design.md` for the full design, or `ai/rules/architecture-summary.md` for a condensed overview.

**Critical concepts:**
- Engine and plugins communicate over Unix socket pairs using YANG RPC (NUL-framed JSON)
- BGP messages use negotiated capabilities (ASN4, ADD-PATH, etc.) for encoding/decoding
- Wire encoding uses buffer-first pattern (`WriteTo`), never allocate-and-return
- Plugins self-register via `init()` + central registry — no manual wiring needed

## Reference Paths

- Architecture docs: `docs/architecture/` (see `ai/INDEX.md` for full map)
- RFC summaries: `rfc/short/` (implementation-ready)
- Full RFCs: `rfc/full/`
- Spec system: `plan/spec-*.md` (active), `plan/learned/` (summaries)
