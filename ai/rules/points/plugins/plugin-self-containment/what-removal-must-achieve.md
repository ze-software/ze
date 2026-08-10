---
kind: directive
level: MUST
stage:
---
1. **Every user-visible feature of that plugin MUST be removed** and nothing else: CLI commands, `show`/`set`/`clear`/`delete`/`update` subtrees, RPC registration, offline command registration, YANG command schema, help and usage text, completion entries, web/looking-glass routes, doctor checks, metrics.
2. **The build MUST stay green.** No dangling reference, no orphaned command spelling, no half-registered command anywhere.
3. **Every other plugin and the core MUST keep fully working.** Removing BGP MUST NOT break iface, firewall, l2tp, or the generic command plumbing.
