# Protocol Subpackage Skeleton (advisory)

**When:** creating a new protocol implementation, adding the first subpackages to a single-package protocol, or reorganizing a protocol's module layout.
**Severity:** advisory

## Directives

One learned layout should fit every protocol (the holo-routing lesson: a fixed
per-protocol skeleton makes each protocol navigable once you know one). This
rule defines that skeleton using the package-naming glossary
(`ai/rules/naming.md` "Package-Naming Glossary") and maps every existing
protocol to it. It is ADVISORY for existing code: no moves, no renames, no
build gate. New protocols follow it; touched code adopts it opportunistically.

## The skeleton

A single-package protocol (root package + `yang/`) needs none of this -- LDP
and RSVP-TE live comfortably at that size. Once a protocol grows subpackages,
the skeleton applies:

| Module | Required? | Holds |
|--------|-----------|-------|
| root package | required | `register.go` registration (`sdk.NewWithConn` engine entry), config plumbing; may BE the engine |
| `packet/` | required | The wire codec: parse + encode the protocol's PDUs/TLVs (glossary: `packet`) |
| `transport/` | required | Socket I/O delivering wire bytes to/from the engine; in-memory loopback for tests welcome |
| `yang/` | required | Embedded + registered YANG schema modules (already uniform across all protocols) |
| engine home | required | The long-lived runtime loop: either the root package (IS-IS, OSPF) or a dedicated `engine/` (BFD, IKE) |
| per-peer state | required when the protocol has per-peer conversations | Named by the protocol's OWN RFC term: `session` (BFD), `adjacency` (IS-IS), `neighbor` (OSPF), `fsm` (BGP, RFC 4271). Do not flatten these to one word -- the RFC name is the discoverable one |
| `types/` | optional | Shared leaf types imported by codec and engine |
| `cli/` or `cmd/` | optional | Operational command handlers (see the glossary trio for which) |
| `redistribute/` | optional | Route redistribution glue |
| domain modules | optional | Protocol concepts named after the RFC concept: `lsdb`, `spf`, `sr`, `crypto`, `eap`, `ipsec`, `auth`, `circuit`, `iface`. Free naming, one concept per package |
| `v<N>/` | optional | Wire-version split (`ospf/v3`): version-specific `packet`/`types`/`transport` under a version dir, shared engine above it |

BFD is the reference layout: `packet` / `engine` / `session` / `transport` /
`auth` / `cmd` / `api` / `yang`.
<!-- source: internal/component/bfd -- subpackage layout -->

## How existing protocols map (probe, 2026-07-08)

| Protocol | Maps cleanly | Exceptions |
|----------|--------------|------------|
| BFD | everything | none -- the reference |
| IS-IS | `packet`, `transport`, root engine, `adjacency` (RFC term), `types`, `yang`, `cli`; `circuit`/`lsdb`/`spf`/`redistribute` domain modules | none |
| OSPF | `packet`, `transport`, root engine, `neighbor` (RFC term), `types`, `yang`, `cli`; `iface`/`lsdb`/`spf`/`sr`/`redistribute` domain; `v3` version dir; `wire` = raw handoff type (glossary sense of `wire`) | none |
| IKE | `engine`, `transport`, `yang`, `cmd`; `crypto`/`eap`/`ipsec`/`dataplane` domain | `wire` is a full codec where the skeleton says `packet` (predates the glossary; kept) |
| BGP | `fsm` (RFC term), `types`, `yang`, `cli`; many platform modules | platform archetype, pre-SDK: `message`+`wireu` for the codec, `reactor`+`server` for the runtime -- documented as historical in the glossary; not a template for new work |
| LDP, RSVP-TE | single-package + `yang/` | below the subpackage threshold; skeleton N/A until they grow |

## The advisory report

`scripts/dev/protocol_skeleton_report.py` lists each protocol's modules
classified against the skeleton (canonical / RFC-named state / version dir /
domain / legacy exception) and prints a one-line summary by default
(`--verbose` for the full table). It ALWAYS exits 0 in report mode -- it is
an advisory lens, not a gate (an enforced skeleton today would need a large
allowlist; see the tiers Path B lesson in `plan/spec-tiers-0-umbrella.md`).
It runs as the last, non-enforcing line of `make ze-tier-check`.
<!-- source: scripts/dev/protocol_skeleton_report.py -- classifier and manifest -->

## Related

- `ai/rules/naming.md` -- the package-naming glossary this skeleton speaks.
- `ai/rules/module-tiers.md` -- which TIER a protocol lives in (this rule is
  about layout WITHIN the protocol; it never moves packages between tiers).
- `ai/rules/plugin-design.md` -- registration and proximity rules.
