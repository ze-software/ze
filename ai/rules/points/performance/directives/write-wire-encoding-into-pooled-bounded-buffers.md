---
kind: directive
level: MUST
stage:
---
- **All wire encoding MUST write into pooled, bounded buffers.**
- **Never use `fmt.Sprintf`, `fmt.Fprintf`, or `fmt.Errorf` when a zero-allocation or lower-allocation alternative exists.**
- **Never use `.String()` concatenation on a hot path when an append-into-buffer pattern exists.**
- **Read this file before any allocation or memory-lifecycle decision.** It carries the conceptual model (data lifecycle, caller-owned buffers, pool strategy) and the mechanical rules for encoding and string formatting.
- Principle: `ai/rules/architecture.md` -- Encapsulation onion + Buffer-first encoding.
- Rationale: `ai/rationale/buffer-first.md`
- Reference: git log -- plan/analysis-printf-allocations.md (completed, removed from tree)
