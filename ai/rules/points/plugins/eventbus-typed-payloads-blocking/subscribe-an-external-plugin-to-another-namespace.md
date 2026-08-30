---
kind: directive
level: MUST
stage:
---
- **`SetStartupSubscriptions(events, peers, format)` subscribes in the protocol component's default namespace (`bgp`). An out-of-tree plugin that observes another namespace MUST call `SetStartupSubscriptionsIn(namespace, events, peers, format)`, and one that discriminates several event types sharing a payload shape MUST call `SetEnvelope(true)` and parse each delivery with `rpc.ParseEventEnvelope`.** An unregistered namespace is warned and skipped, never registered dead. An in-process plugin needs neither: it subscribes to any namespace directly on the `EventBus`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptionsIn, SetEnvelope -->
