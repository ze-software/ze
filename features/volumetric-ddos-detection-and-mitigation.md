# Volumetric DDoS Detection and Mitigation

## Meta

| Field | Value |
|-------|-------|
| Name | Volumetric DDoS Detection and Mitigation |
| Page | docs/guide/ddos-mitigation.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/ddos |
| Real-path tests | test/plugin/ddos-announce-rate-limit.ci, test/plugin/ddos-bps-amplification.ci, test/plugin/ddos-detect-characterize.ci, test/plugin/ddos-detect-external-warns.ci, test/plugin/ddos-detect-mitigate.ci, test/plugin/ddos-direction.ci, test/plugin/ddos-firewall-concurrency.ci, test/plugin/ddos-flow-recent.ci, test/plugin/ddos-flowspec-announce.ci, test/plugin/ddos-incident-confidence.ci, test/plugin/ddos-local-cap-survives-reload.ci, test/plugin/ddos-local-config-removed.ci, test/plugin/ddos-local-max-duration.ci, test/plugin/ddos-local-stale-table-swept.ci, test/plugin/ddos-parent-config-removed.ci, test/plugin/ddos-policy.ci, test/plugin/ddos-show.ci, test/plugin/ddos-timing-leaves.ci, test/plugin/ddos-transit-forward-drop.ci |
| Docs | docs/guide/ddos-mitigation.md |
| Doc review | 2026-10-07: enforceMaxDuration in internal/plugins/ddos/local/responder.go driven once a second by startMaxDurationWorker (register.go), 0 = no cap, YANG range 0..86400 default 3600 in ze-ddos-local-conf.yang; flowspec leaf range 0..604800 default 3600 in ze-ddos-flowspec-conf.yang |
| Defect review | 2026-10-07: open immediate specs spec-fixit-ddos-baseline-restore-staleness.md, spec-fixit-ddos-frag-flood-family.md, spec-fixit-ddos-incident-heartbeat-wiring.md, spec-fixit-ddos-incident-lifecycle-on-teardown.md, spec-fixit-ddos-incident-outbound-durability.md; journal component-rebuilt-during-reload.md row unfixed |
| Extra criteria | supported: a FlowSpec mitigation received and applied by a third-party BGP speaker = none yet |

## Description

The volumetric domain, kept separate from the behavioral anomaly detector above. `ddos-detect` runs two-stage detection over the traffic-monitor feed: per-source packet-rate and bandwidth p99 baselines, attack characterization, and incident confidence scoring. Three responders consume its incidents. `ddos-local` drops box-directed attacks with an on-host nft rule. `ddos-flowspec` originates a FlowSpec or RTBH rule upstream for transit attacks. `ddos-flowtriq` reports incidents to the Flowtriq cloud API. `ddos-observe` holds the incident lifecycle behind `show ddos status` and `show ddos incidents`. Clearing an upstream mitigation is the hard half, because an upstream drop blinds the box's own sensors: the leak probe narrows the announced rule to `probe-rate` and reads the detector's attack-ongoing bandwidth to see whether the flood still arrives. An announce is also withdrawn by `max-mitigation-duration` (checked once a second, `0` means no cap, default 3600s, range 0..604800), by a characterized exemption, and by a victim that reclassifies from remote to local. `ddos-local` enforces its own `max-mitigation-duration` on the on-host drop rule the same way: measured from when the rule went in, checked once a second, `0` means no cap, default 3600s, range 0..86400. <!-- source: internal/plugins/ddos/flowspec/responder.go -- MaxMitigationDuration enforcement, onOngoing --> <!-- source: internal/plugins/ddos/flowspec/probe.go -- leak probe --> <!-- source: internal/plugins/ddos/local/responder.go -- enforceMaxDuration --> <!-- source: internal/plugins/ddos/local/register.go -- startMaxDurationWorker -->
