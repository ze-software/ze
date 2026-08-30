---
kind: directive
level: MUST
stage:
---
**Every red gets ONE question before anything else: does this red say the PRODUCT is wrong?** Answer it by reading the failing assertion and the function that produces the value it names (`ai/rules/evidence.md`). The answer takes one read, and it decides everything that follows.
**A red that says the product is wrong IS the work. It MUST be fixed at the source, and never by weakening the assertion** (`ai/rules/completion.md` governs it from there).
**A red that says the SCAFFOLDING is wrong MUST be left red, named in one line, and stepped over.** Fixture drift, a stale golden file, a runner flag, a harness path, a gate's own bookkeeping: each costs product time and buys the product nothing.
**A scaffolding red is repaired only when it BLINDS you.** The test you cannot read the product's behavior through is worth the repair. The one that merely prints red is not.
