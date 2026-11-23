# RFC Compliance Counting History

This file records every change to how Ze calculates its public RFC compliance
figures, with exact dates, measured before/after counts, and plain-English
explanations. When writing a weekly update (`/ze-weekly-update`), read this
file before interpreting a jump or shift in compliance numbers so the article
clearly separates rule changes from new protocol implementations or tests.

## 2026-11-23 — Superseded proofs, default-path rule, and non-software exclusions

**Owner rulings and code commits:**
- Policy & rules: `bd6ba7a195` (`ai/META-DECISIONS.md`, `ai/rules/rfc-compliance.md`, `ai/rules/rfc-conformance.md`)
- Counting implementation: `f249e6fb96` (`internal/le/rfc/`)
- Summary updates: `97c5303ffd` (14 `rfc/short/*.md` files, 49 rows) + `8d81ffc440` (`RFC4360-6-1`)

**What changed in the calculation:**
1. **Superseded requirements satisfied by a newer RFC now count as proven.**
   When an older RFC requirement is superseded (`{superseded: restated as <RID>}`)
   and its replacement requirement in the newer RFC is proven, or when the
   newer RFC dropped the requirement (`{superseded: dropped}`) and Ze implements
   that newer RFC (including newer protocol versions such as RFC 8210 RTR v1
   superseding RFC 6810 RTR v0), the superseded requirement counts as proven
   rather than an open gap. When the replacement requirement is excluded with
   checked evidence, the superseded requirement leaves Ze's owed count the same
   way. Requirements whose successor RFC Ze has not implemented remain gaps.
2. **Default-path behavior governs compliance, not custom operator config.**
   If Ze's default behavior satisfies an RFC requirement and is verified by
   tests, the fact that an operator can explicitly write custom policy or
   overrides that violate the RFC does not mark the requirement partial or a
   gap (for example, `RFC4360-6-1`).
3. **Non-software obligations are excluded as not applicable.**
   Mandatory rules that bind only human operators or network topology
   (`RFC9234-6-1`, `RFC8210-9-2`, `RFC7432-5-13`, `RFC9552-5.2-5`,
   `RFC9552-8.2.6-2`), or IANA registries, IETF processes, and future RFC
   authors (`RFC4360-7-1/7-2`, `RFC3623-A-1`, `RFC3630-6-1..4`, `RFC5838-5-3`,
   `RFC6396-5.1-1`, `RFC7012-2.1-1`, `RFC7684-6.5-1..2`, `RFC7770-2.3-1`,
   `2.7-1`, `2-1`, `5.2-2`, `5.3-1`), now move out of Ze's owed count as
   `{not-applicable}` rather than inflating open gaps.
4. **Section-heading role scoping is accepted.**
   Requirements under sections explicitly scoped to a role Ze does not fill
   (such as RFC 4035 §2 `Zone Signing`, RFC 4035 §3.1.5 `Responding to Queries
   for Type AXFR or IXFR`, and RFC 3954 §9 `The Collector Side`) can cite the
   section or ancestor heading in `{not-applicable: heading "..."; binds ...}`
   verified against `rfc/full/<stem>.txt` and `Meta Ze roles`.

**Measured effect across the 230 walked RFCs (2026-11-23):**

| Metric | Before (`f249e6fb96^`) | After (`97c5303ffd`) | Delta |
|---|---:|---:|---:|
| Total MUSTs | 5,188 | 5,188 | 0 |
| Not owed (checked exclusions) | 22 | 75 | +53 |
| Kept MUSTs (owed denominator) | 5,166 | 5,113 | -53 |
| Proven MUSTs (Ze + OS) | 2,777 (2,774 Ze + 3 OS) | 2,818 (2,815 Ze + 3 OS; 37 via superseded) | +41 |
| Open — proof pending | 416 | 409 | -7 |
| Open — OS testing gap | 232 | 232 | 0 |
| Open — exclusion unchecked | 28 | 28 | 0 |
| Open — not assessed | 174 | 174 | 0 |
| Open — not built | 1,539 | 1,452 | -87 |
| Built MUSTs denominator (`kept - not-built`) | 3,627 | 3,661 | +34 |
| **All MUSTs proven** (`proven / kept`) | **53.8%** | **55.1%** | **+1.3 pp** |
| **Built MUSTs proven** (`proven / built`) | **76.6%** | **77.0%** | **+0.4 pp** |
| Optional (SHOULD/MAY) proven | 138 / 2,359 | 163 / 2,359 | +25 |

*(Note: of the +41 proven MUSTs, 37 come directly from superseded-row
resolution, 1 from removing the invalid `{partial}` on `RFC4360-6-1` in
`8d81ffc440`, and 3 from concurrent BGP/RFC proof commits landed in the same
window. Of the +53 not-owed rows, 49 come from the 14 updated `rfc/short/*.md`
summaries and 4 from superseded rows whose replacement is already excluded.)*

**How to explain this in the next weekly update:**
State explicitly that the jump from `53.8%` to `55.1%` (all MUSTs) and `76.6%`
to `77.0%` (built MUSTs) includes a counting-rule refinement, not just newly
written protocol tests: superseded requirements satisfied by a newer RFC Ze
implements now count as proven (`+37` MUSTs), `RFC4360-6-1` is no longer
penalized for custom operator policy (`+1` proven), and 53 operator, topology,
IANA/RFC-author, and non-Ze-role rows moved out of the owed denominator.

## 2026-11-13 — Two-part compliance view (built vs. all MUSTs) and six-bucket open split

- Separated the open MUST count into six explicit buckets (`proof pending`,
  `OS testing gap`, `exclusion unchecked`, `not assessed`, `not built`, plus
  `not owed` excluded from the denominator).
- Published two percentages side by side: share of built MUSTs proven
  (`proven / (kept - not-built)`) and share of all owed MUSTs proven
  (`proven / kept`).

## Until 2026-10-31 — Whole-tree summary headline before strict discrimination proofs

- Earlier figures counted requirements linked to a passing test before Ze
  required recorded fault-detection (discrimination) proofs for every counted
  MUST, which made the headline percentage drop sharply once unverified tests
  moved into `proof pending`.
