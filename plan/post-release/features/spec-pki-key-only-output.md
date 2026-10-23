# Spec: pki-key-only-output

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Key-only PEM output is an absent command and can wait independently of guarding existing bundle export. |
| Depends | `plan/release/fixes/spec-pki-show-private-key.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Add the originally requested key-only PKCS#8 PEM output after the existing key-exposure guard is available. The new command must reuse that guard, reject unauthorized or unconfirmed callers without key material, and preserve the existing certificate and bundle forms.

## Source and Evidence

Split from `plan/release/fixes/spec-pki-show-private-key.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/release/fixes/spec-pki-show-private-key.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | authorised `private-key pem` on a cert with a key | returns only the PKCS#8 `PRIVATE KEY` PEM block, no certificate |
| AC-2 | cert has no private key | clear error, no key material |
| AC-3 | unauthorised / unconfirmed caller | request rejected; no key in response |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Existing bundle guard, original AC-4 and certificate-only AC-5; original AC-3 for the existing bundle | `plan/release/fixes/spec-pki-show-private-key.md` |
| New command, original AC-1/AC-2 and original AC-3 for the new form | `plan/post-release/features/spec-pki-key-only-output.md` |

Work assigned to `plan/release/fixes/spec-pki-show-private-key.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.
