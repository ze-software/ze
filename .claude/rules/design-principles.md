# Design Principles

Rationale: `.claude/rationale/design-principles.md`

## Rules

| Principle | Rule |
|-----------|------|
| YAGNI | Do not build features/abstractions not immediately needed |
| Simplicity | Boring code that obviously works > clever code that might work |
| No identity wrappers | Wrapper must transform interface (type conversion, error wrapping, defaults). Delete if it just delegates |
| Single responsibility | Each function/struct/package does ONE thing. "And" in name = split it |
| Explicit > implicit | No hidden magic, convention-based behavior, or silent defaults |
| Minimize coupling | Components know as little as possible about each other. High→low dependency direction |
| Interface segregation | Clients depend only on methods they use |
| No premature abstraction | Three concrete implementations before abstracting |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail. Every input can be malformed |

## File Size

| Lines | Action |
|-------|--------|
| < 600 | Fine |
| 600–1000 | Monitor growth |
| > 1000 | Split by responsibility |

## The 3-Fix Rule

**BLOCKING:** If 3 fix attempts for the same problem fail, STOP. Report to user. Three failures = wrong mental model.

## Scalability Checklist

```
[ ] No premature abstraction (3+ use cases?)
[ ] No speculative features (needed NOW?)
[ ] Single responsibility per component
[ ] Explicit behavior (no hidden magic?)
[ ] Minimal coupling
[ ] Consistent naming
[ ] Testable in isolation
[ ] Next-developer test: would they understand this in 30 seconds?
```
