# IPsec CLI and Diagnostics

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec CLI and Diagnostics |
| Kind | daemon |
| Scope | partial |
| Scope gaps | monitor vpn ipsec, the web page and the Prometheus gauges have no real-path test |
| Level | experimental |
| Components | internal/component/ike/cmd, internal/component/web/page_vpn_ipsec.go |
| Real-path tests | test/ipsec/ipsec-sa-show.ci, test/ipsec/ipsec-status-show.ci, test/ipsec/ipsec-peer-show.ci, test/ipsec/ipsec-clear-sa.ci, test/ipsec/ipsec-dataplane-show.ci, test/ipsec/ipsec-show-dataplane-kernel.ci, test/ipsec/ipsec-show-sa-counters.ci |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-26: re-read after the IKE virtual-IP commits changed docs/guide/ipsec.md; every source anchor resolves; the belief and kernel gauges are still registered under internal/component/ike/; added the `virtual-ip` SA column (virtualIPText in show_ipsec.go), the ze_ipsec_virtual_ip_leases gauge (publishVirtualIPGauges) and the 90% ipsec-virtual-ip-pool doctor warning (virtualIPPoolWarnPercent in doctor_virtual_ip.go) |
| Defect review | 2026-10-07: audit found no open immediate spec against the IPsec command surface; journal rows naming a Component, not each re-verified here: comment-describes-superseded-behaviour.md:20 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

`show vpn ipsec sa/status/peer`, `clear vpn ipsec sa all`, `clear vpn ipsec sa peer <name>` (the bare form is refused), `monitor vpn ipsec` (live SA event stream). `show vpn ipsec dataplane sa/policy/drift` reads Linux XFRM state and compares expected Child SAs with the SAD. VPP and noop report unsupported enumeration. SA displays carry kernel byte and packet counters, with null for unavailable measurements, and `show vpn ipsec sa` lists a peer's leased inner address under `virtual-ip`. Web page at `/show/vpn/ipsec/` with SA table. Health reports degraded for drift or an unknown dataplane observation. Prometheus retains the belief gauges `ze_ipsec_sa_count`, `ze_ipsec_tunnel_up`, and `ze_ipsec_tunnel_degraded`. Kernel gauges `ze_ipsec_dataplane_sa_count{if_id}` and `ze_ipsec_dataplane_drift{peer}` remove their series when observation fails. Show commands reach the shared pipe layer; `ze help command --json` publishes each command's operators. Address operators require declared address fields. `ze_ipsec_virtual_ip_leases{pool}` counts the virtual IP addresses live IKE SAs hold in each `remote-access pool`, and `ze doctor` warns under `ipsec-virtual-ip-pool` when one family of a pool is at least 90% held. <!-- source: internal/component/ike/cmd/show_ipsec.go -- readSADCounters, addChildCounters --> <!-- source: internal/component/ike/cmd/show_dataplane.go -- init, handleShowVPNIPsecDataplaneDrift, dataplaneReadError --> <!-- source: internal/component/ike/cmd/monitor_ipsec.go -- monitor vpn ipsec --> <!-- source: internal/component/ike/cmd/ipsec.go -- clear vpn ipsec --> <!-- source: internal/component/ike/engine/health.go -- checkIPsecHealth --> <!-- source: internal/component/ike/engine/metrics.go -- Update, publishDataplaneGauges, clearDataplaneGauges, publishVirtualIPGauges --> <!-- source: internal/component/ike/engine/doctor_virtual_ip.go -- checkIPsecVirtualIPPool --> <!-- source: internal/component/ike/cmd/show_ipsec.go -- virtualIPText -->
