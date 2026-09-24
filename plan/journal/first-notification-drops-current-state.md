| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-22 | - | `monitor.handleLinkUpdate` (`internal/plugins/iface/netlink/monitor_linux.go`) | The first update for an existing interface emitted `created` and returned before reporting its down state. Native RSVP therefore missed the link failure. | Emit the current carrier state after `created`. Actual monitor startup fails before the fix and passes afterward; deletion and later transitions also pass. |
