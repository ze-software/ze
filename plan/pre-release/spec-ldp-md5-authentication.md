# Spec: LDP TCP MD5 signature option

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

RFC 5036 Section 2.9 makes the TCP MD5 Signature option "a configurable option". Ze reads no LDP password (`register.go::parseLDPConfig`) and sets no TCP_MD5SIG on the session socket (`register.go::ldpSessionDialer`). Once the leaf and the setsockopt exist, the signing itself is Linux TCP, so the proof asserts the socket option ze installs.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-2.9-1` [MUST] "The use of the TCP MD5 Signature Option mechanism that protects against spoofed TCP segments in LDP session connection streams MUST be supported as a configurable option (§2.9)" -- `parseLDPConfig` reads no password and `ldpSessionDialer` sets no TCP_MD5SIG
