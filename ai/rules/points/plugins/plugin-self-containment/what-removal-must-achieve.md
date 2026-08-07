---
kind: directive
level:
stage:
---
1. **Remove every user-visible feature of that plugin** and nothing else: CLI commands, `show`/`set`/`clear`/`delete`/`update` subtrees, RPC registration, offline command registration, YANG command schema, help and usage text, completion entries, web/looking-glass routes, doctor checks, metrics.
2. **Keep the build green.** No dangling reference, no orphaned command spelling, no half-registered command anywhere.
3. **Keep every other plugin and the core fully working.** Removing BGP must not break iface, firewall, l2tp, or the generic command plumbing.
