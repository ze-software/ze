# Bug Review Inventory Report

Generated: 2026-06-19
Parent spec: `plan/spec-bug-review-0-umbrella.md`
Child spec: `plan/spec-bug-review-1-inventory-and-self-containment.md`

## Summary

| Check | Result | Evidence |
|-------|--------|----------|
| Generated import rows accounted | PASS | `internal/component/plugin/all/all.go#B835` lines 12-137, 140-229, 232-233, 236-275 |
| Directory candidates classified | PASS | `find` over `internal/plugins/*`, `internal/component/bgp/plugins/*`, `internal/component/bgp/plugins/nlri/*`, `internal/component/bgp/plugins/cmd/*` |
| Registry classes represented | PASS | `registry.Register`, `pluginserver.RegisterRPCs`, `codegen:skip`, YANG register files, command registry imports |
| In-scope rows without child assignment | PASS, count 0 | Child assignment rules below |
| Exclusions have reasons | PASS | Exclusion table below |

## Audit Tests

| Test | Status | Evidence |
|------|--------|----------|
| BugReviewInventoryAllImportsAccounted | PASS | All import rows are covered by exactly one group in the generated import ledger below |
| BugReviewInventoryRegistriesAccounted | PASS | Plugin registry, RPC registry, YANG registry, command registry, event namespace, and codegen-skip roots are represented |
| BugReviewInventoryNoUnassignedRows | PASS | Group assignment table has no blank child for in-scope rows |
| BugReviewInventoryExclusionsHaveReason | PASS | Every excluded candidate class names the reason |
| BugReviewInventoryReadableByChildren | PASS | Child 2, 3, and 4 scope rules are explicit and stable |

## Generated Import Ledger

Source: `internal/component/plugin/all/all.go#B835`.

| Row group | Source lines | Count | Surface class | Assignment |
|-----------|--------------|-------|---------------|------------|
| SCHEMA | 12-137 | 126 | YANG module registration from component, BGP plugin, central command, core IPC, and `internal/plugins/*/yang` packages | Child 2 for non-BGP system/component/central surfaces, Child 3 for BGP core schema, Child 4 for BGP plugin schemas |
| PLUG | 140-229 | 90 | Runtime plugin packages and component-owned plugin packages | Child 2 for plugin engine/system/component-owned non-BGP plugins, Child 4 for BGP plugin packages, Child 3 for BGP protocol core package rows |
| EVT | 232-233 | 2 | Event namespace packages | Child 2 unless event is BGP-core-specific, then Child 3 |
| RPC | 236-275 | 40 | Online RPC command packages registered through `pluginserver.RegisterRPCs` | Child 2 for non-BGP/system/generic commands, Child 3 for BGP core command seams, Child 4 for BGP plugin command packages |
| TOTAL | 12-275 | 258 | Generated compiled import rows | Unassigned count 0 |

### Assignment Rules

| Rule | Paths |
|------|-------|
| Child 2, plugin engine/system | `internal/component/plugin/*`, `pkg/plugin/*`, `internal/plugins/*`, non-BGP component-owned plugin surfaces, central generic command schemas and RPC handlers |
| Child 3, BGP engine core | `internal/component/bgp/plugin`, `internal/component/bgp/yang`, `internal/component/bgp/reactor/filter`, BGP core API/forwarding/session/wire/attribute/capability/context/filterapi files |
| Child 4, BGP plugins/codecs | `internal/component/bgp/plugins/*`, nested `nlri/*`, `cmd/*`, BGP plugin schemas, BGP plugin RPC command packages, BGP plugin attribute/filter/family/capability surfaces |
| Child 5, final backlog | Child reports and accepted finding fix specs only |

### Notable Generated Rows

| ID | Source | Classification | Assignment | Notes |
|----|--------|----------------|------------|-------|
| SCHEMA-BGP-CMD | all.go lines 14-44 | BGP command and BGP plugin schemas | Child 4 except `internal/component/bgp/cli/yang` and BGP core schema rows, which are Child 3 | Command YANG is user-visible review scope |
| SCHEMA-CENTRAL-VERB | all.go lines 47-56 | Generic root or central command schema | Child 2 | Self-containment review checks whether only generic roots remain central |
| SCHEMA-SYSTEM | all.go lines 89-137 | System plugin and command schemas | Child 2 | Includes schema-only command packages such as `*-cmd/yang` |
| PLUG-BGP | all.go lines 141-179 | BGP plugin runtime packages | Child 4 | Includes `capa`, direct plugins, and nested NLRI families |
| PLUG-COMPONENT | all.go lines 180-192 | Component-owned plugin/runtime packages | Child 2 except `bgp/reactor/filter`, which is Child 3 | LDP, RSVP-TE, iface, flowexport, traffic, VPP, firewall IRR |
| PLUG-SYSTEM | all.go lines 193-229 | System plugin runtime packages | Child 2 | Includes backends under `fib`, `firewall`, `iface`, and `traffic` |
| EVT-NAMESPACE | all.go lines 232-233 | Event namespace registrations | Child 2 | `config/transaction` and `isis`; if later protocol review finds IS-IS-specific defects, route to owner spec |
| RPC-BGP | all.go lines 238-249 | BGP plugin RPC packages | Child 4 | BGP command handlers owned by BGP plugins |
| RPC-GENERIC | all.go lines 236-237, 250-275 | Non-BGP and generic RPC packages | Child 2, with BGP core seam review in Child 3 where handler calls reactor APIs | Handler owner decides final route |

