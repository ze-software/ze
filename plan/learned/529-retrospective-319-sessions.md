# Retrospective: 319 Sessions, 532 Learned Summaries

Date: 2026-04-05

## What This Is

Mining of 319 Claude Code session files (1.09 GB) and 532 learned summaries to find recurring patterns, friction points, and systemic issues. Inspired by great_cto's /update session-history-mining approach.

## Key Numbers

| Metric | Value |
|---|---|
| Total sessions | 319 (1.09 GB) |
| Marathon sessions (>2M) | 42% |
| Median session duration | 63 min |
| Sessions over 2 hours | 40% |
| User correction messages | 134 across 80 sampled sessions |
| Skills invoked per session | 0.15 (barely used) |
| Tool parallelism | Zero -- every turn used exactly 1 tool call |

## Five Systemic Problems (by frequency)

### 1. "Done" means "tests pass" (47 corrections)

Claude says "all pass" and stops. Docs, audit, learned summary, spec completion gates all left undone. The user had to ask "what is left?" session after session. Rules added: quality.md ("tests are step 10 of 12"), anti-rationalization.md.

### 2. Features not wired (8+ specs, #1 in mistake log)

Code written, unit tests pass, feature unreachable from config/CLI/API. Documented across SSH, LG, forward pool, web, and more. The fix (.ci functional tests) is itself the most commonly deferred item.

### 3. Over-engineering before reading code (10+ specs)

Elaborate solutions designed before understanding what exists. Examples: 5-stage async protocol for sync calls (deleted), custom HTMX shim when htmx.min.js vendored, Span type later removed. Root cause: "what would be elegant?" instead of "what does the code do?"

### 4. Dismissing test failures (5 sessions, most distress)

Despite RECURRING + ZERO TOLERANCE in mistake log, Claude continued saying "pre-existing unrelated." Caused the strongest negative user reactions across all sessions.

### 5. git add/commit running despite being forbidden

193 git add calls, 54 git commit calls in sampled sessions. Now blocked by hooks.

## Surprising Findings

- **Skills barely used**: 47 total invocations / 319 sessions. User types naturally, not slash commands. Skills are for Claude's internal structure and contributors, not the primary user.
- **Zero tool parallelism**: No multi-tool turns observed. Significant efficiency loss in marathon sessions.
- **66% of editing sessions have 20+ turns before first edit**: Heavy upfront reading validates "read before writing" rules but suggests room for efficiency improvement.

## From Learned Summaries: Top Patterns

| Pattern | Frequency | Root Cause |
|---|---|---|
| Over-engineering before understanding | 10+ specs | Starting from elegance not reality |
| Features not wired | 8+ specs | Unit tests = false confidence |
| goimports cascade friction | 8+ specs | Auto-linter removes imports on transient states |
| Import cycles block restructuring | 7+ specs | Deep coupling between components |
| Wrong production path identified | 6+ specs | Finding "a" implementation, not "the" implementation |
| Tests prove mechanism not behavior | 5+ specs | Testing code path instead of AC text |
| Concurrency bugs in hot paths | 5+ specs | Channel/goroutine shutdown timing |
| Deferred functional tests never land | Pervasive | "Needs infrastructure" justification |

## Architecture Patterns That Succeed

- Registry + init() self-registration
- Zero-copy / offset-based / lazy parsing
- Declarative state reconciliation (desired vs actual)
- "Expand then contract" for refactoring
- Instrument before theorizing

## What Was Done in Response

1. Session-start hook suggests `/ze-status` when no spec selected
2. Pre-commit hook now blocks specs with zero .ci test references
3. 19 skills consolidated with ze- prefix, cross-references, model tiering
4. `/ze-commit` includes .claude health check (stale refs, broken links)

## Gotchas

- Deferred functional tests are the #1 gap that rules and hooks have not yet closed
- goimports cascades remain a tooling limitation -- documented but not solvable
- Import cycles require the arch-0 restructuring to fully resolve
- UI/web specs should use behavior ACs, not layout ACs
