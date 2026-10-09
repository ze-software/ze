# BMP Delivery (RFC 7854)

## Meta

| Field | Value |
|-------|-------|
| Name | BMP Delivery (RFC 7854) |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 7854 obligations the ledger marks Partial |
| Level | experimental |
| Components | internal/component/bgp/plugins/bmp |
| Real-path tests | test/plugin/bmp-sender-route-monitoring.ci, test/plugin/bmp-sender-peer-up-open.ci, test/plugin/bmp-sender-statistics.ci, test/plugin/bmp-sender-route-mirroring.ci, test/plugin/bmp-locrib.ci, test/plugin/bmp-sessions-show.ci |
| Interop | bgp/bmp-frr, bgp/bmp-locrib-pmacct, bgp/bmp-statistics-pmacct |
| RFCs | rfc7854 |
| Docs | docs/guide/bmp.md |
| Doc review | 2026-10-09: every source anchor in the Description resolves to its file and symbol; the 256 MiB bound is txQueueLimitBytes = 256 << 20 in internal/component/bgp/plugins/bmp/txqueue.go |
| Defect review | 2026-10-09: spec-bmp-statistics-timeout-sends-no-report closed, the statistics-timeout leaf now drives a periodic Statistics Report (internal/component/bgp/plugins/bmp/statistics.go::sendStatisticsReports); journal constant-reported-as-measured-state (Loc-RIB ORIGIN literal) taken as open |
| Extra criteria | supported: a collector that reads slower than Ze produces leaves the producer unblocked and resets at the byte bound = test/plugin/bmp-slow-collector-reset.ci |

## Description

Each collector session owns a bounded FIFO transmit queue, drained by its own goroutine off the producer's, so a slow collector never blocks the BGP path. The bound is 256 MiB of queued-but-unwritten bytes per session, sized to absorb a full Loc-RIB dump (about 1M IPv4 best paths at roughly 120 bytes of Route Monitoring each is about 120 MB). Reaching it resets the session rather than dropping a message, because RFC 7854 has no back-pressure signal. The byte bound is NOT the defense against a wedged collector: one that stops reading entirely is caught seconds earlier by the per-write deadline. It bites for the other shape, a collector that keeps reading but steadily slower than Ze produces, where every write succeeds and the backlog is what grows. A Loc-RIB dump closes every family it owes, including one with nothing to send. <!-- source: internal/component/bgp/plugins/bmp/txqueue.go -- txQueueLimitBytes, txQueue -->
