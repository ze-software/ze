---
kind: directive
level: MUST NOT
stage:
---
- **EventBus MUST NOT be used for request and response.** It is pub/sub with no return channel, so emitting a request event and subscribing for a response event adds correlation IDs, timeouts and two event registrations that a direct function call avoids entirely.
