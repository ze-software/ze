---
kind: directive
level: MAY
stage:
excepted-by: go-standards/no-backwards-compatibility/the-plugin-api-is-frozen-once-released
---
**Code under `internal/` is not user-exposed, so it MAY be changed freely, forever. No shims, no deprecation layers, and no "keep the old name working".**
