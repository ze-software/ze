# Deferrals: fixit-private-asn-leak-deferred-nil-api-fail-open

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-private-asn-leak-deferred-nil-api-fail-open functional-proof | sibling slices (filter_ordered.go egress/ingress guards, reactor.go SetPluginServerAny) remained | implemented 2026-07-21 with RED-first unit tests (guard driven from its entry-point chain methods); no user-facing surface and the trigger is a Go-level guard state, so no QEMU/.ci (the spec's designated Wiring-Test opt-out) | plan/learned/1236-fixit-private-asn-leak-deferred-nil-api-fail-open.md | done |

