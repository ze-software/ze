# Protocol Implementation

**When:** implementing or changing a protocol, an external API, a wire format, or a backend that applies operator config
**Severity:** blocking
**Related:** rfc-compliance, completion, planning, go-standards, architecture, plugins

## Directives

- **When a spec lists RFC summaries in its Required Reading section, you MUST read ALL of them before making any design recommendations or protocol claims.**
- **Code that implements external APIs or protocols MUST reference the upstream spec inline.**
- **If the implementation cannot deliver EXACTLY what the operator's config asks for, `ze config verify` / `ze config commit` MUST fail with a clear error. Silent approximation, truncation, or "best-effort" mapping are bugs.**
- **One learned layout SHOULD fit every protocol (the holo-routing lesson: a fixed per-protocol skeleton makes each protocol navigable once you know one).** The skeleton below uses the package-naming glossary (`ai/rules/go-standards.md` "Package-Naming Glossary") and maps every existing protocol to it.
- **The skeleton is ADVISORY for existing code: no moves, no renames, no build gate. New protocols SHOULD follow it; touched code MAY adopt it opportunistically.**
- Conformance to the RFC text itself is governed by `ai/rules/rfc-compliance.md`, which stays a separate always-on rule.

## RFC Summaries Before Design

### Mechanical Rule

1. Spec lists RFC summaries under Required Reading -> you MUST read every one that exists
2. If a summary is marked "MUST CREATE" -> you MUST create it BEFORE design work
3. Design recommendations MUST NOT be made before all summaries are read and annotated
4. You MUST NOT cite RFC section numbers, PDU formats, or protocol semantics from memory when a summary exists (or is expected to exist) in the repo

### Banned Reasoning

| Excuse | Reality |
|--------|---------|
| "I know this RFC from training" | Training conflates draft versions. Read the summary. |
| "The design doesn't depend on wire format details" | You don't know that until you've read it. |
| "I'll read it later when implementing" | Design decisions made without RFC constraints get reworked. |
| "The spec already describes the algorithm" | The spec may be wrong. The RFC summary is the authority. |

## Self-Documenting Code

### Rules

| Situation | Required |
|-----------|----------|
| Implementing an external API | Comment block with upstream repo URL, spec/endpoints file, consuming projects |
| Following an RFC | `// RFC NNNN Section X.Y -- see rfc/short/rfcNNNN.md` near the relevant code |
| Matching another project's format | URL to the format definition |
| Using a vendored library | Version and source URL in the import area or vendor manifest |

### Format

- **The reference block MUST sit at file top, after the `// Design:` and `// Related:` lines** (`## Examples` shows the shape).

### Not Required

Inline reference is OPTIONAL for:
- Internal APIs (ze-to-ze communication)
- Standard library usage
- Well-known protocols where the RFC number in a comment suffices

## Protocol Subpackage Skeleton (advisory)

### The skeleton

A single-package protocol (root package + `yang/`) needs none of this: LDP and RSVP-TE live comfortably at that size. Once a protocol grows subpackages, the skeleton applies.

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

- **BFD SHOULD be treated as the reference layout:** `packet` / `engine` / `session` / `transport` / `auth` / `cmd` / `api` / `yang`.
<!-- source: internal/component/bfd -- subpackage layout -->

### How existing protocols map

| Protocol | Maps cleanly | Exceptions |
|----------|--------------|------------|
| BFD | everything | none -- the reference |
| IS-IS | `packet`, `transport`, root engine, `adjacency` (RFC term), `types`, `yang`, `cli`; `circuit`/`lsdb`/`spf`/`redistribute` domain modules | none |
| OSPF | `packet`, `transport`, root engine, `neighbor` (RFC term), `types`, `yang`, `cli`; `iface`/`lsdb`/`spf`/`sr`/`redistribute` domain; `v3` version dir; `wire` = raw handoff type (glossary sense of `wire`) | none |
| IKE | `engine`, `transport`, `yang`, `cmd`; `crypto`/`eap`/`ipsec`/`dataplane` domain | `wire` is a full codec where the skeleton says `packet` (predates the glossary; kept) |
| BGP | `fsm` (RFC term), `types`, `yang`, `cli`; many platform modules | platform archetype, pre-SDK: `message`+`wireu` for the codec, `reactor`+`server` for the runtime -- documented as historical in the glossary; not a template for new work |
| LDP, RSVP-TE | single-package + `yang/` | below the subpackage threshold; skeleton N/A until they grow |

