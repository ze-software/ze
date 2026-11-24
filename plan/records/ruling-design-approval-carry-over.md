# Ruling: An approval carries over to a change that only records Thomas's own rulings

- **Date:** 2026-11-24
- **Owner:** Thomas Mangin
- **Context:** A design approval binds the SHA-256 digest of a spec's `## Task`, `## Acceptance Criteria`, `## Design documents` and `## Design presentation`. Any change of those sections made the approval stale, and only `./le spec design approve`, run from his terminal, renewed it. `plan/release/features/spec-fleet-1-device-registry.md` was approved at digest `e1897d459b10` (the `Design-approved:` line of commit `80f69b9f18`). Afterwards two rows were added to `## Design decisions` sections of pages its files link, each a ruling of his own (`docs/architecture/pki/tls-listeners.md`, the looking glass on HTTP and HTTPS; `docs/architecture/api/architecture.md`, the `| implied` same-line ruling), so its `## Design documents` owed two new rows; and another session's tree-wide rename changed the decision text one existing row quotes (`ze:backend "nft"` became `ze:backend "kernel"`). The gate sent the design back to him for approval, against the rule in `ai/META-DECISIONS.md`, "How agents work".

## Decision

He was asked which way to resolve it:

- **(a)** "fix the tooling: let an approval carry over when the judge confirms the changes only record your own rulings or text renames"
- **(b)** "keep the gate and correct the rule's wording"

Thomas's answer:

> "a"

The rule the tooling implements, as `ai/META-DECISIONS.md` records it:

> A change to an approved design that only writes in a choice Thomas himself made (an answer he gave to a question the spec put to him) is not put back to him for approval. The independent design judge still judges the changed text, and the work continues once it approves; Thomas may review it later or not. A change that carries anything he did not decide goes back to him as before.

## What this requires

1. An approval of one design text covers a later text when an independent `ze-design-judge` agent recorded `approve` on the later text, and a mechanical comparison with the text he approved proves the change holds nothing he did not decide.
2. The comparison accepts two changes only, both in `## Design documents`: a row added for a row of a page's `## Design decisions` section, with the verdict `agrees` or `silent`; and a row whose quoted statement alone changed to the page's current words, when the page no longer holds the approved quote.
3. Everything else is refused by name and goes back to him: a change of the Task, the Acceptance Criteria or the Design presentation, a removed row, a changed verdict, an added or changed `departs` row, a quote the page does not hold.
4. Every gate that reads an approval (the claim, the edit that makes a spec ready, the commit trailer, the closure's departure check) honours the carried approval through the one lookup, and the `Design-approved:` line the commit writes records the chain: the digest he approved, the later digest, the judge's sentence and his original words.
5. The text he approved is recovered from the commit that carries his approval, or from a copy the approval keeps beside its record. No copy is kept by hand.
