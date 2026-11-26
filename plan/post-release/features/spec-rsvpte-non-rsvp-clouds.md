# Spec: RSVP-TE non-RSVP neighbor tracking and IP-tunnel hop notification (RFC 3209 Section 4.2.5, RFC 2205 Section 3.8)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Post-release capability split from `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` by owner decision (2026-11-24): unstarted non-RSVP neighbor tracking and routing-protocol IP-tunnel notification do not block first release. |
| RFC gaps | RFC3209-4.2.5-1, RFC2205-3-31 |
| Depends | `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decision (2026-11-24)

Split out of `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` by Thomas's 2026-11-24 ruling:

| Requirements | Decision | Owning spec |
|--------------|----------|-------------|
| `RFC2205-3-30` (TTL vs `Send_TTL` non-RSVP hop detection and ADSPEC Global Break bit) | Keep in `plan/release/fixes/` and implement for release 1. | `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` |
| `RFC3209-4.2.5-1` (non-RSVP neighbor tracking and `LABEL_REQUEST` suppression) and `RFC2205-3-31` (routing-protocol IP-tunnel notification of non-RSVP hops) | Split out and move both unstarted capabilities to `plan/post-release/features/`. | `plan/post-release/features/spec-rsvpte-non-rsvp-clouds.md` (this spec) |

## Task

**Backlog boundary:** This skeleton records two unstarted post-release capabilities split from `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md`. Its requirements are neither completed nor waived, and the owner's authorization to implement `RFC2205-3-30` for release 1 does not authorize starting these capabilities.

`plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` covers IPv4 TTL vs RSVP `Send_TTL` non-RSVP-hop detection and passing that signal to traffic control (`RFC2205-3-30`). That mechanism cannot detect non-RSVP hops hidden inside IP encapsulating tunnels (where the inner IPv4 TTL is decremented only at the tunnel endpoints), and Ze holds no per-neighbor RSVP-capability table or manual configuration declaring a neighbor non-RSVP-capable. This post-release spec owns both remaining obligations:

1. Tracking or configuring known non-RSVP-capable neighbors and suppressing the `LABEL_REQUEST` object (and sending the RFC 3209 Section 4.2.5 PathErr when MPLS label negotiation crosses a known non-RSVP router), instead of `internal/plugins/rsvpte/build.go::buildPath` appending `LABEL_REQUEST` unconditionally.
2. Receiving non-RSVP-hop notifications from routing protocols when a resolved route traverses an IP encapsulating tunnel (extending the route lookup boundary in `internal/plugins/rsvpte/routing.go` and `internal/plugins/rsvpte/transport_linux.go::ResolveRoute`), and feeding that signal into the same traffic-control `Non_RSVP_Hop_flag` path established by `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md`.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.2.5-1 | "This means that if a router has a neighbor that is known to not be RSVP capable, the router MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers." (S4.2.5) | Absent: Ze holds no RSVP-capability knowledge of a neighbor; `internal/plugins/rsvpte/build.go::buildPath` appends `LABEL_REQUEST` unconditionally |
| RFC2205-3-31 | "For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included." (§3) | Absent: `internal/plugins/rsvpte/transport_linux.go::ResolveRoute` supplies native next-hop, interface index, lookup address and MTU, not RSVP-capability or IP-encapsulating-tunnel metadata about the route |
