# Design Principles

Rationale: `.claude/rationale/design-principles.md`

| Principle | Rule |
|-----------|------|
| YAGNI | Don't build what's not immediately needed |
| Simplicity | Boring code that obviously works > clever code |
| No identity wrappers | Wrapper must transform (type conversion, error wrapping, defaults). Delete if just delegates |
| Single responsibility | One thing per function/struct/package. "And" in name = split |
| Explicit > implicit | No hidden magic, convention-based behavior, silent defaults |
| Minimize coupling | Components know minimum about each other. High→low dependency |
| Interface segregation | Clients depend only on methods they use |
| No premature abstraction | Three concrete implementations before abstracting |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail. Every input can be malformed |
| Do it right | Ze does the hard thing properly — zero-copy, pool dedup, buffer-first. Never trade correctness for speed of implementation |

## File Size

| Lines | Action |
|-------|--------|
| < 600 | Fine |
| 600–1000 | Monitor |
| > 1000 | Split by responsibility |

## 3-Fix Rule

**BLOCKING:** 3 fix attempts for same problem fail → STOP. Report to user. Wrong mental model.

## Scalability Checklist

```
[ ] No premature abstraction (3+ use cases?)
[ ] No speculative features (needed NOW?)
[ ] Single responsibility per component
[ ] Explicit behavior (no hidden magic?)
[ ] Minimal coupling
[ ] Consistent naming
[ ] Testable in isolation
[ ] Next-developer test: understood in 30 seconds?
```
