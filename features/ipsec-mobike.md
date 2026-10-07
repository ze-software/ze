# IPsec MOBIKE

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec MOBIKE |
| Page | docs/guide/ipsec.md#mobike-endpoint-movement |
| Kind | protocol |
| Scope | partial |
| Scope gaps | IPv6 and transport-mode peers do not advertise MOBIKE, kernels without XFRM_MSG_MIGRATE_STATE and CONFIG_XFRM_MIGRATE do not advertise it, the RFC 4555 ledger entry is Partial |
| Level | experimental |
| Components | internal/component/ike/engine/mobike.go, internal/component/ike/dataplane |
| Interop | ipsec/mobike-initiator, ipsec/mobike-responder |
| RFCs | rfc4555 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: checked the XFRM_MSG_MIGRATE_STATE message (xfrmMsgMigrateState in internal/component/ike/dataplane/xfrm_migrate_linux.go) and mobike.go; the row states the scenarios have no recorded run, which is true |
| Defect review | 2026-10-07: plan/spec-ipsec-11-mobike.md is stale; journal rows naming a Component, not each re-verified here: false-synchronization-claim.md:17, guard-added-to-one-half-of-a-pair.md:29, silent-fall-through.md:65, stale-artifact-reused.md:60, test-against-broken-path.md:60 |
| Extra criteria | supported: live migration proven on the appliance kernel = test/ipsec/ipsec-mobike-migrate.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

RFC 4555 MOBIKE for IPv4 tunnel-mode peers, as initiator or responder: address updates, UDP 4500 source selection, COOKIE2 return-routability checks, NO_NATS_ALLOWED under the per-peer `nat-traversal` policy, and live XFRM state migration that keeps the sequence number and replay window. A peer advertises MOBIKE only when the kernel provides Linux 7.2's `XFRM_MSG_MIGRATE_STATE` API and `CONFIG_XFRM_MIGRATE`; other kernels, other backends and transport-mode peers do not. The `mobike-initiator` and `mobike-responder` interop scenarios exist with no recorded run, and the RFC 4555 ledger entry is Partial. <!-- source: internal/component/ike/engine/mobike.go -- negotiation, path selection and return routability --> <!-- source: internal/component/ike/dataplane/dataplane.go -- TunnelMigrator -->
