---
kind: table
level: MUST
stage:
---
| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Plugin name |
| `Description` | string | Yes | Human-readable description |
| `RunEngine` | func(Conn, Conn) int | Yes | Engine-mode entry point |
| `CLIHandler` | func([]string) int | Yes | CLI dispatch handler |
| `Families` | []string | No | Address families ("afi/safi") |
| `CapabilityCodes` | []uint8 | No | Capability codes decoded |
| `ConfigRoots` | []string | No | Config roots plugin wants |
| `Dependencies` | []string | No | Plugin names that MUST also be loaded. Missing name -> `ErrMissingDependency`. |
| `OptionalDependencies` | []string | No | Plugin names the owner uses when loaded but can run without. Missing name is silently skipped (no error). Owner must handle runtime absence gracefully. |
| `YANG` | string | No | YANG schema content |
| `InProcessNLRIDecoder` | func | No | NLRI decode |
| `InProcessNLRIEncoder` | func | No | NLRI encode |
| `EventTypes` | []string | No | Event types this plugin produces (registered at startup) |
| `SendTypes` | []string | No | Send types this plugin enables (e.g., ["enhanced-refresh"]). Registered dynamically at startup. |
| `Claims` | []string | No | Exclusive runtime roles this plugin takes over from another plugin's default behavior (e.g., ["bgp-peer-up-replay"]). See "Exclusive Role Claims" above. |
| `PeerUpBarrier` | bool | No | This plugin registers the peer (forward target, per-peer cut) on the peer-up event, so the peer's initial-sync End-of-RIB must not overtake it. See "Peer-Up Barrier" above. |
| `DoctorChecks` | []DoctorCheckDef | No | Doctor readiness checks this plugin provides. Each entry carries metadata (name, phase, order, platforms, codes) and a check function. Component is set from the plugin Name. See `ai/rules/repo-maintenance.md`. |
| `Features` | string | No | Space-separated flags ("nlri yang capa") |
