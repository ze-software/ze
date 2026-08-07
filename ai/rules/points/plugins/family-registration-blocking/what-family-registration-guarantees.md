---
kind: table
level:
stage:
---
| Rule | Detail |
|------|--------|
| One canonical name per family | No aliases. The `afiStr/safiStr` arguments form the canonical `<afi>/<safi>` string. |
| Registration is fatal on conflict | `family.MustRegister` panics if AFI or SAFI numbers collide with a different name. Same name + same numbers is a no-op. |
| Plugin owns the SAFI name | `vpn` plugin chose `mpls-vpn`; `flowspec` plugin chose `flow`. The plugin is the authority. |
| External plugins use the protocol | Forked plugins declare families in `declare-registration` (Stage 1) with AFI/SAFI numbers; the engine forwards to `family.RegisterFamily` via `registerPluginFamilies` in `plugin/server/startup.go`. See `docs/architecture/api/process-protocol.md`. |
| Test packages call `family.RegisterTestFamilies()` | If a test exercises a SAFI not registered by an internal plugin, register it via the helper in `internal/core/family/testfamilies.go`. |
