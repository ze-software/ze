---
kind: directive
level: MUST
stage:
---
- `<component>` may contain hyphens for a multi-word name (`ddos-detect`, `firewall-irr`). The `:conf`/`:cmd`/`:api` kind is never fused into it with a hyphen and is never dropped. A plugin's `conf` and `cmd` modules MUST use the same scheme (today `ddos-local` mixes `urn:ze:ddos-local-conf` with `urn:ze:ddos-local:cmd`, and that is the bug this row forbids).
- Reserved prefixes: `zt` = ze-types, `ze` = ze-extensions. Never reuse them for another import.
