---
kind: note
level:
stage:
---
Without this line, an interface change (e.g. `Emit` gaining `any`) compiles
the stub against an outdated signature and only fails when the test
actually constructs the stub. The current 8 stub files
(`pkg/ze/ze_test.go`, `internal/plugins/{sysrib,ntp}/*_test.go`,
`internal/plugins/iface/dhcp/*_test.go`,
`internal/plugins/iface/netlink/monitor_linux_test.go`,
`internal/component/iface/{migrate_linux,integration_helpers_linux,config}_test.go`,
`internal/component/plugin/{server,manager}/*_test.go`,
`internal/component/bgp/plugins/rib/rib_bestchange_test.go`) all carry
this assertion. New stubs without it should fail review.
