# 085 — Announce L3VPN

## Objective

Add `AnnounceL3VPN` and `WithdrawL3VPN` reactor methods to enable L3VPN (MPLS VPN, SAFI 128) route announcements via API.

## Decisions

- Adapted the existing `AnnounceLabeledUnicast` pattern for L3VPN: same structure (build VPNParams → UpdateBuilder → send), different NLRI encoding.
- Route Target (RT) parsed as extended community (RFC 4360): supports 2-byte ASN (Type 0), 4-byte ASN (Type 2), and IP:NN (Type 1) formats.
- Withdrawal uses first label from stack: RFC allows using any label for withdrawal since the prefix identifies the route uniquely.

## Gotchas

- None beyond what RFC 4364/4360 document.

## Files

- `internal/reactor/reactor.go` — AnnounceL3VPN, WithdrawL3VPN, buildL3VPNParams, parseRouteTarget
