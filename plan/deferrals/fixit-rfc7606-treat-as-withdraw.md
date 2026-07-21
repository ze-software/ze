# Deferrals: fixit-rfc7606-treat-as-withdraw

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-rfc7606-treat-as-withdraw functional-proof | AC-8 two-family MP_UNREACH split + AC-9 non-negotiated family are reactor-scope | implemented 2026-07-21 with unit+reactor tests (per-family split + noPoolBufID forward-cache for RS clients; negotiation-aware synthesis); no QEMU needed (conformance is the ze-peer path, interop N/A) | plan/learned/1188-fixit-rfc7606-treat-as-withdraw.md | done |

