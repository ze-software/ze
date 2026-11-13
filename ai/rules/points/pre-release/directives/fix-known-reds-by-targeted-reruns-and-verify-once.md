---
kind: directive
level: MUST
stage:
rationale: ai/rationale/known-reds-first.md
---
**When a run leaves reds, you MUST write them into one list of known reds, and you MUST NOT start another full verify while a listed red is still open.** A full verify here is any run wider than the failure: lint over the tree, `./le test unit all`, a whole functional suite. No agent runs `./le verify` at all.
**Each fix MUST be proven by rerunning only the test that failed: the one Go test with `-run` over its package, or the one `.ci` selection, alone and under the load that broke it.** The package or the suite around it is the next step out, and only after the test itself passes. The recipes are `docs/contributing/running-commands.md` for a Go test and `docs/functional-tests.md`, "Run a single test", for a `.ci`. The main thread runs every run this rule names, and an agent lists it as an owed command.
**When every listed red passes its targeted rerun, the package tests and the functional tests the fixes reach MUST run once, directly, in the main checkout.** The reds they leave become the next list. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and a full verification is Thomas's release check.
