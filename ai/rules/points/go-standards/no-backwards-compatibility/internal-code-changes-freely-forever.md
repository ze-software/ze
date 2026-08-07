---
kind: note
level:
stage:
excepted-by: go-standards/no-backwards-compatibility/the-plugin-api-is-frozen-once-released
---
Code under `internal/` is not user-exposed. It follows the no-backwards-compatibility rule forever: change it freely, no shims, no deprecation layers, no "keep the old name working".
