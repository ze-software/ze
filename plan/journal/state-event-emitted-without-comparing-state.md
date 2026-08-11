# State event emitted without comparing state

An event that names a state transition is emitted from a message that means
"something about this object changed". Every consumer then redoes the work of a
transition that did not happen.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-11 | fixit-route-removal-protocol-blind | iface monitor | `(*monitor).handleLinkUpdate` (`internal/plugins/iface/netlink/monitor_linux.go`) emits `interface/up` or `interface/down` on EVERY RTM_NEWLINK for a link it already knows, with no comparison against the state it last reported. An MTU change on an up link emits `up`. The link handlers in `internal/component/iface/register.go` then moved a route that had already moved, once per event, under `dhcpMu` | not fixed at the emitter. Each consumer was made idempotent instead: the route handlers carry `routeMetricState`. Deduplicating at the emitter also drops the refresh `(*resolver).onLinkEvent` (`internal/component/iface/resolve.go`) takes from these events for a rename or a MAC change |
