| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-02 | - | plugins | A goroutine a plugin starts itself carries no recover, so a panic in it ends the plugin process instead of the loop. The SDK recovers command dispatch and bridge callbacks only (`pkg/plugin/sdk/sdk_dispatch.go`). 18 packages start one and guard none: rsvpte 7, ldp 5, vrrp/flowexport/fib 4 each. Only iface and ospf guard theirs. | firewall-irr guarded in `refreshAll` (`internal/component/firewall/plugins/irr/irr.go`); the other 17 are open |
