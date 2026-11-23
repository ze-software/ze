# draft-ietf-idr-bgp-multisession — Multisession BGP

> **Status: DECLINED — Ze will never implement this specification.**
>
> This document is kept under `rfc/declined/` for reference only. It is not
> enrolled, has no requirement rows, and carries no conformance obligations
> (`rfc/declined/README.md`).

## Meta

| Field | Value |
|-------|-------|
| Document | `draft-ietf-idr-bgp-multisession-07` |
| Title | Multisession BGP |
| Authors | J. Scudder, C. Appanna, I. Varlashkin |
| Date | September 12, 2012 |
| IETF status | Expired Internet-Draft (expired March 16, 2013; never became an RFC) |
| Ze decision | **Never implement** (owner decision, `ai/META-DECISIONS.md`) |
| Ze support | None |
| Source text | `rfc/declined/texts/draft-ietf-idr-bgp-multisession.txt` |

## Why Ze Will Never Implement This

- The draft expired on March 16, 2013 without advancing to an RFC.
- It has no general router adoption and nothing in production uses it.
- Owner ruling (`ai/META-DECISIONS.md`, "ExaBGP compatibility"): Ze does not
  support BGP multi-session, and any ExaBGP test that depends on it is a
  documented difference rather than owed work.

## What the Draft Proposed

Augments MP-BGP (RFC 4760) to allow multiple concurrent BGP sessions between
the same pair of peer IP addresses, separated by capability values (most
commonly per-AFI/SAFI) for fault isolation.

### Multisession BGP Capability (Section 4)

- **Capability code:** 68 (`0x44`, IANA allocated; Cisco prestandard used 131 / `0x83`, see RFC 8810)
- **Capability length:** variable (`1 + len(Session Id)`)

```
   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |G|  Reserved   |  Session Id   ~
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **G (1 bit):** deprecated grouping bit from earlier revisions.
- **Reserved (7 bits):** set to zero on send, ignored on receipt.
- **Session Id (variable):** list of 1-octet BGP capability codes used to
  distinguish sessions; empty or `[1]` means AFI/SAFI distinguishes sessions.

### Proposed OPEN Message Error Subcodes (Section 5)

| Subcode | Name |
|---------|------|
| 7 | Capability Value Mismatch |
| 8 | Grouping Conflict |
| 9 | Grouping Required |

### Collision Handling (Sections 6–7)

Replaced RFC 4271 Section 6.8 collision detection with a check scoped to
matching peer IP addresses and overlapping values for the capabilities listed
in Session Id.

## How Ze Handles It

- **Wire / Capabilities:** Ze never advertises capability code 68 or 131. If a
  peer sends either in OPEN, Ze's parser preserves it as `Unknown` and ignores
  it, per RFC 5492 Section 3 (`docs/architecture/wire/capabilities.md`).
- **Configuration & Migration:** Ze config syntax has no `multi-session`
  keyword (`ze config migrate` rejects it as unknown). `ze exabgp migrate`
  emits a warning on stderr when an ExaBGP config contains
  `capability { multi-session; }` and omits it from the migrated config
  (`docs/architecture/config/deprecated-options.md`).
- **ExaBGP Bridge:** `negotiated.multisession` is always `false`. A peer
  selector carrying `family-allowed <family-list>` matches no session, while
  `family-allowed in-open` matches Ze's single session
  (`docs/exabgp/exabgp-differences.md`, `docs/architecture/exabgp-bridge.md`).
