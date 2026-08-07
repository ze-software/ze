---
kind: directive
level:
stage:
---
**A leaf whose value carries a physical unit (time, rate, size) states the unit once, via the YANG `units` statement, keeps the leaf name unit-free, and carries a protocol-sane `default`:** `leaf hello-interval { type uint32; units seconds; default 10; }`
