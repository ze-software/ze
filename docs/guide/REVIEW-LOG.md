# Documentation Review Log

Factual discrepancies found by cross-referencing docs against YANG schemas,
register.go files, CLI dispatch tables, and functional test configs.

Date: 2026-03-21

## ~~Critical: Config Syntax~~ FIXED

### ~~Peer key must be a name, not an IP address~~ FIXED

Config examples in guides fixed to use named peers with `remote { ip; as; }`.
README.md and features.md were already correct.

Files fixed: configuration.md, exabgp-migration.md, graceful-restart.md,
monitoring.md, cli.md, route-injection.md.

Files already correct: README.md, bgp-role.md, add-path.md, rpki.md,
route-reflection.md, features.md.

### ~~Config fields peer-address, local-address, local-as, peer-as~~ NOT AN ISSUE

features.md already uses `remote { ip; as; }` / `local { ip; as; }`.
README.md already uses correct syntax. Original report was stale.

## ~~Critical: Plugin Names~~ RESOLVED

Code was renamed to `bgp-nlri-*` prefix (commit 4d8a7b1c). Docs and code now match.
plugins.md NLRI table also uses correct names (`bgp-nlri-flowspec`, `bgp-nlri-labeled`).
No remaining issues.

### ~~Plugin count is wrong~~ NOT AN ISSUE

README.md already says 21 plugins with correct tables.

## ~~Critical: CLI Command Paths~~ NOT AN ISSUE

### ~~ze validate, not ze bgp validate~~ NOT AN ISSUE

configuration.md already uses `ze validate`. config-reload.md already uses `ze validate`.
Original report was stale.

### ~~ze config migrate, not ze bgp config migrate~~ NOT AN ISSUE

configuration.md already uses `ze config migrate`. Original report was stale.

## ~~Medium: Address Families~~ RESOLVED

### ~~VPN family name inconsistency~~

~~Plugin register.go: `ipv4/vpn`, `ipv6/vpn`~~
~~YANG schema + guides: `ipv4/mpls-vpn`, `ipv6/mpls-vpn`~~
~~features.md: "IPv4 VPN" (ambiguous)~~

Resolved: vpn plugin's `types.go` now registers `mpls-vpn` as the canonical
SAFI name to align with the YANG schema (`ze-types.yang:261`) and
configuration/architecture docs. All `.go`, `.ci`, and `.md` references
collapsed to `ipv4/mpls-vpn`/`ipv6/mpls-vpn` in the family-registry refactor.

## ~~Medium: Signal Descriptions~~ NOT AN ISSUE

### ~~ze signal commands use SSH, not Unix signals~~ NOT AN ISSUE

cli.md already describes commands by effect ("Graceful shutdown", "Reload configuration"),
not by signal name. Original report was stale.

### Missing ze signal restart

cli.md already documents `ze signal restart`. Other guides may benefit from mentioning it.

### ~~SIGALRM does not exist~~ NOT AN ISSUE

config-reload.md does not mention SIGALRM. Signal table lists only SIGHUP, SIGTERM/SIGINT,
SIGUSR1. Original report was stale.

## ~~Medium: Test Count~~ TO VERIFY

README test count should be verified with `grep -r "^func Test" --include="*_test.go" | wc -l`.

## ~~Low: RIB Command Naming~~ NOT AN ISSUE

features.md already uses `rib routes received` / `rib routes sent`. Original report was stale.

## ~~Low: Missing Plugin in Guide~~ NOT AN ISSUE

plugins.md protocol table already includes `bgp-llnh` (line 134).
