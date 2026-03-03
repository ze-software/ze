# 005 — RFC Annotation

## Objective

Annotate all protocol implementation files with RFC section references and document violations found, to establish a foundation for RFC compliance verification.

## Decisions

Mechanical annotation effort. Violations found were captured in the plan and merged into the alignment implementation plan (spec 006).

## Patterns

None beyond what the resulting violation list covers.

## Gotchas

- Several violations found that were not on the original 26-item alignment list: AS_CONFED segment handling (RFC 6793), AS4_PATH merge semantics (RFC 4271 §9.1.2.2), EVPN Type 5 fixed-field encoding (RFC 9136), BGP-LS descriptor TLV encoding (RFC 7752).
- Maximum message length (RFC 4271 §4.1 says ≤4096) was intentionally unenforced because RFC 8654 extended messages allow 65535 — annotated as "may be intentional."

## Files

- All files in `internal/bgp/message/`, `internal/bgp/capability/`, `internal/bgp/attribute/`, `internal/bgp/nlri/`, `internal/bgp/fsm/` — RFC section comments added inline.
