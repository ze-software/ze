# Deferrals: radius-acct-timewheel

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-radius-acct-timewheel (Known Limitations) | RADIUS accounting packet content (which attributes are emitted) | The timewheel spec covers interim-update scheduling only; packet content was the sibling spec's concern. LANDED 2026-08-03 in commit `ee5bc83028`: `buildAcctPacket` (`internal/component/l2tp/plugins/authradius/acct.go`) now emits Framed-IP-Address from the IPCP-negotiated subscriber address and NAS-Port-Id from the operator template, and spec-radius-subscriber-attributes closed on 2026-09-03. The four attributes that spec left out are homed at `plan/spec-finish-l2tp.md` | commit `ee5bc83028` (spec-radius-subscriber-attributes, closed) | done |

