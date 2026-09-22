---
kind: table
level:
stage:
---
Verification follows the agreed task. These safeguards still apply.

| Unchanged | Why it is not instrument work |
|-----------|-------------------------------|
| Correctness and interoperability of implemented capabilities (`ai/rules/rfc-compliance.md`) | Existing behavior needs proof and exposed defects need fixes. Absent RFC features remain explicit gaps pending separate implementation scope |
| The ban on deleting or weakening a test to clear a red (`ai/rules/testing.md`) | Weakening hides a product defect. Leaving the test red hides nothing, so it is the permitted move |
| The ban on calling half-written product code finished (`ai/rules/completion.md`) | A completion claim is about the product, and a red test never earns one either way |
| A structural gate charged to your own commit: lint, generated artifacts, tier | It costs seconds rather than the gate's half hour, and it says the tree is BROKEN rather than merely unverified |
| Reading the producer before claiming what code does (`ai/rules/evidence.md`) | Routing a red needs the same one read this rule already asks for |