## Directory Reconciliation

| Directory set | Current candidates | Classification |
|---------------|--------------------|----------------|
| `internal/plugins/*` | 64 direct directories from workspace tree and `find` output | System plugins, command-only roots, backend implementations, support/offline command providers |
| `internal/component/bgp/plugins/*` | 30 direct directories from `find`, including `capa`, `cmd`, and `nlri` | BGP plugin tree. `cmd` and `nlri` are containers whose child packages carry the review rows |
| `internal/component/bgp/plugins/nlri/*` | 10 family directories: evpn, flowspec, labeled, ls, mup, mvpn, rtc, srpolicy, vpls, vpn | Child 4 family-chain review |
| `internal/component/bgp/plugins/cmd/*` | 8 command directories: cache, commit, monitor, peer, policy, raw, rib, update | Child 4 command/schema/handler review |
| Component-owned plugin rows from all.go | BGP reactor filter, firewall IRR, flowexport backends, iface, iface CLI, ldp, rsvpte, traffic, traffic CLI, vpp | Child 2 unless BGP-core-specific row is routed to Child 3 |

### Directory-Only Command Providers

These packages intentionally do not appear in `plugin/all/all.go` because they are command roots wired by `cmd/ze`, not runtime plugin-manager packages. They remain in review scope for command wiring and self-containment in Child 2.

Evidence: `codegen:skip` search reported these register files; `cmd/ze/ze_core_dispatch.go#B708` lines 55-66 imports most of them, `cmd/ze/setup_features_distro.go#A006` lines 10-12 imports distro setup roots, and `cmd/ze/setup_features_setup.go#6076` line 11 imports provision.

| ID | Package | Wiring evidence | Assignment |
|----|---------|-----------------|------------|
| DIR-SKIP-COMPLETION | `internal/plugins/completion` | `cmd/ze/ze_core_dispatch.go#B708` line 55 | Child 2 |
| DIR-SKIP-CONNECT | `internal/plugins/connect` | `cmd/ze/setup_features_distro.go#A006` line 10 | Child 2 |
| DIR-SKIP-CRASHES | `internal/plugins/crashes` | `cmd/ze/ze_core_dispatch.go#B708` line 56 | Child 2 |
| DIR-SKIP-DEBUG | `internal/plugins/debug` | `cmd/ze/ze_core_dispatch.go#B708` line 57 | Child 2 |
| DIR-SKIP-DIAG | `internal/plugins/diag` | `cmd/ze/ze_core_dispatch.go#B708` line 58 | Child 2 |
| DIR-SKIP-EXABGP | `internal/plugins/exabgp` | `cmd/ze/ze_core_dispatch.go#B708` line 59 | Child 2, active overlap with `spec-exabgp-compat-sync.md` |
| DIR-SKIP-EXPLAIN | `internal/plugins/explain` | `cmd/ze/ze_core_dispatch.go#B708` line 60 | Child 2 |
| DIR-SKIP-HOST | `internal/plugins/host` | `cmd/ze/ze_core_dispatch.go#B708` line 61 | Child 2 |
| DIR-SKIP-INIT | `internal/plugins/init` | `cmd/ze/ze_core_dispatch.go#B708` line 62 | Child 2 |
| DIR-SKIP-LOCAL | `internal/plugins/local` | `cmd/ze/setup_features_distro.go#A006` line 11 | Child 2 |
| DIR-SKIP-PASSWD | `internal/plugins/passwd` | `cmd/ze/ze_core_dispatch.go#B708` line 63 | Child 2 |
| DIR-SKIP-PROVISION | `internal/plugins/provision` | `cmd/ze/setup_features_setup.go#6076` line 11 | Child 2 |
| DIR-SKIP-SIGNAL | `internal/plugins/signal` | `cmd/ze/ze_core_dispatch.go#B708` line 64 | Child 2 |
| DIR-SKIP-SKILLS | `internal/plugins/skills` | `cmd/ze/ze_core_dispatch.go#B708` line 65 | Child 2 |
| DIR-SKIP-SUPPORT | `internal/plugins/support` | `cmd/ze/ze_core_dispatch.go#B708` line 66 | Child 2 |
| DIR-SKIP-SYSTEMD | `internal/plugins/systemd` | `cmd/ze/setup_features_distro.go#A006` line 12 | Child 2 |
| DIR-SKIP-DOCTOR | `internal/component/doctor` | `cmd/ze/ze_core_dispatch.go#B708` line 54 | Child 2 |

