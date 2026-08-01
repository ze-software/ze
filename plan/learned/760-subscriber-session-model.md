# 760 -- Unified Subscriber Session Model

## Context

Ze had three independent session models (PPPoE, L2TP, PPP) with no shared subscriber type. PPPoE did not emit EventBus events. Auth, pool, shaping, accounting, and CoA were wired only to L2TP. PPPoE sessions were invisible to downstream plugins and show commands. The goal was a shared Session struct, subscriber event namespace, and transport-generic handler registries that both access types use.

## Decisions

- Chose subscriber event bridge pattern (subscribe to L2TP events, re-emit as subscriber events) over modifying the L2TP reactor directly, because the reactor is heavily tested and the bridge is a clean separation of concerns.
- Chose `*Session` in the internal registry map over `Session` values, because Session is 408 bytes and range-copies were flagged by gocritic. Public API still returns value copies.
- Chose L2TP handler delegation (l2tp.RegisterAuthHandler wraps and delegates to subscriber.RegisterAuthHandler) over having plugins import subscriber directly, to preserve backward compatibility for existing L2TP plugins without code changes.
- Kept prefix handlers L2TP-scoped over generalizing them, because PrefixRequest/PrefixResult carry TunnelID/SessionID tuples specific to L2TP. Generalization deferred until PPPoE needs DHCPv6-PD.
- Chose DefaultRegistry as a package-level singleton over per-subsystem registries, because both L2TP and PPPoE need to write to the same registry and show commands need a single query point.

## Consequences

- Any new access type (IPoE) follows the same pattern: populate subscriber.DefaultRegistry, emit subscriber events, register auth/pool handlers via subscriber.Register*.
- L2TP plugins (l2tpauthradius, l2tppool, l2tpshaper) continue to work unchanged; they register via l2tp.Register* which delegates internally.
- CoA can now match sessions by Acct-Session-Id across access types. PPPoE Disconnect-Message is logged but not wired to PPPoE teardown (needs pppoe.Subsystem.TeardownSession).
- `show subscriber summary` is the single pane of glass for all session types. Access-type-specific show commands (show l2tp, show pppoe) remain for transport-level detail.

## Gotchas

- PPPoE eventConsumer previously only handled EventSessionDown. Expanding it to handle SessionUp and SessionIPAssigned required careful ordering: subscriber registry add must happen before the event emit so subscribers see the session.
- L2TP subsystem tests pass nil EventBus to Start(). The subscriber bridge panics on nil bus. Guard with `if bus != nil` before creating the bridge.
- `AuthRespondFunc` and `AuthResult` are structurally identical between l2tp and subscriber packages but are different named types. The delegation layer must explicitly convert fields.

## Files

### Created
- `internal/component/l2tp/subscriber/session.go`
- `internal/component/l2tp/subscriber/registry.go`
- `internal/component/l2tp/subscriber/handler_registry.go`
- `internal/component/l2tp/subscriber/service.go`
- `internal/component/l2tp/subscriber/metrics.go`
- `internal/component/l2tp/subscriber/register.go`
- `internal/component/l2tp/subscriber/events/events.go`
- `internal/component/l2tp/subscriber/registry_test.go`
- `internal/component/l2tp/subscriber/handler_registry_test.go`
- `internal/component/l2tp/subscriber/events/events_test.go`
- `internal/component/l2tp/subscriber_bridge.go`
- `internal/component/l2tp/pppoe/drain.go`
- `internal/component/l2tp/subscriber/cmd/subscriber.go`
- `internal/component/cmd/subscriber/yang/ze-subscriber-cmd.yang`
- `internal/component/cmd/subscriber/yang/embed.go`
- `internal/component/cmd/subscriber/yang/register.go`

### Modified
- `internal/component/l2tp/handler_registry.go` (delegate auth/pool to subscriber)
- `internal/component/l2tp/subsystem.go` (wire subscriber bridge, bind metrics, publish service)
- `internal/component/l2tp/pppoe/subsystem.go` (expand event consumer, wire auth/pool drains, emit subscriber events)
- `internal/component/l2tp/plugins/auth_radius/coa.go` (add subscriber registry lookup for CoA)
- `internal/component/plugin/all/all.go` (regenerated)
