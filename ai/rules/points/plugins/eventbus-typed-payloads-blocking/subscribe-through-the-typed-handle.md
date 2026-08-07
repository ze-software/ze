---
kind: note
level: MUST
stage:
---
Subscribers MUST type-assert via the typed handle (`Event[T].Subscribe`)
rather than calling `bus.Subscribe` directly. The handle's wrapper logs a
warn on type mismatch; raw `bus.Subscribe` callers swallow mismatches
silently. The legacy `events.AsString` shim exists only for events that
have not yet migrated to a typed handle and is not for new code.
