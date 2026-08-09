# Component Boundaries: the four interfaces

`plugin.Server` was one object spanning six concerns: plugin lifecycle, event
subscription, event dispatch, RPC routing, BGP hooks and startup coordination.
Three unrelated things were called "Hub". The bus was not content-agnostic: it
carried `BGPHooks` callbacks and typed event constants. This page holds the
model that replaced it.

`docs/architecture/core-design.md` Section 19 carries the resulting import
hierarchy, one row per component. This page carries the decisions behind it.

## The four components

| Interface | Implementation |
|-----------|----------------|
| `ze.Engine` | `internal/component/engine/engine.go` (supervisor, startup and shutdown ordering) |
| `ze.ConfigProvider` | `internal/component/config/provider.go` |
| `ze.PluginManager` | `internal/component/plugin/manager/manager.go` |
| `ze.Subsystem` | implemented by each subsystem |

<!-- source: pkg/ze/engine.go -- Engine -->
<!-- source: pkg/ze/config.go -- ConfigProvider -->
<!-- source: pkg/ze/plugin.go -- PluginManager -->
<!-- source: pkg/ze/subsystem.go -- Subsystem -->

**The interfaces are public, in `pkg/ze/`, not in `internal/`.** External
plugins depend on them.

**A subsystem is not a plugin, and neither is a subtype of the other.** The BGP
daemon is a subsystem: it owns TCP and the FSM. `bgp-rib`, `bgp-rs` and `bgp-gr`
are plugins.

## The Bus was deleted, not kept beside the stream system

The model started at five components. The fifth, a content-agnostic Bus with
byte payloads and hierarchical `/` topics, duplicated the stream event system in
`plugin/server/dispatch.go`. Both gave in-process pub/sub with fan-out. The
stream system already had schema-validated event types, DirectBridge zero-copy
for internal plugins, and TLS delivery to external plugins. The Bus had none of
those and was never fully wired: about 14 production call sites used it, and
most were unfinished.

The Bus was absorbed into the stream system behind a public `ze.EventBus`
interface backed by `Server.Emit` and `Server.Subscribe`. The API shapes mapped
one to one, so the migration was mechanical: `Publish` to `Emit`, `Subscribe` to
`Subscribe`, and a topic string to a `(namespace, event-type)` pair.
<!-- source: internal/component/plugin/server/engine_event.go -- Emit, Subscribe -->
<!-- source: internal/core/events/events.go -- namespace and event type registry -->

## What this buys

- A new subsystem is added by implementing `ze.Subsystem` and registering with
  the Engine. No plumbing.
- Plugins talk through `ze.EventBus`. No plugin package imports another.
- `internal/component/plugin/` imports nothing from `internal/component/bgp/`
  or from any subsystem.
- `ConfigProvider` is the single authority for config. The editor, the web UI,
  subsystems and plugins all use it.
- DirectBridge is invisible through the interface. An in-process plugin
  dispatches through the bridge hot path and an external one takes the stream
  RPC path.
- One pub/sub backbone instead of two, and one import for an external plugin
  author.

## What cost the most

Eliminating `BGPHooks` touched the most code: every event-publishing path moved
from callback injection to a direct publish. The Engine phase was the highest
risk, because the new startup sequence changed component initialization order.
Wiring the extracted components into the production startup path was
substantially more work than extracting the implementations.
