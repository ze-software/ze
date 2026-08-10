---
kind: directive
level: MUST
stage:
---
- `<component>` MAY contain hyphens for a multi-word name (`ddos-detect`, `firewall-irr`). The `:conf`/`:cmd`/`:api` kind MUST NOT be fused into it with a hyphen and MUST NOT be dropped. A plugin's `conf` and `cmd` modules MUST use the same scheme (today `ddos-local` mixes `urn:ze:ddos-local-conf` with `urn:ze:ddos-local:cmd`, and that is the bug this row forbids).
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. They MUST NOT be reused for another import.