### The advisory report

`internal/le/protocolskeleton/protocolskeleton.go` lists each protocol's modules classified against the skeleton (canonical / RFC-named state / version dir / domain / legacy exception) and prints a one-line summary by default (`--verbose` for the full table). It ALWAYS exits 0 in report mode: it is an advisory lens, not a gate (an enforced skeleton today would need a large allowlist; see the tiers Path B lesson in `spec-tiers-0-umbrella`, closed). It runs as the last, non-enforcing line of `./le tier check`.
<!-- source: internal/le/protocolskeleton/protocolskeleton.go -- classifier and manifest -->

## Exact Or Reject

### In Practice

| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Qdisc backend cannot reproduce | Map to closest supported | `qdisc <type>: not supported by backend <name>` |
| Filter type backend cannot match | Skip that filter | `filter <type>: not supported by backend <name>` |
| Backend has fewer slots than classes configured | Discard extras | Error naming capacity + actual count |
| Backend maps N inputs to same output (name truncation) | Second overwrites first | `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at backend's wire format | Truncate/wrap | `<value> out of range <lo..hi>` |
| Rate/burst/DSCP outside representable range | Silently clamp | Reject with valid range in message |

### Checklist For Every Backend

- [ ] Every accepted config path MUST produce backend state matching EXACTLY. No approximation
- [ ] Every capacity/limit/bound MUST be checked in the verifier BEFORE Apply time
- [ ] Every narrowing numeric input MUST have an explicit range check naming the valid range
- [ ] Every name subject to truncation MUST reject when it would truncate (distinct inputs != same stored name)
- [ ] Not-yet-implemented feature MUST reject with a "deferred" message, not quiet ignore

### Banned Phrases In Code Comments

| Banned | Usually means |
|--------|---------------|
| "for now we just truncate" | Silent data loss; reject at verify |
| "close enough approximation" | Not the operator's config; reject |
| "MVP only handles the first N" | Classes beyond N silently missing; reject |
| "best-effort translation" | Pick one: exact, or reject |
| "future optimization can batch them" (when un-batched path is wrong) | Fix correctness first |

- **Caught yourself writing one? Stop. You MUST design it properly, or reject in the verifier and record in the source's `plan/deferrals/<source>.md` shard.**

### Mechanical Check

Before marking any backend/translator spec done, for every path that accepts config and writes state:

1. Lossy field -> MUST the pre-check reject in verifier?
2. Bounded output structure -> MUST the capacity check reject when exceeded?
3. Truncated name -> MUST the length check reject before truncation?
4. Numeric narrowing -> MUST there be an explicit range check with the valid range in the error?

- **One "no" = operator intent silently discarded. You MUST fix it before commit.**

## Related Rules

A protocol change MUST also comply with these rules:
- `ai/rules/rfc-compliance.md` -- conformance to the RFC text, and the ratchets that keep RFC testing valid. It stays separate and always-on.
- `completion.md` -- silent wiring gaps. Exact-or-reject is the backend-translation specialization: wired but lossy = not done.
- `completion.md` -- "best effort" = "probably fine".
- `planning.md` -- deferred features reject at commit AND are recorded with a destination spec.
- `ai/rules/go-standards.md` -- the package-naming glossary this skeleton speaks.
- `ai/rules/architecture.md` -- which TIER a protocol lives in (the skeleton is about layout WITHIN the protocol; it never moves packages between tiers).
- `ai/rules/plugins.md` -- registration and proximity rules.

## Rationale

Rationale: `ai/rationale/exact-or-reject.md`.

### Why RFC summaries beat memory

Training knowledge of RFCs is unreliable: drafts change between versions, details get conflated across similar RFCs, and wire format specifics (field offsets, PDU sizes, flag positions) are frequently wrong from memory. The RFC summaries in `rfc/short/` are the verified source of truth.

### Why the inline reference is mandatory

Claude has no long-term memory across sessions. When reading code, inline references to external specs, APIs, and upstream projects provide the context needed to understand constraints and ensure continued alignment. Without them, every session must rediscover what the code is implementing.

## Examples

```
// Implements the birdwatcher API consumed by Alice-LG.
// Reference: https://github.com/alice-lg/birdwatcher
// API spec: https://github.com/alice-lg/birdwatcher/blob/master/endpoints.go
```
