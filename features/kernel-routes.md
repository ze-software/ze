# Kernel Routes

## Meta

| Field | Value |
|-------|-------|
| Name | Kernel Routes |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/kernel, internal/core/routewatch, internal/core/rtproto |
| Docs | docs/architecture/core-design.md |
| Doc review | 2026-10-07: handleRouteEvent in internal/plugins/kernel/kernel.go drops rtproto.IsZe (249-252 in internal/core/rtproto/rtproto.go), rtprotKernel 2 and rtprotRedirect 1 and emits every other protocol; run calls withdrawAll after ctx.Done; routewatch.Global is shared with internal/plugins/fib/kernel |
| Defect review | 2026-10-07: no immediate spec or journal row names internal/plugins/kernel; test/parse/redistribute-kernel.ci proves config parsing only |
| Extra criteria | supported: a network-namespace test proving the protocol filter and the withdraw on shutdown = none yet |

## Description

Redistribute externally-installed kernel routes into BGP via `redistribute { import kernel }`. Consumes parsed route events from a shared netlink route watcher (`internal/core/routewatch/`). Routes Ze installs itself are filtered out (the protocols `rtproto.IsZe` names: GTSM 249, fib-kernel 250, static 251, policy-route 252), as are RTPROT_KERNEL (2) and RTPROT_REDIRECT (1). A route of every other protocol is emitted as a RouteChangeBatch, for example DHCP (16), PPP/manual (BOOT=3) and admin static (STATIC=4) routes. Tracks announced prefixes; withdraws all on shutdown. IPv4 and IPv6. Shares a single netlink subscription with fib-kernel (route re-assertion). <!-- source: internal/plugins/kernel/kernel.go -- routeObserver.handleRouteEvent, withdrawAll --> <!-- source: internal/core/rtproto/rtproto.go -- IsZe --> <!-- source: internal/plugins/kernel/events/events.go -- redistevents producer --> <!-- source: internal/core/routewatch/routewatch.go -- shared netlink subscription -->
