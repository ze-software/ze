---
kind: directive
level: MUST
stage:
---
**Naming convention:** subsystem and log keys MUST use dots (`bgp.gr`,
`bgp.rib`). Plugin names registered with `registry.Register()` MUST use
hyphens (`bgp-gr`, `bgp-rib`). The two are NOT the same string. The hub
canonicalizes hyphen -> dot for in-process subsystem names (`938df51d`). When
a new plugin is added, it MUST be registered with the hyphen form, and every
config / log / env consumer MUST use the dot form (or the canonicalized form,
depending on which side of the hub it lives on).
