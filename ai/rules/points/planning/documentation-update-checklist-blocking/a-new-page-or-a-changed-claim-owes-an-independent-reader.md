---
kind: directive
level: MUST
stage:
rationale: plan/journal/reference-checked-claim-unchecked.md
---
- **A spec that CREATES a `docs/architecture/` page, or CHANGES a claim one of those pages makes about the code, MUST run `/ze-review-docs` over that page before it closes.** Record the pass in the spec's Pre-Commit Verification "Documentation Verified" table, which `/ze-close` already fills: name the page, the reviewing session, and the claims it checked.
- **A new page or a changed claim is the whole trigger. A typo, a link repair, a heading move, a rename and a formatting pass owe no reader**, because none of them states anything new about the code.
- **No gate discharges this.** A gate checks that a path resolves and that a named symbol is declared; a sentence that is WRONG about a symbol that exists passes every one of them, and the resolving anchor under it makes the sentence look checked. Only a reader can falsify prose.
