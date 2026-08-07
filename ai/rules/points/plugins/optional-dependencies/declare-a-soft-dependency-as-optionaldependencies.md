---
kind: note
level:
stage:
---
Plugins that USE another plugin when it is present but can run without it
declare the relationship as `OptionalDependencies` rather than `Dependencies`.
Example: `bgp-rs` uses `bgp-adj-rib-in` for replay-on-peer-up when present,
and disables replay (with a one-shot WARN log) when it is not.
