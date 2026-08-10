---
kind: directive
level: MUST NOT
stage:
---
**Anti-pattern:** MUST NOT use EventBus for request/response. EventBus is
pub/sub with no return channel. Emitting a request event and subscribing for a response event
adds complexity (correlation IDs, timeouts, two event registrations) that a
direct function call avoids entirely.
