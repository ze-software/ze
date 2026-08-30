# Doctor and Runtime Health Checks

Ze has two tiers of self-diagnosis. `ze doctor` runs offline against config and
the environment. The health registry runs inside the daemon and turns runtime
anomalies into warnings and a `/health` verdict.

<!-- source: internal/component/doctor/doctor.go -- offline checks and listener collection -->
<!-- source: internal/core/health/registry.go -- runtime health registry -->

## Discovery beats enumeration

Listener checks are derived from the YANG schema, through
`RegisterListenerDefault` and `CollectListenersWithDefaults`, with a hardcoded
list as a fallback. A new `ze:listener` service is then covered by doctor with
no doctor code change. The alternative, extending the YANG compiler to
propagate `refine` defaults, would have touched every consumer of the compiler.

<!-- source: internal/component/config/listener_defaults.go -- builtin listener defaults -->
<!-- source: internal/le/portdefaults/actions.go -- Answer -->

Two constraints hold this together:

- The ze YANG compiler does not process `refine`, so `LeafNode.Default` is
  always empty for a `uses zt:listener` child. Every schema-driven default
  lookup has to allow for that, not only the listener one.
- `config.YANGSchema()` succeeds with an empty schema when no YANG module is
  registered, which is the normal unit-test context. The schema-driven path
  checks that the discovered service list is non-empty and falls back, or a
  hand-built config tree gets no listener checks at all.

`show doctor` reaches the same checks through a provider
(`diagnostic.RegisterDoctorProvider`) rather than by importing `cmd/ze/doctor`
from `internal/component/cmd/show`. The dependency direction stays cmd to
internal.

## Probe the role the service actually plays

NTP is a client in ze, not a server. The reachability check sends an SNTP
probe over UDP to each configured server, matching the daemon's own clock-skew
probe. A TCP connect would hit a closed port and report a false failure. Doctor
does bind UDP/123 separately, for the NTP listener check.

Injectable function variables (`httpHead`, `probeWritable`,
`ntpServerReachable`) isolate the probes in tests, which matches the existing
doctor test style. `probeWritableDir` removes its temporary file before
checking the `Close()` error, otherwise a failing close leaks the file.

## Health checks produce warnings before reading them

`checkFirewallHealth` calls `AuditTables()` and `checkIfaceHealth` calls
`CheckAllInterfaceErrors()`. Each kernel call runs on a goroutine with a
one-second timeout, so a stuck kernel call cannot stall the health endpoint.

<!-- source: internal/component/firewall/audit.go -- firewall drift audit -->
<!-- source: internal/component/iface/health.go -- interface error counter tracking -->
<!-- source: internal/component/bgp/reactor/session_health.go -- EOR timeout and session anomaly -->

Four bounds keep the checks honest:

| Check | Bound | Reason |
|-------|-------|--------|
| Route-count anomaly | 100-prefix floor | a route-server client or a management peer holds 1 to 5 routes, and any ratio on those is noise |
| Per-family EOR | one timer per peer, expected against received family count | the warning clears only when every negotiated family has sent EOR, not on the first one |
| Firewall drift | skipped while `LastApplied() == nil` | before the first apply there is nothing to compare, so every table reads as drift |
| VPP health | `os.Stat` on the socket before dialing | without it a host with no VPP answers 503 forever |

Prefix counting is unconditional rather than gated on a configured maximum,
because the anomaly detector needs a count on every peer. An early return in
`applyPrefixDelta` skips the family-string allocation when neither metrics nor
warnings are configured.

The FIB pending map is capped at 10000 entries, so a catastrophic backend
failure cannot grow it without bound.

## The dependency inventory is a test

`TestDoctorDependencyInventory` holds an expected total. Adding a runtime
dependency without a doctor check fails it, which forces the decision to be
made rather than forgotten.

## Which check a new dependency needs

| New dependency | Doctor check |
|----------------|--------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel module requirement | `/proc/modules` check (Linux) |
| New listen address or port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity and expiry check |
| Embedded certificate material | Parse the certificate and check its validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs or sysctl dependency | Read and write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family or handle |

## Where each owner registers its check

<!-- source: internal/core/diagnostic -- RegisterDoctorCheck -->

| Dependency owner | Registration mechanism |
|------------------|------------------------|
| Internal plugin (registered by `registry.Register`) | The `Registration.DoctorChecks` field. The doctor runner bridges these at execution time through `checks_plugin_registry.go`. The check function takes `registry.DoctorCheckContext` and returns `[]rpc.DoctorCheckDiagnostic`, and Component is set from the plugin name. `l2tpauthradius/register.go` is the reference example |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from the owning component's `init()` |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from the SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from the backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or a dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the absent owner explicit |

## The two tests a new check carries

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when its config block is present, and emits the registered code | The owning package beside the registration, or `internal/component/doctor` when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor`, or the existing functional suite for that entry point |
