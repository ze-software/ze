# 709 -- Kernel Route Redistribution

## Context

Ze's redistribution framework supported `connected` and `static` sources but not kernel-installed routes from external tools (DHCP, PPP, manual `ip route`). VyOS uses `redistribute kernel` for this. The fib/kernel plugin already owned a netlink route subscription for re-assertion, but a second subscription for redistribution would double syscall and parsing cost on DFZ-scale tables (~950k routes at startup).

## Decisions

- Chose shared `routewatch` package in `internal/core/` over a second independent netlink subscription, because parsing 950k routes twice is measurable on DFZ boxes.
- Chose synchronous fanout over per-consumer channels, because both consumers are O(1) per event and async adds complexity without benefit for two non-blocking handlers.
- Chose package-level global Watcher (`routewatch.Global()`) over injection via `ConfigureRouteWatcher` callback, because `loader_create.go` doesn't exist and the plugin architecture uses self-registration via `init()`.
- Filtered RTPROT_KERNEL(2) consumer-side in kernel-redistribute over filtering in routewatch, because the redistribute-orchestrator is stateless (no per-prefix per-source tracking) and overlapping with the connected plugin causes incorrect withdrawals. Differs from FRR where the internal RIB deduplicates.
- Filtered RTPROT_REDIRECT(1) consumer-side (transient ICMP redirects cause BGP churn).
- Used `ListExisting: true` for the shared subscription over `false`, because kernel-redistribute needs the initial snapshot and fib/kernel's `handleExternalChange` no-ops on unmanaged prefixes.

## Consequences

- Any future netlink route consumer registers with `routewatch.Global()` instead of opening a new subscription.
- The `routewatch.Watcher` cannot be restarted after a netlink socket failure (shared fate by design). A daemon restart is needed.
- fib/kernel now receives initial-dump events (harmless, filtered by managed-prefix check), which is a behavior change from `ListExisting: false`.

## Gotchas

- `netip.Addr{}.String()` returns `"invalid IP"`, not `""`. The fib/kernel migration initially passed this to `handleExternalChange` for no-gateway routes, corrupting the observability JSON. Fixed with an `IsValid()` guard.
- The shutdown sequence matters: `unreg()` must run before `withdrawAll()` to prevent late events re-announcing routes after withdrawal.
- Two plugin-list tests (`TestAvailablePlugins`, `TestAllPluginsRegistered`) need manual updates when adding a new plugin.
- `all.go` is code-generated (`DO NOT EDIT` header) but was edited manually. `make generate` should be run to verify the codegen picks up the new plugin.

## Files

- `internal/core/routewatch/` (new): shared netlink route subscription with fanout
- `internal/plugins/kernel/` (new): kernel redistribute plugin
- `internal/plugins/fib/kernel/monitor_linux.go` (modified): migrated to routewatch consumer
- `internal/component/plugin/all/all.go` (modified): added kernel plugin imports
- `test/parse/redistribute-kernel.ci` (new): config parse test
