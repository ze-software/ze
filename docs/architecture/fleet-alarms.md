# Fleet alarm conditions

The first-release alarm design covers the conditions a fleet hub can derive from device declarations, managed sessions, health reports and samples. The alarm list and history are proposed in Fleet UI-5. Fleet UI-8 adds same-device causes. Their commands are the first-release operator surface. [Alarm list design](../../plan/release/features/spec-fleet-ui-5-alarm-list.md) · [Alarm cause design](../../plan/release/features/spec-fleet-ui-8-alarm-causes.md)

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| For a declared device that has never connected, Thomas chose “After the configured offline grace period from declaration”. | Waiting for the first authenticated connection before starting the timer, or restarting the grace period at every hub restart or unrelated configuration reload | plan/release/features/spec-fleet-ui-5-alarm-list.md | 2026-11-25 |

## Device offline

Record the time the hub first accepts a device declaration. If no connection arrives before the configured delay expires, the hub opens Device offline at the end of that interval. Connecting before expiry cancels the pending opening. A device that has connected before uses its last disconnect as the start of the next interval. Removing a declaration ends its tracking. Re-adding the same name begins a new interval. A restart preserves the recorded start of a continuing declaration.

For a declaration already present when this facility is first enabled, begin timing at the first durable observation of that declaration. Its actual historical declaration time may be unknown. [Planned acceptance test](../../plan/release/features/spec-fleet-ui-5-alarm-list.md)
