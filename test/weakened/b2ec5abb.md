# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestMTUCmdSchemaOwnsShowMTU | Moved, not lost: the ze-mtu-cmd module moved from internal/plugins/mtu-cmd/yang to internal/component/mtu/yang (spec-plugin-ownership-imports route B), and its schema test moved with it to internal/component/mtu/yang/cmd_schema_test.go in the previous commit. |
| TestPingCmdSchemaOwnsPing | Moved, not lost: same move, now internal/component/ping/yang/cmd_schema_test.go. |
| TestTracerouteCmdSchemaOwnsTraceroute | Moved, not lost: same move, now internal/component/traceroute/yang/cmd_schema_test.go. |
