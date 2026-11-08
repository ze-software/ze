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
| Interop | ipsec/mobike-initiator, ipsec/mobike-responder, topology/mgmt-tunnel, topology/mgmt-tunnel-ed25519 |
| RFCs | rfc4555 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-11-08: re-read after the home-CPE lab commit added the topology Interop entries; childMigrationAvailable in mobike.go still gates advertisement on the TunnelMigrator captured in bindSockets (sa.go), tunnel mode and a NAT-T socket; swanSAVerdict in internal/le/interoplab/topology/check_vpn.go refuses an SA off port 4500; ./le feature report confirms no green run of any of the four scenarios in features/runs/ipsec-mobike.json. Earlier, 2026-10-26: re-read after the IKE virtual-IP commits changed docs/guide/ipsec.md; XFRM_MSG_MIGRATE_STATE (xfrmMsgMigrateState in internal/component/ike/dataplane/xfrm_migrate_linux.go), TunnelMigrator in dataplane.go and mobike.go still hold the described behavior; the description still matches |
| Defect review | 2026-10-07: plan/release/fixes/spec-ipsec-11-mobike.md is stale; journal rows naming a Component, not each re-verified here: false-synchronization-claim.md:17, guard-added-to-one-half-of-a-pair.md:44, silent-fall-through.md:65, stale-artifact-reused.md:60, test-against-broken-path.md:58 |
| Extra criteria | supported: live migration proven on the appliance kernel = test/ipsec/ipsec-mobike-migrate.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

RFC 4555 MOBIKE for IPv4 tunnel-mode peers, as initiator or responder: address updates, UDP 4500 source selection, COOKIE2 return-routability checks, NO_NATS_ALLOWED under the per-peer `nat-traversal` policy, and live XFRM state migration that keeps the sequence number and replay window. A peer advertises MOBIKE only when the kernel provides Linux 7.2's `XFRM_MSG_MIGRATE_STATE` API and `CONFIG_XFRM_MIGRATE`; other kernels, other backends and transport-mode peers do not. The `mgmt-tunnel` and `mgmt-tunnel-ed25519` topology scenarios check that strongSwan holds Ze's IKE SA on UDP 4500 with no NAT on the path, which only MOBIKE's move explains (RFC 4555 Section 3.3). These two and the `mobike-initiator` and `mobike-responder` interop scenarios have no run recorded for this feature, and the RFC 4555 ledger entry is Partial. <!-- source: internal/component/ike/engine/mobike.go -- negotiation, path selection and return routability --> <!-- source: internal/component/ike/dataplane/dataplane.go -- TunnelMigrator --> <!-- source: internal/le/interoplab/topology/check_vpn.go -- swanSAVerdict -->
