---
kind: directive
level: MUST
stage:
---
- **Test code is held to ONE standard: it MUST run, and it MUST be correct about the product.** The coverage targets above are for the code that ships. A test helper, a fixture builder, a `.ci` or `.et` script and the runners under `test/` need no coverage figure, no boundary sweep, and no test of their own. Spend that budget on the behavior under test, which is the only thing an operator ever meets.
- **A bug in test code that leads to NO TESTING is load-bearing, and it is fixed like product code.** A test the runner never selects, a skip that reports green, a harness that never reaches the code under test, a fixture that builds the wrong scenario, an assertion nothing evaluates: the suite claims coverage it does not have, and that claim is what the product is shipped on.
- **What else still applies is everything that decides what a test PROVES: it fails when the behavior breaks, it asserts the acceptance criterion rather than the mechanism, it never encodes a violation, and a gate still refuses what it exists to refuse.** Those are the sections around this one, and none of them is softened here. A defect in test-only code outside that set is a NOTE in review (`ai/rules/planning.md`, "Critical Review Is the Central Deliverable"), never a spec.
- **A tool that already carries tests keeps them.** Native Go tests beside packages under `internal/le/` exist because a gate that stops refusing is a product-visible failure. This point removes an obligation to ADD coverage over harness code; it removes no test that is there.