## Registry Reconciliation

| Registry class | Evidence | Assignment |
|----------------|----------|------------|
| Plugin registry | `registry.Register` search found BGP plugin registrations, component protocol registrations, and system plugin registrations | Child 2 or Child 4 by owner path; BGP core seam rows go to Child 3 |
| RPC registry | `pluginserver.RegisterRPCs` search found BGP command handlers, generic handlers, and component-owned command handlers | Child 2 or Child 4 by owner path; BGP core API seam review in Child 3 when handler calls reactor/core APIs |
| YANG registry | generated `*/yang/register.go` files and all.go SCHEMA rows | Same owner as schema package |
| Command registry | `codegen:skip` roots register with `internal/component/command/registry` and are imported by `cmd/ze` | Child 2 |
| Event namespace | all.go EVT rows for `config/transaction` and `isis` | Child 2 for registry mechanics; protocol-owner specs for protocol behavior |
| Env/doctor/metrics | Identified by owner package during child reviews | Child 2 for system plugins, Child 4 for BGP plugins, Child 3 for BGP core metrics/seams |

## Active Spec Overlap

Source: `tmp/session/selected-spec#E66B`.

| Active spec | Inventory impact |
|-------------|------------------|
| `spec-exabgp-compat-sync.md` | Findings against `internal/plugins/exabgp` should reference this active spec before creating a new fix spec |
| `spec-route-config-plugin-migration.md` | Findings against route config, static/routingtable/policy-route, or related plugin migration code should be routed carefully |

## Exclusions

| Excluded class | Reason |
|----------------|--------|
| `vendor/**` | Third-party code, not authored plugin/core scope |
| `tmp/**` and untracked scratch logs | Session-local or generated scratch data, not compiled plugin/core source |
| generated `internal/component/plugin/all/all.go` | Evidence source only; canonical source is generator and package registry files |
| generated `*/yang/embed.go` and `*/yang/register.go` | Glue generated from canonical YANG modules. Review the owning YANG and owner package instead unless glue itself is generated incorrectly |
| pure test helper packages with no production import | Out of runtime review scope unless a finding concerns tests or missing regression coverage |
| root command packages imported only under setup/distro build tags | In scope for Child 2 command wiring when imported by a `cmd/ze/*` build-tag composition file, otherwise excluded from runtime plugin-manager scope |

## Child Scope Handoff

| Child | Must cover |
|-------|------------|
| Child 2 | Plugin engine, SDK/RPC bridge, `internal/plugins`, directory-only command roots, non-BGP component-owned plugin packages, central generic command/RPC/schema surfaces, config transaction event namespace mechanics |
| Child 3 | BGP core engine packages: wire/session/reactor/message/attribute/capability/context/filterapi, BGP core schema and CLI/API seams, forwarding/cache/reload paths |
| Child 4 | All BGP plugin packages and BGP command/NLRI/schema/RPC rows under `internal/component/bgp/plugins`, including `capa` and nested families |
| Child 5 | Load this report and all child reports; confirm no missing inventory rows; create fix specs for accepted findings |

## Inventory Findings and Observations

| ID | Severity | Classification | Evidence | Disposition |
|----|----------|----------------|----------|-------------|
| INV-OBS-1 | NOTE | Generated AGENTS architecture list omits BGP plugin directory `capa` while `all.go` imports `internal/component/bgp/plugins/capa` | `internal/component/plugin/all/all.go#B835` line 144 and `find` output under `internal/component/bgp/plugins/capa` | Treat `capa` as in-scope for Child 4. This is documentation/inventory drift, not a production bug |
| INV-OBS-2 | NOTE | Several `internal/plugins/*` packages are command roots deliberately skipped by plugin importer | `codegen:skip` search and `cmd/ze` blank imports | Treat as Child 2 command-wiring scope, not missing runtime plugin imports |

## Assumptions Resolved

| ID | Final status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `all.go` import groups plus directory and registry searches identify compiled plugin/schema/RPC surfaces; directory-only command roots are separately wired by `cmd/ze` |
| A-2 | confirmed | SCHEMA rows include schema-only command directories and are assigned to child scope |
| A-3 | confirmed | Component-owned protocol/plugin rows from all.go are assigned to Child 2 or Child 3 by owner; BGP plugin rows assigned to Child 4 |
| A-4 | confirmed | Generated files are evidence; canonical owners and exclusions are listed above |
