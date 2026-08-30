---
kind: directive
level: MUST NOT
stage:
---
**A load excuse MUST NOT be written into a `plan/known-failures/` shard.** `checkLoadExcuses` (`internal/le/doc/wiring/docwiring.go`) fails a changed shard that carries one.
**The gate checks the EXCUSE, not the existence of a shard.** A red whose mechanism is genuinely unknown still belongs there.
