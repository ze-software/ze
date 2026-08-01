# Learned: spec-policy-routing -- Policy-based routing (Surfprotect)

## What was built

Policy routing plugin (`internal/plugins/policyroute/`) that steers subscriber traffic via nftables packet marking and ip rule table selection. Replaces the VyOS Surfprotect policy routing config on the Exa LNS.

1. **Config parsing**: `policy { route <name> { interface; rule { from; then; } } }` with full from-block match criteria (source/dest address, ports, protocol, tcp-flags, @set references) and then-block actions (accept, drop, table N, next-hop IP, tcp-mss N).
2. **Translation layer**: PolicyRoute structs translated to firewall.Table (type route, hook prerouting, priority -150). Interface wildcard prepended to every rule. All policies merged into one `ze_pr` table.
3. **Mark + table allocation**: fwmark range 0x50000-0x5FFFF, auto table range 2000-2999. Sequential allocation with dedup (same next-hop reuses table).
4. **Linux netlink**: ip rule add/del for fwmark-to-table mapping, route add/del for auto-managed default routes in next-hop tables.
5. **SDK lifecycle**: Full 5-stage (OnConfigure, OnConfigVerify, OnConfigApply, OnConfigRollback) with journal-based rollback. Depends on firewall plugin for nftables backend.
6. **CLI**: `ze policy show` returns JSON with policy names, interfaces, rules, actions.

Added: MatchTCPFlags format test, 6 functional .ci tests, `ze-test policy` runner, docs (guide, features, comparison, command catalogue, plugins, configuration).

## Key decisions

- **Plugin, not component.** Policy routing registers as a plugin (`internal/plugins/`) like static routes, not a component. It depends on the firewall component for nftables backend access.
- **Single ze_pr table for all policies.** One nftables table with one chain, terms named `<policy>-<rule>`. Keeps rule ordering simple and avoids the complexity of multiple tables competing for hook priority.
- **Inline CLI in register.go.** The `formatPolicies` function lives in register.go instead of a separate `cmd/show.go`. A single command does not warrant a separate file.
- **Table range validation at config parse time.** User-specified table IDs in 1000-2999 (ze-reserved) and 253-255 (kernel system) are rejected during parsing, not at apply time.
- **Conflicting terminal actions rejected.** Only one of accept/drop/table/next-hop allowed per rule. Detected at parse time with a clear error listing the conflicting actions.

## What surprised us

- **All core code was already implemented.** The spec was in `blocked` status but six files of production code, a YANG schema, and 19 unit tests were already complete. The missing pieces were exclusively functional tests, documentation, and the test runner.
- **fw-8 and static-routes dependencies were already closed.** Both blocking specs had learned summaries in `plan/learned/`, meaning the blocker had cleared without updating this spec's status.

## Patterns reinforced

- **Audit before implementing.** This spec's entire implementation was done before the audit session began. The audit step prevented reimplementing existing code.
- **Firewall table registry pattern.** `firewall.RegisterTables("policy-routes", tables)` + `firewall.ApplyAll()` is the right way for a non-firewall plugin to contribute nftables tables. The registry merges all owners' tables so no Apply call deletes another owner's tables.
- **Reserved ranges prevent collisions.** VRF tables (1000-1999) and policy routing auto-tables (2000-2999) have non-overlapping ranges by construction, and user-specified tables are validated to exclude both.

## Files

None recorded.
