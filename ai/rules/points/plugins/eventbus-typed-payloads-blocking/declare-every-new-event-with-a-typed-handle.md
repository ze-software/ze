---
kind: directive
level: MUST
stage:
---
- **A new event MUST be declared with `events.Register[T](namespace, eventType)`, or with `events.RegisterSignal(namespace, eventType)` when it carries no payload.** `pkg/ze/eventbus.go` carries `payload any`, so the registry is the single source of truth for the payload type. Producers call `Handle.Emit(bus, payload)` and consumers call `Handle.Subscribe(bus, func(T))`.
