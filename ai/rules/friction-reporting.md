# Friction Reporting

When encountering anything that causes confusion, wasted effort, or surprise — **immediately inform the developer** and propose a `.claude` improvement to prevent it in future sessions.

## Triggers

| Category | Examples |
|----------|----------|
| **Surprise** | Pattern contradicts documented conventions, behavior differs from what rules describe |
| **Confusion** | Ambiguous rule, unclear which of two conventions applies, conflicting guidance |
| **Missing docs** | Had to investigate something that should have been documented (file purpose, data flow step, registration pattern) |
| **Wasted effort** | Searched for something in the wrong place, wrote code that duplicated existing functionality, misunderstood a boundary |
| **Stale info** | Rule or doc references deleted/renamed files, describes a pattern the code no longer follows |
| **Tooling friction** | Hook rejects valid code, linter config doesn't match rules, make target behaves unexpectedly |

## Format

```
Friction: [what happened]
Impact: [time/effort wasted, or risk if not addressed]
Proposed fix: [specific .claude file + change]
```

## When

- During research, implementation, or review
- As soon as noticed — do not wait until end of session

## Do Not Report

- Things that are simply unfamiliar (read docs first)
- Intentional deviations already documented in specs or rationale files
- One-off issues unlikely to recur
