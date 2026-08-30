---
kind: directive
level: MUST
stage:
rationale: ai/rationale/buffer-first.md
---
- **All wire encoding MUST write into pooled, bounded buffers.**
- **`fmt.Sprintf`, `fmt.Fprintf` and `fmt.Errorf` MUST NOT be used where a zero-allocation or lower-allocation alternative exists.**
- **`.String()` concatenation MUST NOT be used on a hot path where an append-into-buffer pattern exists.**
- **This file MUST be read before any allocation or memory-lifecycle decision.** It carries the obligations; the model behind them (data lifecycle, caller-owned buffers, pool strategy, the wire abstractions) is `docs/architecture/buffer-architecture.md`, and the string-building reference is `docs/architecture/textbuf-string-building.md`.
- Principle: `ai/rules/architecture.md` -- encapsulation onion plus buffer-first encoding.
