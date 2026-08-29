# Deferrals: pki-full-chain

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-pki-full-chain (design) | Looking-glass TLS serves a PKI-stored chain: `cmd/ze/hub/service_lg.go` keeps the self-signed-only `LoadOrGenerateCert` path; extend by consuming `pki.ServerTLSMaterial` like web/DoT/DoH | Design bounded to web + dnsserver consumers to keep the spec reviewable; same pattern applies cleanly later | `plan/future/spec-followup-pki-chain.md` (work item added; re-homed 2026-07-16 from prose. A FOLLOWUP spec, not `spec-pki-full-chain` itself: that spec bounded its scope deliberately and is already `ready`, and pointing here would orphan this row again at its closure -- exactly how the web-cli-ux and appliance-evidence rows were lost) | deferred |
| 2026-07-10 | spec-pki-full-chain (design) | Multi-intermediate chains: `intermediate` holds a single certificate (`pki/config.go`), so a 4-tier CA (leaf + 2 intermediates) cannot be expressed; extend `intermediate` to a list | Single-intermediate covers the common case; the spec's doctor chain check reports AKI/SKI mismatch so the gap stays visible | `plan/future/spec-followup-pki-chain.md` (work item added; re-homed 2026-07-16 from prose. Same reasoning as the looking-glass row above) | deferred |

