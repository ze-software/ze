---
kind: directive
level: MUST
stage:
---
- **A subscriber MUST type-assert through the typed handle (`Event[T].Subscribe`) rather than calling `bus.Subscribe` directly.** The handle's wrapper logs a warn on a type mismatch; a raw `bus.Subscribe` caller swallows the mismatch in silence. The legacy `events.AsString` shim covers events not yet migrated to a typed handle, and MUST NOT be used in new code.
