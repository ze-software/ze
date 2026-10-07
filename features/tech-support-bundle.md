# Tech-Support Bundle (`ze support`)

## Meta

| Field | Value |
|-------|-------|
| Name | Tech-Support Bundle (`ze support`) |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/support, internal/plugins/support |
| Real-path tests | test/ui/support-basic.ci, test/ui/support-reason.ci, test/ui/support-module-filter.ci, test/ui/support-exclude.ci |
| Docs | docs/guide/production-diagnostics.md |
| Doc review | 2026-10-07: moduleRegistry in internal/component/support/modules.go holds exactly the 20 named modules and no SMART module, as the row now says |
| Defect review | 2026-10-07: plan/spec-support-export.md is blocked new scope, not a defect; no journal row names internal/component/support |
| Extra criteria | supported: archive carries one JSON file per module with the documented schema = test/ui/support-archive-schema.ci |

## Description

Offline archive generator with 20 pure-Go modules (no shell-outs, gokrazy-safe): version, doctor, host, platform, config (sanitized), crashes, disk, interfaces (netlink), routes (netlink), neighbors (netlink), env, sysctl, runtime, dmesg, sockets, kernel modules, conntrack, file descriptors, DNS, firewall (nftables via netlink). Module selection (`--module`/`--exclude`), time scoping (`--since`), privacy-by-default (`--sensitive` to include secrets), reason metadata (`--reason`), JSON manifest (`--json`). Archive: `ze-support-<hostname>-<timestamp>.tar.gz` with one JSON file per module. No NOS vendor produces structured JSON-per-module output. None of the modules collects SMART disk health. <!-- source: internal/plugins/support/ -- Run, collect, moduleRegistry, sanitizeConfig --> <!-- source: internal/component/support/modules.go -- moduleRegistry -->
