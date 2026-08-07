---
kind: directive
level:
stage:
---
**Engine (in-process) subscribers deliver synchronously within `Emit`;
plugin-process subscribers deliver asynchronously** (`Emit` returns the
plugin-process delivery count -- see `internal/core/events/typed.go`). A
request/re-emit correlation that assumes all subscribers answered by the time
`Emit` returns is therefore only safe when every subscriber is in-process. The
redistribute late-join replay (`redistevents.ReplayRequest` + echoed `ReplayID`
token) correlates a returning `RouteChangeBatch` to the requesting peer via an
opaque token the producer echoes, and holds the `ReplayID -> peer` map for a TTL
rather than dropping it right after `Emit`, precisely because an out-of-process
producer's re-emit arrives after `Emit` returns.
<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- ReplayID token + TTL map -->
<!-- source: internal/core/redistevents/events.go -- ReplayRequest event -->
<!-- source: internal/core/events/typed.go -- Emit returns plugin-process delivery count -->
