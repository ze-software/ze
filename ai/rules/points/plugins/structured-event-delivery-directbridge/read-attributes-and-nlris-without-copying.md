---
kind: note
level:
stage:
---
Plugins read attributes via `AttrsWire.Get(code)` (lazy, per-attribute) and NLRIs via `WireUpdate.NLRI()` / `MPReach()` / `MPUnreach()` (zero-copy byte slices). External/forked plugins continue receiving JSON text via `OnEvent`.
