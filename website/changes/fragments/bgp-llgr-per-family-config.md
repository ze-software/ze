---
spec: bgp-llgr-per-family-config
topic: Graceful Restart
feature: bgp-protocol
found-by: rfc-check
---

## Before

Long-Lived Graceful Restart (RFC 9494) was configured with a single peer-wide
`long-lived-stale-time` leaf inside `graceful-restart`. Setting it stamped
capability code 71 onto every address family negotiated on that BGP session,
preventing operators from enabling LLGR on one address family of a dual-stack
or multi-family peer while leaving another off, or from setting different
Long-Lived Stale Times per address family.

## After

`graceful-restart` is split into symmetric `short-lived` (code 64) and
`long-lived` (code 71) containers, both carrying a keyed `family` list.
Conventional GR settings live under `short-lived` (`restart-time` and optional
`family`), and omitting `short-lived` sends code 64 with Restart Time 0 and no
family tuples (helper-only GR or RFC 9494 Section 4.1 GR-phase skip). LLGR is
off by default and enabled only for address families explicitly listed under
`long-lived { family { <afi/safi> <stale-time> } }`, each with its own 24-bit
`stale-time` (0..16777215 seconds). Both `short-lived` and `long-lived` family
lists are validated against the peer's carried session families during offline
`ze config validate`, commit verification, and daemon startup. The removed
`long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }`
syntax is rejected at schema validation.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
