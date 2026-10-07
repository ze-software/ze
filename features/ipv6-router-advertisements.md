# IPv6 Router Advertisements

## Meta

| Field | Value |
|-------|-------|
| Name | IPv6 Router Advertisements |
| Page | docs/features/interfaces.md#ipv6-router-advertisements |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/iface/ra, internal/component/iface/config_ra.go, internal/core/ndp |
| Real-path tests | test/plugin/iface-ra-slaac.ci, test/parse/iface-router-advertisement.ci, test/parse/iface-router-advertisement-invalid.ci, test/parse/iface-vpp-rejects-router-advertisement.ci |
| Docs | docs/features/interfaces.md |
| Doc review | 2026-10-07: rdnss server leaf-list max-elements 8 in ze-iface-conf.yang; counters ze_iface_ra_sent_total and ze_iface_ra_solicited_total registered in ifacera.go; matches the row |
| Defect review | 2026-10-07: plan/journal/identity-default-hides-a-mapping.md row 7 may name the doctor forwarding check; RFC 4861/8106 not enrolled (plan/pre-release/spec-rfc4861-rfc8106-enrolment.md) |
| Extra criteria | supported: RFC 4861 and RFC 8106 enrolled in the RFC ledger (plan/pre-release/spec-rfc4861-rfc8106-enrolment.md) = none yet; supported: SLAAC against a non-Ze host = none yet |

## Description

Ze sends IPv6 Router Advertisements on a LAN interface unit (RFC 4861). This is the job radvd does on other systems. Hosts build addresses by stateless address autoconfiguration (SLAAC), learn a default router, and learn DNS resolvers (RFC 8106 RDNSS, up to 8 servers). The `router-advertisement` container sits in the per-unit `ipv6` container. It carries the M and O flags, the per-prefix L and A flags, four timers, a prefix list, and the resolver block. The container is Linux only and netlink only. A `backend vpp` tree rejects it at config verify. Config verify applies the cross-leaf rules a YANG range cannot express. `minimum-interval` is at most 0.75 x `maximum-interval`, and `preferred-lifetime` is at most `valid-lifetime`. A prefix with host bits is rejected rather than masked, and so is the link-local prefix. Two zero values are legal input. `router-lifetime 0` advertises prefixes and resolvers while Ze is not a default router. `rdnss lifetime 0` tells hosts to stop using the resolvers. The `iface-ra` plugin owns the socket and the timers. Every advertisement leaves with Hop Limit 255. The sender joins `ff02::2` for Router Solicitations and sends to `ff02::1`. Each unsolicited interval is random between the two configured bounds, and a solicited answer is rate limited. A sender that stops sends one advertisement with a Router Lifetime of 0. Counters are `ze_iface_ra_sent_total{interface}` and `ze_iface_ra_solicited_total{interface}`. `ze doctor` reports `doctor-iface-ra-forwarding` when an advertising interface has IPv6 forwarding off. <!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container router-advertisement --> <!-- source: internal/component/iface/config_ra.go -- raValidate, raParsePrefixEntry --> <!-- source: internal/plugins/iface/ra/sender_linux.go -- openRASocket, run, sendFinal --> <!-- source: internal/plugins/iface/ra/ifacera.go -- SetMetricsRegistry --> <!-- source: internal/core/ndp/schedule.go -- UnsolicitedInterval --> <!-- source: internal/plugins/iface/ra/doctor.go -- checkRAForwarding -->
