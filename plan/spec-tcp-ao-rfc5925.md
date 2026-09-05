# Spec: tcp-ao-rfc5925

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Implement TCP-AO (RFC 5925) as an authentication option for BGP sessions.**

Owner ruling, 2026-08-12: **"TCP-AO (RFC 5925) is a future."** So this does not hold the
first release.

### Why this belongs in `plan/future/` rather than `plan/`

`plan/future/README.md` refuses defects, and this is not one. Ze implements TCP MD5
(RFC 2385) today: `internal/component/bgp/reactor/{config,peer_settings,reactor_peers}.go`
carry the setting and `internal/component/doctor/checks_config.go` checks it. A session
authenticates. **Nothing Ze does is wrong; a stronger mechanism is absent.**

RFC 7454 Section 5.1 prefers TCP-AO over TCP MD5, but that preference is conditional on
the implementation existing. It states no obligation Ze currently violates.

### Where the requirement came from

the retired deferral shard "bcp194-0-umbrella" carries the row, dated 2026-08-08, whose Destination
cell read "needs a destination spec". **This spec is that destination.** The row's own
reasoning stands and is worth keeping:

> The recommendation is conditional on the implementation existing, and Ze has none. It is
> a feature rather than a conformance gap, and it is large enough to need its own spec
> before it can be an obligation. Folding it into child 3 would put a new authentication
> mechanism inside a session-hardening spec.

## What the work involves

Sized only enough to say it is not small. A design phase settles each of these.

| Piece | Note |
|-------|-------|
| Kernel support | TCP-AO needs `TCP_AO_ADD_KEY` and its siblings, which reached mainline Linux in 6.7. Ze's runtime kernel config (`gokrazy/kernel/`) must enable it, and `ai/rules/platform-linux.md` requires a QEMU test for the linux-only path |
| Key management | RFC 5925 defines a Master Key Tuple with a key id, a receive key id, and an algorithm. This is a larger config surface than MD5's single shared secret, and `ai/rules/config.md` governs the YANG |
| Key rollover | The mechanism exists so keys CAN be rotated without dropping the session. A design that cannot roll a key over delivers little beyond MD5 |
| Coexistence | A peer supporting only MD5 must still work. Decide whether a peer can be configured for both and how the choice is made |
| Interop | `ai/rules/interop-and-goal-validation.md` requires a test against another implementation. FRR and BIRD both support TCP-AO; `test/interop/scenarios/` is the home |

## What must not happen

**Do not treat this spec's existence as license to annotate an RFC 7454 requirement
`{gap}` or `{not-applicable}`.** `ai/rules/rfc-compliance.md` reserves that to the owner,
and the ruling above defers the WORK, not the honesty of the ledger. RFC 7454 states no
MUST here, so nothing is owed today.
