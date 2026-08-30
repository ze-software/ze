# Protocol Implementation

**When:** implementing or changing a protocol, an external API, a wire format, or a backend that applies operator config
**Severity:** blocking
**Related:** rfc-compliance, completion, planning, go-standards, architecture, plugins

## Directives

- **When a spec lists RFC summaries in its Required Reading section, you MUST read ALL of them before making any design recommendations or protocol claims.**
- **Code that implements external APIs or protocols MUST reference the upstream spec inline.**
- **If the implementation cannot deliver EXACTLY what the operator's config asks for, `ze config verify` / `ze config commit` MUST fail with a clear error. Silent approximation, truncation, or "best-effort" mapping are bugs.**
- **One learned layout SHOULD fit every protocol, so a reader who knows one protocol can navigate the next.** The skeleton speaks the package-naming glossary in `ai/rules/go-standards.md`, and `docs/architecture/protocol-skeleton.md` holds its modules and how each existing protocol maps to them.
- **The skeleton is ADVISORY for existing code: no moves, no renames, no build gate. New protocols SHOULD follow it; touched code MAY adopt it opportunistically.**
- Conformance to the RFC text itself is governed by `ai/rules/rfc-compliance.md`, which stays a separate always-on rule.

## RFC Summaries Before Design

1. Spec lists RFC summaries under Required Reading -> you MUST read every one that exists
2. If a summary is marked "MUST CREATE" -> you MUST create it BEFORE design work
3. Design recommendations MUST NOT be made before all summaries are read and annotated
4. You MUST NOT cite RFC section numbers, PDU formats, or protocol semantics from memory when a summary exists (or is expected to exist) in the repo

**This reasoning MUST NOT be acted on:**

| Excuse | Reality |
|--------|---------|
| "I know this RFC from training" | Training conflates draft versions. Read the summary |
| "The design doesn't depend on wire format details" | You do not know that until you have read it |
| "I'll read it later when implementing" | Design decisions made without RFC constraints get reworked |
| "The spec already describes the algorithm" | The spec can be wrong. The RFC summary is the authority |

## Self-Documenting Code

**Each situation below MUST carry the inline reference its row names:**

| Situation | Required |
|-----------|----------|
| Implementing an external API | A comment block with the upstream repo URL, the spec or endpoints file, and the consuming projects |
| Following an RFC | `// RFC NNNN Section X.Y -- see rfc/short/rfcNNNN.md` near the relevant code |
| Matching another project's format | The URL of the format definition |
| Using a vendored library | The version and source URL, in the import area or the vendor manifest |

**The reference block MUST sit at the top of the file, after the `// Design:` and `// Related:` lines:**

```
// Implements the birdwatcher API consumed by Alice-LG.
// Reference: https://github.com/alice-lg/birdwatcher
// API spec: https://github.com/alice-lg/birdwatcher/blob/master/endpoints.go
```

Inline reference is OPTIONAL for:
- Internal APIs (ze-to-ze communication)
- Standard library usage
- Well-known protocols where the RFC number in a comment suffices

## Protocol Subpackage Skeleton (advisory)

- **A new protocol SHOULD follow the subpackage skeleton, and BFD SHOULD be treated as the reference layout:** `packet` / `engine` / `session` / `transport` / `auth` / `cmd` / `api` / `yang`.
- **A protocol at root-package-plus-`yang` size needs none of it.** The skeleton applies once a protocol grows subpackages.
- **Existing code MAY adopt it opportunistically. No moves, no renames, and no gate.** `./le protocol-skeleton report` is a lens and always exits 0.
- The modules, the five classes, and how each existing protocol maps are in `docs/architecture/protocol-skeleton.md`.
<!-- source: internal/component/bfd -- subpackage layout -->

## Exact Or Reject

**Each situation below MUST be rejected at verify time, with the error its row names:**

| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Qdisc backend cannot reproduce | Map to closest supported | `qdisc <type>: not supported by backend <name>` |
| Filter type backend cannot match | Skip that filter | `filter <type>: not supported by backend <name>` |
| Backend has fewer slots than classes configured | Discard extras | Error naming capacity and actual count |
| Backend maps N inputs to the same output (name truncation) | Second overwrites first | `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at the backend's wire format | Truncate or wrap | `<value> out of range <lo..hi>` |
| Rate, burst, or DSCP outside the representable range | Silently clamp | Reject with the valid range in the message |

**Before a backend or translator spec is marked done, every path that accepts config and writes state MUST satisfy all of these:**
- Every accepted config path MUST produce backend state matching EXACTLY. No approximation
- Every capacity, limit, and bound MUST be checked in the verifier BEFORE Apply time
- Every narrowing numeric input MUST have an explicit range check naming the valid range
- Every name subject to truncation MUST reject when it would truncate, so distinct inputs never become the same stored name
- A not-yet-implemented feature MUST reject with a "deferred" message, never a quiet ignore

**These phrases MUST NOT appear in a code comment. Each one names a silent approximation:**

| Banned | Usually means |
|--------|---------------|
| "for now we just truncate" | Silent data loss; reject at verify |
| "close enough approximation" | Not the operator's config; reject |
| "MVP only handles the first N" | Classes beyond N silently missing; reject |
| "best-effort translation" | Pick one: exact, or reject |
| "future optimization can batch them" (when the un-batched path is wrong) | Fix correctness first |

- **Caught yourself writing one? Stop. You MUST design it properly, or reject in the verifier and record in the source's `plan/deferrals/<source>.md` shard.**

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
