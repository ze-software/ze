# 439 -- RIB Extensibility

## Context

Three hardcoded patterns in the RIB and attribute packages coupled core code to plugin-specific knowledge: a switch statement in `community.go:String()` that named LLGR communities, boolean fields (`Stale`, `LLGRStale`) on RouteEntry that grew with each GR mechanism, and a command switch in `rib_commands.go` that expanded with every new RIB operation. Adding GR or LLGR features required editing core files. The goal was to make these extension points plugin-driven so that GR, LLGR, and future plugins extend RIB behavior without modifying core code.

## Decisions

- Chose a package-level `communityNames` map with `RegisterCommunityName()` over keeping the hardcoded switch. RFC 1997 built-ins pre-populated; plugins register during init(). Init-time only, read-only at runtime, no mutex needed.
- Chose `StaleLevel uint8` with `DepreferenceThreshold = 2` over `Stale bool` + `LLGRStale bool`. Level 0 = fresh, 1 = GR-stale (competes normally per RFC 4724), 2+ = LLGR-stale (depreferred per RFC 9494). Wider than needed today (3 values) to avoid another refactor for future stale mechanisms.
- Chose package-level `registeredCommands` map in `rib_commands.go` over adding `RIBCommands` field to the Registration struct. Simpler, keeps command registration local to the RIB package.
- Chose generic composable commands (`attach-community`, `delete-with-community`, `mark-stale [level]`) over LLGR-specific commands (`enter-llgr`). The RIB has zero LLGR knowledge; the GR plugin composes generic operations via DispatchCommand.

## Consequences

- Adding a new well-known community name requires one `RegisterCommunityName()` call in an init() function, not editing `attribute/community.go`.
- Adding a new stale mechanism requires choosing a level number and passing it to `mark-stale`. No new boolean fields, no best-path code changes.
- Adding new RIB commands requires calling `registerCommand()` in `rib_commands.go`, not extending a switch.
- The composable command pattern means future protocol extensions (RPKI stale, custom purge policies) can reuse the same building blocks without RIB modifications.

## Gotchas

- The spec designed `RIBCommands` on the Registration struct and `enter-llgr` as a dedicated command. The implementation chose a better design (package-local registry, composable commands). The spec became stale before implementation started because the LLGR spec series implemented the work with different design choices.
- `RegisterCommunityName` is idempotent (same value+name is a no-op) but rejects re-registration with a different name. This is intentional: silent override would mask plugin conflicts.

## Files

- `internal/component/bgp/attribute/community.go` -- community name registry
- `internal/component/bgp/plugins/rib/rib_commands.go` -- command registry + dispatch + composable community commands
- `internal/component/bgp/plugins/rib/storage/routeentry.go` -- StaleLevel uint8, StaleLevelFresh, DepreferenceThreshold
- `internal/component/bgp/plugins/rib/bestpath.go` -- threshold-based stale comparison in ComparePair
- `internal/component/bgp/plugins/gr/register.go` -- LLGR community registration + dependencies
