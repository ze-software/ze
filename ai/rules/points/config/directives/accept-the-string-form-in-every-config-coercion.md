---
kind: directive
level: MUST
stage:
---
**Every delivered config value arrives as a JSON string, so every coercion MUST carry a `case string:` arm and MUST NOT assert `v.(bool)` or `v.(float64)` directly.** `./le config coercion check` refuses both shapes. A `leaf-list` MUST be read with `configvalue.LeafList`, a `list` with `configvalue.ListEntries`, an `ordered-by user` list with `configorder.Entries`, and a plugin's config MUST be lowered with `Tree.ToPluginMap` rather than `Tree.ToMap`; a slice assertion MUST NOT be used on any of them. The shape each node arrives in is `docs/architecture/config/yang-config-design.md`.
