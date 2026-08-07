---
kind: note
level: MUST
stage:
---
`pkg/ze/eventbus.go` carries `payload any`. New events MUST be declared via
`events.Register[T](namespace, eventType)` (typed) or
`events.RegisterSignal(namespace, eventType)` (no payload). Producers call
`Handle.Emit(bus, payload)` and consumers call
`Handle.Subscribe(bus, func(T))`; the registry is the single source of
truth for the payload type.
