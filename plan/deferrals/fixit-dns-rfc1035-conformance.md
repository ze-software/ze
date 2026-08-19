# Deferrals: fixit-dns-rfc1035-conformance

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-fixit-dns-rfc1035-conformance WP-4 | `RFC1035-4.2-1`, zone transfer. `answerQuestions` (`internal/plugins/geodns/server.go`) switches on `q.Qtype` with no AXFR or IXFR case, so QTYPE 252 falls to the default branch and draws NOERROR plus the zone SOA; no `AXFR` or `IXFR` token exists anywhere under `internal/`. The smallest honest increment is not a zone-transfer server but a refusal: route QTYPE AXFR and IXFR to REFUSED on TCP and refuse AXFR on UDP | The owner ruled RFC 1035 out of scope on 2026-08-18, while the fixit backlog was being drained for the first release. The reading underneath is also unsettled and is his to settle: RFC 1035 Section 4.2 says "Zone refresh activities must use virtual circuits because of the need for reliable transfer", which constrains the TRANSPORT of a zone refresh rather than obliging a server to implement one, and Ze performs no zone refresh, so it never enters that condition. Calling the row `{not-applicable}` on that reading would lower what Ze owes, which `ai/rules/rfc-compliance.md` reserves to him | plan/spec-fixit-dns-rfc1035-conformance.md | deferred |
| 2026-08-19 | spec-fixit-dns-rfc1035-conformance AC-24 | Enrolment of `rfc1035` in `rfc/enrolled.txt`, which would gate its MUSTs | Blocked on the `RFC1035-4.2-1` ruling above: enrolling gates a requirement whose disposition is undecided | plan/spec-fixit-dns-rfc1035-conformance.md | deferred |
