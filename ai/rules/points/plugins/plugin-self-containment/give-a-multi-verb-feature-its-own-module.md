---
kind: note
level:
stage:
---
When one feature spreads across several verbs (a one-shot root command, a `show`
view, a `monitor` stream, a `resolve` variant), give the feature its own module
`internal/component/<feature>` that owns every one of those commands, rather than
scattering them across the verb packages. Create the module if none exists. When
two such modules would share low-level primitives (e.g. ping and traceroute both
build ICMP echo packets and resolve targets), extract those primitives to a
`internal/core/<x>` package (e.g. `internal/core/probe`) so neither feature
module depends on the other or on a central verb package.
