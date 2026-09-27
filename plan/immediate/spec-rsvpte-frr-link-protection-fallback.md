# Spec: rsvpte-frr-link-protection-fallback

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A transit LSR acting as a Point of Local Repair arms no protection at all when
an LSP asks for node protection and only a link bypass (merging at the next
hop) is configured. RFC 4090 Section 6 asks the PLR to fall back to link
protection when node protection is not feasible. Found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`, and read at the
producer in HEAD on 2026-09-27. An operator meets it as a protected LSP that
reports no local protection available and loses traffic on a link failure a
configured bypass could have repaired.

Related: `plan/immediate/spec-mpls-9-rsvp-te-one-to-one-backup.md` (one-to-one
backup, a different protection method; neither spec depends on the other).

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 4090 Section 6 (no row in `rfc/short/rfc4090.md`) | "If the "node protection desired" flag is set, the PLR SHOULD try to provide node protection; if this is not feasible, the PLR SHOULD then try to provide link protection." | `internal/plugins/rsvpte/frr.go::(*engine).selectBypass` | With `NodeProtection` set it looks only for a bypass merging at the NNHOP, skips every bypass without `NodeProtection`, and returns no match; it never retries at the NHOP. `TestNodeProtectionNeedsNodeBypass` (`internal/plugins/rsvpte/frr_test.go`) asserts `lsp.Bypass` is nil in exactly that case | no row, so no verdict |
| D2 | RFC 4090 Section 6 (RFC4090-6-6), consequence of fixing D1 | "The PLR SHOULD also set the "node protection" flag if the backup path protects against the failure of the immediate downstream node, and, if the path does not, the PLR SHOULD clear the "node protection" flag. This MUST be done if the "node protection desired" flag was set in the SESSION_ATTRIBUTE object." | `internal/plugins/rsvpte/frr.go::rroProtectionFlags` | Sets the RRO node protection bit from the REQUEST (`PSB.Protection.NodeProtection`), relying on selectBypass arming only node bypasses for node requests. Once D1 arms a link bypass for a node request, that bit would be set over a link-only backup unless it reads the armed bypass's capability | latent today, live once D1 lands |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | a node-protection request with only a link bypass at the NHOP arms that bypass (red against HEAD) |
| Both polarities | a node bypass at the NNHOP is still preferred when both exist; no bypass at either hop arms nothing |
| RRO flags | armed link fallback: protection-available set, node protection clear; armed node bypass: node protection set |
| Test correction | `TestNodeProtectionNeedsNodeBypass` asserts the refusal the RFC advises against; it is rewritten to the fallback, not deleted |
| Rows | a verbatim row for the Section 6 fallback sentence in `rfc/short/rfc4090.md`, tagged and discriminated with `./le rfc discriminate-record` |
| Interop | an RSVP-TE FRR scenario against FRR or another RSVP-TE speaker as head end, the PLR holding only a link bypass |

## Owner decisions

| Row | Question |
|-----|----------|
| D1 | The sentence is a SHOULD and `TestNodeProtectionNeedsNodeBypass` pins the opposite choice. Implement the link-protection fallback and rewrite that test, or keep the refusal as an owner-approved deviation recorded with its RFC section and reason (it is then not counted as conformant)? |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4090.md`, `rfc/full/rfc4090.txt` Section 4.4 (RRO flags) and Section 6

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/rsvpte/frr.go` - `selectBypass`, `rroProtectionFlags`, `bypassEstablished`
- [ ] `internal/plugins/rsvpte/frr_test.go` - `TestNodeProtectionNeedsNodeBypass`

**Behavior to change:** D1, then D2 with it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a Path message carrying a SESSION_ATTRIBUTE with "node protection desired" (or a FAST_REROUTE object) arrives at a transit LSR

### Transformation Path
1. the PSB records the protection request
2. `selectBypass` matches a configured bypass against the remaining ERO (NHOP, NNHOP)
3. the armed bypass sets the backup label; `rroProtectionFlags` writes the RRO flags in the Resv sent upstream

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| wire Path message to PSB | RSVP decode | No |
| PSB to bypass selection | engine call | No |
| bypass state to RRO in Resv | RRO encode | No |

### Integration Points
- the configured bypass list (`bypassConfig`, `NodeProtection` and `MergePoint`)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Path with node protection desired, link bypass only | → | `selectBypass` fallback, `rroProtectionFlags` | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | node protection desired, only a link bypass at the NHOP | the link bypass is armed; RRO protection-available set, node protection clear |
| AC-2 | node protection desired, node bypass at the NNHOP and link bypass at the NHOP | the node bypass is armed; RRO node protection set |
| AC-3 | node protection desired, no bypass at either hop | nothing armed; protection flags clear |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| fallback and preference | `internal/plugins/rsvpte/frr_test.go` | D1, AC-1 to AC-3 | [to fill in design] |
| RRO flag follows the armed bypass | `internal/plugins/rsvpte/frr_test.go` | D2 | [to fill in design] |

## Files to Modify

- `internal/plugins/rsvpte/frr.go`, `internal/plugins/rsvpte/frr_test.go`, `rfc/short/rfc4090.md`, the RSVP-TE FRR page under `docs/` that describes bypass selection

## Implementation Steps

1. Owner decision on D1; 2. failing tests; 3. fix `selectBypass` and `rroProtectionFlags`; 4. row, discrimination and verdict; 5. page edit.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
