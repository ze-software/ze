# 786 -- Backend-Dispatched Health Checks

## Context

`cmd/show/health_checks.go` was a dependency magnet: it imported `firewall` to call `AuditTables()` and hardcoded `/run/vpp/api.sock` to probe VPP health. These backend-specific checks belonged in their backend plugin packages, not in a central show command package. The goal was to apply the same registration pattern already used for RPC handlers (which had been moved earlier) to health checks.

## Decisions

- Moved `checkFirewallHealth` to `plugins/firewall/nft/health.go` and `checkVPPHealth` to `plugins/iface/vpp/health.go`, over leaving them in `cmd/show/` with cross-component imports.
- Used explicit `RegisterHealthCheck()` exported function called from `register.go` init(), over direct `health.Register()` in `init()`. The hook `block-init-register.sh` enforces explicit-over-implicit registration.
- Inlined the warning-code check (two codes) in the moved firewall health check, over exporting `checkWarningCodes()`. Two hardcoded strings do not justify a new cross-package dependency.
- Did not move `checkIfaceHealth`, `checkBGPHealth`, `checkFIBHealth`, or `checkPluginHealth` because they are not backend-specific.

## Consequences

- `cmd/show/` no longer imports `firewall`. One fewer cross-component edge.
- VPP socket path knowledge is now local to the VPP plugin. Enables mixed-backend future: each backend registers its own health independently, and `health.Check()` aggregates without a central package knowing which backends exist.
- IKE (`ike/engine/health.go`) and PKI (`pki/health.go`) already followed this pattern. Firewall and VPP now match.

## Gotchas

- The original spec described moving `vpp_trace.go` and `firewall.go` RPC handlers, but that work was already completed. The spec needed rewriting to target the actual remaining coupling (health checks).
- The `block-init-register.sh` hook rejects `health.Register()` inside `init()`. Must use the exported-function pattern (`RegisterHealthCheck()` called from existing init).

## Files

- `internal/plugins/firewall/nft/health.go` (created)
- `internal/plugins/firewall/nft/health_test.go` (created)
- `internal/plugins/iface/vpp/health.go` (created)
- `internal/plugins/iface/vpp/health_test.go` (created)
- `internal/plugins/firewall/nft/register.go` (modified: added RegisterHealthCheck call)
- `internal/plugins/iface/vpp/register.go` (modified: added RegisterHealthCheck call)
- `internal/component/cmd/show/health_checks.go` (modified: removed two functions, imports, constant)
- `internal/component/cmd/show/health_checks_test.go` (modified: removed moved function reference)
- `internal/component/cmd/show/show.go` (modified: removed two health.Register calls)
