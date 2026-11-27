---
spec: dropped-error-3-access-security
topic: Diagnostics
found-by: review
---

## Before

Errors in IPsec/IKEv2, PPP, PPPoE, and RADIUS admin authentication were caught and logged only at Debug or Info, hiding them at the default WARN log level and keeping them off `show errors` and `show doctor`. Failed XFRM rollback deletes, non-ENOENT Child SA and SPD removals, established IKE Delete/INFORMATIONAL/notify build failures, IKE datagram and DF-clear probe send failures, Child SA lifecycle event emit failures, hash-and-URL certificate fetch failures, periodic NAT-T keepalive send failures, live PPP IPv6 Router Advertisement send failures, and RADIUS concluding EAP mismatches all went unseen unless Debug or Info logging was enabled. Malformed inbound IKE packets, malformed PPP/LCP/IPCP/IPv6CP frames, and malformed PPPoE discovery frames were dropped without Prometheus counters.

## After

Operator-actionable failures across IPsec/IKEv2, PPP, subscriber RADIUS Framed-MTU staging, and RADIUS admin EAP now emit WARN log lines with registered doctor codes and remediation commands, and raise events on the daemon report bus (`show errors` and `show doctor`) in both internal and forked plugin modes. Periodic and hot-path failures (NAT-T keepalive sends, established IKE sends, DF-clear path probes, hash-and-URL fetches, and PPP IPv6 Router Advertisement sends) warn and raise once per failure condition until recovery rather than flooding logs. Untrusted malformed wire traffic is counted without per-packet warnings on four Prometheus series: `ze_ipsec_malformed_inbound_total`, `ze_ipsec_send_failures_total`, `ze_ppp_malformed_frames_total`, and `ze_pppoe_discovery_parse_errors_total`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
