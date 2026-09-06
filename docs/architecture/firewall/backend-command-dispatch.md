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
by the native write hook, which enforces explicit over implicit registration.
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

The warning-code check was INLINED in the moved firewall health check rather than
exported as `checkWarningCodes()`. Two hardcoded strings do not justify a new
cross-package dependency.

## The ruleset handler answers with its sets, and lets an owner add to them

<!-- source: internal/plugins/firewall/nft/cmd_show.go -- handleShowFirewallRuleset -->

`handleShowFirewallRuleset` returned `table`, `family` and `chains` and no set
elements at all, so an operator reading a ruleset saw the chains that named a
set and nothing about what was in it. It now returns `sets` beside them.

It then calls `show.Enrich("show firewall ruleset", data)` before returning. A
registered enricher adds what only its owner knows: `firewall-domain` attaches
the DNS name that supplied each address, which is why the addresses had to be
in the payload first. An owner with nothing to add returns nothing, so a node
running neither plugin renders exactly as before.

The alternative was a provenance field on the shared `firewall.SetElement`,
carried unused by copp, policy-routes, flowspec, vrrp and firewall-irr. That is
the per-feature edit to a shared field list `ai/rules/principles.md` refuses,
and it is the same argument this page already makes for health checks: the
knowledge belongs to the owner, and the central package aggregates without
knowing which owners exist.

## Consequences

`cmd/show/` no longer imports `firewall`: one fewer cross-component edge. VPP
socket-path knowledge is local to the VPP plugin.

This is what makes a mixed-backend future work. Each backend registers its own
health check, and `health.Check()` aggregates without any central package knowing
which backends exist. IKE (`ike/engine/health.go`) and PKI (`pki/health.go`)
already followed this shape.
