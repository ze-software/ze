---
kind: table
level:
stage:
---
Verification follows the agreed task. These safeguards still apply.

| Unchanged | Why it is not instrument work |
|-----------|-------------------------------|
| Correctness and interoperability of implemented capabilities (`ai/rules/rfc-compliance.md`) | Existing behavior needs proof and exposed defects need fixes. An absent requirement of an implemented RFC stays a recorded gap until it is implemented, and it is owed work |
| The ban on deleting or weakening a test to clear a red (`ai/rules/testing.md`) | Weakening hides a product defect. Leaving the test red hides nothing, so it is the permitted move |
| The ban on calling half-written product code finished (`ai/rules/completion.md`) | A completion claim is about the product, and a red test never earns one either way |
| Reading the producer before claiming what code does (`ai/rules/evidence.md`) | Routing a red needs the same one read this rule already asks for |
