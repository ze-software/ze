---
kind: directive
level: MUST NOT
stage:
---
- `<component>` MAY contain hyphens for a multi-word name (`ddos-detect`, `firewall-irr`). The `:conf`, `:cmd` or `:api` kind MUST NOT be fused into it with a hyphen, and it MUST NOT be dropped. A plugin's `conf` and `cmd` modules MUST use the same scheme. The module identity table, and the one plugin that currently breaks this, is `docs/architecture/config/yang-config-design.md`.
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. They MUST NOT be reused for another import.
