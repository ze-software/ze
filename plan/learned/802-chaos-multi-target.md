# 802 -- ze-chaos Multi-Target Config Generation (FRR, BIRD)

## Context

ze-chaos could only generate Ze-specific config and fork Ze. To test the same BGP scenario against FRR (bgpd) and BIRD for interop comparison, ze-chaos needed config generators and fork support for both daemons. The key architectural difference: Ze uses per-peer listen ports on 127.0.0.1, while FRR/BIRD use a single BGP port with peers identified by source IP address. The simulator already had `LocalAddr` support for per-peer source binding, so the orchestrator change was minimal.

## Decisions

- Added `--application ze|frr|bird` and `--binary <path>` over reusing `--ze` flag, because `--ze` was Ze-specific (plugin run directives) and overloading it for different daemons would confuse the semantics
- FRR/BIRD configs written to temp files (cleaned up on shutdown) over stdin pipe, because neither daemon accepts config on stdin
- BIRD multicast channels (`ipv4 multicast`, `ipv6 multicast`) excluded from the mapping because BIRD 2 has no separate multicast channel type and would reject the config
- Ze-specific flags (`--ssh`, `--web-ui`, `--lg`, `--ze-mcp`, `--ze-pprof`) emit warnings when used with non-ze targets over silently ignoring them
- `orchestratorConfig` changed from pass-by-value to pass-by-pointer because adding the `target` field pushed it past the 256-byte gocritic `hugeParam` threshold

## Consequences

- `ze-chaos --config-only --application frr` and `--application bird` produce valid configs from the same seed
- `ze-chaos --application frr --binary /usr/lib/frr/bgpd` forks bgpd with a temp config file
- FRR/BIRD targets require loopback aliases on macOS (`sudo ifconfig lo0 alias 127.0.0.x`); Linux works out of the box (127.0.0.0/8 is all routed to lo)
- `--pipe` and `--in-process` are blocked for non-ze targets (Ze-specific features)

## Gotchas

- FRR config generation needs the same ipv4/unicast fallback for empty Families as Ze and BIRD generators. The three generators have independent code paths; a change to the family defaulting logic in one must be mirrored in the others.
- The `--ze` flag error message in `forkZe` needed updating to reference `--binary` after the flag rename. Stale flag references in error messages survive compilation and tests.
- BIRD 2 channel types are not a 1:1 mapping from AFI/SAFI. Only `ipv4` and `ipv6` (unicast) are valid channel keywords. Complex families (VPN, EVPN, FlowSpec, multicast) need either special handling or exclusion.

## Files

- `internal/chaos/scenario/target.go` (new: Target type, ParseTarget, SinglePort, DefaultBinary)
- `internal/chaos/scenario/config_frr.go` (new: GenerateFRRConfig)
- `internal/chaos/scenario/config_bird.go` (new: GenerateBIRDConfig)
- `internal/chaos/orchestrator/fork.go` (modified: forkDaemon for temp-file launch, nil-safe Shutdown)
- `cmd/ze/ze_chaos_run.go` (modified: --application, --binary flags, target-aware config/fork)
- `internal/chaos/orchestrator/run.go` (modified: single-port dialing with per-peer source addr)
