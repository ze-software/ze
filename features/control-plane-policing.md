# Control-Plane Policing (CoPP)

## Meta

| Field | Value |
|-------|-------|
| Name | Control-Plane Policing (CoPP) |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/copp |
| Real-path tests | test/firewall/copp-bgp.ci, test/firewall/copp-over-limit-drop.ci, test/firewall/copp-trusted.ci, test/firewall/copp-withdraw.ci |
| Docs | docs/guide/flowspec-protected-router.md |
| Doc review | 2026-10-07: checked protected-port 179 and over-limit default accept in internal/plugins/copp/yang, and the doctor check in internal/plugins/copp/doctor.go |
| Defect review | 2026-10-07: audit found no open immediate spec against internal/plugins/copp; journal rows naming a Component, not each re-verified here: component-rebuilt-during-reload.md:8, declared-format-contradicts-payload.md:16, gate-excludes-part-of-its-population.md:71, zero-value-as-valid-answer.md:32 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Rate-limit new TCP connections to the BGP listen port (TCP/179) to protect against connection-flood DDoS. Generates an nft input-hook chain via the firewall registry: established/related sessions pass at full rate, operator-supplied trusted-source prefixes bypass the limit, new connections are rate-limited. Configurable rate, burst, protected-port (default 179), trusted-source prefix list, and over-limit policy (accept/drop, default accept). The over-limit policy rides the rate-limit term as an nftables inverted limiter. The input chain keeps an accept policy under both settings, so no CoPP config locks the operator out. Doctor check verifies the CoPP input chain is active when configured. <!-- source: internal/plugins/copp/register.go -- copp plugin registration --> <!-- source: internal/plugins/copp/translate.go -- coppPolicy to firewall.Table translation --> <!-- source: internal/plugins/copp/config.go -- YANG config parsing --> Compile-out-able with the `ze_copp` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_copp -->
