---
kind: directive
level:
stage:
---
**Naming convention:** subsystem and log keys use dots (`bgp.gr`, `bgp.rib`).
Plugin names registered with `registry.Register()` use hyphens (`bgp-gr`,
`bgp-rib`). The two are NOT the same string. The hub canonicalizes hyphen ->
dot for in-process subsystem names (`938df51d`). When you add a new plugin,
register it with the hyphen form AND make sure every config / log / env
consumer uses the dot form (or the canonicalized form, depending on which
side of the hub it lives on).
