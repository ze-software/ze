# Design Principles

Rationale + examples: `.claude/rationale/design-principles.md`.
Detail for the pool/buffer/lazy principles: `rules/buffer-first.md`,
`rules/exact-or-reject.md`.

| Principle | Rule |
|-----------|------|
| YAGNI | Don't build what's not immediately needed |
| Simplicity | Boring code that obviously works > clever code |
| No identity wrappers | Wrapper must transform (type conversion, error wrapping, defaults). A struct holding raw data + accessor methods is an identity wrapper -- pass the data, use existing type methods |
| Single responsibility | One thing per function/struct/package. "And" in name = split |
| Explicit > implicit | No hidden magic, convention-based behavior, silent defaults |
| Minimize coupling | Components know the minimum about each other. High->low dependency |
| Interface segregation | Clients depend only on methods they use |
| No premature abstraction | Three concrete implementations before abstracting |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail; every input can be malformed |
| Do it right | Zero-copy, pool dedup, buffer-first. Never trade correctness for implementation speed |
| Exact or reject | Backend/translator cannot apply config EXACTLY -> verify/commit fails with a clear error. No silent approximation. `rules/exact-or-reject.md` |
| Durability over velocity | "Never revisit this code" > "get to commit fast". Rework wastes more time than thoroughness |
| Encapsulation onion | Allocate one buffer at the outermost protocol layer; slice inward with specialised types (`WireUpdate`, `PackContext`). Peel by narrowing the window, never by copying |
| Buffer-first encoding | Write side: all wire encoding into pooled, bounded buffers via `WriteTo(buf, off) int`. No `append`, `make`, or `buildFoo() []byte` in helpers. `rules/buffer-first.md` |
| No `make` where pools exist | Variable-N `make([]byte, N)` on a wire-facing path comes from a pre-allocated, bounded pool. `make` is OK for fixed-size headers and one-shot startup allocations |
| Pool strategy by goroutine shape | Single-backing ring (single reactor goroutine, sequential) OR `sync.Pool` seeded for peak (multiple concurrent goroutines). All buffers in a pool are the SAME MAX size |
| Lazy over eager | Read side: raw byte slices + offset iterators (`Next()`), not parsed structs or collected slices. Consumer acts on data directly. N->0-until-needed, not N->1 |
| Zero-copy, copy-on-modify | Allocate at receive (Incoming Peer Pool); share read-only through forwarding; copy only when egress filters modify (Outgoing Peer Pool); release after send. Global Shared Pool handles overflow |

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
