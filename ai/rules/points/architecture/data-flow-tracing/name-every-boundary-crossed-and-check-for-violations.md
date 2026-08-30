---
kind: directive
level: MUST
stage:
---
- [ ] 3. You MUST name every boundary crossing: Engine <-> Plugin (JSON over pipes), FSM <-> Reactor (event types), WireUpdate <-> RIB (attribute refs), Caps <-> EncodingContext (`internal/core/bgp/context`)
- [ ] 4. You MUST check for: violations? Bypassed layers? Unintended coupling? Duplicated functionality? Broken zero-copy?
