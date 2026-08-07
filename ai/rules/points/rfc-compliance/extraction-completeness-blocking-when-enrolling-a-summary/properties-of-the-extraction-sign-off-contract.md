---
kind: directive
level: MUST
stage:
---
- **Only dispositions are authored.** Sites, sections, quotes, the register and every published count are DERIVED from the source text at check time. A hand-typed "sites seen" is a claim, and claims are what this removes.
- **A generated skeleton can never pass.** The writer emits only UNCLASSIFIED dispositions and an unclassified site fails the check, so mass-generating artifacts makes the gate redder rather than greener.
- **The register is derived and a stronger claim is refused.** `rfc2119`, `prose`, or `manual-walk`. Measured over the 166 enrolled RFCs on 2026-07-29: 101 / 64 / 1. 23 have no capitalised MUST-level keyword SITE at all while declaring 172 gated MUSTs between them, so a keyword-only check would have been vacuously green for a large minority of the corpus.
- **The bound is over keyword-visible sites, not over obligations.** Recall can be near zero for an indicative-prose section (RFC 4271 §8.2.2: 35168 characters, one capitalised keyword). `unsourced-ids` records an obligation the extractor cannot see. This raises a floor from zero; it does not reach a ceiling.
- **A FIRST sign-off is reviewed, not ratcheted.** `check_extraction_ratchet` compares a stem against its own HEAD row, so a stem signing off for the first time has no baseline and could exclude every site. The published per-RFC exclusion ratio is the control; read it before you approve one.
