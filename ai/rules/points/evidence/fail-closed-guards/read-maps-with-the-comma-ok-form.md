---
kind: directive
level: SHOULD
stage:
---
**Go's bare map read is the archetype: `m[k]` on an absent key yields the zero value with no signal. You SHOULD write `v, ok := m[k]` and handle `!ok` explicitly.**
