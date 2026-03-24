# Design Principles

Rationale: `.claude/rationale/design-principles.md`

| Principle | Rule |
|-----------|------|
| YAGNI | Don't build what's not immediately needed |
| Simplicity | Boring code that obviously works > clever code |
| No identity wrappers | Wrapper must transform (type conversion, error wrapping, defaults). Delete if just delegates. A struct holding raw data + accessor methods is an identity wrapper — pass the data, use existing type methods directly |
| Single responsibility | One thing per function/struct/package. "And" in name = split |
| Explicit > implicit | No hidden magic, convention-based behavior, silent defaults |
| Minimize coupling | Components know minimum about each other. High→low dependency |
| Interface segregation | Clients depend only on methods they use |
| No premature abstraction | Three concrete implementations before abstracting |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail. Every input can be malformed |
| Do it right | Ze does the hard thing properly — zero-copy, pool dedup, buffer-first. Never trade correctness for speed of implementation |
| Durability over velocity | Optimize for "never revisit this code" not "get to commit fast". Missing edge cases, shallow tests, unwired features all create rework. Rework wastes more of the user's time than thoroughness ever could |
| Lazy over eager | Pass raw byte slices, not parsed structs. Use offset-based iterators (Next() yields one element), not collected slices. Consumer walks data and acts directly — no intermediate maps or structs built to iterate once. Never wrap raw data in a new struct with accessor methods — use existing wire type methods or standalone functions. Optimizing N→1 is wrong when the answer is N→0-until-needed |

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
