# Spec: LDP Address message send side

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze decodes the Address and Address Withdraw messages into `Session.addPeerAddresses` but never SENDS one: `wire.go` has no Address message encoder and `register.go::runSession` never calls one. A peer therefore cannot map ze's LDP Identifier to ze's interface addresses, so it cannot resolve the data-plane next hop for a label ze advertises. This is base LDP an operator meets on the first release, not an optional feature.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-2.7-1` [MUST] "To enable LSRs to map between a peer LDP Identifier and the peer's addresses, an LSR must be able to map the next hop address for a prefix to an LDP Identifier and an LDP Identifier to the peer's addresses, and LSRs advertise their addresses using LDP Address and Address Withdraw messages (§2.7)" -- receive half only; no Address message encoder in `wire.go` and no send call in `register.go::runSession`
