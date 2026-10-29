---
kind: directive
level: MUST
stage:
rationale: ai/rationale/git-safety.md
---
**A finished chunk MUST be committed when it finishes, not when the session does, and one commit carries one logical change.** The question after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and each piece of a spec's implementation are separate commits, and none of them MUST wait behind the spec's review gate: a piece that improves the software, compiles and works lands then, while a change too small to stand on its own rides with the piece it belongs to.
