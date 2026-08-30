---
kind: table
level:
stage:
---
This rule is about the INSTRUMENTS. Every rule below is about the SOFTWARE, and none of them moves.

| Unchanged | Why it is not instrument work |
|-----------|-------------------------------|
| Product correctness, RFC conformance, interop (`ai/rules/rfc-compliance.md`) | They are properties of the thing being shipped, and a peer that rejects ze has met a product defect |
| The ban on deleting or weakening a test to clear a red (`ai/rules/testing.md`) | Weakening hides a product defect. Leaving the test red hides nothing, so it is the permitted move |
| The ban on calling half-written product code finished (`ai/rules/completion.md`) | A completion claim is about the product, and a red test never earns one either way |
| A structural gate charged to your own commit: lint, generated artifacts, tier | It costs seconds rather than the gate's half hour, and it says the tree is BROKEN rather than merely unverified |
| Reading the producer before claiming what code does (`ai/rules/evidence.md`) | Routing a red needs the same one read this rule already asks for |
