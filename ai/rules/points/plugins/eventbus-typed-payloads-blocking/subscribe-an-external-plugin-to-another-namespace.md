---
kind: directive
level:
stage:
---
**External-plugin startup subscriptions carry a namespace and an optional
envelope.** `SetStartupSubscriptions(events, peers, format)` subscribes in the
protocol component's default namespace (`bgp`). To observe another namespace
(e.g. `vpn-ipsec`) from an out-of-tree plugin, call
`SetStartupSubscriptionsIn(namespace, events, peers, format)`; an unregistered
namespace is warned and skipped, never registered dead. To discriminate several
event types that share a payload shape (e.g. `sa-up` vs `sa-down`), call
`SetEnvelope(true)` and parse each delivery with `rpc.ParseEventEnvelope`
(`{namespace,event,payload}`); the default remains the bare payload, byte-for-byte
unchanged. All three are additive (new `omitempty` fields, new `Set*` methods):
no existing SDK signature or wire byte changed. In-process plugins do not need
this: they subscribe to any namespace directly on the `EventBus`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptionsIn, SetEnvelope -->
<!-- source: internal/component/plugin/server/dispatch.go -- registerSubscriptions, buildEventEnvelope -->
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput.Namespace/Envelope, EventEnvelope -->
