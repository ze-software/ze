# FlowSpec-to-Firewall Bridge

## Meta

| Field | Value |
|-------|-------|
| Name | FlowSpec-to-Firewall Bridge |
| Kind | daemon |
| Scope | partial |
| Scope gaps | RFC 8955 and RFC 8956 predicates the ledger marks Partial |
| Level | experimental |
| Components | internal/plugins/flowspec-firewall |
| Real-path tests | test/plugin/flowspec-fw-add.ci, test/plugin/flowspec-fw-withdraw.ci, test/plugin/flowspec-fw-withdraw-removes-table.ci, test/plugin/flowspec-fw-protocol-sctp.ci, test/plugin/flowspec-fw-untranslatable-keeps-others.ci, test/plugin/flowspec-fw-legacy-table-removed.ci |
| Interop | bgp/bgp-flowspec-sctp-gobgp |
| RFCs | rfc8955, rfc8956 |
| Docs | docs/guide/flowspec-protected-router.md |
| Doc review | 2026-10-07: the ze_flowspec table name matches tableName in internal/plugins/flowspec-firewall/state.go |
| Defect review | 2026-10-07: journal table-ownership row judged stale by the audit; no plan/immediate spec names internal/plugins/flowspec-firewall |
| Extra criteria | supported: a translated rule drops matching packets in nftables = test/firewall/flowspec-fw-packet-drop.ci |

## Description

`flowspec-firewall` plugin converts BGP FlowSpec rules into nftables entries in a dedicated `ze_flowspec` table.
