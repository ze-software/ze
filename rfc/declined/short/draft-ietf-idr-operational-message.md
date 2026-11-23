# draft-ietf-idr-operational-message — BGP OPERATIONAL Message

> **Status: DECLINED — Ze will never implement this specification.**
>
> This document is kept under `rfc/declined/` for reference only. It is not
> enrolled, has no requirement rows, and carries no conformance obligations
> (`rfc/declined/README.md`).

## Meta

| Field | Value |
|-------|-------|
| Document | `draft-ietf-idr-operational-message-00` |
| Title | BGP OPERATIONAL Message |
| Authors | D. Freedman, R. Raszuk, R. Shakir |
| Date | February 29, 2012 |
| IETF status | Expired Internet-Draft (expired September 1, 2012; never became an RFC) |
| Ze decision | **Never implement** (owner decision, `ai/META-DECISIONS.md`) |
| Ze support | None |
| Source text | `rfc/declined/texts/draft-ietf-idr-operational-message.txt` |

## Why Ze Will Never Implement This

- The draft expired on September 1, 2012 without advancing to an RFC.
- Section 9 requested a BGP Message Type and a BGP Capability Code from IANA,
  and neither was ever allocated. ExaBGP invented private values (`0xB9` / 185
  for the capability and `0x06` for the message type); Ze does not put private
  unassigned codepoints on the wire.
- It has no general router adoption and nothing in production uses it.
- Owner ruling (`ai/META-DECISIONS.md`, "ExaBGP compatibility"): Ze does not
  support the BGP OPERATIONAL message, and any ExaBGP test that depends on it
  is a documented difference rather than owed work.

## What the Draft Proposed

Defined an in-band BGP message type (`OPERATIONAL`) negotiated via a BGP
capability (RFC 5492) to exchange diagnostic, advisory, and prefix-count
information across a BGP session using Type-Length-Value containers.

### TLV Format (Section 3.2)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |      Type (2 octets)          |       Length (2 octets)       |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                      Value (Variable)                         |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Proposed TLV Type Groups (Section 9)

| Group | Types | Purpose |
|-------|-------|---------|
| ADVISE | 1 (ADM), 2 (ASM) | Human-readable operator advisories |
| STATE | 3–4 (RPCQ/RPCP), 5–6 (APCQ/APCP), 7–8 (LPCQ/LPCP), 9 (SSQ) | Prefix counts and simple state queries |
| DUMP | 10 (DUP), 11 (MUP), 12 (MUD), 13 (SSP) | Dropped/malformed UPDATE dumps and state responses |
| CONTROL | 65534 (MP), 65535 (NS) | Max Permitted and Not Satisfied control replies |

## How Ze Handles It

- **Wire / Capabilities:** Ze never advertises capability code 185 (`0xB9`) and
  never originates or accepts message type `0x06`. Received unknown capability
  codes are preserved as `Unknown` per RFC 5492 Section 3
  (`docs/architecture/wire/capabilities.md`).
- **Configuration & Migration:** Neither Ze syntax nor `ze exabgp migrate`
  accepts an `operational { ... }` block; both reject it as an unknown field
  (`docs/architecture/config/deprecated-options.md`).
- **ExaBGP Bridge:** `negotiated.operational` is always `false`, and
  `announce operational ...` commands are refused with a message stating that
  Ze does not support the OPERATIONAL message
  (`docs/exabgp/exabgp-differences.md`, `docs/architecture/exabgp-bridge.md`).
