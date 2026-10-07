# DHCP Server Named Ranges

## Meta

| Field | Value |
|-------|-------|
| Name | DHCP Server Named Ranges |
| Kind | protocol |
| Scope | partial |
| Scope gaps | several ranges per subnet are proven by unit tests only, no interop against a real DHCP client |
| Level | experimental |
| Components | internal/plugins/dhcpserver |
| Real-path tests | test/dhcp/dhcp-range-inside-subnet.ci, test/parse/dhcp-server-config.ci, test/parse/dhcp-server-multi-subnet.ci |
| RFCs | rfc2131 |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: checked list range in internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang and the per-range bitmap in pool.go |
| Defect review | 2026-10-07: audit found no open immediate spec and no journal row against internal/plugins/dhcpserver |

## Description

Multiple named address ranges per subnet for segmented allocation. Each range has an independent bitmap pool. Compile-out-able with the `ze_dhcpserver` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_dhcpserver -->
