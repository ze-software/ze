---
kind: directive
level: MUST
stage:
---
**The barrier MUST be counted over the delivery set, never the registry.** A
plugin that declares the barrier but is not subscribed to state events never
takes delivery, so counting it would cost every peer the full barrier timeout.
