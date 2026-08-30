---
kind: directive
level: MUST
stage:
---
- **Deleting a plugin's package directory plus its blank import in `internal/component/plugin/all/all.go` MUST remove every user-visible feature of that plugin and nothing else:** CLI commands, `show`/`set`/`clear`/`delete`/`update` subtrees, RPC registration, offline command registration, YANG command schema, help and usage text, completion entries, web and looking-glass routes, doctor checks, metrics.
- **The build MUST stay green after that deletion:** no dangling reference, no orphaned command spelling, no half-registered command anywhere.
- **Every other plugin and the core MUST keep fully working after it.** Removing BGP MUST NOT break iface, firewall, l2tp or the generic command plumbing.
