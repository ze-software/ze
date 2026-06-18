# ISO / non-IETF Standard References

Summaries of standards that are **not** published by the IETF and therefore do
not belong under `rfc/`. ISO/IEC documents (for example ISO/IEC 10589, the base
IS-IS standard) are not freely redistributable, so this tree holds clean-room
summaries only, never the standard text itself.

Layout mirrors `rfc/`:

| Path | Contents |
|------|----------|
| `iso/short/` | Clean-room short summaries (the verified source of truth for implementation), one file per standard |

When implementing standard functionality, code MUST reference the standard and
clause, for example `// ISO/IEC 10589 clause 7.3.15: "<requirement>"`.

## Index

| Standard | Title | Summary | Status |
|----------|-------|---------|--------|
| ISO/IEC 10589:2002 | Intermediate System to Intermediate System intra-domain routeing protocol | [short/iso10589.md](short/iso10589.md) | Reference (IS-IS base, see `plan/spec-isis-*`) |

Related IETF RFCs for IS-IS (IP support and extensions) live under `rfc/short/`:
RFC 1195, RFC 2966, RFC 3786, RFC 3787, RFC 5301, RFC 5303, RFC 5304, RFC 5305,
RFC 5308, RFC 5310.
