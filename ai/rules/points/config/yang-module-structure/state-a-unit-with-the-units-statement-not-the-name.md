---
kind: directive
level: MUST
stage:
---
**A leaf whose value carries a physical unit (time, rate, size) MUST state the unit once, via the YANG `units` statement, MUST keep the leaf name unit-free, and MUST carry a protocol-sane `default`:** `leaf hello-interval { type uint32; units seconds; default 10; }`
