---
kind: table
level: MUST
stage:
---
| Situation | What you MUST do |
|-----------|------------------|
| Full conformance and full proof of it are reachable | Implement it and prove it with a tagged test. Do not ask which subset Thomas wants |
| Anything short of full conformance or full proof of conformance looks like the answer | You are not authorized to pick it. STOP and ask Thomas -- see "Implement Full Compliance. Ask Thomas Only Before Doing LESS" below |
| You find code that does not do what the RFC requires, and the goal of the work in hand depends on that path | Fix it here. A known wire-visible violation the work depends on is a defect you are now the entry point for (`ai/rules/completion.md`) |
| You find code that does not do what the RFC requires, and the goal of the work in hand does not depend on it | Write the spec, close the work in hand, and put the violation to Thomas the moment it closes, quoting the requirement id and the RFC section text (`ai/rules/completion.md`). The ask is owed the same session; the fix waits for his answer. Leaving it as a comment, a report line, or an unhomed row is banned |
| A test pins the non-conformant behaviour | The TEST is wrong. A fixture, golden file, or assertion encoding a violation is not evidence the violation is intended -- it is the violation with a green bar on top. Fix the code, then correct the test and say so |
| A code comment calls the deviation deliberate | A comment is its author's belief, not a decision record (`ai/rules/evidence.md`). Check the RFC text, then `plan/learned/` for a real ruling. Absent one, the RFC wins |
| The RFC requirement is not in `rfc/short/<stem>.md` | An unextracted obligation is still an obligation. Add the checklist row (see Extraction Completeness) -- the gate's silence is not conformance |
| Conforming would change behaviour operators rely on | Say so plainly and ask which way to fix it. Never silently keep the violation, and never present "leave it non-conformant" as an option |
| An exemption genuinely applies (e.g. RFC 7947 route-server transparency) | Gate it on the exact condition the exempting RFC names. An exemption applied unconditionally is a violation for every case it was not written for |
