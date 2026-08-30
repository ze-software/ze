---
kind: directive
level: MUST
stage:
rationale: ai/rationale/design-principles.md
---
**Every design decision MUST satisfy these principles. Where a principle names a rule, that rule governs the detail.**

| Principle | Rule |
|-----------|------|
| YAGNI | Do not build what is not needed now |
| Simplest correct solution | The simplest answer that is FULLY correct, and nothing beyond it. It cuts machinery, never correctness. `ai/rules/simplicity.md` owns this and is BLOCKING |
| Simplicity | Boring code that obviously works beats clever code |
| No identity wrappers | A wrapper MUST transform: a type conversion, error wrapping, a default. A struct holding raw data plus accessors is an identity wrapper, so pass the data and use the existing type's methods |
| Single responsibility | One thing per function, struct and package. "And" in the name means split it |
| Explicit over implicit | No hidden magic, no convention-based behavior, no silent default |
| Minimize coupling | Each component knows the minimum about the others, and dependencies run high to low |
| Interface segregation | A client depends only on the methods it uses |
| Abstract when you can | Two concrete use cases justify an abstraction. Abstract at the second, do not wait for a third |
| Design for change | Isolate volatility behind a stable interface |
| Fail-mode awareness | Every external call CAN fail and every input CAN be malformed |
| Do it right | Zero-copy, pool dedup, buffer-first. Correctness MUST NOT be traded for implementation speed |
| Exact or reject | A backend or translator that cannot apply config EXACTLY MUST fail verify or commit with a clear error. No silent approximation. `ai/rules/protocol.md` |
| Durability over velocity | "Never revisit this code" beats "get to commit fast". Rework costs more than thoroughness |
| Encapsulation onion | Allocate one buffer at the outermost protocol layer and slice inward with specialised types. Peel by narrowing the window, never by copying. `docs/architecture/buffer-architecture.md` |
| Buffer-first encoding | All wire encoding goes into pooled bounded buffers through `WriteTo(buf, off) int`. `ai/rules/performance.md` |
| No `make` where pools exist | A variable-N `make([]byte, N)` on a wire-facing path MUST come from a bounded pool. `make` stays correct for a fixed-size header and a one-shot startup allocation |
| Pool strategy by goroutine shape | A single-backing ring for one sequential goroutine, a pool seeded for peak where several goroutines share. Every buffer in one pool is the SAME maximum size |
| Lazy over eager | Read side: raw byte slices plus offset iterators (`Next()`), never parsed structs or collected slices |
| Zero-copy, copy-on-modify | Allocate at receive, share read-only through forwarding, copy only when an egress filter modifies, release after send. `docs/architecture/buffer-architecture.md` |
