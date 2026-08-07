---
kind: directive
level:
stage:
---
**Listener service (default: looking-glass, web).** The feature plugs into the construction registry (`cmd/ze/hub/service_registry.go`). A gated `service_<x>.go` builds a `Service` (the `Reconfigurable` listener-migration contract + `Name` + `Shutdown`); `register_<x>.go`'s `init()` calls `registerService("<x>", build<X>Service, wireMigrator)`. The hub iterates the registry in `buildServices(deps)` and routes the built service via `registerBuiltService`. Generic inputs cross the boundary as plain values in `ServiceDeps`; **no `internal/component/<x>` type may appear in `ServiceDeps` or any always-on signature**: widen always-on handles to `Reconfigurable` (as `SetWeb`/`SetLG` do). A second construction path (e.g. a `ze start --web` standalone mode) goes through a nil-able seam var set from the gated registration, never a direct always-on import.
