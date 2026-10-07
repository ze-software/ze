# Policy Routing

## Meta

| Field | Value |
|-------|-------|
| Name | Policy Routing |
| Page | docs/guide/policy-routing.md |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/plugins/policyroute |
| Real-path tests | test/policy/policy-boot-apply.ci, test/policy/policy-interface-list.ci, test/policy/policy-interface-list-counters.ci, test/policy/policy-next-hop.ci, test/policy/policy-reload.ci, test/policy/policy-set-table.ci, test/policy/policy-tcp-flags.ci, test/policy/policy-tcp-mss.ci |
| Docs | docs/guide/policy-routing.md |
| Doc review | 2026-10-07: Description re-read against code after the guest run: actions (buildActions, translate.go), wildcard interfaces (config.go), one term per interface (ruleTerms), verify/apply reload (register.go) and the auto table range 2000-2999 (autoTableBase/autoTableMax, marks.go) all hold |
| Defect review | 2026-10-07: journal one-members-bad-input-fails-every-member (2026-09-09) taken as open; no plan/immediate spec names internal/plugins/policyroute |

## Description

Policy-based routing via nftables packet marking and kernel ip rules. Steers traffic to alternate routing tables or next-hops based on L3/L4 match criteria (address, port, protocol, TCP flags, set references). Actions: accept (bypass), drop, table N (fwmark + ip rule), next-hop (auto-managed table from 2000-2999), tcp-mss clamping. Interface binding by name or wildcard prefix (e.g., `l2tp*`). A policy that names several interfaces matches traffic on any of them. Ze installs one nftables rule per named interface. Config reload reconciles nftables tables, ip rules, and auto-managed routes. <!-- source: internal/plugins/policyroute/register.go -- plugin registration --> <!-- source: internal/plugins/policyroute/translate.go -- config to nftables translation --> <!-- source: internal/plugins/policyroute/rules_linux.go -- ip rule and auto-route management --> Compile-out-able with the `ze_policyroute` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_policyroute -->
