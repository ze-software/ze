---
kind: directive
level: MUST
stage:
---
**A finished chunk MUST be committed when it finishes, not when the session does (owner directive, 2026-08-21).** Work that is done and green sits in one working tree, where the next `git clean`, checkout or crashed session destroys it, and where every later chunk has to be diffed around it. The question to answer after each piece of work is "does this stand on its own", never "am I finished for the day". A defect fix, a rule change, a gate repair and a spec's implementation are four commits, and the first three MUST NOT wait behind the fourth's review gate.
