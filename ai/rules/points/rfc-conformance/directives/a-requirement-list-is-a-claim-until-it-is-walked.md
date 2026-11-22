---
kind: directive
level: MUST
stage:
rationale: ai/rationale/rfc-compliance.md
---
**A conformance number MUST NOT be published for a document whose requirement list has not been walked against the RFC's own text, and any number that mixes walked and unwalked documents MUST say which part is bounded.** `rfc/short/<stem>.md` is authored. Every gate in `internal/le/rfc` compares that list to the tests, and none of them reads the RFC, so a green gate over an unwalked summary measures the list rather than the software. The extraction sign-off in `rfc/extraction/<stem>.json` is the only artifact that bounds it, and until one exists the document's figures are unverified.
**A requirement list a model produced MUST be treated as a claim, never as evidence.** Asked for the MUST-level obligations of a document that states none, a generator does not answer "none": it writes plausible ones.
**A published figure that later proves wrong MUST be corrected on the surface that carried it, and the correction MUST name what was measured.** Restating the new number alone repeats the original error; the correction says what the earlier figure counted.
**The two arithmetics of a walk MUST both be run, because they catch opposite failures.** Forward, every normative site the extractor can see is mapped to a requirement or excluded with a reason: that catches an obligation the summary missed. Reverse, every gated requirement the summary declares is the target of some site or is declared `unsourced-ids`: that catches an obligation the summary invented. A walk that runs only the forward half leaves the fabrications in place.
