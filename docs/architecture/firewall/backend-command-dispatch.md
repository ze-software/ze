# Backend-Dispatched Health Checks

`cmd/show/health_checks.go` was a dependency magnet. It imported `firewall` to
call `AuditTables()` and hardcoded `/run/vpp/api.sock` to probe VPP health. Those
checks belong in the backend plugin package, not in a central show-command
package.

## The move

<!-- source: internal/plugins/firewall/nft/health.go -- firewall health check -->
<!-- source: internal/plugins/iface/vpp/health.go -- VPP health check -->

`checkFirewallHealth` moved to `plugins/firewall/nft/health.go`, and
`checkVPPHealth` moved to `plugins/iface/vpp/health.go`. It is the registration
pattern the RPC handlers already use.

`checkIfaceHealth`, `checkBGPHealth`, `checkFIBHealth` and `checkPluginHealth`
did NOT move. They are not backend-specific.

## Registration is explicit

<!-- source: internal/plugins/firewall/nft/register.go -- RegisterHealthCheck -->

Each package exports `RegisterHealthCheck()` and calls it from the existing
`register.go` `init()`. A direct `health.Register()` inside `init()` is rejected
by `.claude/hooks/block-init-register.sh`, which enforces explicit over implicit
registration.

The warning-code check was INLINED in the moved firewall health check rather than
exported as `checkWarningCodes()`. Two hardcoded strings do not justify a new
cross-package dependency.

## Consequences

`cmd/show/` no longer imports `firewall`: one fewer cross-component edge. VPP
socket-path knowledge is local to the VPP plugin.

This is what makes a mixed-backend future work. Each backend registers its own
health check, and `health.Check()` aggregates without any central package knowing
which backends exist. IKE (`ike/engine/health.go`) and PKI (`pki/health.go`)
already followed this shape.
