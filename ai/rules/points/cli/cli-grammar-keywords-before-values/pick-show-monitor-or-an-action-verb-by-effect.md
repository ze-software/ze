---
kind: directive
level: MUST
stage:
---
- **No -- it only reports current state.** Use `show` (one snapshot) or
  `monitor` (a continuous stream of the same read). These are the read-only
  verbs (`command.IsReadOnlyVerb`); they never alter protocol or dataplane
  state. **Deep introspection stays here:** a view that only *observes* internal
  state (`show ospf database opaque-area detail`, `show bgp peer name <n> rib`)
  is `show`, however low-level -- reading is not debugging.
- **Yes, and it is a normal operational action or lifecycle change.** Use the
  existing action verbs (`request`, `clear`, `create`, `set`/`delete`, `update`,
  `cache`). Not `debug`.
- **Yes, and the change is a deliberate diagnostic PERTURBATION of the running
  protocol/dataplane** -- inject, force, corrupt, drop, or toggle a fault/
  injection mode for testing or introspection. Use `debug`. A `debug` command
  changes the router's behaviour, so it MUST be double-gated: authz (`deny
  debug`) plus an explicit, fail-closed runtime enablement
  (see `internal/plugins/ospf/debug_enable.go`).
